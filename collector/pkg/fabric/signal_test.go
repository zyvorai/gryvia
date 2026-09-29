package fabric

import (
	"encoding/binary"
	"os"
	"regexp"
	"strconv"
	"testing"
)

func encode(s Signal) []byte {
	b := make([]byte, SignalSize)
	le := binary.LittleEndian
	le.PutUint64(b[offTimestamp:], s.TimestampNS)
	le.PutUint32(b[offPID:], s.PID)
	le.PutUint32(b[offCgroupLo:], s.CgroupLo)
	b[offType] = s.Type
	b[offNCCLOp] = s.NCCLOp
	le.PutUint32(b[offRank:], s.Rank)
	le.PutUint32(b[offWorldSize:], s.WorldSize)
	le.PutUint32(b[offPeerRank:], s.PeerRank)
	le.PutUint64(b[offLatency:], s.LatencyNS)
	le.PutUint64(b[offPeerLatency:], s.PeerLatencyNS)
	le.PutUint64(b[offBytes:], s.Bytes)
	le.PutUint32(b[offRetry:], s.Retries)
	le.PutUint32(b[offRNR:], s.RNR)
	copy(b[offComm:], s.Comm)
	return b
}

func TestDecodeRoundTrip(t *testing.T) {
	want := Signal{
		TimestampNS: 1, PID: 2, CgroupLo: 3, Type: SigStraggler, NCCLOp: 4,
		Rank: 5, WorldSize: 6, PeerRank: 7, LatencyNS: 8, PeerLatencyNS: 9,
		Bytes: 10, Retries: 11, RNR: 12, Comm: "python3",
	}
	got, ok := Decode(encode(want))
	if !ok || got != want {
		t.Fatalf("got %+v ok=%v, want %+v", got, ok, want)
	}
}

func TestDecodeShortAndFullComm(t *testing.T) {
	if _, ok := Decode(make([]byte, SignalSize-1)); ok {
		t.Error("short record must be rejected")
	}
	b := make([]byte, SignalSize)
	for i := offComm; i < SignalSize; i++ {
		b[i] = 'x' // no NUL terminator
	}
	s, ok := Decode(b)
	if !ok || len(s.Comm) != commLen {
		t.Errorf("unterminated comm: ok=%v len=%d", ok, len(s.Comm))
	}
}

// TestLayoutMatchesC checks the Go offsets against the _Static_asserts that
// pin struct fabric_signal in ebpf/headers/fabric_signal.h, so a change on
// either side fails one of the two builds.
func TestLayoutMatchesC(t *testing.T) {
	src, err := os.ReadFile("../../../ebpf/headers/fabric_signal.h")
	if err != nil {
		t.Skipf("C header not available: %v", err)
	}
	if m := regexp.MustCompile(`sizeof\(struct fabric_signal\) == (\d+)`).FindSubmatch(src); m == nil {
		t.Fatal("no sizeof assert for fabric_signal in header")
	} else if n, _ := strconv.Atoi(string(m[1])); n != SignalSize {
		t.Errorf("C sizeof=%d, Go SignalSize=%d", n, SignalSize)
	}
	want := map[string]int{
		"pid": offPID, "cgroup_id_lo": offCgroupLo, "signal_type": offType,
		"nccl_op": offNCCLOp, "rank": offRank, "world_size": offWorldSize,
		"peer_rank": offPeerRank, "latency_ns": offLatency,
		"peer_latency_ns": offPeerLatency, "bytes": offBytes,
		"retry_count": offRetry, "rnr_count": offRNR, "comm": offComm,
	}
	re := regexp.MustCompile(`offsetof\(struct fabric_signal, (\w+)\) == (\d+)`)
	found := map[string]bool{}
	for _, m := range re.FindAllSubmatch(src, -1) {
		field, off := string(m[1]), func() int { n, _ := strconv.Atoi(string(m[2])); return n }()
		w, ok := want[field]
		if !ok {
			t.Errorf("C asserts unknown field %s", field)
			continue
		}
		found[field] = true
		if w != off {
			t.Errorf("%s: C offset %d, Go offset %d", field, off, w)
		}
	}
	for f := range want {
		if !found[f] {
			t.Errorf("no _Static_assert for fabric_signal.%s in header", f)
		}
	}
	// The signal types must match the enum too.
	for name, v := range map[string]uint8{
		"FABRIC_SIG_STRAGGLER": SigStraggler, "FABRIC_SIG_RDMA_RETRY": SigRDMARetry,
		"FABRIC_SIG_GDS": SigGDS, "FABRIC_SIG_OVERLAP": SigOverlap,
	} {
		m := regexp.MustCompile(name + `\s*=\s*(\d+)`).FindSubmatch(src)
		if m == nil {
			t.Errorf("%s missing from header", name)
			continue
		}
		if n, _ := strconv.Atoi(string(m[1])); n != int(v) {
			t.Errorf("%s: C %d, Go %d", name, n, v)
		}
	}
}
