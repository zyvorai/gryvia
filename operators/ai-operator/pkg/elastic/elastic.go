package elastic

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gryviav1 "github.com/zyvorai/gryvia/operators/ai-operator/api/v1"
)

const (
	// Annotations used to communicate elastic config to pods.
	AnnotationMinNodes = "gryvia.io/elastic-min-nodes"
	AnnotationMaxNodes = "gryvia.io/elastic-max-nodes"
	AnnotationElastic  = "gryvia.io/elastic-enabled"

	// Environment variable names injected into pods for torchrun integration.
	EnvMinNodes       = "ELASTIC_MIN_NODES"
	EnvMaxNodes       = "ELASTIC_MAX_NODES"
	EnvRendezvousPort = "TORCHELASTIC_RDZV_PORT"
	EnvRendezvousHost = "TORCHELASTIC_RDZV_HOST"
	EnvRendezvousID   = "TORCHELASTIC_RDZV_ID"
	EnvMaxRestarts    = "TORCHELASTIC_MAX_RESTARTS"

	// Default rendezvous port for torchrun.
	DefaultRendezvousPort = "29400"

	// cooldownPeriod prevents rapid successive scale operations.
	cooldownPeriod = 60 * time.Second
)

// ElasticConfig defines the elastic scaling bounds for a job.
type ElasticConfig struct {
	MinNodes int32 `json:"minNodes"`
	MaxNodes int32 `json:"maxNodes"`
}

// ScaleState tracks the current elastic state of a job.
type ScaleState struct {
	JobName       string
	Namespace     string
	CurrentNodes  int32
	MinNodes      int32
	MaxNodes      int32
	GPUsPerNode   int32
	LastScaleTime time.Time
}

// ElasticScaler monitors GPU availability and elastically scales distributed
// training jobs between their configured min and max node counts.
type ElasticScaler struct {
	client client.Client
	log    logr.Logger
	mu     sync.Mutex
	states map[string]*ScaleState // key = namespace/jobName
}

// NewElasticScaler creates a new ElasticScaler.
func NewElasticScaler(c client.Client) *ElasticScaler {
	return &ElasticScaler{
		client: c,
		log:    ctrl.Log.WithName("elastic-scaler"),
		states: make(map[string]*ScaleState),
	}
}

// Register adds a job to elastic scaling management. If the job's spec does
// not have elastic config (via annotations or the Distributed field), this
// is a no-op.
func (es *ElasticScaler) Register(job *gryviav1.GryviaAIJob) error {
	cfg := getElasticConfig(job)
	if cfg == nil {
		return nil // Not an elastic job.
	}

	es.mu.Lock()
	defer es.mu.Unlock()

	key := fmt.Sprintf("%s/%s", job.Namespace, job.Name)
	currentNodes := int32(1)
	gpusPerNode := job.Spec.GPUs
	if job.Spec.Distributed != nil && job.Spec.Distributed.Enabled {
		if job.Spec.Distributed.Nodes > 0 {
			currentNodes = job.Spec.Distributed.Nodes
		}
		if job.Spec.Distributed.GpusPerNode > 0 {
			gpusPerNode = job.Spec.Distributed.GpusPerNode
		}
	}

	// Clamp the initial node count to the elastic bounds.
	if currentNodes < cfg.MinNodes {
		currentNodes = cfg.MinNodes
	}
	if currentNodes > cfg.MaxNodes {
		currentNodes = cfg.MaxNodes
	}

	es.states[key] = &ScaleState{
		JobName:      job.Name,
		Namespace:    job.Namespace,
		CurrentNodes: currentNodes,
		MinNodes:     cfg.MinNodes,
		MaxNodes:     cfg.MaxNodes,
		GPUsPerNode:  gpusPerNode,
	}

	es.log.Info("Registered elastic job",
		"job", key,
		"minNodes", cfg.MinNodes,
		"maxNodes", cfg.MaxNodes,
		"currentNodes", currentNodes,
	)

	return nil
}

// Unregister removes a job from elastic scaling management.
func (es *ElasticScaler) Unregister(namespace, jobName string) {
	es.mu.Lock()
	defer es.mu.Unlock()
	delete(es.states, fmt.Sprintf("%s/%s", namespace, jobName))
}

// ScaleUp expands a running job to use additional GPUs by adding more nodes.
// The additionalGPUs parameter is interpreted as additional nodes worth of
// GPUs (additionalGPUs / gpusPerNode).
func (es *ElasticScaler) ScaleUp(ctx context.Context, job *gryviav1.GryviaAIJob, additionalGPUs int32) error {
	es.mu.Lock()
	key := fmt.Sprintf("%s/%s", job.Namespace, job.Name)
	state, exists := es.states[key]
	if !exists {
		es.mu.Unlock()
		return fmt.Errorf("job %s is not registered for elastic scaling", key)
	}

	// Enforce cooldown.
	if !state.LastScaleTime.IsZero() && time.Since(state.LastScaleTime) < cooldownPeriod {
		es.mu.Unlock()
		return fmt.Errorf("scale cooldown in effect for %s; last scale was %v ago",
			key, time.Since(state.LastScaleTime).Round(time.Second))
	}

	gpusPerNode := state.GPUsPerNode
	if gpusPerNode <= 0 {
		gpusPerNode = 1
	}
	additionalNodes := additionalGPUs / gpusPerNode
	if additionalNodes <= 0 {
		additionalNodes = 1
	}

	newCount := state.CurrentNodes + additionalNodes
	if newCount > state.MaxNodes {
		newCount = state.MaxNodes
	}

	if newCount == state.CurrentNodes {
		es.mu.Unlock()
		return fmt.Errorf("job %s already at max nodes (%d)", key, state.MaxNodes)
	}

	state.CurrentNodes = newCount
	state.LastScaleTime = time.Now()
	es.mu.Unlock()

	es.log.Info("Scaling up elastic job",
		"job", key,
		"newNodes", newCount,
		"additionalGPUs", additionalGPUs,
	)

	return es.applyScale(ctx, job, newCount)
}

// ScaleDown gracefully shrinks a running job by releasing some nodes.
// The releaseGPUs parameter is interpreted as nodes worth of GPUs to release.
func (es *ElasticScaler) ScaleDown(ctx context.Context, job *gryviav1.GryviaAIJob, releaseGPUs int32) error {
	es.mu.Lock()
	key := fmt.Sprintf("%s/%s", job.Namespace, job.Name)
	state, exists := es.states[key]
	if !exists {
		es.mu.Unlock()
		return fmt.Errorf("job %s is not registered for elastic scaling", key)
	}

	// Enforce cooldown.
	if !state.LastScaleTime.IsZero() && time.Since(state.LastScaleTime) < cooldownPeriod {
		es.mu.Unlock()
		return fmt.Errorf("scale cooldown in effect for %s; last scale was %v ago",
			key, time.Since(state.LastScaleTime).Round(time.Second))
	}

	gpusPerNode := state.GPUsPerNode
	if gpusPerNode <= 0 {
		gpusPerNode = 1
	}
	releaseNodes := releaseGPUs / gpusPerNode
	if releaseNodes <= 0 {
		releaseNodes = 1
	}

	newCount := state.CurrentNodes - releaseNodes
	if newCount < state.MinNodes {
		newCount = state.MinNodes
	}

	if newCount == state.CurrentNodes {
		es.mu.Unlock()
		return fmt.Errorf("job %s already at min nodes (%d)", key, state.MinNodes)
	}

	state.CurrentNodes = newCount
	state.LastScaleTime = time.Now()
	es.mu.Unlock()

	es.log.Info("Scaling down elastic job",
		"job", key,
		"newNodes", newCount,
		"releasedGPUs", releaseGPUs,
	)

	return es.applyScale(ctx, job, newCount)
}

// applyScale updates the StatefulSet replica count and relevant environment
// variables to match the new node count.
func (es *ElasticScaler) applyScale(ctx context.Context, job *gryviav1.GryviaAIJob, newNodeCount int32) error {
	stsName := fmt.Sprintf("%s-training", job.Name)

	sts := &appsv1.StatefulSet{}
	if err := es.client.Get(ctx, types.NamespacedName{
		Namespace: job.Namespace,
		Name:      stsName,
	}, sts); err != nil {
		return fmt.Errorf("failed to get StatefulSet %s: %w", stsName, err)
	}

	// Update replica count.
	sts.Spec.Replicas = &newNodeCount

	// Update WORLD_SIZE environment variable in the trainer container.
	if len(sts.Spec.Template.Spec.Containers) > 0 {
		container := &sts.Spec.Template.Spec.Containers[0]
		gpusPerNode := job.Spec.GPUs
		if job.Spec.Distributed != nil && job.Spec.Distributed.GpusPerNode > 0 {
			gpusPerNode = job.Spec.Distributed.GpusPerNode
		}
		worldSize := newNodeCount * gpusPerNode

		updated := false
		for i, env := range container.Env {
			if env.Name == "WORLD_SIZE" {
				container.Env[i].Value = fmt.Sprintf("%d", worldSize)
				updated = true
				break
			}
		}
		if !updated {
			container.Env = append(container.Env, corev1.EnvVar{
				Name:  "WORLD_SIZE",
				Value: fmt.Sprintf("%d", worldSize),
			})
		}

		// Ensure elastic-specific environment variables are present.
		es.ensureElasticEnvVars(container, job, newNodeCount)
	}

	// Update annotations to reflect new elastic state.
	if sts.Spec.Template.Annotations == nil {
		sts.Spec.Template.Annotations = make(map[string]string)
	}
	sts.Spec.Template.Annotations[AnnotationElastic] = "true"
	sts.Spec.Template.Annotations["gryvia.io/elastic-current-nodes"] = fmt.Sprintf("%d", newNodeCount)

	if err := es.client.Update(ctx, sts); err != nil {
		return fmt.Errorf("failed to update StatefulSet %s: %w", stsName, err)
	}

	// Update the GryviaAIJob status.
	job.Status.GpusAllocated = newNodeCount * job.Spec.GPUs
	if job.Spec.Distributed != nil && job.Spec.Distributed.GpusPerNode > 0 {
		job.Status.GpusAllocated = newNodeCount * job.Spec.Distributed.GpusPerNode
	}

	return nil
}

// ensureElasticEnvVars injects or updates PyTorch Elastic (torchrun)
// environment variables in the container.
func (es *ElasticScaler) ensureElasticEnvVars(container *corev1.Container, job *gryviav1.GryviaAIJob, currentNodes int32) {
	cfg := getElasticConfig(job)
	if cfg == nil {
		return
	}

	elasticVars := map[string]string{
		EnvMinNodes:       fmt.Sprintf("%d", cfg.MinNodes),
		EnvMaxNodes:       fmt.Sprintf("%d", cfg.MaxNodes),
		EnvRendezvousPort: DefaultRendezvousPort,
		EnvRendezvousHost: fmt.Sprintf("%s-training-0.%s-headless", job.Name, job.Name),
		EnvRendezvousID:   fmt.Sprintf("%s-%s", job.Namespace, job.Name),
		EnvMaxRestarts:    "3",
	}

	for name, value := range elasticVars {
		found := false
		for i, env := range container.Env {
			if env.Name == name {
				container.Env[i].Value = value
				found = true
				break
			}
		}
		if !found {
			container.Env = append(container.Env, corev1.EnvVar{
				Name:  name,
				Value: value,
			})
		}
	}
}

// EvaluateScaling checks GPU availability and determines whether any elastic
// jobs should be scaled up or down. Returns a list of scaling recommendations.
func (es *ElasticScaler) EvaluateScaling(ctx context.Context) ([]ScaleRecommendation, error) {
	// Get available GPUs across the cluster.
	nodes := &corev1.NodeList{}
	if err := es.client.List(ctx, nodes); err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	pods := &corev1.PodList{}
	if err := es.client.List(ctx, pods); err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	// Calculate total available GPUs.
	totalAvailable := int32(0)
	for _, node := range nodes.Items {
		if !isNodeReady(node) {
			continue
		}
		nodeAvail := getNodeAvailableGPUs(node, pods.Items)
		if nodeAvail > 0 {
			totalAvailable += int32(nodeAvail)
		}
	}

	es.mu.Lock()
	defer es.mu.Unlock()

	var recommendations []ScaleRecommendation

	for key, state := range es.states {
		// Cooldown check.
		if !state.LastScaleTime.IsZero() && time.Since(state.LastScaleTime) < cooldownPeriod {
			continue
		}

		if totalAvailable >= state.GPUsPerNode && state.CurrentNodes < state.MaxNodes {
			// There are free GPUs -- recommend scale up.
			additionalNodes := totalAvailable / state.GPUsPerNode
			newCount := state.CurrentNodes + additionalNodes
			if newCount > state.MaxNodes {
				newCount = state.MaxNodes
			}
			if newCount > state.CurrentNodes {
				recommendations = append(recommendations, ScaleRecommendation{
					JobKey:    key,
					Direction: ScaleDirectionUp,
					FromNodes: state.CurrentNodes,
					ToNodes:   newCount,
					Reason:    fmt.Sprintf("%d GPUs available, can expand from %d to %d nodes", totalAvailable, state.CurrentNodes, newCount),
				})
				// Reserve the GPUs for this recommendation.
				totalAvailable -= (newCount - state.CurrentNodes) * state.GPUsPerNode
			}
		}
	}

	return recommendations, nil
}

// GetState returns the current elastic state for a job, or nil if not registered.
func (es *ElasticScaler) GetState(namespace, jobName string) *ScaleState {
	es.mu.Lock()
	defer es.mu.Unlock()

	key := fmt.Sprintf("%s/%s", namespace, jobName)
	if state, exists := es.states[key]; exists {
		cpy := *state
		return &cpy
	}
	return nil
}

// ScaleDirection indicates whether to scale up or down.
type ScaleDirection string

const (
	ScaleDirectionUp   ScaleDirection = "up"
	ScaleDirectionDown ScaleDirection = "down"
)

// ScaleRecommendation represents a suggested scaling action.
type ScaleRecommendation struct {
	JobKey    string
	Direction ScaleDirection
	FromNodes int32
	ToNodes   int32
	Reason    string
}

// BuildElasticPodTemplate augments a pod template with elastic training
// environment variables and annotations. This should be called from the
// controller when building the StatefulSet for an elastic job.
func BuildElasticPodTemplate(template *corev1.PodTemplateSpec, job *gryviav1.GryviaAIJob) {
	cfg := getElasticConfig(job)
	if cfg == nil {
		return
	}

	// Set annotations.
	if template.Annotations == nil {
		template.Annotations = make(map[string]string)
	}
	template.Annotations[AnnotationElastic] = "true"
	template.Annotations[AnnotationMinNodes] = fmt.Sprintf("%d", cfg.MinNodes)
	template.Annotations[AnnotationMaxNodes] = fmt.Sprintf("%d", cfg.MaxNodes)

	// Inject environment variables into the first container.
	if len(template.Spec.Containers) > 0 {
		container := &template.Spec.Containers[0]

		elasticVars := []corev1.EnvVar{
			{Name: EnvMinNodes, Value: fmt.Sprintf("%d", cfg.MinNodes)},
			{Name: EnvMaxNodes, Value: fmt.Sprintf("%d", cfg.MaxNodes)},
			{Name: EnvRendezvousPort, Value: DefaultRendezvousPort},
			{Name: EnvRendezvousHost, Value: fmt.Sprintf("%s-training-0.%s-headless", job.Name, job.Name)},
			{Name: EnvRendezvousID, Value: fmt.Sprintf("%s-%s", job.Namespace, job.Name)},
			{Name: EnvMaxRestarts, Value: "3"},
		}

		for _, ev := range elasticVars {
			found := false
			for i, existing := range container.Env {
				if existing.Name == ev.Name {
					container.Env[i].Value = ev.Value
					found = true
					break
				}
			}
			if !found {
				container.Env = append(container.Env, ev)
			}
		}
	}
}

// getElasticConfig extracts elastic configuration from a GryviaAIJob.
// It checks annotations first, then falls back to the Distributed spec.
func getElasticConfig(job *gryviav1.GryviaAIJob) *ElasticConfig {
	// Check annotations.
	if job.Annotations != nil {
		minStr, hasMin := job.Annotations[AnnotationMinNodes]
		maxStr, hasMax := job.Annotations[AnnotationMaxNodes]
		if hasMin && hasMax {
			minN, errMin := strconv.ParseInt(minStr, 10, 32)
			maxN, errMax := strconv.ParseInt(maxStr, 10, 32)
			if errMin == nil && errMax == nil && minN > 0 && maxN >= minN {
				return &ElasticConfig{
					MinNodes: int32(minN),
					MaxNodes: int32(maxN),
				}
			}
		}
	}

	// Check if the distributed config implies elastic scaling
	// (i.e., the number of nodes can be adjusted).
	if job.Spec.Distributed != nil && job.Spec.Distributed.Enabled && job.Spec.Distributed.Nodes > 1 {
		// If no explicit elastic config, allow scaling from half to full.
		minNodes := job.Spec.Distributed.Nodes / 2
		if minNodes < 1 {
			minNodes = 1
		}
		return &ElasticConfig{
			MinNodes: minNodes,
			MaxNodes: job.Spec.Distributed.Nodes,
		}
	}

	return nil
}

// isNodeReady checks if a node condition indicates readiness.
func isNodeReady(node corev1.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

// getNodeAvailableGPUs calculates available GPUs on a node by subtracting
// GPU requests from non-terminal pods.
func getNodeAvailableGPUs(node corev1.Node, pods []corev1.Pod) int64 {
	var totalGPUs int64
	gpuResource := corev1.ResourceName("nvidia.com/gpu")

	if gpuAlloc, ok := node.Status.Allocatable[gpuResource]; ok {
		totalGPUs = gpuAlloc.Value()
	}
	// Also check the label.
	if totalGPUs == 0 {
		if countStr, exists := node.Labels["gryvia.io/gpu-count"]; exists {
			if count, err := strconv.ParseInt(countStr, 10, 64); err == nil {
				totalGPUs = count
			}
		}
	}

	var usedGPUs int64
	for _, pod := range pods {
		if pod.Spec.NodeName != node.Name {
			continue
		}
		if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
			continue
		}
		for _, c := range pod.Spec.Containers {
			if req, ok := c.Resources.Requests[gpuResource]; ok {
				usedGPUs += req.Value()
			} else if lim, ok := c.Resources.Limits[gpuResource]; ok {
				usedGPUs += lim.Value()
			}
		}
	}

	available := totalGPUs - usedGPUs
	if available < 0 {
		return 0
	}
	return available
}

// NewElasticResourceRequirements creates resource requirements with GPU limits
// for an elastic job, used when building or updating StatefulSets.
func NewElasticResourceRequirements(gpusPerNode int32, baseResources corev1.ResourceRequirements) corev1.ResourceRequirements {
	result := baseResources.DeepCopy()
	if result.Limits == nil {
		result.Limits = corev1.ResourceList{}
	}
	result.Limits["nvidia.com/gpu"] = *resource.NewQuantity(int64(gpusPerNode), resource.DecimalSI)
	return *result
}

// IsElasticJob returns true if the job is configured for elastic scaling.
func IsElasticJob(job *gryviav1.GryviaAIJob) bool {
	return getElasticConfig(job) != nil
}

// GetElasticBounds returns the min/max nodes for an elastic job.
// Returns (0, 0) if the job is not elastic.
func GetElasticBounds(job *gryviav1.GryviaAIJob) (minNodes, maxNodes int32) {
	cfg := getElasticConfig(job)
	if cfg == nil {
		return 0, 0
	}
	return cfg.MinNodes, cfg.MaxNodes
}

// ReconcileElasticAnnotations ensures the job's annotations reflect the
// current elastic state. Called during controller reconciliation.
func ReconcileElasticAnnotations(job *gryviav1.GryviaAIJob) {
	cfg := getElasticConfig(job)
	if cfg == nil {
		return
	}

	if job.Annotations == nil {
		job.Annotations = make(map[string]string)
	}
	job.Annotations[AnnotationElastic] = "true"
	job.Annotations[AnnotationMinNodes] = fmt.Sprintf("%d", cfg.MinNodes)
	job.Annotations[AnnotationMaxNodes] = fmt.Sprintf("%d", cfg.MaxNodes)

	// Store in status for visibility.
	if job.Status.Message == "" {
		job.Status.Message = fmt.Sprintf("Elastic: min=%d max=%d nodes", cfg.MinNodes, cfg.MaxNodes)
	}
}

// NewElasticStatefulSetMeta returns ObjectMeta suitable for an elastic
// StatefulSet, including labels and annotations for elastic management.
func NewElasticStatefulSetMeta(job *gryviav1.GryviaAIJob) metav1.ObjectMeta {
	labels := map[string]string{
		"gryvia.io/job":     job.Name,
		"gryvia.io/type":    job.Spec.Type,
		"gryvia.io/elastic": "true",
	}

	annotations := map[string]string{
		AnnotationElastic: "true",
	}

	cfg := getElasticConfig(job)
	if cfg != nil {
		annotations[AnnotationMinNodes] = fmt.Sprintf("%d", cfg.MinNodes)
		annotations[AnnotationMaxNodes] = fmt.Sprintf("%d", cfg.MaxNodes)
	}

	return metav1.ObjectMeta{
		Name:        fmt.Sprintf("%s-training", job.Name),
		Namespace:   job.Namespace,
		Labels:      labels,
		Annotations: annotations,
	}
}
