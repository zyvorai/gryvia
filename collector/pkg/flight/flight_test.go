package flight

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

func TestRecordCollectiveKeepsIdentityAndDoesNotEvictEvents(t *testing.T) {
	r := New("node-a")
	id := Identity{Namespace: "ml", Job: "train", Pod: "train-0", Node: "node-a"}
	r.Record(Event{Identity: id, Kind: "tcp_retransmit", Retransmits: 2})
	rank := uint32(3)
	for i := 1; i <= 3*maxEvents; i++ { // far more than the event ring holds
		r.RecordCollective(Event{Identity: id, Operation: "allreduce", Bytes: 4096, DurationNs: 10,
			CommOrdinal: 1, CommSeq: uint64(i), CommRank: &rank, CommWorld: 8})
	}
	// ignored: no identity, no ordinal, no seq
	r.RecordCollective(Event{Identity: Identity{}, CommOrdinal: 1, CommSeq: 1})
	r.RecordCollective(Event{Identity: id, CommOrdinal: 0, CommSeq: 1})
	r.RecordCollective(Event{Identity: id, CommOrdinal: 1, CommSeq: 0})
	rep, ok := r.Report("ml", "train", 500)
	if !ok || len(rep.Events) != 1 || rep.Events[0].Kind != "tcp_retransmit" {
		t.Fatalf("collective flood evicted or polluted events: %+v", rep.Events)
	}
	if len(rep.Collectives) != 200 || rep.Collectives[0].Kind != "nccl_collective_seq" {
		t.Fatalf("collectives = %d", len(rep.Collectives))
	}
	last := rep.Collectives[len(rep.Collectives)-1]
	if last.CommSeq != uint64(3*maxEvents) || last.CommRank == nil || *last.CommRank != 3 || last.CommWorld != 8 {
		t.Errorf("newest collective = %+v", last)
	}
	b, _ := json.Marshal(last)
	if !strings.Contains(string(b), `"comm_ordinal":1`) || !strings.Contains(string(b), `"comm_seq"`) {
		t.Errorf("json %s", b)
	}
	// a job with only collectives still has a report
	r2 := New("n")
	r2.RecordCollective(Event{Identity: id, CommOrdinal: 1, CommSeq: 1})
	if rep, ok := r2.Report("ml", "train", 10); !ok || len(rep.Events) != 0 || len(rep.Collectives) != 1 {
		t.Errorf("collectives-only report: ok=%v %+v", ok, rep)
	}
}

func TestJobOfPod(t *testing.T) {
	r := NewResolver("node-a", t.TempDir())
	raw := `{"items":[{"metadata":{"uid":"11111111-2222-3333-4444-555555555555","name":"train-0","namespace":"ml","labels":{"gryvia.io/job":"train"}},"spec":{"nodeName":"node-a"}},
	{"metadata":{"uid":"aaaaaaaa-2222-3333-4444-555555555555","name":"other","namespace":"ml"},"spec":{"nodeName":"node-a"}}]}`
	if err := r.SetPods([]byte(raw)); err != nil {
		t.Fatal(err)
	}
	if j, ok := r.JobOfPod("ml", "train-0"); !ok || j != "train" {
		t.Errorf("got %q %v", j, ok)
	}
	if _, ok := r.JobOfPod("ml", "other"); ok {
		t.Error("pod without a job label must not resolve")
	}
	if _, ok := r.JobOfPod("kube-system", "train-0"); ok {
		t.Error("namespace must match")
	}
}
