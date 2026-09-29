package scheduler

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
// NOT WIRED: nothing calls these yet.  scoreNodes works on integer node labels
// and has no per-node fabric status to read; a caller that has scoreDelta for
// a node can subtract FabricPenalty(scoreDelta) from that node's score.

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
