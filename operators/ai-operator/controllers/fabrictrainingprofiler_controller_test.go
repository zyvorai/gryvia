package controllers

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	tensorreaperv1 "github.com/ssahani/TensorReaper/operators/ai-operator/api/v1"
)

func newProfilerTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = tensorreaperv1.AddToScheme(s)
	_ = corev1.AddToScheme(s)
	return s
}

func newProfilerReconciler(objs ...client.Object) (*FabricTrainingProfilerReconciler, client.Client) {
	scheme := newProfilerTestScheme()
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		WithStatusSubresource(&tensorreaperv1.FabricTrainingProfiler{}, &tensorreaperv1.FabricAIJob{}).
		Build()
	r := &FabricTrainingProfilerReconciler{
		Client: fakeClient,
		Scheme: scheme,
		Log:    ctrl.Log.WithName("test"),
	}
	return r, fakeClient
}

func newTestProfiler(name, namespace string) *tensorreaperv1.FabricTrainingProfiler {
	return &tensorreaperv1.FabricTrainingProfiler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: tensorreaperv1.FabricTrainingProfilerSpec{
			Target: tensorreaperv1.ProfilerTarget{
				Type:        "auto",
				JobSelector: map[string]string{"team": "ml"},
			},
			Output: tensorreaperv1.ProfilerOutput{
				StoreInJobStatus:  true,
				EmitEvents:        false,
				PrometheusMetrics: true,
			},
		},
	}
}

func TestProfiler_Reconcile_NotFound(t *testing.T) {
	r, _ := newProfilerReconciler()

	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "nonexistent", Namespace: "default"}}
	result, err := r.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error for not-found, got %v", err)
	}
	if result.Requeue || result.RequeueAfter > 0 {
		t.Errorf("expected no requeue for not-found resource")
	}
}

func TestProfiler_Reconcile_DeletionTimestamp(t *testing.T) {
	now := metav1.Now()
	fp := newTestProfiler("test-profiler", "default")
	fp.DeletionTimestamp = &now
	fp.Finalizers = []string{"keep-for-test"}

	r, _ := newProfilerReconciler(fp)

	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "test-profiler", Namespace: "default"}}
	result, err := r.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error during deletion, got %v", err)
	}
	if result.Requeue || result.RequeueAfter > 0 {
		t.Error("expected no requeue for deleted resource")
	}
}

func TestProfiler_Reconcile_NoMatchingJobs(t *testing.T) {
	fp := newTestProfiler("test-profiler", "default")
	r, fakeClient := newProfilerReconciler(fp)

	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "test-profiler", Namespace: "default"}}
	result, err := r.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error with no matching jobs, got %v", err)
	}
	if result.RequeueAfter != profilerRequeueInterval {
		t.Errorf("expected requeue after %v, got %v", profilerRequeueInterval, result.RequeueAfter)
	}

	updated := &tensorreaperv1.FabricTrainingProfiler{}
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "test-profiler", Namespace: "default"}, updated); err != nil {
		t.Fatalf("failed to get updated profiler: %v", err)
	}
	// Should have Ready condition with NoJobs reason
	found := false
	for _, cond := range updated.Status.Conditions {
		if cond.Type == "Ready" && cond.Reason == "NoJobs" {
			found = true
		}
	}
	if !found {
		t.Error("expected Ready condition with NoJobs reason")
	}
}

func TestProfiler_FindMatchingJobs_JobRef(t *testing.T) {
	job := &tensorreaperv1.FabricAIJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "target-job",
			Namespace: "default",
		},
		Spec: tensorreaperv1.FabricAIJobSpec{
			Type:  "training",
			Image: "pytorch:latest",
			GPUs:  4,
		},
	}

	fp := &tensorreaperv1.FabricTrainingProfiler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-profiler",
			Namespace: "default",
		},
		Spec: tensorreaperv1.FabricTrainingProfilerSpec{
			Target: tensorreaperv1.ProfilerTarget{
				Type:   "job-ref",
				JobRef: "target-job",
			},
		},
	}

	r, _ := newProfilerReconciler(job, fp)

	jobs, err := r.findMatchingJobs(context.Background(), fp)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].Name != "target-job" {
		t.Errorf("expected job target-job, got %s", jobs[0].Name)
	}
}

func TestProfiler_FindMatchingJobs_JobRefNotFound(t *testing.T) {
	fp := &tensorreaperv1.FabricTrainingProfiler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-profiler",
			Namespace: "default",
		},
		Spec: tensorreaperv1.FabricTrainingProfilerSpec{
			Target: tensorreaperv1.ProfilerTarget{
				Type:   "job-ref",
				JobRef: "nonexistent",
			},
		},
	}

	r, _ := newProfilerReconciler(fp)

	jobs, err := r.findMatchingJobs(context.Background(), fp)
	if err != nil {
		t.Fatalf("expected no error for not-found job-ref, got %v", err)
	}
	if len(jobs) != 0 {
		t.Errorf("expected 0 jobs for not-found job-ref, got %d", len(jobs))
	}
}

func TestProfiler_FindMatchingJobs_EmptyJobRef(t *testing.T) {
	fp := &tensorreaperv1.FabricTrainingProfiler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-profiler",
			Namespace: "default",
		},
		Spec: tensorreaperv1.FabricTrainingProfilerSpec{
			Target: tensorreaperv1.ProfilerTarget{
				Type:   "job-ref",
				JobRef: "",
			},
		},
	}

	r, _ := newProfilerReconciler(fp)

	_, err := r.findMatchingJobs(context.Background(), fp)
	if err == nil {
		t.Error("expected error for empty job-ref")
	}
}

func TestProfiler_ShouldProfileJob_WarmupNotReached(t *testing.T) {
	fp := newTestProfiler("test-profiler", "default")
	fp.Spec.Target.AutoProfile = &tensorreaperv1.AutoProfileConfig{
		WarmupSteps:     200,
		CooldownMinutes: 30,
	}

	job := &tensorreaperv1.FabricAIJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-job",
			Namespace: "default",
		},
		Status: tensorreaperv1.FabricAIJobStatus{
			Metrics: &tensorreaperv1.JobMetrics{
				Step: 50, // Below warmup threshold
			},
		},
	}

	r, _ := newProfilerReconciler(fp)
	// The shouldProfileJob delegates to profiler.ShouldProfile which needs the
	// step to be >= warmupSteps. With step=50 and warmup=200, should not profile.
	result := r.shouldProfileJob(fp, job)
	if result {
		t.Error("expected shouldProfileJob to return false when step < warmupSteps")
	}
}

func TestProfiler_UpdateCondition(t *testing.T) {
	r, _ := newProfilerReconciler()
	fp := newTestProfiler("test", "default")

	r.updateCondition(fp, "Ready", metav1.ConditionTrue, "OK", "All good")
	if len(fp.Status.Conditions) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(fp.Status.Conditions))
	}

	r.updateCondition(fp, "Ready", metav1.ConditionFalse, "Error", "Something wrong")
	if len(fp.Status.Conditions) != 1 {
		t.Fatalf("expected 1 condition after update, got %d", len(fp.Status.Conditions))
	}
	if fp.Status.Conditions[0].Status != metav1.ConditionFalse {
		t.Error("expected condition status False after update")
	}
}

func TestProfiler_MergeRecommendations(t *testing.T) {
	now := metav1.Now()
	existing := []tensorreaperv1.ProfilerRecommendation{
		{JobName: "old-job", Category: "batchSize", Severity: "info", Description: "old", EstimatedImpact: "5%", Timestamp: now},
	}
	newRecs := []tensorreaperv1.ProfilerRecommendation{
		{JobName: "new-job", Category: "dataLoading", Severity: "warning", Description: "new", EstimatedImpact: "20%", Timestamp: now},
	}

	merged := mergeRecommendations(existing, newRecs, 10)
	if len(merged) != 2 {
		t.Fatalf("expected 2 merged recommendations, got %d", len(merged))
	}
	// New should be first
	if merged[0].JobName != "new-job" {
		t.Errorf("expected new recommendation first, got %s", merged[0].JobName)
	}

	// Test truncation
	merged = mergeRecommendations(existing, newRecs, 1)
	if len(merged) != 1 {
		t.Fatalf("expected 1 recommendation after truncation, got %d", len(merged))
	}
}

func TestProfiler_Capitalize(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "Hello"},
		{"", ""},
		{"H", "H"},
		{"batchSize", "BatchSize"},
	}
	for _, tt := range tests {
		result := capitalize(tt.input)
		if result != tt.expected {
			t.Errorf("capitalize(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestProfiler_LabelsMatch(t *testing.T) {
	tests := []struct {
		name         string
		objectLabels map[string]string
		selector     map[string]string
		expected     bool
	}{
		{"empty selector matches all", map[string]string{"a": "1"}, nil, true},
		{"matching labels", map[string]string{"a": "1", "b": "2"}, map[string]string{"a": "1"}, true},
		{"non-matching labels", map[string]string{"a": "1"}, map[string]string{"a": "2"}, false},
		{"missing label", map[string]string{"a": "1"}, map[string]string{"b": "1"}, false},
		{"nil object labels", nil, map[string]string{"a": "1"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := labelsMatch(tt.objectLabels, tt.selector)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}
