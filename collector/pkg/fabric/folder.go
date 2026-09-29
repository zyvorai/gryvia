package fabric

import (
	"encoding/json"
	"math"
	"net/http"
	"sort"
	"sync"
	"time"
)

const (
	// DefaultWindow is how long a signal contributes to a job's status.
	DefaultWindow = 5 * time.Minute
	// maxSamples bounds the signals kept per job.
	maxSamples = 4096
)

// Score-penalty thresholds and weights (see ScoreDelta).
const (
	p99SlowMS       = 50.0
	retryRateHigh   = 0.02
	gdsHitLow       = 0.5
	stragglerHitsHi = 10
	overlapIdleHigh = 0.3 // fraction of the window spent in sync inside a collective
	cnpRateHigh     = 100 // RoCEv2 CNPs per second, averaged over the window

	penaltyP99       = 0.25
	penaltyRDMA      = 0.35
	penaltyGDS       = 0.15
	penaltyStraggler = 0.25
	penaltyOverlap   = 0.15
	penaltyCNP       = 0.15
)

// JobKey identifies the job a signal belongs to. It is filled from Bind (a
// pid -> pod -> job resolver); signals from unbound pids are grouped under
// the "_unattributed" namespace by process name.
type JobKey struct {
	Namespace string `json:"namespace"`
	Job       string `json:"job"`
}

// Status is the folded view of one job's recent signals; it carries the
// fields of GryviaFabricSignal.status.
type Status struct {
	StragglerRank uint32  `json:"stragglerRank"`
	StragglerPID  uint32  `json:"stragglerPid"`
	StragglerHits uint64  `json:"stragglerHits"`
	NCCLP99MS     float64 `json:"ncclP99ms"`
	RDMARetryRate float64 `json:"rdmaRetryRate"`
	GDSHitRatio   float64 `json:"gdsHitRatio"`
	GDSMeasured   bool    `json:"gdsMeasured"`
	// OverlapIdleRatio is the time spent in cudaDeviceSynchronize nested inside
	// ncclAllReduce (GPU idle while communication is in flight), as a fraction
	// of the window, summed over threads and capped at 1.
	OverlapIdleRatio float64 `json:"overlapIdleRatio"`
	// CNPRate is RoCEv2 congestion notification packets per second averaged
	// over the window (node-level, reported under job _node/cnp).
	CNPRate float64 `json:"cnpRate"`
	// InferWaitP99MS is the p99 accept -> first recv wait on inference ports.
	// Informational: it does not feed ScoreDelta.
	InferWaitP99MS float64   `json:"inferWaitP99ms"`
	ScoreDelta     float64   `json:"scoreDelta"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type sample struct {
	at  time.Time
	sig Signal
}

// Folder aggregates signals per job over a sliding window.
type Folder struct {
	window time.Duration
	now    func() time.Time

	mu        sync.Mutex
	jobs      map[JobKey][]sample
	pidToJob  map[uint32]JobKey
	gdsDirect bool
}

// NewFolder creates a Folder. A non-positive window means DefaultWindow.
func NewFolder(window time.Duration) *Folder {
	if window <= 0 {
		window = DefaultWindow
	}
	return &Folder{
		window:   window,
		now:      time.Now,
		jobs:     map[JobKey][]sample{},
		pidToJob: map[uint32]JobKey{},
	}
}

// SetGDSDirectVisible tells the folder whether the nvidia-fs kernel hook is
// attached. Without it every cuFile call is "bounce or unknown", so the direct
// ratio is meaningless and is neither reported nor penalised.
func (f *Folder) SetGDSDirectVisible(v bool) {
	f.mu.Lock()
	f.gdsDirect = v
	f.mu.Unlock()
}

// Bind attributes a pid to a job (called by the cgroup -> pod resolver).
func (f *Folder) Bind(pid uint32, ns, job string) {
	f.mu.Lock()
	f.pidToJob[pid] = JobKey{Namespace: ns, Job: job}
	f.mu.Unlock()
}

// Unbind forgets a pid.
func (f *Folder) Unbind(pid uint32) {
	f.mu.Lock()
	delete(f.pidToJob, pid)
	f.mu.Unlock()
}

// CNPJob is the node-level key under which CNP counts are folded: congestion
// notifications belong to the NIC, not to a pod.
var CNPJob = JobKey{Namespace: "_node", Job: "cnp"}

// AddCNP records n RoCEv2 congestion notification packets counted since the
// previous call (the collector polls roce_cnp's cnp_count map). Zero is ignored.
func (f *Folder) AddCNP(n uint64) {
	if n == 0 {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.addLocked(CNPJob, Signal{Type: SigCNP, Bytes: n, Comm: "cnp"})
}

// Add records one signal.
func (f *Folder) Add(s Signal) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key, ok := f.pidToJob[s.PID]
	if !ok {
		key = JobKey{Namespace: "_unattributed", Job: s.Comm}
	}
	f.addLocked(key, s)
}

func (f *Folder) addLocked(key JobKey, s Signal) {
	list := append(f.jobs[key], sample{at: f.now(), sig: s})
	if len(list) > maxSamples {
		list = append(list[:0], list[len(list)-maxSamples:]...)
	}
	f.jobs[key] = list
}

// Snapshot returns the current status of every job that has signals inside
// the window. Expired signals (and jobs left with none) are dropped.
func (f *Folder) Snapshot() map[JobKey]Status {
	f.mu.Lock()
	defer f.mu.Unlock()
	now := f.now()
	cutoff := now.Add(-f.window)
	out := make(map[JobKey]Status, len(f.jobs))
	for k, list := range f.jobs {
		i := sort.Search(len(list), func(i int) bool { return !list[i].at.Before(cutoff) })
		list = list[i:]
		if len(list) == 0 {
			delete(f.jobs, k)
			continue
		}
		f.jobs[k] = list
		out[k] = fold(list, f.gdsDirect, f.window)
	}
	return out
}

func fold(list []sample, gdsDirect bool, window time.Duration) Status {
	var st Status
	var lat, inferLat []float64
	var errs, posted, direct, total, overlapNS, cnps uint64
	for _, sm := range list {
		s := sm.sig
		if sm.at.After(st.UpdatedAt) {
			st.UpdatedAt = sm.at
		}
		switch s.Type {
		case SigStraggler:
			st.StragglerHits++
			st.StragglerRank = s.Rank
			st.StragglerPID = s.PID
			lat = append(lat, float64(s.LatencyNS)/1e6)
		case SigRDMARetry:
			errs += uint64(s.Retries) + uint64(s.RNR)
			posted += uint64(s.WorldSize)
		case SigGDS:
			total += s.Bytes
			if s.Retries == 1 {
				direct += s.Bytes
			}
		case SigOverlap:
			overlapNS += s.LatencyNS
		case SigInferWait:
			inferLat = append(inferLat, float64(s.LatencyNS)/1e6)
		case SigCNP:
			cnps += s.Bytes
		}
	}
	st.NCCLP99MS = percentile(lat, 0.99)
	if errs > 0 {
		if posted == 0 {
			st.RDMARetryRate = 1
		} else {
			st.RDMARetryRate = math.Min(1, float64(errs)/float64(posted))
		}
	}
	if gdsDirect && total > 0 {
		st.GDSMeasured = true
		st.GDSHitRatio = float64(direct) / float64(total)
	}
	st.InferWaitP99MS = percentile(inferLat, 0.99)
	if secs := window.Seconds(); secs > 0 {
		st.OverlapIdleRatio = math.Min(1, float64(overlapNS)/1e9/secs)
		st.CNPRate = float64(cnps) / secs
	}
	st.ScoreDelta = ScoreDelta(st)
	return st
}

// percentile returns the nearest-rank percentile of v (0 for empty input).
func percentile(v []float64, p float64) float64 {
	if len(v) == 0 {
		return 0
	}
	sort.Float64s(v)
	idx := int(math.Ceil(p*float64(len(v)))) - 1
	if idx < 0 {
		idx = 0
	}
	return v[idx]
}

// ScoreDelta is a penalty in [0,1] the topology scorer can subtract.
// 0 = healthy fabric, 1 = do not place another gang here.
func ScoreDelta(st Status) float64 {
	d := 0.0
	if st.NCCLP99MS > p99SlowMS {
		d += penaltyP99
	}
	if st.RDMARetryRate > retryRateHigh {
		d += penaltyRDMA
	}
	if st.GDSMeasured && st.GDSHitRatio < gdsHitLow {
		d += penaltyGDS
	}
	if st.StragglerHits > stragglerHitsHi {
		d += penaltyStraggler
	}
	if st.OverlapIdleRatio > overlapIdleHigh {
		d += penaltyOverlap
	}
	if st.CNPRate > cnpRateHigh {
		d += penaltyCNP
	}
	return math.Max(0, math.Min(d, 1))
}

// JobStatus is one entry of the HTTP response.
type JobStatus struct {
	JobKey
	Status
}

// List returns the snapshot as a stable-ordered slice.
func (f *Folder) List() []JobStatus {
	snap := f.Snapshot()
	out := make([]JobStatus, 0, len(snap))
	for k, st := range snap {
		out = append(out, JobStatus{JobKey: k, Status: st})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Namespace != out[j].Namespace {
			return out[i].Namespace < out[j].Namespace
		}
		return out[i].Job < out[j].Job
	})
	return out
}

// ServeHTTP serves GET /api/v1/fabric: per-job fabric status and score.
func (f *Folder) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(f.List()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
