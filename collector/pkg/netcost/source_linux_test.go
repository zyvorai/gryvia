//go:build linux

package netcost

import (
	"os"
	"testing"

	"github.com/cilium/ebpf"
)

// TestReadMapKernel creates a real LRU hash map with the key/value sizes of traffic_costs, fills it and reads
// it back with ReadMap. It needs CAP_BPF, so it only runs with GRYVIA_KERNEL_TEST=1 (as root).
func TestReadMapKernel(t *testing.T) {
	if os.Getenv("GRYVIA_KERNEL_TEST") != "1" {
		t.Skip("set GRYVIA_KERNEL_TEST=1 and run as root")
	}
	m, err := ebpf.NewMap(&ebpf.MapSpec{Type: ebpf.LRUHash, KeySize: 8, ValueSize: 32, MaxEntries: 16})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	want := costValue{BytesSent: 1234, BytesRecv: 99, PacketsSent: 2, PacketsRecv: 1}
	if err := m.Put(costKey{Src: ip("10.0.1.1"), Dst: ip("8.8.8.8")}, want); err != nil {
		t.Fatal(err)
	}
	got, err := ReadMap(m)
	if err != nil || len(got) != 1 {
		t.Fatalf("%v %v", got, err)
	}
	if IPv4(got[0].Local) != "10.0.1.1" || IPv4(got[0].Remote) != "8.8.8.8" || got[0].BytesSent != 1234 || got[0].BytesRecv != 99 {
		t.Fatalf("%+v", got[0])
	}
}
