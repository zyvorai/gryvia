package scheduler

import "math"

// Fabric health adjustment for node scores.
//
// The collector (collector/pkg/fabric, GET /api/v1/fabric or the
// GryviaFabricSignal status.scoreDelta) folds NCCL stragglers, RDMA retries,
// GPUDirect Storage misses, GPU-idle-while-comm and RoCE congestion
// notifications into a single penalty in [0,1]: 0 = healthy fabric, 1 = do not
// place another gang here.  This module cannot import the collector (each
// operator is its own Go module), so the already-computed value is passed in
// as a plain float64 instead of a fabric.Status.
//
// NOT WIRED: nothing calls these yet, and nothing produces a per-node scoreDelta
// map: the collector's status is per job (GryviaFabricSignal.status) and, with
// several nodes per job, last-writer-wins, so it is not a node signal.
// scoreNodes works on integer node labels and has no per-node fabric status to
// read.  RescoreNodes below is the optional hook a future caller can use once a
// per-node source exists; it is inert with a nil or empty map.

// MaxFabricPenalty is the most points a sick fabric can subtract, so it
// cannot outrank a healthy node on topology alone.
const MaxFabricPenalty = 25.0

// FabricPenalty converts the collector's scoreDelta into points to subtract.
// Values outside [0,1] (and NaN) are clamped, so the result is always in
// [0, MaxFabricPenalty].
func FabricPenalty(scoreDelta float64) float64 {
	if !(scoreDelta > 0) { // also true for NaN
		return 0
	}
	if scoreDelta > 1 {
		scoreDelta = 1
	}
	return MaxFabricPenalty * scoreDelta
}

// ApplyFabricPenalty returns base minus the fabric penalty, never negative.
func ApplyFabricPenalty(base, scoreDelta float64) float64 {
	s := base - FabricPenalty(scoreDelta)
	if s < 0 {
		return 0
	}
	return s
}

// RescoreNodes returns a copy of nodes (the scheduler's NodeScore) with
// ApplyFabricPenalty applied, rounded to whole points, to every node that has an
// entry in scoreDeltas (node name -> the collector's scoreDelta).  A nil or empty map, or a node without an entry, leaves the score
// unchanged, so the hook is a no-op until something feeds it.  The input slice
// is never modified and the order is preserved.  It takes no part in any
// scheduling decision today.
func RescoreNodes(nodes []NodeScore, scoreDeltas map[string]float64) []NodeScore {
	out := make([]NodeScore, len(nodes))
	copy(out, nodes)
	if len(scoreDeltas) == 0 {
		return out
	}
	for i := range out {
		if d, ok := scoreDeltas[out[i].NodeName]; ok {
			out[i].Score = int(math.Round(ApplyFabricPenalty(float64(out[i].Score), d)))
		}
	}
	return out
}
