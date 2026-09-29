// Package flight keeps a bounded, node-local training event timeline.
package flight

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
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
	// Observations are deliberately not interpreted as GPU utilization or wire bytes.
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
	if ok {
		copyEvents = g.tail(limit)
	}
	r.mu.RUnlock()
	if !ok {
		return Report{}, false
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

// Window returns the retained events of the job at or after since, oldest first.
// truncated is true when the bounded ring is full and its oldest retained event is
// still inside the window: older events in the window were evicted, so counts
// derived from the result are lower bounds. ok is false when the node has no
// attributed events for the job.
func (r *Recorder) Window(ns, job string, since time.Time) (events []Event, truncated, ok bool) {
	r.mu.RLock()
	g, found := r.jobs[key(ns, job)]
	var all []Event
	if found {
		all = g.tail(maxEvents)
		truncated = len(g.buf) >= maxEvents
	}
	r.mu.RUnlock()
	if !found {
		return nil, false, false
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].Time.Before(all[j].Time) })
	i := sort.Search(len(all), func(i int) bool { return !all[i].Time.Before(since) })
	if truncated && i > 0 {
		truncated = false // the oldest retained event predates the window
	}
	return all[i:], truncated, true
}

// Jobs lists the (namespace, job) pairs with retained events.
func (r *Recorder) Jobs() [][2]string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([][2]string, 0, len(r.jobs))
	for k := range r.jobs {
		if i := strings.IndexByte(k, '/'); i > 0 {
			out = append(out, [2]string{k[:i], k[i+1:]})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i][0]+"/"+out[i][1] < out[j][0]+"/"+out[j][1] })
	return out
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
