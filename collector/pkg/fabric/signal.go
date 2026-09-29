// Package fabric decodes the fabric_signal ring buffers emitted by
// ebpf/straggler.c, rdma_health.c, gds_trace.c, overlap.c and infer_latency.c
// (roce_cnp.c only keeps counters, see AddCNP), folds them per
// job, and derives a [0,1] score penalty for the topology scorer.
//
// The signals are a side channel next to gpu_event (which is frozen at 72
// bytes); the struct layout mirrors ebpf/headers/fabric_signal.h.
package fabric

import (
	"encoding/binary"
	"errors"

	"github.com/cilium/ebpf/ringbuf"
)

// Signal types, matching enum fabric_signal_type in fabric_signal.h.
const (
	SigStraggler uint8 = 1 // rank skew on a collective
	SigRDMARetry uint8 = 2 // QP retry / RNR above threshold
	SigGDS       uint8 = 3 // GPU-direct vs bounce-buffer read
	SigOverlap   uint8 = 4 // cudaDeviceSynchronize nested in an in-flight ncclAllReduce
	SigInferWait uint8 = 5 // inference socket accept -> first recv
	// SigCNP is synthesised by the collector from the roce_cnp cnp_count map
	// (Bytes = CNP packets since the previous poll); it never appears on a ring.
	SigCNP uint8 = 6
)

// Slots of roce_cnp.c's per-CPU cnp_count array (CNP_SLOT_* in fabric_signal.h).
const (
	CNPSlotCNP  uint32 = 0 // RoCEv2 congestion notification packets
	CNPSlotRoCE uint32 = 1 // all RoCEv2 packets
)

// SignalSize is sizeof(struct fabric_signal).
const SignalSize = 80

// Byte offsets of struct fabric_signal fields (little endian). The unit tests
// check them against the _Static_asserts in fabric_signal.h.
const (
	offTimestamp   = 0
	offPID         = 8
	offCgroupLo    = 12
	offType        = 16
	offNCCLOp      = 17
	offRank        = 20
	offWorldSize   = 24
	offPeerRank    = 28
	offLatency     = 32
	offPeerLatency = 40
	offBytes       = 48
	offRetry       = 56
	offRNR         = 60
	offComm        = 64
	commLen        = 16
)

// Signal is the userspace view of struct fabric_signal. Field meaning depends
// on Type (see the comment in fabric_signal.h):
//
//	SigStraggler: Rank/PeerRank, LatencyNS/PeerLatencyNS, Bytes, NCCLOp.
//	SigRDMARetry: Rank = QP number, WorldSize = post calls, Retries/RNR =
//	              error completions since the previous signal.
//	SigGDS:       LatencyNS, Bytes, Retries = 1 when nvidia-fs confirmed direct.
//	SigOverlap:   LatencyNS = sync duration, Retries = nested ncclAllReduce depth.
//	SigInferWait: LatencyNS = accept -> first recv, Rank = local TCP port.
//	SigCNP:       Bytes = CNP packets (userspace only).
type Signal struct {
	TimestampNS   uint64
	PID           uint32
	CgroupLo      uint32
	Type          uint8
	NCCLOp        uint8
	Rank          uint32
	WorldSize     uint32
	PeerRank      uint32
	LatencyNS     uint64
	PeerLatencyNS uint64
	Bytes         uint64
	Retries       uint32
	RNR           uint32
	Comm          string
}

// Decode parses one ring buffer record. It returns false for a short record.
func Decode(b []byte) (Signal, bool) {
	if len(b) < SignalSize {
		return Signal{}, false
	}
	le := binary.LittleEndian
	return Signal{
		TimestampNS:   le.Uint64(b[offTimestamp:]),
		PID:           le.Uint32(b[offPID:]),
		CgroupLo:      le.Uint32(b[offCgroupLo:]),
		Type:          b[offType],
		NCCLOp:        b[offNCCLOp],
		Rank:          le.Uint32(b[offRank:]),
		WorldSize:     le.Uint32(b[offWorldSize:]),
		PeerRank:      le.Uint32(b[offPeerRank:]),
		LatencyNS:     le.Uint64(b[offLatency:]),
		PeerLatencyNS: le.Uint64(b[offPeerLatency:]),
		Bytes:         le.Uint64(b[offBytes:]),
		Retries:       le.Uint32(b[offRetry:]),
		RNR:           le.Uint32(b[offRNR:]),
		Comm:          cstr(b[offComm : offComm+commLen]),
	}, true
}

func cstr(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}

// Decoder decodes fabric signals from eBPF ring buffers.
type Decoder struct {
	events chan Signal
}

// NewDecoder creates a Decoder with the given channel buffer size.
func NewDecoder(bufSize int) *Decoder {
	if bufSize <= 0 {
		bufSize = 4096
	}
	return &Decoder{events: make(chan Signal, bufSize)}
}

// DecodeRingBuf reads signals from a ring buffer reader and publishes them on
// the events channel. It blocks until the reader is closed.
func (d *Decoder) DecodeRingBuf(reader *ringbuf.Reader) {
	for {
		record, err := reader.Read()
		if err != nil {
			if errors.Is(err, ringbuf.ErrClosed) {
				return
			}
			continue
		}
		s, ok := Decode(record.RawSample)
		if !ok {
			continue
		}
		select {
		case d.events <- s:
		default:
			// Drop when the consumer is behind rather than block the reader.
		}
	}
}

// Events returns the channel on which decoded signals are published.
func (d *Decoder) Events() <-chan Signal {
	return d.events
}

// Close closes the events channel.
func (d *Decoder) Close() {
	close(d.events)
}
