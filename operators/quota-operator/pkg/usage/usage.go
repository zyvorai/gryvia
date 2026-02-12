package usage

import (
	"context"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	kubefabricv1 "github.com/yourusername/kubefabric/operators/quota-operator/api/v1"
)

// CalculateUsage calculates current resource usage for a quota
func CalculateUsage(ctx context.Context, k8sClient client.Client, quota *kubefabricv1.FabricQuota) (*kubefabricv1.QuotaUsage, error) {
	usage := &kubefabricv1.QuotaUsage{}

	// Get all AI jobs in quota namespaces
	totalGPUs := 0
	runningJobs := 0
	queuedJobs := 0
	gpuHours := 0.0

	for _, nsName := range quota.Spec.Namespaces {
		jobs := &kubefabricv1.FabricAIJobList{}
		if err := k8sClient.List(ctx, jobs, client.InNamespace(nsName)); err != nil {
			return nil, err
		}

		for _, job := range jobs.Items {
			switch job.Status.Phase {
			case "Running":
				totalGPUs += job.Spec.Resources.GpuCount
				runningJobs++

				// Calculate GPU hours
				if !job.Status.StartTime.IsZero() {
					duration := time.Since(job.Status.StartTime.Time)
					hours := duration.Hours()
					gpuHours += hours * float64(job.Spec.Resources.GpuCount)
				}

			case "Pending", "Queued":
				queuedJobs++
			}
		}
	}

	usage.AllocatedGPUs = totalGPUs
	usage.RunningJobs = runningJobs
	usage.QueuedJobs = queuedJobs
	usage.GPUHours = gpuHours

	return usage, nil
}

// GetMonthlyGPUHours calculates total GPU hours for the current month
func GetMonthlyGPUHours(ctx context.Context, k8sClient client.Client, quota *kubefabricv1.FabricQuota) (float64, error) {
	// Get start of current month
	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	totalHours := 0.0

	for _, nsName := range quota.Spec.Namespaces {
		jobs := &kubefabricv1.FabricAIJobList{}
		if err := k8sClient.List(ctx, jobs, client.InNamespace(nsName)); err != nil {
			return 0, err
		}

		for _, job := range jobs.Items {
			// Skip jobs not started this month
			if job.Status.StartTime.IsZero() || job.Status.StartTime.Time.Before(startOfMonth) {
				continue
			}

			// Calculate duration
			var duration time.Duration
			if job.Status.CompletionTime.IsZero() {
				// Still running
				duration = time.Since(job.Status.StartTime.Time)
			} else {
				// Completed
				duration = job.Status.CompletionTime.Time.Sub(job.Status.StartTime.Time)
			}

			hours := duration.Hours()
			totalHours += hours * float64(job.Spec.Resources.GpuCount)
		}
	}

	return totalHours, nil
}
