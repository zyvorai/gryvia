// Package decoder provides security event decoding from eBPF ring buffers.
package decoder

import (
	"bytes"
	"encoding/binary"
	"errors"

	"github.com/cilium/ebpf/ringbuf"
)

// Security event type constants.
const (
	SecEvtContainerEscape   uint8 = 1
	SecEvtPrivilegeEscalation uint8 = 2
	SecEvtCryptoMining       uint8 = 3
	SecEvtSensitiveMount     uint8 = 4
	SecEvtSuspiciousExec     uint8 = 5
	SecEvtNetworkViolation   uint8 = 6
	SecEvtFileAccess         uint8 = 7
	SecEvtSyscallAnomaly     uint8 = 8
)

// Security severity constants.
const (
	SeverityCritical uint8 = 1
	SeverityHigh     uint8 = 2
	SeverityMedium   uint8 = 3
	SeverityLow      uint8 = 4
	SeverityInfo     uint8 = 5
)

// SecurityEvent matches the security_event struct from eBPF.
type SecurityEvent struct {
	Timestamp uint64
	PID       uint32
	UID       uint32
	GID       uint32
	EventType uint8
	Severity  uint8
	SyscallNr uint16
	CgroupID  uint64
	Comm      [16]byte
	Path      [256]byte
	SrcIP     uint32
	DstIP     uint32
	DstPort   uint16
	Pad       uint16
	Bytes     uint64
	OldUID    uint32
	NewUID    uint32
}

// CommString returns the process name as a Go string.
func (e *SecurityEvent) CommString() string {
	return string(bytes.TrimRight(e.Comm[:], "\x00"))
}

// PathString returns the file path as a Go string.
func (e *SecurityEvent) PathString() string {
	return string(bytes.TrimRight(e.Path[:], "\x00"))
}

// SecurityEventTypeName returns a human-readable event type name.
func SecurityEventTypeName(t uint8) string {
	switch t {
	case SecEvtContainerEscape:
		return "container_escape"
	case SecEvtPrivilegeEscalation:
		return "privilege_escalation"
	case SecEvtCryptoMining:
		return "crypto_mining"
	case SecEvtSensitiveMount:
		return "sensitive_mount"
	case SecEvtSuspiciousExec:
		return "suspicious_exec"
	case SecEvtNetworkViolation:
		return "network_violation"
	case SecEvtFileAccess:
		return "file_access"
	case SecEvtSyscallAnomaly:
		return "syscall_anomaly"
	default:
		return "unknown"
	}
}

// SeverityName returns a human-readable severity name.
func SeverityName(s uint8) string {
	switch s {
	case SeverityCritical:
		return "critical"
	case SeverityHigh:
		return "high"
	case SeverityMedium:
		return "medium"
	case SeverityLow:
		return "low"
	case SeverityInfo:
		return "info"
	default:
		return "unknown"
	}
}

// SecurityDecoder decodes security events from eBPF ring buffers.
type SecurityDecoder struct {
	events chan SecurityEvent
}

// NewSecurityDecoder creates a SecurityDecoder with the given channel buffer size.
func NewSecurityDecoder(bufSize int) *SecurityDecoder {
	if bufSize <= 0 {
		bufSize = 4096
	}
	return &SecurityDecoder{
		events: make(chan SecurityEvent, bufSize),
	}
}

// DecodeRingBuf reads security events from a ring buffer reader and publishes
// them on the events channel. It blocks until the reader is closed.
func (d *SecurityDecoder) DecodeRingBuf(reader *ringbuf.Reader) {
	for {
		record, err := reader.Read()
		if err != nil {
			if errors.Is(err, ringbuf.ErrClosed) {
				return
			}
			continue
		}

		var ev SecurityEvent
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

// Events returns the read-only channel on which decoded security events are published.
func (d *SecurityDecoder) Events() <-chan SecurityEvent {
	return d.events
}

// Close closes the events channel.
func (d *SecurityDecoder) Close() {
	close(d.events)
}
