// SPDX-License-Identifier: Apache-2.0
//
// Additive signal types for nccl_transport.c, p2p_fallback.c, capture_gate.c and
// the XDP slot map of xdp_mux.c. Keep the numbers aligned with
// ebpf/headers/fabric_signal.h and ebpf/xdp_mux.c (signal_ext_test.go checks it).
// struct fabric_signal stays 80 bytes; do not add fields.
//
// Not yet decoded or loaded: signal.go's Decode does not interpret these types
// and nothing in the operator attaches the new programs or writes their maps.

package fabric

const (
	// SigNCCLTransport is emitted by nccl_transport.c when ncclCommInitRank
	// (or ncclGetUniqueId) returns 0. RetryCount is the transport hint.
	SigNCCLTransport uint8 = 11
	// SigP2PFallback is emitted by p2p_fallback.c. Bytes is the copy size,
	// RetryCount the peer-enable failures, PeerLatencyNS the age of the last one.
	SigP2PFallback uint8 = 12
	// SigCaptureArmed is synthesised by the collector from capture_lease.
	// The kernel program does not write it to a ring buffer.
	SigCaptureArmed uint8 = 13
)

// Transport hints written into nccl_transport's transport_hint map
// (NCCL_XPORT_* in nccl_transport.c).
const (
	XportUnknown = 0
	XportP2P     = 1
	XportSHM     = 2
	XportNet     = 3
	XportNetIB   = 4
	XportNetRoCE = 5
)

// XDP mux slots. Must match ebpf/xdp_mux.c. The mux runs only the first
// populated slot (a successful tail call does not return).
const (
	XDPSlotRoceCNP      uint32 = 0
	XDPSlotPFCPause     uint32 = 1
	XDPSlotDNS          uint32 = 2
	XDPSlotPacketFilter uint32 = 3
)
