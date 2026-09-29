package flight

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestResolverMatchesPodUIDAndNeverGuesses(t *testing.T) {
	d := t.TempDir()
	r := NewResolver("gpu-1", d)
	uid := "12345678-1234-1234-1234-123456789abc"
	data := `{"items":[{"metadata":{"uid":"` + uid + `","name":"train-0","namespace":"ml","labels":{"gryvia.io/job":"train"}},"spec":{"nodeName":"gpu-1"}},{"metadata":{"uid":"87654321-1234-1234-1234-123456789abc","name":"other","namespace":"ml","labels":{"gryvia.io/job":"other"}},"spec":{"nodeName":"gpu-2"}}]}`
	if err := r.SetPods([]byte(data)); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(d, "42"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, "42", "cgroup"), []byte("0::/kubepods.slice/pod"+strings.ReplaceAll(uid, "-", "_")+"/container\n"), 0600); err != nil {
		t.Fatal(err)
	}
	id, ok := r.Resolve(42)
	if !ok || id.Job != "train" || id.Namespace != "ml" || id.Pod != "train-0" {
		t.Fatalf("unexpected identity: %+v %v", id, ok)
	}
	if _, ok := r.Resolve(43); ok {
		t.Fatal("guessed missing process")
	}
	if err := r.SetPods([]byte(`{"items":[]}`)); err != nil {
		t.Fatal(err)
	}
	if _, ok := r.Resolve(42); ok {
		t.Fatal("stale pod survived refresh")
	}
}

func TestRecorderBoundedAndHTTP(t *testing.T) {
	r := New("gpu-1")
	id := Identity{Namespace: "ml", Job: "train", Node: "gpu-1"}
	r.Record(Event{Kind: "unattributed"})
	for i := 0; i < maxEvents+10; i++ {
		r.Record(Event{Identity: id, Kind: "nccl_collective"})
	}
	r.Record(Event{Identity: id, Kind: "pipeline_stall"})
	r.Record(Event{Identity: id, Kind: "tcp_retransmit", Retransmits: 3})
	if got := len(r.jobs["ml/train"].buf); got != maxEvents {
		t.Fatalf("retained %d events", got)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/flight/diagnose?namespace=ml&job=train", nil))
	if w.Code != http.StatusOK {
		t.Fatal(w.Code, w.Body.String())
	}
	var rep Report
	if err := json.Unmarshal(w.Body.Bytes(), &rep); err != nil {
		t.Fatal(err)
	}
	if len(rep.Events) != 200 || rep.Counts["pipeline_stall"] != 1 || rep.Counts["tcp_retransmit"] != 3 || len(rep.Findings) != 2 {
		t.Fatalf("bad report: %+v", rep)
	}
	missing := httptest.NewRecorder()
	r.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/api/v1/flight/diagnose?namespace=ml&job=absent", nil))
	if missing.Code != http.StatusNotFound {
		t.Fatal(missing.Code)
	}
}

func TestRingKeepsNewestInOrder(t *testing.T) {
	r := New("n")
	id := Identity{Namespace: "ml", Job: "train"}
	for i := 0; i < maxEvents+5; i++ {
		r.Record(Event{Identity: id, Kind: "flow", Bytes: uint64(i)})
	}
	rep, _ := r.Report("ml", "train", 3)
	if len(rep.Events) != 3 || rep.Events[2].Bytes != uint64(maxEvents+4) || rep.Events[0].Bytes != uint64(maxEvents+2) {
		t.Fatalf("wrong tail: %+v", rep.Events)
	}
}

func TestPodUIDRegexpExact(t *testing.T) {
	m := podUID.FindSubmatch([]byte("0::/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-pod12345678_1234_1234_1234_123456789abc.slice/cri-containerd-abc.scope"))
	if m == nil || normalizeUID(string(m[1])) != "12345678-1234-1234-1234-123456789abc" {
		t.Fatalf("no exact match: %v", m)
	}
}

func TestWindowAndJobs(t *testing.T) {
	r := New("n1")
	id := Identity{Namespace: "ml", Job: "train", Pod: "p", Node: "n1"}
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 10; i++ {
		r.Record(Event{Time: base.Add(time.Duration(i) * time.Minute), Identity: id, Kind: "k"})
	}
	ev, trunc, ok := r.Window("ml", "train", base.Add(5*time.Minute))
	if !ok || len(ev) != 5 || trunc {
		t.Fatalf("%d %v %v", len(ev), trunc, ok)
	}
	if _, _, ok := r.Window("ml", "none", base); ok {
		t.Fatal("unknown job")
	}
	if j := r.Jobs(); len(j) != 1 || j[0] != [2]string{"ml", "train"} {
		t.Fatalf("%v", j)
	}
	// A full ring whose oldest event is inside the window reports truncation.
	for i := 0; i < maxEvents+5; i++ {
		r.Record(Event{Time: base.Add(time.Hour + time.Duration(i)*time.Second), Identity: id, Kind: "k"})
	}
	if _, trunc, _ := r.Window("ml", "train", base.Add(time.Hour)); !trunc {
		t.Fatal("full ring inside the window must be truncated")
	}
}

func TestResolverPods(t *testing.T) {
	r := NewResolver("n1", t.TempDir())
	raw := `{"items":[
	{"metadata":{"uid":"AAAAAAAA-1111-2222-3333-444444444444","name":"a","namespace":"ml","labels":{"gryvia.io/job":"j"}},"spec":{"nodeName":"n1"},"status":{"phase":"Running","qosClass":"Guaranteed"}},
	{"metadata":{"uid":"bbbbbbbb-1111-2222-3333-444444444444","name":"b","namespace":"ml","labels":{"gryvia.io/job":"j"}},"spec":{"nodeName":"n1"},"status":{"phase":"Succeeded","qosClass":"Guaranteed"}},
	{"metadata":{"uid":"cccccccc-1111-2222-3333-444444444444","name":"c","namespace":"ml","labels":{"gryvia.io/job":"j"}},"spec":{"nodeName":"other"},"status":{"phase":"Running"}}]}`
	if r.Pods("ml", "j") != nil {
		t.Fatal("no snapshot yet: unknown")
	}
	if err := r.SetPods([]byte(raw)); err != nil {
		t.Fatal(err)
	}
	p := r.Pods("ml", "j")
	if len(p) != 1 || p[0].Pod != "a" || p[0].UID != "aaaaaaaa-1111-2222-3333-444444444444" || p[0].QOS != "Guaranteed" {
		t.Fatalf("%+v", p)
	}
	if j := r.Jobs(); len(j) != 1 {
		t.Fatalf("%v", j)
	}
}

func TestResolveIPFailsClosed(t *testing.T) {
	r := NewResolver("gpu-1", t.TempDir())
	pod := func(name, ip, job string, hostNet bool, node string) string {
		lab := ""
		if job != "" {
			lab = `"gryvia.io/job":"` + job + `"`
		}
		hn := "false"
		if hostNet {
			hn = "true"
		}
		return `{"metadata":{"uid":"` + name + `","name":"` + name + `","namespace":"ml","labels":{` + lab + `}},"spec":{"nodeName":"` + node + `","hostNetwork":` + hn + `},"status":{"podIP":"` + ip + `"}}`
	}
	items := []string{
		pod("serve-0", "10.1.2.3", "serve", false, "gpu-1"),
		pod("nolabel", "10.1.2.4", "", false, "gpu-1"),
		pod("hostnet", "192.168.0.1", "serve", true, "gpu-1"),
		pod("elsewhere", "10.1.2.5", "serve", false, "gpu-2"),
		pod("noip", "", "serve", false, "gpu-1"),
	}
	if err := r.SetPods([]byte(`{"items":[` + strings.Join(items, ",") + `]}`)); err != nil {
		t.Fatal(err)
	}
	if id, ok := r.ResolveIP("10.1.2.3"); !ok || id.Pod != "serve-0" || id.Job != "serve" || id.Namespace != "ml" {
		t.Errorf("job pod: %+v %v", id, ok)
	}
	for _, ip := range []string{"10.1.2.4", "192.168.0.1", "10.1.2.5", "10.9.9.9", ""} {
		if _, ok := r.ResolveIP(ip); ok {
			t.Errorf("%q must not resolve", ip)
		}
	}
	r.mu.Lock()
	r.updated = r.updated.Add(-2 * time.Minute)
	r.mu.Unlock()
	if _, ok := r.ResolveIP("10.1.2.3"); ok {
		t.Error("a stale pod list must not resolve")
	}
}
