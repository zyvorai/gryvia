package loader

import (
	"os"
	"regexp"
	"strconv"
	"testing"
)

// TestXDPSlotsMatchHeader pins the Go slot numbers to ebpf/headers/xdp_chain.h.
func TestXDPSlotsMatchHeader(t *testing.T) {
	src, err := os.ReadFile("../../../ebpf/headers/xdp_chain.h")
	if err != nil {
		t.Skipf("header not available: %v", err)
	}
	for name, want := range map[string]uint32{
		"XDP_SLOT_ROCE_CNP": XDPSlotRoceCNP, "XDP_SLOT_PFC_PAUSE": XDPSlotPFCPause,
		"XDP_SLOT_DNS": XDPSlotDNS, "XDP_SLOT_PACKET_FILTER": XDPSlotPacketFilter,
		"XDP_SLOT_ROCE_ECN": XDPSlotRoceECN, "XDP_SLOT_MAX": XDPSlots,
	} {
		m := regexp.MustCompile(`#define ` + name + `\s+(\d+)`).FindSubmatch(src)
		if m == nil {
			t.Fatalf("%s not found in header", name)
		}
		got, _ := strconv.ParseUint(string(m[1]), 10, 32)
		if uint32(got) != want {
			t.Errorf("%s: header %d, Go %d", name, got, want)
		}
	}
}
