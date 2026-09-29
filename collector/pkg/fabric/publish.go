package fabric

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/zyvorai/gryvia/collector/pkg/kube"
)

// PublishInterval is how often fabric status is written to the cluster.
const PublishInterval = 30 * time.Second

// MaxPublishedJobs bounds the jobs handled per cycle (and so the API calls).
const MaxPublishedJobs = 256

const fabricSignalGroupVersion = "gryvia.io/v1alpha1"

// KubeAPI is the part of *kube.Client the publisher and the quota sync use.
type KubeAPI interface {
	Get(ctx context.Context, path string) ([]byte, error)
	MergePatch(ctx context.Context, path string, body []byte) ([]byte, error)
}

// k8sName is a conservative DNS-1123 subdomain check; anything else is never
// put into a request path.
var k8sName = regexp.MustCompile(`^[a-z0-9]([-a-z0-9.]{0,251}[a-z0-9])?$`)

// Publisher copies the folded fabric status of attributed jobs into the status
// subresource of an EXISTING GryviaFabricSignal (spec.jobRef == job name, same
// namespace). It creates nothing, never touches spec, and every error is logged
// and dropped: a failing API server cannot affect the collector.
//
// With several collectors (one per node) hosting pods of one job, each writes
// its own node's view and the last writer wins; the status is advisory.
type Publisher struct {
	API    KubeAPI
	Folder *Folder
	Log    Logger
	// Inference adds the engine latency fields (-infer-metrics) to every patch. Off, the patch
	// body is exactly what it was before the scraper existed.
	Inference bool
}

// InferencePatch holds the engine fields of the status patch. Every field is sent every time
// (null when not measured) so a stale value is removed instead of left behind.
type InferencePatch struct {
	TTFTP99Ms       *float64 `json:"ttftP99ms"`
	ITLP99Ms        *float64 `json:"itlP99ms"`
	QueueTimeP99Ms  *float64 `json:"queueTimeP99ms"`
	E2EP99Ms        *float64 `json:"e2eP99ms"`
	QueueTimeMeanMs *float64 `json:"queueTimeMeanMs"`
	E2EMeanMs       *float64 `json:"e2eMeanMs"`
	RequestsWaiting *int64   `json:"requestsWaiting"`
	KVCacheUsage    *float64 `json:"kvCacheUsage"`
	Engine          *string  `json:"engine"`
}

// signalStatusPatch is the merge-patch body. Every field is sent every time (no
// omitempty) so a value that dropped to zero is overwritten instead of left
// stale; gdsHitRatio is null (removes the field) when it is not measured.
type signalStatusPatch struct {
	Status struct {
		StragglerRank    int32    `json:"stragglerRank"`
		NCCLP99Ms        float64  `json:"ncclP99ms"`
		RDMARetryRate    float64  `json:"rdmaRetryRate"`
		GDSHitRatio      *float64 `json:"gdsHitRatio"`
		OverlapIdleRatio float64  `json:"overlapIdleRatio"`
		CNPRate          float64  `json:"cnpRate"`
		InferWaitP99Ms   float64  `json:"inferWaitP99ms"`
		PFCRate          float64  `json:"pfcRate"`
		ExfilEvents      int64    `json:"exfilEvents"`
		ScoreDelta       float64  `json:"scoreDelta"`
		UpdatedAt        string   `json:"updatedAt"`
		*InferencePatch
	} `json:"status"`
}

func finite(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return v
}

// StatusPatchBody renders the merge-patch for one job's status (without the engine fields).
func StatusPatchBody(st Status) ([]byte, error) { return statusPatchBody(st, false) }

func finitePtr(v *float64) *float64 {
	if v == nil || math.IsNaN(*v) || math.IsInf(*v, 0) {
		return nil
	}
	c := *v
	return &c
}

func inferencePatch(in *Inference) *InferencePatch {
	p := &InferencePatch{}
	if in == nil {
		return p // every field null: nothing measured
	}
	p.TTFTP99Ms, p.ITLP99Ms = finitePtr(in.TTFTP99MS), finitePtr(in.ITLP99MS)
	p.QueueTimeP99Ms, p.E2EP99Ms = finitePtr(in.QueueP99MS), finitePtr(in.E2EP99MS)
	p.QueueTimeMeanMs, p.E2EMeanMs = finitePtr(in.QueueMeanMS), finitePtr(in.E2EMeanMS)
	if w := finitePtr(in.RequestsWaiting); w != nil {
		n := int64(math.Round(math.Max(0, math.Min(*w, 1e12))))
		p.RequestsWaiting = &n
	}
	if kv := finitePtr(in.KVCacheUsage); kv != nil {
		c := math.Max(0, math.Min(1, *kv))
		p.KVCacheUsage = &c
	}
	if in.Engine != "" {
		e := in.Engine
		p.Engine = &e
	}
	return p
}

func statusPatchBody(st Status, withInference bool) ([]byte, error) {
	var p signalStatusPatch
	if withInference {
		p.Status.InferencePatch = inferencePatch(st.Inference)
	}
	s := &p.Status
	if st.StragglerRank > math.MaxInt32 {
		s.StragglerRank = math.MaxInt32
	} else {
		s.StragglerRank = int32(st.StragglerRank)
	}
	s.NCCLP99Ms = finite(st.NCCLP99MS)
	s.RDMARetryRate = finite(st.RDMARetryRate)
	if st.GDSMeasured {
		v := finite(st.GDSHitRatio)
		s.GDSHitRatio = &v
	}
	s.OverlapIdleRatio = finite(st.OverlapIdleRatio)
	s.CNPRate = finite(st.CNPRate)
	s.InferWaitP99Ms = finite(st.InferWaitP99MS)
	s.PFCRate = finite(st.PFCRate)
	if st.ExfilEvents > math.MaxInt64 {
		s.ExfilEvents = math.MaxInt64
	} else {
		s.ExfilEvents = int64(st.ExfilEvents)
	}
	s.ScoreDelta = math.Max(0, math.Min(1, finite(st.ScoreDelta)))
	s.UpdatedAt = st.UpdatedAt.UTC().Format(time.RFC3339)
	return json.Marshal(p)
}

// Publishable reports whether a folder key names a real job: never the
// "_unattributed" bucket, never node-level "_node/*" keys.
func Publishable(k JobKey) bool {
	if k.Job == "" || strings.HasPrefix(k.Namespace, "_") || strings.HasPrefix(k.Job, "_") {
		return false
	}
	return k8sName.MatchString(k.Namespace) && k8sName.MatchString(k.Job)
}

type signalList struct {
	Items []struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
		Spec struct {
			JobRef string `json:"jobRef"`
		} `json:"spec"`
	} `json:"items"`
}

// PublishOnce runs one cycle and returns the number of objects patched.
func (p *Publisher) PublishOnce(ctx context.Context) int {
	if p == nil || p.API == nil || p.Folder == nil {
		return 0
	}
	log := p.Log
	if log == nil {
		log = nopLogger{}
	}
	snap := p.Folder.Snapshot()
	keys := make([]JobKey, 0, len(snap))
	for k := range snap {
		if Publishable(k) {
			keys = append(keys, k)
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Namespace != keys[j].Namespace {
			return keys[i].Namespace < keys[j].Namespace
		}
		return keys[i].Job < keys[j].Job
	})
	if len(keys) > MaxPublishedJobs {
		keys = keys[:MaxPublishedJobs]
	}

	base := "/apis/" + fabricSignalGroupVersion + "/namespaces/"
	byNS := map[string]map[string][]string{} // namespace -> jobRef -> object names
	listed := map[string]bool{}
	patched := 0
	for _, k := range keys {
		if ctx.Err() != nil {
			return patched
		}
		if !listed[k.Namespace] {
			listed[k.Namespace] = true
			raw, err := p.API.Get(ctx, base+url.PathEscape(k.Namespace)+"/gryviafabricsignals")
			if err != nil {
				log.Warnw("fabric status: list GryviaFabricSignal failed", "namespace", k.Namespace, "error", err)
				continue
			}
			var l signalList
			if err := json.Unmarshal(raw, &l); err != nil {
				log.Warnw("fabric status: bad GryviaFabricSignal list", "namespace", k.Namespace, "error", err)
				continue
			}
			m := map[string][]string{}
			for _, it := range l.Items {
				if it.Spec.JobRef != "" && k8sName.MatchString(it.Metadata.Name) {
					m[it.Spec.JobRef] = append(m[it.Spec.JobRef], it.Metadata.Name)
				}
			}
			byNS[k.Namespace] = m
		}
		names := byNS[k.Namespace][k.Job]
		if len(names) == 0 {
			continue // nothing to update: never create
		}
		body, err := statusPatchBody(snap[k], p.Inference)
		if err != nil {
			log.Warnw("fabric status: encode failed", "namespace", k.Namespace, "job", k.Job, "error", err)
			continue
		}
		for _, name := range names {
			path := base + url.PathEscape(k.Namespace) + "/gryviafabricsignals/" + url.PathEscape(name) + "/status"
			if _, err := p.API.MergePatch(ctx, path, body); err != nil {
				var se *kube.StatusError
				if errors.As(err, &se) && se.Code == http.StatusNotFound {
					continue // deleted between list and patch
				}
				log.Warnw("fabric status: patch failed", "namespace", k.Namespace, "name", name, "error", err)
				continue
			}
			patched++
		}
	}
	return patched
}

// Run publishes every PublishInterval until ctx is done.
func (p *Publisher) Run(ctx context.Context) {
	t := time.NewTicker(PublishInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			p.PublishOnce(ctx)
		}
	}
}
