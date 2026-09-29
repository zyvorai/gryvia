package exporter

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/zyvorai/gryvia/collector/pkg/fabric"
)

func gatherValue(t *testing.T, name string, labels map[string]string) (float64, bool) {
	t.Helper()
	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, mf := range mfs {
		if mf.GetName() != name {
			continue
		}
	next:
		for _, m := range mf.GetMetric() {
			for _, lp := range m.GetLabel() {
				if want, ok := labels[lp.GetName()]; ok && want != lp.GetValue() {
					continue next
				}
			}
			if m.GetGauge() != nil {
				return m.GetGauge().GetValue(), true
			}
			if m.GetCounter() != nil {
				return m.GetCounter().GetValue(), true
			}
		}
	}
	return 0, false
}

func TestRecordPFCAndFabricGauges(t *testing.T) {
	m := NewMetrics()
	var perPrio [fabric.PFCPriorities]uint64
	perPrio[3], perPrio[5] = 7, 2
	m.RecordPFC(9, 4, perPrio)
	m.RecordPFC(1, 0, [fabric.PFCPriorities]uint64{3: 1})
	for name, want := range map[string]float64{
		"gryvia_pfc_pause_frames_total":        10,
		"gryvia_pfc_legacy_pause_frames_total": 4,
	} {
		if got, ok := gatherValue(t, name, nil); !ok || got != want {
			t.Errorf("%s = %v ok=%v, want %v", name, got, ok, want)
		}
	}
	if got, ok := gatherValue(t, "gryvia_pfc_priority_pause_frames_total", map[string]string{"priority": "3"}); !ok || got != 8 {
		t.Errorf("priority 3 = %v ok=%v, want 8", got, ok)
	}
	if _, ok := gatherValue(t, "gryvia_pfc_priority_pause_frames_total", map[string]string{"priority": "0"}); ok {
		t.Error("a priority with no pauses must not create a series")
	}

	m.RecordFabric([]fabric.JobStatus{{
		JobKey: fabric.PFCJob,
		Status: fabric.Status{PFCRate: 1500, ExfilEvents: 2, UCXSlowP99MS: 12},
	}})
	labels := map[string]string{"namespace": "_node", "job": "pfc"}
	for name, want := range map[string]float64{
		"gryvia_fabric_pfc_rate": 1500, "gryvia_fabric_exfil_events": 2, "gryvia_fabric_ucx_slow_p99_seconds": 0.012,
	} {
		if got, ok := gatherValue(t, name, labels); !ok || got != want {
			t.Errorf("%s = %v ok=%v, want %v", name, got, ok, want)
		}
	}
	m.RecordFabric(nil)
	if _, ok := gatherValue(t, "gryvia_fabric_pfc_rate", labels); ok {
		t.Error("jobs that aged out must disappear")
	}
}
