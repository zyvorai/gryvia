// Package decoder provides GPU event decoding from eBPF ring buffers.
package decoder

import (
	"bytes"
	"encoding/binary"
	"errors"

	"github.com/cilium/ebpf/ringbuf"
)

// GPUEventType constants matching the kernel-side gpu_event.event_type enum.
const (
	GPUEvtNCCLOp      uint8 = 1
	GPUEvtMemTransfer uint8 = 2
	GPUEvtRDMASend    uint8 = 3
	GPUEvtRDMARecv    uint8 = 4
	GPUEvtCUDALaunch  uint8 = 5
	GPUEvtCUDASync    uint8 = 6
)

// MemDirection constants for GPU memory transfers.
const (
	MemDirH2D  uint8 = 0 // Host to Device
	MemDirD2H  uint8 = 1 // Device to Host
	MemDirD2D  uint8 = 2 // Device to Device
	MemDirPeer uint8 = 3 // Peer (GPU-to-GPU)
)

// NCCLOpType constants for NCCL collective operations.
const (
	NCCLOpAllReduce  uint8 = 0
	NCCLOpBroadcast  uint8 = 1
	NCCLOpReduce     uint8 = 2
	NCCLOpAllGather  uint8 = 3
	NCCLOpReduceScatter uint8 = 4
	NCCLOpSend       uint8 = 5
	NCCLOpRecv       uint8 = 6
)

// GPUEvent matches the gpu_event struct from ebpf/headers/gpu_common.h.
type GPUEvent struct {
	Timestamp    uint64
	PID          uint32
	GPUID        uint32
	EventType    uint8
	Direction    uint8
	NCCLOp       uint8
	Pad          uint8
	Bytes        uint64
	LatencyNs    uint64
	SrcRank      uint32
	DstRank      uint32
	CollectiveID uint32
	WorldSize    uint32
	Comm         [16]byte
}

// CommString returns the process name as a Go string.
func (e *GPUEvent) CommString() string {
	return string(bytes.TrimRight(e.Comm[:], "\x00"))
}

// NCCLOpName returns a human-readable name for the NCCL operation type.
func NCCLOpName(op uint8) string {
	switch op {
	case NCCLOpAllReduce:
		return "AllReduce"
	case NCCLOpBroadcast:
		return "Broadcast"
	case NCCLOpReduce:
		return "Reduce"
	case NCCLOpAllGather:
		return "AllGather"
	case NCCLOpReduceScatter:
		return "ReduceScatter"
	case NCCLOpSend:
		return "Send"
	case NCCLOpRecv:
		return "Recv"
	default:
		return "Unknown"
	}
}

// MemDirectionName returns a human-readable name for a memory direction.
func MemDirectionName(dir uint8) string {
	switch dir {
	case MemDirH2D:
		return "H2D"
	case MemDirD2H:
		return "D2H"
	case MemDirD2D:
		return "D2D"
	case MemDirPeer:
		return "Peer"
	default:
		return "Unknown"
	}
}

// GPUDecoder decodes GPU events from eBPF ring buffers.
type GPUDecoder struct {
	events chan GPUEvent
}

// NewGPUDecoder creates a GPUDecoder with the given channel buffer size.
func NewGPUDecoder(bufSize int) *GPUDecoder {
	if bufSize <= 0 {
		bufSize = 4096
	}
	return &GPUDecoder{
		events: make(chan GPUEvent, bufSize),
	}
}

// DecodeRingBuf reads GPU events from a ring buffer reader and publishes
// them on the events channel. It blocks until the reader is closed or
// returns an error.
func (d *GPUDecoder) DecodeRingBuf(reader *ringbuf.Reader) {
	for {
		record, err := reader.Read()
		if err != nil {
			if errors.Is(err, ringbuf.ErrClosed) {
				return
			}
			continue
		}

		var ev GPUEvent
		if err := binary.Read(bytes.NewReader(record.RawSample), binary.LittleEndian, &ev); err != nil {
			continue
		}

		select {
		case d.events <- ev:
		default:
			// Drop event if channel is full to avoid blocking the reader.
		}
	}
}

// Events returns the read-only channel on which decoded GPU events are published.
func (d *GPUDecoder) Events() <-chan GPUEvent {
	return d.events
}

// Close closes the events channel.
func (d *GPUDecoder) Close() {
	close(d.events)
}
