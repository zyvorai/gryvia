package webhook

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	tensorreaperv1 "github.com/ssahani/TensorReaper/operators/ai-operator/api/v1"
)

// FabricAIJobValidator implements a ValidatingWebhook for FabricAIJob.
// It ensures that submitted jobs have valid GPU configurations, that
// requested GPU types exist in the cluster, and that distributed configs
// are consistent.
type FabricAIJobValidator struct {
	Client  client.Client
	decoder admission.Decoder
	log     logr.Logger
}

// NewFabricAIJobValidator creates a new validator.
func NewFabricAIJobValidator(c client.Client) *FabricAIJobValidator {
	return &FabricAIJobValidator{
		Client: c,
		log:    ctrl.Log.WithName("webhook").WithName("validator"),
	}
}

// Handle processes an admission request for FabricAIJob validation.
func (v *FabricAIJobValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	job := &tensorreaperv1.FabricAIJob{}

	if err := v.decoder.Decode(req, job); err != nil {
		v.log.Error(err, "Failed to decode FabricAIJob")
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("failed to decode request: %w", err))
	}

	v.log.Info("Validating FabricAIJob", "name", job.Name, "namespace", job.Namespace)

	// Run all validations.
	if err := v.validateGPUCount(ctx, job); err != nil {
		return admission.Denied(err.Error())
	}

	if err := v.validateGPUType(ctx, job); err != nil {
		return admission.Denied(err.Error())
	}

	if err := v.validateDistributedConfig(job); err != nil {
		return admission.Denied(err.Error())
	}

	if err := v.validateResourceRequests(job); err != nil {
		return admission.Denied(err.Error())
	}

	if err := v.validateImage(job); err != nil {
		return admission.Denied(err.Error())
	}

	if err := v.validatePriority(job); err != nil {
		return admission.Denied(err.Error())
	}

	return admission.Allowed("FabricAIJob is valid")
}

// validateGPUCount checks that the requested GPU count does not exceed any
// single node's capacity in the cluster.
func (v *FabricAIJobValidator) validateGPUCount(ctx context.Context, job *tensorreaperv1.FabricAIJob) error {
	if job.Spec.GPUs <= 0 {
		return fmt.Errorf("spec.gpus must be greater than 0, got %d", job.Spec.GPUs)
	}

	gpusPerNode := job.Spec.GPUs
	if job.Spec.Distributed != nil && job.Spec.Distributed.Enabled && job.Spec.Distributed.GpusPerNode > 0 {
		gpusPerNode = job.Spec.Distributed.GpusPerNode
	}

	// Find the maximum GPU capacity across all nodes.
	nodes := &corev1.NodeList{}
	if err := v.Client.List(ctx, nodes); err != nil {
		v.log.Error(err, "Failed to list nodes for GPU count validation")
		// Don't block admission if we can't reach the API server.
		return nil
	}

	maxNodeGPUs := int32(0)
	for _, node := range nodes.Items {
		nodeGPUs := int32(0)

		// Check allocatable GPU resource.
		if gpuAlloc, ok := node.Status.Allocatable["nvidia.com/gpu"]; ok {
			nodeGPUs = int32(gpuAlloc.Value())
		}

		// Also check the label.
		if countStr, exists := node.Labels["tensorreaper.ai/gpu-count"]; exists {
			if count, err := strconv.ParseInt(countStr, 10, 32); err == nil && int32(count) > nodeGPUs {
				nodeGPUs = int32(count)
			}
		}

		if nodeGPUs > maxNodeGPUs {
			maxNodeGPUs = nodeGPUs
		}
	}

	if maxNodeGPUs > 0 && gpusPerNode > maxNodeGPUs {
		return fmt.Errorf("requested %d GPUs per node exceeds maximum node capacity of %d GPUs; "+
			"consider using distributed training across multiple nodes", gpusPerNode, maxNodeGPUs)
	}

	return nil
}

// validateGPUType checks that the requested GPU type exists on at least one
// node in the cluster.
func (v *FabricAIJobValidator) validateGPUType(ctx context.Context, job *tensorreaperv1.FabricAIJob) error {
	if job.Spec.GpuType == "" || job.Spec.GpuType == "any" {
		return nil
	}

	nodes := &corev1.NodeList{}
	if err := v.Client.List(ctx, nodes); err != nil {
		v.log.Error(err, "Failed to list nodes for GPU type validation")
		return nil // Don't block if API is unreachable.
	}

	// Collect available GPU types for the error message.
	availableTypes := make(map[string]bool)
	for _, node := range nodes.Items {
		if gpuType, exists := node.Labels["tensorreaper.ai/gpu"]; exists {
			availableTypes[gpuType] = true
			if gpuType == job.Spec.GpuType {
				return nil // Found a matching node.
			}
		}
	}

	if len(availableTypes) == 0 {
		return fmt.Errorf("requested GPU type %q, but no nodes have GPU type labels (tensorreaper.ai/gpu); "+
			"ensure nodes are labeled correctly", job.Spec.GpuType)
	}

	available := make([]string, 0, len(availableTypes))
	for t := range availableTypes {
		available = append(available, t)
	}

	return fmt.Errorf("GPU type %q not found in cluster; available types: %v", job.Spec.GpuType, available)
}

// validateDistributedConfig checks that the distributed training configuration
// is internally consistent.
func (v *FabricAIJobValidator) validateDistributedConfig(job *tensorreaperv1.FabricAIJob) error {
	dist := job.Spec.Distributed
	if dist == nil || !dist.Enabled {
		return nil
	}

	if dist.Nodes <= 0 {
		return fmt.Errorf("distributed.nodes must be greater than 0 when distributed training is enabled, got %d", dist.Nodes)
	}

	if dist.GpusPerNode < 0 {
		return fmt.Errorf("distributed.gpusPerNode must be non-negative, got %d", dist.GpusPerNode)
	}

	// Validate framework.
	validFrameworks := map[string]bool{
		"pytorch":    true,
		"tensorflow": true,
		"horovod":    true,
		"deepspeed":  true,
		"megatron":   true,
		"":           true, // Empty is allowed (defaults to pytorch).
	}
	if !validFrameworks[dist.Framework] {
		return fmt.Errorf("distributed.framework %q is not supported; valid values: pytorch, tensorflow, horovod, deepspeed, megatron",
			dist.Framework)
	}

	// Validate backend.
	validBackends := map[string]bool{
		"nccl": true,
		"gloo": true,
		"mpi":  true,
		"":     true,
	}
	if !validBackends[dist.Backend] {
		return fmt.Errorf("distributed.backend %q is not supported; valid values: nccl, gloo, mpi",
			dist.Backend)
	}

	// Sanity: nodes * gpusPerNode should not be unreasonably large.
	gpusPerNode := dist.GpusPerNode
	if gpusPerNode == 0 {
		gpusPerNode = job.Spec.GPUs
	}
	totalGPUs := dist.Nodes * gpusPerNode
	if totalGPUs > 1024 {
		return fmt.Errorf("total GPU count (%d nodes x %d GPUs/node = %d) seems unreasonably large; "+
			"maximum supported is 1024", dist.Nodes, gpusPerNode, totalGPUs)
	}

	return nil
}

// validateResourceRequests checks that resource requests are properly formed.
func (v *FabricAIJobValidator) validateResourceRequests(job *tensorreaperv1.FabricAIJob) error {
	// Validate CPU requests if specified.
	if cpu, ok := job.Spec.Resources.Requests[corev1.ResourceCPU]; ok {
		if cpu.Cmp(resource.MustParse("0")) <= 0 {
			return fmt.Errorf("CPU request must be positive, got %s", cpu.String())
		}
	}

	// Validate memory requests if specified.
	if mem, ok := job.Spec.Resources.Requests[corev1.ResourceMemory]; ok {
		if mem.Cmp(resource.MustParse("0")) <= 0 {
			return fmt.Errorf("memory request must be positive, got %s", mem.String())
		}
	}

	// Check that limits >= requests when both are specified.
	if cpu, ok := job.Spec.Resources.Requests[corev1.ResourceCPU]; ok {
		if cpuLimit, okL := job.Spec.Resources.Limits[corev1.ResourceCPU]; okL {
			if cpuLimit.Cmp(cpu) < 0 {
				return fmt.Errorf("CPU limit (%s) must be >= CPU request (%s)", cpuLimit.String(), cpu.String())
			}
		}
	}

	if mem, ok := job.Spec.Resources.Requests[corev1.ResourceMemory]; ok {
		if memLimit, okL := job.Spec.Resources.Limits[corev1.ResourceMemory]; okL {
			if memLimit.Cmp(mem) < 0 {
				return fmt.Errorf("memory limit (%s) must be >= memory request (%s)", memLimit.String(), mem.String())
			}
		}
	}

	return nil
}

// validateImage checks that an image is specified.
func (v *FabricAIJobValidator) validateImage(job *tensorreaperv1.FabricAIJob) error {
	if job.Spec.Image == "" {
		return fmt.Errorf("spec.image is required")
	}
	return nil
}

// validatePriority checks that priority is within the valid range.
func (v *FabricAIJobValidator) validatePriority(job *tensorreaperv1.FabricAIJob) error {
	if job.Spec.Priority < 0 || job.Spec.Priority > 100 {
		return fmt.Errorf("spec.priority must be between 0 and 100, got %d", job.Spec.Priority)
	}
	return nil
}

// InjectDecoder injects the admission decoder.
func (v *FabricAIJobValidator) InjectDecoder(d admission.Decoder) error {
	v.decoder = d
	return nil
}
