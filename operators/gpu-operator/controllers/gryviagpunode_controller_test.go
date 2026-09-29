package controllers

import (
	"context"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	gryviav1 "github.com/zyvorai/gryvia/operators/gpu-operator/api/v1"
)

func newGpuNodeTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = gryviav1.AddToScheme(s)
	_ = corev1.AddToScheme(s)
	return s
}

func newGpuNodeReconciler(objs ...client.Object) (*GryviaGpuNodeReconciler, client.Client) {
	scheme := newGpuNodeTestScheme()
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		WithStatusSubresource(&gryviav1.GryviaGpuNode{}).
		Build()
	r := &GryviaGpuNodeReconciler{
		Client: fakeClient,
		Scheme: scheme,
		Log:    ctrl.Log.WithName("test"),
	}
	return r, fakeClient
}

func newTestGryviaGpuNode(name string) *gryviav1.GryviaGpuNode {
	return &gryviav1.GryviaGpuNode{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: gryviav1.GryviaGpuNodeSpec{
			NodeName: "gpu-node-1",
			GpuType:  "H100",
			GpuCount: 8,
			RDMA:     true,
			SRIOV:    false,
			HealthCheck: &gryviav1.HealthCheckConfig{
				Enabled:         true,
				IntervalSeconds: 120,
			},
		},
	}
}

func TestGpuNode_Reconcile_NotFound(t *testing.T) {
	r, _ := newGpuNodeReconciler()

	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "nonexistent"}}
	result, err := r.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error for not-found resource, got %v", err)
	}
	if result.Requeue || result.RequeueAfter > 0 {
		t.Errorf("expected no requeue for not-found resource")
	}
}

func TestGpuNode_Reconcile_AddsFinalizer(t *testing.T) {
	node := newTestGryviaGpuNode("test-gpu-node")
	k8sNode := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "gpu-node-1",
		},
	}
	r, fakeClient := newGpuNodeReconciler(node, k8sNode)

	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "test-gpu-node"}}
	result, err := r.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !result.Requeue {
		t.Error("expected requeue after adding finalizer")
	}

	// Verify finalizer was added
	updated := &gryviav1.GryviaGpuNode{}
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "test-gpu-node"}, updated); err != nil {
		t.Fatalf("failed to get updated resource: %v", err)
	}
	if !controllerutil.ContainsFinalizer(updated, gryviaGpuNodeFinalizer) {
		t.Error("expected finalizer to be added")
	}
}

func TestGpuNode_Reconcile_InitializesStatus(t *testing.T) {
	node := newTestGryviaGpuNode("test-gpu-node")
	controllerutil.AddFinalizer(node, gryviaGpuNodeFinalizer)
	k8sNode := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "gpu-node-1",
		},
	}
	r, fakeClient := newGpuNodeReconciler(node, k8sNode)

	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "test-gpu-node"}}
	result, err := r.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !result.Requeue {
		t.Error("expected requeue after status initialization")
	}

	updated := &gryviav1.GryviaGpuNode{}
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "test-gpu-node"}, updated); err != nil {
		t.Fatalf("failed to get updated resource: %v", err)
	}
	if updated.Status.Phase != PhaseInitializing {
		t.Errorf("expected phase %s, got %s", PhaseInitializing, updated.Status.Phase)
	}
}

func TestGpuNode_Reconcile_DeletionRemovesLabels(t *testing.T) {
	now := metav1.Now()
	node := newTestGryviaGpuNode("test-gpu-node")
	node.DeletionTimestamp = &now
	node.Finalizers = []string{gryviaGpuNodeFinalizer}

	k8sNode := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "gpu-node-1",
			Labels: map[string]string{
				"gryvia.io/gpu":       "H100",
				"gryvia.io/gpu-count": "8",
				"gryvia.io/rdma":      "true",
				"gryvia.io/sriov":     "false",
				"other-label":         "keep-me",
			},
		},
	}
	r, fakeClient := newGpuNodeReconciler(node, k8sNode)

	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "test-gpu-node"}}
	result, err := r.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error during deletion, got %v", err)
	}
	if result.Requeue || result.RequeueAfter > 0 {
		t.Error("expected no requeue after deletion")
	}

	// Verify labels were removed from the Kubernetes node
	updatedNode := &corev1.Node{}
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "gpu-node-1"}, updatedNode); err != nil {
		t.Fatalf("failed to get updated node: %v", err)
	}
	if _, exists := updatedNode.Labels["gryvia.io/gpu"]; exists {
		t.Error("expected gryvia.io/gpu label to be removed")
	}
	if _, exists := updatedNode.Labels["gryvia.io/gpu-count"]; exists {
		t.Error("expected gryvia.io/gpu-count label to be removed")
	}
	if val, exists := updatedNode.Labels["other-label"]; !exists || val != "keep-me" {
		t.Error("expected non-gryvia labels to be preserved")
	}

	// Verify finalizer was removed
	updatedGpu := &gryviav1.GryviaGpuNode{}
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "test-gpu-node"}, updatedGpu); err != nil {
		if apierrors.IsNotFound(err) {
			return // object is gone once its last finalizer is removed
		}
		t.Fatalf("failed to get updated gpu node: %v", err)
	}
	if controllerutil.ContainsFinalizer(updatedGpu, gryviaGpuNodeFinalizer) {
		t.Error("expected finalizer to be removed after deletion")
	}
}

func TestGpuNode_Reconcile_DeletionNodeNotFound(t *testing.T) {
	now := metav1.Now()
	node := newTestGryviaGpuNode("test-gpu-node")
	node.DeletionTimestamp = &now
	node.Finalizers = []string{gryviaGpuNodeFinalizer}

	// No Kubernetes node exists
	r, _ := newGpuNodeReconciler(node)

	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "test-gpu-node"}}
	result, err := r.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error when node not found during deletion, got %v", err)
	}
	if result.Requeue || result.RequeueAfter > 0 {
		t.Error("expected no requeue after deletion with missing node")
	}
}

func TestGpuNode_UpdateCondition(t *testing.T) {
	node := newTestGryviaGpuNode("test-gpu-node")
	r, _ := newGpuNodeReconciler()

	// Add a new condition
	r.updateCondition(node, ConditionHealthy, metav1.ConditionTrue, "Healthy", "All GPUs healthy")
	if len(node.Status.Conditions) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(node.Status.Conditions))
	}
	if node.Status.Conditions[0].Type != ConditionHealthy {
		t.Errorf("expected condition type %s, got %s", ConditionHealthy, node.Status.Conditions[0].Type)
	}
	if node.Status.Conditions[0].Status != metav1.ConditionTrue {
		t.Errorf("expected condition status True, got %s", node.Status.Conditions[0].Status)
	}

	// Update existing condition
	r.updateCondition(node, ConditionHealthy, metav1.ConditionFalse, "Unhealthy", "GPU failure")
	if len(node.Status.Conditions) != 1 {
		t.Fatalf("expected 1 condition after update, got %d", len(node.Status.Conditions))
	}
	if node.Status.Conditions[0].Status != metav1.ConditionFalse {
		t.Errorf("expected condition status False after update, got %s", node.Status.Conditions[0].Status)
	}

	// Add another condition type
	r.updateCondition(node, ConditionDriversInstalled, metav1.ConditionTrue, "Installed", "Drivers OK")
	if len(node.Status.Conditions) != 2 {
		t.Fatalf("expected 2 conditions, got %d", len(node.Status.Conditions))
	}
}

func TestGpuNode_AllConditionsTrue(t *testing.T) {
	node := newTestGryviaGpuNode("test-gpu-node")
	r, _ := newGpuNodeReconciler()

	// No conditions - should be false
	if r.allConditionsTrue(node) {
		t.Error("expected allConditionsTrue to return false when no conditions set")
	}

	// All conditions true
	r.updateCondition(node, ConditionDriversInstalled, metav1.ConditionTrue, "OK", "")
	r.updateCondition(node, ConditionNodeLabeled, metav1.ConditionTrue, "OK", "")
	r.updateCondition(node, ConditionHealthy, metav1.ConditionTrue, "OK", "")
	if !r.allConditionsTrue(node) {
		t.Error("expected allConditionsTrue to return true when all conditions are true")
	}

	// One condition false
	r.updateCondition(node, ConditionHealthy, metav1.ConditionFalse, "Failed", "")
	if r.allConditionsTrue(node) {
		t.Error("expected allConditionsTrue to return false when one condition is false")
	}
}

func TestGpuNode_AnyConditionFalse(t *testing.T) {
	node := newTestGryviaGpuNode("test-gpu-node")
	r, _ := newGpuNodeReconciler()

	// No conditions - should be false
	if r.anyConditionFalse(node) {
		t.Error("expected anyConditionFalse to return false when no conditions set")
	}

	// All true
	r.updateCondition(node, ConditionHealthy, metav1.ConditionTrue, "OK", "")
	if r.anyConditionFalse(node) {
		t.Error("expected anyConditionFalse to return false when all conditions are true")
	}

	// One false
	r.updateCondition(node, ConditionDriversInstalled, metav1.ConditionFalse, "Failed", "")
	if !r.anyConditionFalse(node) {
		t.Error("expected anyConditionFalse to return true when one condition is false")
	}
}

func TestGpuNode_CalculateBackoff(t *testing.T) {
	node := newTestGryviaGpuNode("test-gpu-node")
	r, _ := newGpuNodeReconciler()

	// No unhealthy condition - should return min backoff
	backoff := r.calculateBackoff(node)
	if backoff != 30*time.Second {
		t.Errorf("expected 30s backoff with no conditions, got %v", backoff)
	}

	// With a recent unhealthy condition
	r.updateCondition(node, ConditionHealthy, metav1.ConditionFalse, "Failed", "GPU error")
	backoff = r.calculateBackoff(node)
	if backoff < 30*time.Second {
		t.Errorf("expected at least 30s backoff, got %v", backoff)
	}
	if backoff > 10*time.Minute {
		t.Errorf("expected at most 10m backoff, got %v", backoff)
	}
}

func TestGpuNode_LabelNode(t *testing.T) {
	node := newTestGryviaGpuNode("test-gpu-node")
	node.Spec.Interconnect = "nvlink"
	node.Spec.Labels = map[string]string{
		"gryvia.io/custom": "value",
		"disallowed/label": "should-not-be-set",
	}

	k8sNode := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "gpu-node-1",
		},
	}
	r, fakeClient := newGpuNodeReconciler(node, k8sNode)

	err := r.labelNode(context.Background(), node, k8sNode)
	if err != nil {
		t.Fatalf("expected no error labeling node, got %v", err)
	}

	// Verify labels
	updatedNode := &corev1.Node{}
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "gpu-node-1"}, updatedNode); err != nil {
		t.Fatalf("failed to get updated node: %v", err)
	}

	expectedLabels := map[string]string{
		"gryvia.io/gpu":          "H100",
		"gryvia.io/gpu-count":    "8",
		"gryvia.io/rdma":         "true",
		"gryvia.io/sriov":        "false",
		"gryvia.io/interconnect": "nvlink",
		"gryvia.io/custom":       "value",
	}
	for key, expectedVal := range expectedLabels {
		if val, ok := updatedNode.Labels[key]; !ok || val != expectedVal {
			t.Errorf("expected label %s=%s, got %s (exists=%v)", key, expectedVal, val, ok)
		}
	}

	// Verify disallowed label is not set
	if _, ok := updatedNode.Labels["disallowed/label"]; ok {
		t.Error("expected disallowed label to not be set")
	}
}
