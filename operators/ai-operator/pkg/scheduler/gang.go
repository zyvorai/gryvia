package scheduler

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gryviav1 "github.com/zyvorai/gryvia/operators/ai-operator/api/v1"
)

// PodGroupPhase represents the scheduling phase of a PodGroup.
type PodGroupPhase string

const (
	PodGroupPending   PodGroupPhase = "Pending"
	PodGroupScheduled PodGroupPhase = "Scheduled"
	PodGroupFailed    PodGroupPhase = "Failed"

	// gangScheduleTimeout is how long a gang waits before releasing held
	// resources to break deadlocks with other gangs.
	gangScheduleTimeout = 2 * time.Minute
)

// PodGroup tracks all pods belonging to a single distributed job so they can
// be scheduled atomically (all-or-nothing).
type PodGroup struct {
	// JobName is the GryviaAIJob name that owns this group.
	JobName string
	// Namespace of the owning job.
	Namespace string
	// MinMembers is the minimum number of pods that must be schedulable
	// before any pod is allowed to start.
	MinMembers int32
	// TotalMembers is the desired replica count for the job.
	TotalMembers int32
	// Phase is the current scheduling phase.
	Phase PodGroupPhase
	// NodeAssignments maps pod ordinal index -> node name.
	NodeAssignments map[int32]string
	// GPUsPerPod is the GPU count each pod in the group requires.
	GPUsPerPod int32
	// CreatedAt records when this PodGroup was first created.
	CreatedAt time.Time
	// LastAttempt records the last scheduling attempt.
	LastAttempt time.Time
}

// GangScheduler wraps the existing GPU-aware scoring/filtering logic and
// adds all-or-nothing semantics for distributed training jobs.
type GangScheduler struct {
	client client.Client
	log    logr.Logger
	mu     sync.Mutex
	groups map[string]*PodGroup // key = namespace/jobName
	// heldResources tracks GPUs tentatively reserved by pending gangs.
	// key = node name, value = GPUs held.
	heldResources map[string]int64
}

// NewGangScheduler creates a new GangScheduler instance.
func NewGangScheduler(c client.Client) *GangScheduler {
	return &GangScheduler{
		client:        c,
		log:           ctrl.Log.WithName("gang-scheduler"),
		groups:        make(map[string]*PodGroup),
		heldResources: make(map[string]int64),
	}
}

// GetOrCreatePodGroup returns an existing PodGroup or creates one for the job.
func (gs *GangScheduler) GetOrCreatePodGroup(job *gryviav1.GryviaAIJob) *PodGroup {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	key := fmt.Sprintf("%s/%s", job.Namespace, job.Name)
	if pg, exists := gs.groups[key]; exists {
		return pg
	}

	replicas := int32(1)
	gpusPerPod := job.Spec.GPUs
	if job.Spec.Distributed != nil && job.Spec.Distributed.Enabled {
		if job.Spec.Distributed.Nodes > 0 {
			replicas = job.Spec.Distributed.Nodes
		}
		if job.Spec.Distributed.GpusPerNode > 0 {
			gpusPerPod = job.Spec.Distributed.GpusPerNode
		}
	}

	pg := &PodGroup{
		JobName:         job.Name,
		Namespace:       job.Namespace,
		MinMembers:      replicas,
		TotalMembers:    replicas,
		Phase:           PodGroupPending,
		NodeAssignments: make(map[int32]string),
		GPUsPerPod:      gpusPerPod,
		CreatedAt:       time.Now(),
	}
	gs.groups[key] = pg
	return pg
}

// RemovePodGroup removes the PodGroup and releases any held resources.
func (gs *GangScheduler) RemovePodGroup(namespace, jobName string) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	key := fmt.Sprintf("%s/%s", namespace, jobName)
	if pg, exists := gs.groups[key]; exists {
		gs.releaseHeldResourcesLocked(pg)
		delete(gs.groups, key)
	}
}

// CanScheduleGang checks whether ALL pods in the gang can be placed on the
// given set of nodes. It does not mutate any state.
func (gs *GangScheduler) CanScheduleGang(ctx context.Context, job *gryviav1.GryviaAIJob, nodes []corev1.Node) (bool, error) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	key := fmt.Sprintf("%s/%s", job.Namespace, job.Name)
	pg, exists := gs.groups[key]
	if !exists {
		return false, fmt.Errorf("no PodGroup registered for %s", key)
	}

	// Build a snapshot of available GPUs per node (accounting for held resources).
	pods := &corev1.PodList{}
	if err := gs.client.List(ctx, pods, client.MatchingFields{"status.phase": "Running"}); err != nil {
		pods = &corev1.PodList{}
		if err := gs.client.List(ctx, pods); err != nil {
			return false, fmt.Errorf("failed to list pods: %w", err)
		}
	}
	gpuUsage := calculateGPUUsagePerNode(pods.Items)

	// Add held resources from other gangs (not this one) to usage.
	adjustedUsage := make(map[string]int64)
	for k, v := range gpuUsage {
		adjustedUsage[k] = v
	}
	for nodeName, held := range gs.heldResources {
		// Subtract this gang's own holds so we don't double-count.
		ownHold := int64(0)
		for _, assignedNode := range pg.NodeAssignments {
			if assignedNode == nodeName {
				ownHold += int64(pg.GPUsPerPod)
			}
		}
		adjustedUsage[nodeName] += held - ownHold
	}

	gpusNeeded := pg.GPUsPerPod
	requiredNodes := int(pg.MinMembers)

	// Filter and score eligible nodes.
	eligible := filterNodes(nodes, job, adjustedUsage, gpusNeeded)
	if len(eligible) < requiredNodes {
		return false, nil
	}

	scored := scoreNodes(eligible, job, adjustedUsage)
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})

	return len(scored) >= requiredNodes, nil
}

// ScheduleGang attempts to atomically assign all pods in the gang to nodes.
// Returns a map of pod ordinal -> node name, or an error if the gang cannot
// be placed. On failure, any tentatively held resources are released.
func (gs *GangScheduler) ScheduleGang(ctx context.Context, job *gryviav1.GryviaAIJob, nodes []corev1.Node) (map[int32]string, error) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	key := fmt.Sprintf("%s/%s", job.Namespace, job.Name)
	pg, exists := gs.groups[key]
	if !exists {
		return nil, fmt.Errorf("no PodGroup registered for %s", key)
	}

	pg.LastAttempt = time.Now()

	// Check for deadlock: if we have been trying too long, release and fail.
	if !pg.CreatedAt.IsZero() && time.Since(pg.CreatedAt) > gangScheduleTimeout && pg.Phase == PodGroupPending {
		gs.releaseHeldResourcesLocked(pg)
		pg.Phase = PodGroupFailed
		return nil, fmt.Errorf("gang scheduling timed out for %s after %v; released held resources", key, gangScheduleTimeout)
	}

	// Build GPU usage snapshot.
	pods := &corev1.PodList{}
	if err := gs.client.List(ctx, pods, client.MatchingFields{"status.phase": "Running"}); err != nil {
		pods = &corev1.PodList{}
		if err := gs.client.List(ctx, pods); err != nil {
			return nil, fmt.Errorf("failed to list pods: %w", err)
		}
	}
	gpuUsage := calculateGPUUsagePerNode(pods.Items)

	// Add held resources from other gangs to the snapshot.
	for nodeName, held := range gs.heldResources {
		gpuUsage[nodeName] += held
	}
	// Remove this gang's own holds (if any from a previous partial attempt).
	for _, nodeName := range pg.NodeAssignments {
		gpuUsage[nodeName] -= int64(pg.GPUsPerPod)
		if gpuUsage[nodeName] < 0 {
			gpuUsage[nodeName] = 0
		}
	}

	gpusNeeded := pg.GPUsPerPod
	requiredNodes := int(pg.MinMembers)

	// Filter eligible nodes.
	eligible := filterNodes(nodes, job, gpuUsage, gpusNeeded)
	if len(eligible) < requiredNodes {
		gs.log.Info("Not enough eligible nodes for gang",
			"job", key,
			"required", requiredNodes,
			"eligible", len(eligible),
		)
		return nil, fmt.Errorf("not enough eligible nodes for gang %s: need %d, found %d", key, requiredNodes, len(eligible))
	}

	// Score and sort eligible nodes.
	scored := scoreNodes(eligible, job, gpuUsage)
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})

	if len(scored) < requiredNodes {
		return nil, fmt.Errorf("not enough scored nodes for gang %s: need %d, found %d", key, requiredNodes, len(scored))
	}

	// Assign pods to nodes atomically.
	assignments := make(map[int32]string, requiredNodes)
	for i := 0; i < requiredNodes; i++ {
		ordinal := int32(i)
		nodeName := scored[i].NodeName
		assignments[ordinal] = nodeName

		// Hold GPUs on this node.
		gs.heldResources[nodeName] += int64(gpusNeeded)
	}

	pg.NodeAssignments = assignments
	pg.Phase = PodGroupScheduled

	gs.log.Info("Gang scheduled successfully",
		"job", key,
		"assignments", assignments,
	)

	return assignments, nil
}

// DetectDeadlocks checks all pending PodGroups and releases resources for
// those that have exceeded the scheduling timeout.
func (gs *GangScheduler) DetectDeadlocks() []string {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	var deadlocked []string
	for key, pg := range gs.groups {
		if pg.Phase != PodGroupPending {
			continue
		}
		if time.Since(pg.CreatedAt) > gangScheduleTimeout {
			gs.log.Info("Deadlock detected, releasing held resources", "podGroup", key)
			gs.releaseHeldResourcesLocked(pg)
			pg.Phase = PodGroupFailed
			deadlocked = append(deadlocked, key)
		}
	}
	return deadlocked
}

// releaseHeldResourcesLocked releases all GPU holds for a PodGroup.
// The caller must hold gs.mu.
func (gs *GangScheduler) releaseHeldResourcesLocked(pg *PodGroup) {
	for _, nodeName := range pg.NodeAssignments {
		gs.heldResources[nodeName] -= int64(pg.GPUsPerPod)
		if gs.heldResources[nodeName] <= 0 {
			delete(gs.heldResources, nodeName)
		}
	}
	pg.NodeAssignments = make(map[int32]string)
}

// GetPodGroup returns the PodGroup for a given job, or nil if not found.
func (gs *GangScheduler) GetPodGroup(namespace, jobName string) *PodGroup {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	key := fmt.Sprintf("%s/%s", namespace, jobName)
	return gs.groups[key]
}

// GetHeldGPUs returns the number of GPUs tentatively held on a given node.
func (gs *GangScheduler) GetHeldGPUs(nodeName string) int64 {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	return gs.heldResources[nodeName]
}

// ResetPodGroup moves a failed PodGroup back to pending so it can be retried.
func (gs *GangScheduler) ResetPodGroup(namespace, jobName string) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	key := fmt.Sprintf("%s/%s", namespace, jobName)
	if pg, exists := gs.groups[key]; exists {
		gs.releaseHeldResourcesLocked(pg)
		pg.Phase = PodGroupPending
		pg.CreatedAt = time.Now()
	}
}
