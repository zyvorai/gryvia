package queue

import (
	"container/heap"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gryviav1 "github.com/zyvorai/gryvia/operators/ai-operator/api/v1"
)

// QueuedJob wraps a FabricAIJob with queue metadata.
type QueuedJob struct {
	Job        *gryviav1.FabricAIJob
	Team       string
	Priority   int32
	EnqueuedAt time.Time
	// GPUs is the total GPU count for this job.
	GPUs int32
	// CPUs is the total CPU millicores requested.
	CPUs int64
	// MemoryBytes is the total memory requested.
	MemoryBytes int64
	// index is used by the priority queue.
	index int
}

// priorityQueue implements heap.Interface for QueuedJob.
// Higher priority jobs come first; ties are broken by enqueue time.
type priorityQueue []*QueuedJob

func (pq priorityQueue) Len() int { return len(pq) }

func (pq priorityQueue) Less(i, j int) bool {
	if pq[i].Priority != pq[j].Priority {
		return pq[i].Priority > pq[j].Priority
	}
	return pq[i].EnqueuedAt.Before(pq[j].EnqueuedAt)
}

func (pq priorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *priorityQueue) Push(x interface{}) {
	n := len(*pq)
	item := x.(*QueuedJob)
	item.index = n
	*pq = append(*pq, item)
}

func (pq *priorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	*pq = old[:n-1]
	return item
}

// JobQueue manages an ordered queue of FabricAIJobs with fair-share
// scheduling, backfill support, and priority-based preemption.
type JobQueue struct {
	mu       sync.Mutex
	log      logr.Logger
	client   client.Client
	pq       priorityQueue
	jobIndex map[string]int // namespace/name -> index in pq
	drf      *DRFScheduler
}

// NewJobQueue creates a new fair-share job queue backed by DRF.
func NewJobQueue(c client.Client, capacity ClusterCapacity) *JobQueue {
	jq := &JobQueue{
		log:      ctrl.Log.WithName("job-queue"),
		client:   c,
		pq:       make(priorityQueue, 0),
		jobIndex: make(map[string]int),
		drf:      NewDRFScheduler(capacity),
	}
	heap.Init(&jq.pq)
	return jq
}

// RegisterTeam registers a team with a fair-share weight derived from quota.
func (jq *JobQueue) RegisterTeam(team string, weight float64) {
	jq.mu.Lock()
	defer jq.mu.Unlock()
	jq.drf.RegisterTeam(team, weight)
}

// Enqueue adds a job to the queue.
func (jq *JobQueue) Enqueue(job *gryviav1.FabricAIJob) error {
	jq.mu.Lock()
	defer jq.mu.Unlock()

	key := jobKey(job)
	if _, exists := jq.jobIndex[key]; exists {
		return fmt.Errorf("job %s is already queued", key)
	}

	gpus := totalGPUs(job)
	cpus, mem := extractCPUMemory(job)
	team := teamForJob(job)

	qj := &QueuedJob{
		Job:         job,
		Team:        team,
		Priority:    job.Spec.Priority,
		EnqueuedAt:  time.Now(),
		GPUs:        gpus,
		CPUs:        cpus,
		MemoryBytes: mem,
	}

	heap.Push(&jq.pq, qj)
	jq.jobIndex[key] = qj.index

	jq.log.Info("Job enqueued",
		"job", key,
		"team", team,
		"priority", job.Spec.Priority,
		"gpus", gpus,
	)

	return nil
}

// Dequeue removes and returns the highest-priority job that can be scheduled.
// It uses DRF to enforce fairness: among jobs with equal priority, the job
// belonging to the team with the lowest dominant share is selected first.
func (jq *JobQueue) Dequeue() *QueuedJob {
	jq.mu.Lock()
	defer jq.mu.Unlock()

	if jq.pq.Len() == 0 {
		return nil
	}

	// Find the best candidate using DRF among top-priority jobs.
	topPriority := jq.pq[0].Priority
	bestIdx := 0
	bestDominantShare := float64(1<<63 - 1)

	for i, qj := range jq.pq {
		if qj.Priority < topPriority {
			break // Lower priority, stop scanning.
		}
		share := jq.drf.GetTeamShare(qj.Team)
		if share < bestDominantShare {
			bestDominantShare = share
			bestIdx = i
		}
	}

	// Remove the selected job.
	item := heap.Remove(&jq.pq, bestIdx).(*QueuedJob)
	delete(jq.jobIndex, jobKey(item.Job))

	// Record the allocation in DRF.
	jq.drf.RecordAllocation(item.Team,
		float64(item.GPUs),
		float64(item.CPUs)/1000.0,
		float64(item.MemoryBytes),
	)

	jq.log.Info("Job dequeued",
		"job", jobKey(item.Job),
		"team", item.Team,
		"dominantShare", bestDominantShare,
	)

	return item
}

// Peek returns the next job that would be dequeued without removing it.
func (jq *JobQueue) Peek() *QueuedJob {
	jq.mu.Lock()
	defer jq.mu.Unlock()

	if jq.pq.Len() == 0 {
		return nil
	}

	// Same DRF-aware selection as Dequeue, but non-destructive.
	topPriority := jq.pq[0].Priority
	bestIdx := 0
	bestDominantShare := float64(1<<63 - 1)

	for i, qj := range jq.pq {
		if qj.Priority < topPriority {
			break
		}
		share := jq.drf.GetTeamShare(qj.Team)
		if share < bestDominantShare {
			bestDominantShare = share
			bestIdx = i
		}
	}

	return jq.pq[bestIdx]
}

// Remove removes a specific job from the queue (e.g., on cancellation).
func (jq *JobQueue) Remove(namespace, name string) bool {
	jq.mu.Lock()
	defer jq.mu.Unlock()

	key := fmt.Sprintf("%s/%s", namespace, name)
	idx, exists := jq.jobIndex[key]
	if !exists {
		return false
	}

	heap.Remove(&jq.pq, idx)
	delete(jq.jobIndex, key)
	// Rebuild the index since heap.Remove may have shuffled positions.
	jq.rebuildIndex()

	jq.log.Info("Job removed from queue", "job", key)
	return true
}

// Len returns the current queue length.
func (jq *JobQueue) Len() int {
	jq.mu.Lock()
	defer jq.mu.Unlock()
	return jq.pq.Len()
}

// Backfill returns a list of smaller jobs that can fit in the remaining
// cluster capacity while a large job at the head of the queue is waiting.
// This prevents small jobs from being blocked by a single large job.
func (jq *JobQueue) Backfill(availableGPUs int32, availableCPUs int64, availableMemory int64) []*QueuedJob {
	jq.mu.Lock()
	defer jq.mu.Unlock()

	if jq.pq.Len() <= 1 {
		return nil
	}

	var backfillable []*QueuedJob
	remainingGPUs := availableGPUs
	remainingCPUs := availableCPUs
	remainingMem := availableMemory

	// Skip the head-of-line job (it's the one waiting for resources).
	// Try to fit subsequent jobs in remaining capacity.
	for i := 1; i < jq.pq.Len(); i++ {
		qj := jq.pq[i]
		if qj.GPUs <= remainingGPUs &&
			qj.CPUs <= remainingCPUs &&
			qj.MemoryBytes <= remainingMem {

			backfillable = append(backfillable, qj)
			remainingGPUs -= qj.GPUs
			remainingCPUs -= qj.CPUs
			remainingMem -= qj.MemoryBytes
		}
	}

	// Actually remove backfillable jobs from the queue.
	for _, qj := range backfillable {
		key := jobKey(qj.Job)
		if idx, exists := jq.jobIndex[key]; exists {
			heap.Remove(&jq.pq, idx)
			delete(jq.jobIndex, key)
			jq.rebuildIndex()

			jq.drf.RecordAllocation(qj.Team,
				float64(qj.GPUs),
				float64(qj.CPUs)/1000.0,
				float64(qj.MemoryBytes),
			)
		}
	}

	if len(backfillable) > 0 {
		jq.log.Info("Backfill selected jobs",
			"count", len(backfillable),
			"remainingGPUs", remainingGPUs,
		)
	}

	return backfillable
}

// FindPreemptionCandidates returns lower-priority jobs that could be preempted
// to make room for the given high-priority job.
func (jq *JobQueue) FindPreemptionCandidates(job *gryviav1.FabricAIJob) []*QueuedJob {
	jq.mu.Lock()
	defer jq.mu.Unlock()

	gpusNeeded := totalGPUs(job)
	var candidates []*QueuedJob
	gpusFreed := int32(0)

	// Walk the queue from lowest priority (tail) upward.
	for i := jq.pq.Len() - 1; i >= 0; i-- {
		qj := jq.pq[i]
		if qj.Priority >= job.Spec.Priority {
			continue // Don't preempt equal or higher priority.
		}
		candidates = append(candidates, qj)
		gpusFreed += qj.GPUs
		if gpusFreed >= gpusNeeded {
			break
		}
	}

	if gpusFreed < gpusNeeded {
		return nil // Can't free enough GPUs even by preempting everything.
	}

	return candidates
}

// PreemptJobs removes the specified jobs from the queue and releases their
// DRF allocations. Returns the jobs that were preempted. The caller is
// responsible for actually stopping the running pods.
func (jq *JobQueue) PreemptJobs(candidates []*QueuedJob) []*QueuedJob {
	jq.mu.Lock()
	defer jq.mu.Unlock()

	var preempted []*QueuedJob
	for _, qj := range candidates {
		key := jobKey(qj.Job)
		if idx, exists := jq.jobIndex[key]; exists {
			heap.Remove(&jq.pq, idx)
			delete(jq.jobIndex, key)
			jq.rebuildIndex()

			jq.drf.ReleaseAllocation(qj.Team,
				float64(qj.GPUs),
				float64(qj.CPUs)/1000.0,
				float64(qj.MemoryBytes),
			)

			preempted = append(preempted, qj)
			jq.log.Info("Job preempted",
				"job", key,
				"priority", qj.Priority,
			)
		}
	}

	return preempted
}

// ReleaseJobResources should be called when a job completes or is deleted to
// release its DRF allocation.
func (jq *JobQueue) ReleaseJobResources(job *gryviav1.FabricAIJob) {
	jq.mu.Lock()
	defer jq.mu.Unlock()

	team := teamForJob(job)
	gpus := totalGPUs(job)
	cpus, mem := extractCPUMemory(job)

	jq.drf.ReleaseAllocation(team,
		float64(gpus),
		float64(cpus)/1000.0,
		float64(mem),
	)
}

// UpdateCapacity updates the DRF scheduler's view of cluster capacity.
func (jq *JobQueue) UpdateCapacity(ctx context.Context) error {
	nodes := &corev1.NodeList{}
	if err := jq.client.List(ctx, nodes); err != nil {
		return fmt.Errorf("failed to list nodes: %w", err)
	}

	var totalGPU, totalCPU, totalMem float64
	for _, node := range nodes.Items {
		if gpu, ok := node.Status.Allocatable["nvidia.com/gpu"]; ok {
			totalGPU += float64(gpu.Value())
		}
		if cpu, ok := node.Status.Allocatable[corev1.ResourceCPU]; ok {
			totalCPU += float64(cpu.MilliValue()) / 1000.0
		}
		if mem, ok := node.Status.Allocatable[corev1.ResourceMemory]; ok {
			totalMem += float64(mem.Value())
		}
	}

	jq.mu.Lock()
	defer jq.mu.Unlock()

	jq.drf.Capacity = ClusterCapacity{
		TotalGPUs:   totalGPU,
		TotalCPUs:   totalCPU,
		TotalMemory: totalMem,
	}

	// Recalculate all dominant shares.
	for _, alloc := range jq.drf.Allocations {
		jq.drf.updateDominantShare(alloc)
	}

	return nil
}

// GetDRFScheduler returns the underlying DRF scheduler for inspection.
func (jq *JobQueue) GetDRFScheduler() *DRFScheduler {
	return jq.drf
}

// rebuildIndex reconstructs the jobIndex map from the current heap.
// Must be called with jq.mu held.
func (jq *JobQueue) rebuildIndex() {
	jq.jobIndex = make(map[string]int, jq.pq.Len())
	for i, qj := range jq.pq {
		qj.index = i
		jq.jobIndex[jobKey(qj.Job)] = i
	}
}

// jobKey returns a unique key for a job.
func jobKey(job *gryviav1.FabricAIJob) string {
	return fmt.Sprintf("%s/%s", job.Namespace, job.Name)
}

// totalGPUs computes the total GPU count for a job.
func totalGPUs(job *gryviav1.FabricAIJob) int32 {
	if job.Spec.Distributed != nil && job.Spec.Distributed.Enabled {
		nodes := job.Spec.Distributed.Nodes
		if nodes <= 0 {
			nodes = 1
		}
		gpusPerNode := job.Spec.Distributed.GpusPerNode
		if gpusPerNode <= 0 {
			gpusPerNode = job.Spec.GPUs
		}
		return nodes * gpusPerNode
	}
	return job.Spec.GPUs
}

// teamForJob determines which team a job belongs to.
// It uses the namespace as the team identifier. Operators can label
// namespaces with team names for more granular control.
func teamForJob(job *gryviav1.FabricAIJob) string {
	if team, ok := job.Labels["gryvia.io/team"]; ok && team != "" {
		return team
	}
	return job.Namespace
}

// extractCPUMemory extracts CPU (millicores) and memory (bytes) from a job's
// resource requests.
func extractCPUMemory(job *gryviav1.FabricAIJob) (cpuMillis int64, memBytes int64) {
	if cpu, ok := job.Spec.Resources.Requests[corev1.ResourceCPU]; ok {
		cpuMillis = cpu.MilliValue()
	}
	if mem, ok := job.Spec.Resources.Requests[corev1.ResourceMemory]; ok {
		memBytes = mem.Value()
	}
	return
}
