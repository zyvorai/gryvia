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
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	tensorreaperv1 "github.com/ssahani/TensorReaper/operators/quota-operator/api/v1"
)

func newQuotaTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = tensorreaperv1.AddToScheme(s)
	_ = corev1.AddToScheme(s)
	return s
}

func newQuotaReconciler(objs ...client.Object) (*FabricQuotaReconciler, client.Client) {
	scheme := newQuotaTestScheme()
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		WithStatusSubresource(&tensorreaperv1.FabricQuota{}, &tensorreaperv1.FabricAIJob{}).
		Build()
	r := &FabricQuotaReconciler{
		Client: fakeClient,
		Scheme: scheme,
	}
	return r, fakeClient
}

func newTestQuota(name string) *tensorreaperv1.FabricQuota {
	return &tensorreaperv1.FabricQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: tensorreaperv1.FabricQuotaSpec{
			Team:       "ml-team",
			Namespaces: []string{"ml-prod", "ml-staging"},
			GPUQuota: tensorreaperv1.GPUQuotaSpec{
				MaxGPUs:        64,
				MaxGPUsPerJob:  8,
				AllowedGPUTypes: []string{"H100", "A100"},
				MaxRunningJobs: 10,
			},
			Priority: 50,
			Budget: &tensorreaperv1.BudgetSpec{
				MonthlyBudget:  10000.0,
				AlertThreshold: 80.0,
				HardLimit:      true,
			},
		},
	}
}

func TestQuota_Reconcile_NotFound(t *testing.T) {
	r, _ := newQuotaReconciler()

	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "nonexistent"}}
	result, err := r.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Requeue || result.RequeueAfter > 0 {
		t.Error("expected no requeue for not-found")
	}
}

func TestQuota_Reconcile_AddsFinalizer(t *testing.T) {
	quota := newTestQuota("test-quota")
	ns1 := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ml-prod"}}
	ns2 := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ml-staging"}}

	r, fakeClient := newQuotaReconciler(quota, ns1, ns2)

	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "test-quota"}}
	result, err := r.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !result.Requeue {
		t.Error("expected requeue after adding finalizer")
	}

	updated := &tensorreaperv1.FabricQuota{}
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "test-quota"}, updated); err != nil {
		t.Fatalf("failed to get updated quota: %v", err)
	}
	if !controllerutil.ContainsFinalizer(updated, fabricQuotaFinalizer) {
		t.Error("expected finalizer to be added")
	}
}

func TestQuota_Reconcile_DeletionRemovesLabels(t *testing.T) {
	now := metav1.Now()
	quota := newTestQuota("test-quota")
	quota.DeletionTimestamp = &now
	quota.Finalizers = []string{fabricQuotaFinalizer}

	ns1 := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ml-prod",
			Labels: map[string]string{
				"tensorreaper.ai/team":  "ml-team",
				"tensorreaper.ai/quota": "test-quota",
				"other-label":           "keep",
			},
		},
	}
	ns2 := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ml-staging",
			Labels: map[string]string{
				"tensorreaper.ai/team":  "ml-team",
				"tensorreaper.ai/quota": "test-quota",
			},
		},
	}

	r, fakeClient := newQuotaReconciler(quota, ns1, ns2)

	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "test-quota"}}
	result, err := r.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error during deletion, got %v", err)
	}
	if result.Requeue || result.RequeueAfter > 0 {
		t.Error("expected no requeue after deletion")
	}

	// Verify labels removed from namespaces
	updatedNs1 := &corev1.Namespace{}
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "ml-prod"}, updatedNs1); err != nil {
		t.Fatalf("failed to get namespace: %v", err)
	}
	if _, exists := updatedNs1.Labels["tensorreaper.ai/team"]; exists {
		t.Error("expected tensorreaper.ai/team label to be removed from namespace")
	}
	if val, exists := updatedNs1.Labels["other-label"]; !exists || val != "keep" {
		t.Error("expected non-tensorreaper labels to be preserved")
	}

	// Verify finalizer removed
	updatedQuota := &tensorreaperv1.FabricQuota{}
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "test-quota"}, updatedQuota); err != nil {
		t.Fatalf("failed to get quota: %v", err)
	}
	if controllerutil.ContainsFinalizer(updatedQuota, fabricQuotaFinalizer) {
		t.Error("expected finalizer to be removed")
	}
}

func TestQuota_LabelNamespaces(t *testing.T) {
	quota := newTestQuota("test-quota")
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ml-prod",
		},
	}

	r, fakeClient := newQuotaReconciler(quota, ns)
	err := r.labelNamespaces(context.Background(), quota)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	updatedNs := &corev1.Namespace{}
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "ml-prod"}, updatedNs); err != nil {
		t.Fatalf("failed to get namespace: %v", err)
	}
	if updatedNs.Labels["tensorreaper.ai/team"] != "ml-team" {
		t.Errorf("expected team label ml-team, got %s", updatedNs.Labels["tensorreaper.ai/team"])
	}
	if updatedNs.Labels["tensorreaper.ai/quota"] != "test-quota" {
		t.Errorf("expected quota label test-quota, got %s", updatedNs.Labels["tensorreaper.ai/quota"])
	}
}

func TestQuota_LabelNamespaces_NotFound(t *testing.T) {
	quota := newTestQuota("test-quota")
	// Namespace "ml-prod" does not exist, "ml-staging" does not exist either
	r, _ := newQuotaReconciler(quota)

	err := r.labelNamespaces(context.Background(), quota)
	// Should not error on missing namespaces
	if err != nil {
		t.Fatalf("expected no error for missing namespaces, got %v", err)
	}
}

func TestQuota_UpdateStatus(t *testing.T) {
	quota := newTestQuota("test-quota")
	controllerutil.AddFinalizer(quota, fabricQuotaFinalizer)

	r, fakeClient := newQuotaReconciler(quota)

	err := r.updateStatus(context.Background(), quota, "Active", "All good")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	updated := &tensorreaperv1.FabricQuota{}
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "test-quota"}, updated); err != nil {
		t.Fatalf("failed to get quota: %v", err)
	}
	if updated.Status.Phase != "Active" {
		t.Errorf("expected phase Active, got %s", updated.Status.Phase)
	}

	// Check Ready condition
	foundReady := false
	for _, cond := range updated.Status.Conditions {
		if cond.Type == "Ready" {
			foundReady = true
			if cond.Status != metav1.ConditionTrue {
				t.Error("expected Ready condition True for Active phase")
			}
		}
	}
	if !foundReady {
		t.Error("expected Ready condition to be set")
	}

	// Failed phase should set condition False
	err = r.updateStatus(context.Background(), quota, "Failed", "error")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "test-quota"}, updated); err != nil {
		t.Fatalf("failed to get quota: %v", err)
	}
	for _, cond := range updated.Status.Conditions {
		if cond.Type == "Ready" && cond.Status != metav1.ConditionFalse {
			t.Error("expected Ready condition False for Failed phase")
		}
	}
}

func TestQuota_EnforceQuota_RejectsExceedingJobs(t *testing.T) {
	quota := newTestQuota("test-quota")
	quota.Status.Phase = "BudgetExceeded"
	controllerutil.AddFinalizer(quota, fabricQuotaFinalizer)

	job := &tensorreaperv1.FabricAIJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "big-job",
			Namespace: "ml-prod",
		},
		Spec: tensorreaperv1.FabricAIJobSpec{
			GPUs:    16,
			GpuType: "H100",
		},
		Status: tensorreaperv1.FabricAIJobStatus{
			Phase: "Pending",
		},
	}

	r, fakeClient := newQuotaReconciler(quota, job)

	err := r.enforceQuota(context.Background(), quota)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	updated := &tensorreaperv1.FabricAIJob{}
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "big-job", Namespace: "ml-prod"}, updated); err != nil {
		t.Fatalf("failed to get job: %v", err)
	}

	// Job exceeds MaxGPUsPerJob (16 > 8) and budget exceeded
	if updated.Status.Phase != "Rejected" {
		t.Errorf("expected phase Rejected, got %s", updated.Status.Phase)
	}
}
