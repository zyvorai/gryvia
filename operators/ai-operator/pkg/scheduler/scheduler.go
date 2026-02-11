package scheduler

import (
	"context"
	"fmt"
	"sort"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubefabricv1 "github.com/yourusername/kubefabric/operators/ai-operator/api/v1"
)

// NodeScore represents a node with its scheduling score
type NodeScore struct {
	NodeName string
	Score    int
}

// FindOptimalNodes finds the best nodes for running an AI job
func FindOptimalNodes(ctx context.Context, k8sClient client.Client, job *kubefabricv1.FabricAIJob) ([]string, error) {
	// Get all nodes
	nodes := &corev1.NodeList{}
	if err := k8sClient.List(ctx, nodes); err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	// Filter nodes based on job requirements
	eligibleNodes := filterNodes(nodes.Items, job)
	if len(eligibleNodes) == 0 {
		return nil, fmt.Errorf("no nodes meet the job requirements")
	}

	// Score nodes
	scoredNodes := scoreNodes(eligibleNodes, job)

	// Sort by score (highest first)
	sort.Slice(scoredNodes, func(i, j int) bool {
		return scoredNodes[i].Score > scoredNodes[j].Score
	})

	// Determine how many nodes we need
	requiredNodes := 1
	if job.Spec.Distributed != nil && job.Spec.Distributed.Enabled {
		requiredNodes = int(job.Spec.Distributed.Nodes)
	}

	if len(scoredNodes) < requiredNodes {
		return nil, fmt.Errorf("not enough nodes: need %d, found %d", requiredNodes, len(scoredNodes))
	}

	// Select top N nodes
	selectedNodes := make([]string, requiredNodes)
	for i := 0; i < requiredNodes; i++ {
		selectedNodes[i] = scoredNodes[i].NodeName
	}

	return selectedNodes, nil
}

// filterNodes filters nodes based on job requirements
func filterNodes(nodes []corev1.Node, job *kubefabricv1.FabricAIJob) []corev1.Node {
	var eligible []corev1.Node

	for _, node := range nodes {
		// Check if node is ready
		if !isNodeReady(node) {
			continue
		}

		// Check GPU type if specified
		if job.Spec.GpuType != "" && job.Spec.GpuType != "any" {
			if gpuType, exists := node.Labels["kubefabric.ai/gpu"]; !exists || gpuType != job.Spec.GpuType {
				continue
			}
		}

		// Check RDMA requirement
		if job.Spec.Network == "rdma" {
			if rdma, exists := node.Labels["kubefabric.ai/rdma"]; !exists || rdma != "true" {
				continue
			}
		}

		// Check SR-IOV requirement
		if job.Spec.Network == "sriov" {
			if sriov, exists := node.Labels["kubefabric.ai/sriov"]; !exists || sriov != "true" {
				continue
			}
		}

		// Check custom node selector
		if !matchesNodeSelector(node, job.Spec.NodeSelector) {
			continue
		}

		eligible = append(eligible, node)
	}

	return eligible
}

// scoreNodes assigns a score to each node based on various factors
func scoreNodes(nodes []corev1.Node, job *kubefabricv1.FabricAIJob) []NodeScore {
	scored := make([]NodeScore, len(nodes))

	for i, node := range nodes {
		score := 0

		// Prefer nodes with matching GPU type
		if job.Spec.GpuType != "" {
			if gpuType, exists := node.Labels["kubefabric.ai/gpu"]; exists && gpuType == job.Spec.GpuType {
				score += 50
			}
		}

		// Prefer nodes with RDMA if requested
		if job.Spec.Network == "rdma" {
			if rdma, exists := node.Labels["kubefabric.ai/rdma"]; exists && rdma == "true" {
				score += 30
			}
		}

		// Prefer nodes with NVLink/NVSwitch for multi-GPU jobs
		if job.Spec.GPUs > 1 {
			if interconnect, exists := node.Labels["kubefabric.ai/interconnect"]; exists {
				if interconnect == "NVSwitch" {
					score += 40
				} else if interconnect == "NVLink" {
					score += 30
				}
			}
		}

		// Prefer nodes with more available resources
		// (In production, you'd query actual GPU availability)
		score += calculateResourceScore(node)

		scored[i] = NodeScore{
			NodeName: node.Name,
			Score:    score,
		}
	}

	return scored
}

// isNodeReady checks if a node is ready
func isNodeReady(node corev1.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

// matchesNodeSelector checks if node matches the selector
func matchesNodeSelector(node corev1.Node, selector map[string]string) bool {
	if selector == nil {
		return true
	}

	for key, value := range selector {
		if nodeValue, exists := node.Labels[key]; !exists || nodeValue != value {
			return false
		}
	}
	return true
}

// calculateResourceScore calculates a score based on available resources
func calculateResourceScore(node corev1.Node) int {
	// In production, you'd query actual GPU availability from metrics
	// For now, we'll use a simplified score based on node capacity

	score := 0

	// Prefer nodes with more memory
	if memory, ok := node.Status.Allocatable[corev1.ResourceMemory]; ok {
		memoryGB := memory.Value() / (1024 * 1024 * 1024)
		score += int(memoryGB / 100) // 1 point per 100GB
	}

	// Prefer nodes with more CPUs
	if cpu, ok := node.Status.Allocatable[corev1.ResourceCPU]; ok {
		score += int(cpu.Value() / 10) // 1 point per 10 CPUs
	}

	return score
}
