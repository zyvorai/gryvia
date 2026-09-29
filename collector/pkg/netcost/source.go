package netcost

import (
	"context"
	"time"

	"github.com/cilium/ebpf"
)

// ScanInterval is how often the counter map is folded into the Meter.
const ScanInterval = 15 * time.Second

// costKey / costValue mirror struct cost_key / struct cost_value of ebpf/cost_tracker.c (8 and 32 bytes).
type costKey struct{ Src, Dst uint32 }
type costValue struct{ BytesSent, BytesRecv, PacketsSent, PacketsRecv uint64 }

// ReadMap reads the whole traffic_costs map. Iteration over a hash map that changes concurrently can repeat a
// key; Meter.Observe ignores the repeats, so a pair is never counted twice.
func ReadMap(m *ebpf.Map) ([]Sample, error) {
	var (
		k   costKey
		v   costValue
		out []Sample
	)
	it := m.Iterate()
	for it.Next(&k, &v) {
		out = append(out, Sample{Local: k.Src, Remote: k.Dst, BytesSent: v.BytesSent, BytesRecv: v.BytesRecv})
		if len(out) >= 4*MaxPairs {
			break
		}
	}
	return out, it.Err()
}

// RunScan folds the map into the meter every ScanInterval until ctx is done.
func RunScan(ctx context.Context, m *ebpf.Map, meter *Meter, onError func(error)) {
	t := time.NewTicker(ScanInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			samples, err := ReadMap(m)
			if err != nil {
				if onError != nil {
					onError(err)
				}
				continue
			}
			meter.Observe(samples, now)
		}
	}
}
