// Package flight keeps a bounded, node-local training event timeline.
package flight

import (
	"encoding/json"
	"net/http"
	"sort"
	"sync"
	"time"
)

const (
	maxJobs   = 128
	maxEvents = 2048
)

type Identity struct {
	Namespace string `json:"namespace"`
	Job       string `json:"job"`
	Pod       string `json:"pod"`
	Node      string `json:"node"`
	Rank      string `json:"rank,omitempty"`
}

type Event struct {
	Time        time.Time `json:"time"`
	Identity    Identity  `json:"identity"`
	Source      string    `json:"source"`
	Kind        string    `json:"kind"`
	Operation   string    `json:"operation,omitempty"`
	Bytes       uint64    `json:"bytes,omitempty"`
	Retransmits uint32    `json:"retransmits,omitempty"`
	DurationNs  uint64    `json:"duration_ns,omitempty"`
	// TraceID is the W3C trace id of the request this event belongs to, attached at report
	// time from the node-local trace index (see Correlator). TraceMatch says how sure the
	// match is: "tuple" (same TCP 4-tuple) or "port_time" (same pod and local port within a
	// couple of seconds; used for events that carry no 4-tuple).
	TraceID    string `json:"traceId,omitempty"`
	TraceMatch string `json:"traceMatch,omitempty"`
	// Tuple and LocalPort are inputs to trace correlation only; they are never serialised (the
	// timeline does not expose remote addresses).
	Tuple     *Tuple `json:"-"`
	LocalPort uint16 `json:"-"`
	// Observations are deliberately not interpreted as GPU utilization or wire bytes.
}

// Tuple is an IPv4 TCP 4-tuple as seen from one endpoint; direction is not significant to
// correlation.
type Tuple struct {
	SrcIP   [4]byte
	SrcPort uint16
	DstIP   [4]byte
	DstPort uint16
}

// Correlator maps events to W3C trace ids (implemented by trace.Index).
type Correlator interface {
	// ByTuple returns the trace observed on the same 4-tuple (either direction) closest to at.
	ByTuple(t Tuple, at time.Time) (traceID string, ok bool)
	// ByPodPort returns the trace id of the only request seen for pod (namespace/name) on local
	// port near at; ok is false when there is none or when several distinct traces compete.
	ByPodPort(namespace, pod string, port uint16, at time.Time) (traceID string, ok bool)
}

type Finding struct {
	Code     string `json:"code"`
	Evidence string `json:"evidence"`
}
type Report struct {
	Namespace string         `json:"namespace"`
	Job       string         `json:"job"`
	Node      string         `json:"node"`
	Scope     string         `json:"scope"`
	Events    []Event        `json:"events"`
	Findings  []Finding      `json:"findings"`
	Counts    map[string]int `json:"counts"`
}

// ring is a fixed-capacity circular timeline; appends never shift memory.
type ring struct {
	buf   []Event
	start int // index of the oldest event once full
}

func (g *ring) add(e Event) {
	if len(g.buf) < maxEvents {
		g.buf = append(g.buf, e)
		return
	}
	g.buf[g.start] = e
	g.start = (g.start + 1) % maxEvents
}

func (g *ring) last() time.Time {
	if len(g.buf) < maxEvents {
		return g.buf[len(g.buf)-1].Time
	}
	return g.buf[(g.start+maxEvents-1)%maxEvents].Time
}

// tail returns up to n newest events, oldest first.
func (g *ring) tail(n int) []Event {
	total := len(g.buf)
	if n > total {
		n = total
	}
	out := make([]Event, 0, n)
	for i := total - n; i < total; i++ {
		out = append(out, g.buf[(g.start+i)%total])
	}
	return out
}

type Recorder struct {
	mu   sync.RWMutex
	jobs map[string]*ring
	node string
	corr Correlator
}

// SetCorrelator enables trace id annotation of reports. nil (the default) disables it.
func (r *Recorder) SetCorrelator(c Correlator) {
	r.mu.Lock()
	r.corr = c
	r.mu.Unlock()
}

// annotate sets TraceID/TraceMatch on e when the correlator knows the request.
func annotate(c Correlator, e *Event) {
	if c == nil {
		return
	}
	if e.Tuple != nil {
		if id, ok := c.ByTuple(*e.Tuple, e.Time); ok {
			e.TraceID, e.TraceMatch = id, "tuple"
			return
		}
	}
	if e.LocalPort != 0 && e.Identity.Pod != "" {
		if id, ok := c.ByPodPort(e.Identity.Namespace, e.Identity.Pod, e.LocalPort, e.Time); ok {
			e.TraceID, e.TraceMatch = id, "port_time"
		}
	}
}

// TraceEvents returns the events of the given jobs (namespace/job) that correlate with traceID,
// oldest first, at most limit. It needs a Correlator; without one it returns nil.
func (r *Recorder) TraceEvents(traceID string, jobs []Identity, limit int) []Event {
	r.mu.RLock()
	c := r.corr
	if c == nil {
		r.mu.RUnlock()
		return nil
	}
	var all []Event
	seen := map[string]bool{}
	for _, j := range jobs {
		k := key(j.Namespace, j.Job)
		if seen[k] {
			continue
		}
		seen[k] = true
		if g, ok := r.jobs[k]; ok {
			all = append(all, g.tail(len(g.buf))...)
		}
	}
	r.mu.RUnlock()
	var out []Event
	for _, e := range all {
		annotate(c, &e)
		if e.TraceID == traceID {
			out = append(out, e)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Time.Before(out[j].Time) })
	if limit > 0 && len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out
}

func New(node string) *Recorder { return &Recorder{jobs: make(map[string]*ring), node: node} }
func key(ns, job string) string { return ns + "/" + job }

// Record ignores unattributed events. Eviction bounds memory independent of job count.
func (r *Recorder) Record(e Event) {
	if e.Identity.Job == "" || e.Identity.Namespace == "" {
		return
	}
	if e.Time.IsZero() {
		e.Time = time.Now().UTC()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	k := key(e.Identity.Namespace, e.Identity.Job)
	if _, exists := r.jobs[k]; !exists && len(r.jobs) >= maxJobs {
		oldest := ""
		var when time.Time
		for name, g := range r.jobs {
			if oldest == "" || g.last().Before(when) {
				oldest = name
				when = g.last()
			}
		}
		delete(r.jobs, oldest)
	}
	g, ok := r.jobs[k]
	if !ok {
		g = &ring{}
		r.jobs[k] = g
	}
	g.add(e)
}

func (r *Recorder) Report(ns, job string, limit int) (Report, bool) {
	if limit <= 0 || limit > 200 {
		limit = 200
	}
	r.mu.RLock()
	g, ok := r.jobs[key(ns, job)]
	var copyEvents []Event
	corr := r.corr
	if ok {
		copyEvents = g.tail(limit)
	}
	r.mu.RUnlock()
	if !ok {
		return Report{}, false
	}
	for i := range copyEvents {
		annotate(corr, &copyEvents[i])
	}
	sort.SliceStable(copyEvents, func(i, j int) bool { return copyEvents[i].Time.Before(copyEvents[j].Time) })
	rep := Report{Namespace: ns, Job: job, Node: r.node, Scope: "node-local; observed events only", Events: copyEvents, Counts: map[string]int{}, Findings: []Finding{}}
	for _, e := range copyEvents {
		if e.Kind == "tcp_retransmit" {
			rep.Counts[e.Kind] += int(e.Retransmits)
		} else {
			rep.Counts[e.Kind]++
		}
	}
	if n := rep.Counts["tcp_retransmit"]; n > 0 {
		rep.Findings = append(rep.Findings, Finding{"tcp_retransmits_observed", "Observed " + itoa(n) + " TCP retransmissions on closed connections; investigate path and NIC counters."})
	}
	if n := rep.Counts["pipeline_stall"]; n > 0 {
		rep.Findings = append(rep.Findings, Finding{"pipeline_stalls_observed", "Observed " + itoa(n) + " gaps between CUDA API calls; verify GPU activity with DCGM or CUPTI."})
	}
	return rep, true
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func (r *Recorder) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	q := req.URL.Query()
	ns, job := q.Get("namespace"), q.Get("job")
	if ns == "" || job == "" {
		http.Error(w, "namespace and job required", http.StatusBadRequest)
		return
	}
	rep, ok := r.Report(ns, job, 200)
	if !ok {
		http.Error(w, "no attributed events on this node", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(rep)
}
