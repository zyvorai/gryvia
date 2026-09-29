package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	gryviav1 "github.com/zyvorai/gryvia/operators/gpu-operator/api/v1"
)

const (
	benchmarkStatePending   = "pending"
	benchmarkStateRunning   = "running"
	benchmarkStateCompleted = "completed"
	benchmarkStateFailed    = "failed"
	benchmarkStateRegressed = "regressed"
)

// GryviaBenchmarkReconciler reconciles a GryviaBenchmark object
type GryviaBenchmarkReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

//+kubebuilder:rbac:groups=gryvia.io,resources=gryviabenchmarks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=gryvia.io,resources=gryviabenchmarks/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=gryvia.io,resources=gryviabenchmarks/finalizers,verbs=update
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=pods/log,verbs=get

// Reconcile is part of the main kubernetes reconciliation loop
func (r *GryviaBenchmarkReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("gryviabenchmark", req.NamespacedName)

	// Fetch the GryviaBenchmark instance
	benchmark := &gryviav1.GryviaBenchmark{}
	err := r.Get(ctx, req.NamespacedName, benchmark)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("GryviaBenchmark resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get GryviaBenchmark")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !benchmark.ObjectMeta.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	// Initialize status if needed
	if benchmark.Status.State == "" {
		benchmark.Status.State = benchmarkStatePending
		if err := r.Status().Update(ctx, benchmark); err != nil {
			log.Error(err, "Failed to update initial status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Skip reconciliation for completed/failed benchmarks
	if benchmark.Status.State == benchmarkStateCompleted ||
		benchmark.Status.State == benchmarkStateFailed ||
		benchmark.Status.State == benchmarkStateRegressed {
		return ctrl.Result{}, nil
	}

	// Reconcile the benchmark
	result, err := r.reconcileBenchmark(ctx, benchmark)
	if err != nil {
		log.Error(err, "Failed to reconcile benchmark")
		return result, err
	}

	return result, nil
}

func (r *GryviaBenchmarkReconciler) reconcileBenchmark(ctx context.Context, benchmark *gryviav1.GryviaBenchmark) (ctrl.Result, error) {
	log := r.Log.WithValues("gryviabenchmark", benchmark.Name)

	// Phase 1: Create the benchmark job
	if benchmark.Status.State == benchmarkStatePending {
		if err := r.ensureBenchmarkJob(ctx, benchmark); err != nil {
			log.Error(err, "Failed to create benchmark job")
			return ctrl.Result{RequeueAfter: 10 * time.Second}, err
		}

		benchmark.Status.State = benchmarkStateRunning
		now := metav1.Now()
		benchmark.Status.StartTime = &now

		if err := r.Status().Update(ctx, benchmark); err != nil {
			return ctrl.Result{}, err
		}

		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// Phase 2: Monitor the benchmark job
	if benchmark.Status.State == benchmarkStateRunning {
		jobName := r.getBenchmarkJobName(benchmark)
		job := &batchv1.Job{}
		err := r.Get(ctx, types.NamespacedName{
			Namespace: benchmark.Namespace,
			Name:      jobName,
		}, job)
		if err != nil {
			if errors.IsNotFound(err) {
				log.Info("Benchmark job not found, recreating")
				benchmark.Status.State = benchmarkStatePending
				if updateErr := r.Status().Update(ctx, benchmark); updateErr != nil {
					return ctrl.Result{}, updateErr
				}
				return ctrl.Result{Requeue: true}, nil
			}
			return ctrl.Result{}, err
		}

		// Check job completion
		if job.Status.Succeeded > 0 {
			log.Info("Benchmark job completed successfully")
			now := metav1.Now()
			benchmark.Status.CompletionTime = &now

			// Calculate duration
			if benchmark.Status.StartTime != nil {
				duration := now.Time.Sub(benchmark.Status.StartTime.Time)
				benchmark.Status.Duration = formatDuration(duration)
			}

			// Collect results (in a real implementation, parse job logs)
			benchmark.Status.Results = r.collectResults(benchmark)

			// Collect hardware info
			benchmark.Status.Hardware = r.collectHardwareInfo(benchmark)

			// Compare against baseline
			if benchmark.Spec.Baseline != nil && benchmark.Spec.Baseline.Enabled {
				comparison := r.compareBaseline(benchmark)
				benchmark.Status.Comparison = comparison

				if comparison.Regressed > 0 {
					benchmark.Status.State = benchmarkStateRegressed
				} else {
					benchmark.Status.State = benchmarkStateCompleted
				}
			} else {
				benchmark.Status.State = benchmarkStateCompleted
			}

			if err := r.Status().Update(ctx, benchmark); err != nil {
				return ctrl.Result{}, err
			}

			return ctrl.Result{}, nil
		}

		if job.Status.Failed > 0 {
			log.Info("Benchmark job failed")
			now := metav1.Now()
			benchmark.Status.CompletionTime = &now
			benchmark.Status.State = benchmarkStateFailed

			if benchmark.Status.StartTime != nil {
				duration := now.Time.Sub(benchmark.Status.StartTime.Time)
				benchmark.Status.Duration = formatDuration(duration)
			}

			if err := r.Status().Update(ctx, benchmark); err != nil {
				return ctrl.Result{}, err
			}

			return ctrl.Result{}, nil
		}

		// Job still running
		return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
	}

	return ctrl.Result{}, nil
}

func (r *GryviaBenchmarkReconciler) ensureBenchmarkJob(ctx context.Context, benchmark *gryviav1.GryviaBenchmark) error {
	jobName := r.getBenchmarkJobName(benchmark)

	// Check if job already exists
	existingJob := &batchv1.Job{}
	err := r.Get(ctx, types.NamespacedName{
		Namespace: benchmark.Namespace,
		Name:      jobName,
	}, existingJob)
	if err == nil {
		return nil // already exists
	}
	if !errors.IsNotFound(err) {
		return err
	}

	// Build the benchmark job based on type
	job := r.buildBenchmarkJob(benchmark, jobName)

	if err := controllerutil.SetControllerReference(benchmark, job, r.Scheme); err != nil {
		return err
	}

	return r.Create(ctx, job)
}

func (r *GryviaBenchmarkReconciler) buildBenchmarkJob(benchmark *gryviav1.GryviaBenchmark, jobName string) *batchv1.Job {
	var backoffLimit int32
	gpuCount := benchmark.Spec.Target.GpuCount
	if gpuCount <= 0 {
		gpuCount = 1
	}

	container := r.buildBenchmarkContainer(benchmark, gpuCount)

	nodeSelector := make(map[string]string)
	if benchmark.Spec.Target.GpuType != "" {
		nodeSelector["gryvia.io/gpu"] = benchmark.Spec.Target.GpuType
	}
	for k, v := range benchmark.Spec.Target.NodeSelector {
		nodeSelector[k] = v
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: benchmark.Namespace,
			Labels: map[string]string{
				"gryvia.io/benchmark": benchmark.Name,
				"gryvia.io/type":      benchmark.Spec.Type,
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoffLimit,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"gryvia.io/benchmark": benchmark.Name,
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers:    []corev1.Container{container},
					NodeSelector:  nodeSelector,
				},
			},
		},
	}

	return job
}

func (r *GryviaBenchmarkReconciler) buildBenchmarkContainer(benchmark *gryviav1.GryviaBenchmark, gpuCount int32) corev1.Container {
	var image string
	var command []string
	var args []string

	switch benchmark.Spec.Type {
	case "mlperf":
		image = "nvcr.io/nvidia/pytorch:24.01-py3"
		command = []string{"python", "-m", "mlperf.train"}
		if benchmark.Spec.MLPerf != nil {
			if benchmark.Spec.MLPerf.Workload != "" {
				args = append(args, "--workload="+benchmark.Spec.MLPerf.Workload)
			}
			if benchmark.Spec.MLPerf.BatchSize > 0 {
				args = append(args, fmt.Sprintf("--batch-size=%d", benchmark.Spec.MLPerf.BatchSize))
			}
			if benchmark.Spec.MLPerf.Precision != "" {
				args = append(args, "--precision="+benchmark.Spec.MLPerf.Precision)
			}
		}

	case "nccl":
		image = "nvcr.io/nvidia/pytorch:24.01-py3"
		command = []string{"/usr/bin/all_reduce_perf"}
		if benchmark.Spec.NCCL != nil {
			if benchmark.Spec.NCCL.Iterations > 0 {
				args = append(args, fmt.Sprintf("-n=%d", benchmark.Spec.NCCL.Iterations))
			}
			if benchmark.Spec.NCCL.WarmupIters > 0 {
				args = append(args, fmt.Sprintf("-w=%d", benchmark.Spec.NCCL.WarmupIters))
			}
		}

	case "gpu-memory":
		image = "nvcr.io/nvidia/cuda:12.3.1-devel-ubuntu22.04"
		command = []string{"/usr/local/cuda/extras/demo_suite/bandwidthTest"}
		args = []string{"--device=all"}

	case "io-throughput":
		image = "nixery.dev/shell/fio"
		command = []string{"fio"}
		if benchmark.Spec.IOThroughput != nil {
			args = append(args, "--name=benchmark")
			if benchmark.Spec.IOThroughput.FileSize != "" {
				args = append(args, "--size="+benchmark.Spec.IOThroughput.FileSize)
			}
			if benchmark.Spec.IOThroughput.BlockSize != "" {
				args = append(args, "--bs="+benchmark.Spec.IOThroughput.BlockSize)
			}
			if benchmark.Spec.IOThroughput.Threads > 0 {
				args = append(args, fmt.Sprintf("--numjobs=%d", benchmark.Spec.IOThroughput.Threads))
			}
			args = append(args, "--output-format=json")
		}

	case "custom":
		if benchmark.Spec.Custom != nil {
			image = benchmark.Spec.Custom.Image
			command = benchmark.Spec.Custom.Command
			args = benchmark.Spec.Custom.Args
		} else {
			image = "busybox"
			command = []string{"echo", "no custom config provided"}
		}

	default:
		image = "busybox"
		command = []string{"echo", "unsupported benchmark type"}
	}

	container := corev1.Container{
		Name:    "benchmark",
		Image:   image,
		Command: command,
		Args:    args,
		Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				"nvidia.com/gpu": *resource.NewQuantity(int64(gpuCount), resource.DecimalSI),
			},
		},
	}

	return container
}

func (r *GryviaBenchmarkReconciler) collectResults(benchmark *gryviav1.GryviaBenchmark) map[string]gryviav1.BenchmarkResultValue {
	results := make(map[string]gryviav1.BenchmarkResultValue)

	// In a real implementation, these would be parsed from job logs.
	// For now, return placeholder structure based on benchmark type.
	switch benchmark.Spec.Type {
	case "mlperf":
		results["throughput"] = gryviav1.BenchmarkResultValue{
			Value: 0,
			Unit:  "images/sec",
		}
	case "nccl":
		results["bandwidth"] = gryviav1.BenchmarkResultValue{
			Value: 0,
			Unit:  "GB/s",
		}
	case "gpu-memory":
		results["h2d_bandwidth"] = gryviav1.BenchmarkResultValue{
			Value: 0,
			Unit:  "GB/s",
		}
		results["d2h_bandwidth"] = gryviav1.BenchmarkResultValue{
			Value: 0,
			Unit:  "GB/s",
		}
	case "io-throughput":
		results["read_throughput"] = gryviav1.BenchmarkResultValue{
			Value: 0,
			Unit:  "MB/s",
		}
		results["write_throughput"] = gryviav1.BenchmarkResultValue{
			Value: 0,
			Unit:  "MB/s",
		}
	}

	return results
}

func (r *GryviaBenchmarkReconciler) collectHardwareInfo(benchmark *gryviav1.GryviaBenchmark) *gryviav1.BenchmarkHardwareInfo {
	return &gryviav1.BenchmarkHardwareInfo{
		GpuModel: benchmark.Spec.Target.GpuType,
		GpuCount: benchmark.Spec.Target.GpuCount,
	}
}

func (r *GryviaBenchmarkReconciler) compareBaseline(benchmark *gryviav1.GryviaBenchmark) *gryviav1.BenchmarkComparison {
	comparison := &gryviav1.BenchmarkComparison{}

	if benchmark.Spec.Baseline == nil || benchmark.Status.Results == nil {
		return comparison
	}

	threshold := benchmark.Spec.Baseline.RegressionThreshold
	if threshold <= 0 {
		threshold = 5
	}

	for metricName, baselineValue := range benchmark.Spec.Baseline.Values {
		result, ok := benchmark.Status.Results[metricName]
		if !ok {
			continue
		}

		if baselineValue == 0 {
			comparison.Stable++
			continue
		}

		changePercent := ((result.Value - baselineValue) / baselineValue) * 100

		if changePercent < -threshold {
			comparison.Regressed++
		} else if changePercent > threshold {
			comparison.Improved++
		} else {
			comparison.Stable++
		}
	}

	return comparison
}

func (r *GryviaBenchmarkReconciler) getBenchmarkJobName(benchmark *gryviav1.GryviaBenchmark) string {
	return fmt.Sprintf("%s-bench", benchmark.Name)
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh%dm", hours, minutes)
}

// SetupWithManager sets up the controller with the Manager.
func (r *GryviaBenchmarkReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gryviav1.GryviaBenchmark{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}
