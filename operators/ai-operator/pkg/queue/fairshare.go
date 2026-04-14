package queue

import (
	"math"
	"sort"
)

// ResourceVector represents a normalized resource request as fractions of
// total cluster capacity. Each dimension is in [0, 1].
type ResourceVector struct {
	GPU    float64
	CPU    float64
	Memory float64
}

// ClusterCapacity holds the total allocatable resources in the cluster.
type ClusterCapacity struct {
	TotalGPUs   float64
	TotalCPUs   float64
	TotalMemory float64 // bytes
}

// TeamAllocation holds a team's fair-share weight and current consumption.
type TeamAllocation struct {
	// Team identifier (namespace or team label).
	Team string
	// Weight derived from FabricQuota (higher weight = larger fair share).
	Weight float64
	// Allocated tracks the resources currently consumed by this team.
	Allocated ResourceVector
	// DominantShare is the maximum of the normalized resource dimensions.
	DominantShare float64
}

// DRFScheduler implements Dominant Resource Fairness.
// At each scheduling step it selects the team with the smallest dominant
// share (the bottleneck resource fraction) to receive the next allocation.
type DRFScheduler struct {
	Capacity    ClusterCapacity
	Allocations map[string]*TeamAllocation
}

// NewDRFScheduler creates a DRFScheduler with the given cluster capacity.
func NewDRFScheduler(capacity ClusterCapacity) *DRFScheduler {
	return &DRFScheduler{
		Capacity:    capacity,
		Allocations: make(map[string]*TeamAllocation),
	}
}

// RegisterTeam adds or updates a team with the given weight.
// Weight should be derived from the FabricQuota's MaxGPUs or explicit priority.
func (d *DRFScheduler) RegisterTeam(team string, weight float64) {
	if weight <= 0 {
		weight = 1.0
	}
	if alloc, exists := d.Allocations[team]; exists {
		alloc.Weight = weight
		return
	}
	d.Allocations[team] = &TeamAllocation{
		Team:   team,
		Weight: weight,
	}
}

// RecordAllocation adds a resource consumption to a team.
func (d *DRFScheduler) RecordAllocation(team string, gpus, cpus, memoryBytes float64) {
	alloc, exists := d.Allocations[team]
	if !exists {
		d.RegisterTeam(team, 1.0)
		alloc = d.Allocations[team]
	}

	alloc.Allocated.GPU += gpus
	alloc.Allocated.CPU += cpus
	alloc.Allocated.Memory += memoryBytes
	d.updateDominantShare(alloc)
}

// ReleaseAllocation removes a resource consumption from a team.
func (d *DRFScheduler) ReleaseAllocation(team string, gpus, cpus, memoryBytes float64) {
	alloc, exists := d.Allocations[team]
	if !exists {
		return
	}

	alloc.Allocated.GPU = math.Max(0, alloc.Allocated.GPU-gpus)
	alloc.Allocated.CPU = math.Max(0, alloc.Allocated.CPU-cpus)
	alloc.Allocated.Memory = math.Max(0, alloc.Allocated.Memory-memoryBytes)
	d.updateDominantShare(alloc)
}

// updateDominantShare recalculates the dominant share for a team.
// The dominant share is the maximum fraction across all resource dimensions,
// divided by the team's weight to implement weighted fairness.
func (d *DRFScheduler) updateDominantShare(alloc *TeamAllocation) {
	var gpuShare, cpuShare, memShare float64

	if d.Capacity.TotalGPUs > 0 {
		gpuShare = alloc.Allocated.GPU / d.Capacity.TotalGPUs
	}
	if d.Capacity.TotalCPUs > 0 {
		cpuShare = alloc.Allocated.CPU / d.Capacity.TotalCPUs
	}
	if d.Capacity.TotalMemory > 0 {
		memShare = alloc.Allocated.Memory / d.Capacity.TotalMemory
	}

	dominant := math.Max(gpuShare, math.Max(cpuShare, memShare))

	// Divide by weight so higher-weight teams get proportionally more.
	if alloc.Weight > 0 {
		alloc.DominantShare = dominant / alloc.Weight
	} else {
		alloc.DominantShare = dominant
	}
}

// SelectNextTeam returns the team that should receive the next resource
// allocation, i.e., the one with the smallest weighted dominant share.
func (d *DRFScheduler) SelectNextTeam() string {
	if len(d.Allocations) == 0 {
		return ""
	}

	var candidates []*TeamAllocation
	for _, a := range d.Allocations {
		candidates = append(candidates, a)
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].DominantShare == candidates[j].DominantShare {
			// Tie-break: higher weight goes first.
			return candidates[i].Weight > candidates[j].Weight
		}
		return candidates[i].DominantShare < candidates[j].DominantShare
	})

	return candidates[0].Team
}

// GetTeamShare returns the dominant share for a team.
func (d *DRFScheduler) GetTeamShare(team string) float64 {
	if alloc, exists := d.Allocations[team]; exists {
		return alloc.DominantShare
	}
	return 0
}

// GetTeamAllocation returns a copy of the team's allocation, or nil.
func (d *DRFScheduler) GetTeamAllocation(team string) *TeamAllocation {
	if alloc, exists := d.Allocations[team]; exists {
		cpy := *alloc
		return &cpy
	}
	return nil
}

// RankedTeams returns all teams sorted by dominant share (ascending).
func (d *DRFScheduler) RankedTeams() []TeamAllocation {
	result := make([]TeamAllocation, 0, len(d.Allocations))
	for _, a := range d.Allocations {
		result = append(result, *a)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].DominantShare < result[j].DominantShare
	})
	return result
}

// CanFit checks whether adding the given resources to a team would keep
// total cluster usage within capacity.
func (d *DRFScheduler) CanFit(gpus, cpus, memoryBytes float64) bool {
	var totalGPU, totalCPU, totalMem float64
	for _, a := range d.Allocations {
		totalGPU += a.Allocated.GPU
		totalCPU += a.Allocated.CPU
		totalMem += a.Allocated.Memory
	}

	if d.Capacity.TotalGPUs > 0 && totalGPU+gpus > d.Capacity.TotalGPUs {
		return false
	}
	if d.Capacity.TotalCPUs > 0 && totalCPU+cpus > d.Capacity.TotalCPUs {
		return false
	}
	if d.Capacity.TotalMemory > 0 && totalMem+memoryBytes > d.Capacity.TotalMemory {
		return false
	}
	return true
}
