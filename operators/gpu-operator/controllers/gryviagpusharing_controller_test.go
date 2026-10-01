package controllers

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	gryviav1 "github.com/zyvorai/gryvia/operators/gpu-operator/api/v1"
)

func sharingGpuNode(name, nodeName, gpuType string, gpus int) *gryviav1.GryviaGpuNode {
	return &gryviav1.GryviaGpuNode{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       gryviav1.GryviaGpuNodeSpec{NodeName: nodeName, GpuType: gpuType, GpuCount: gpus},
	}
}

// reconcileSharing runs one reconcile of policy against a fake cluster and returns the nodes.
func reconcileSharing(t *testing.T, policy *gryviav1.GryviaGPUSharingPolicy, objs ...client.Object) (client.Client, *gryviav1.GryviaGPUSharingPolicy) {
	t.Helper()
	scheme := newGpuNodeTestScheme()
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(append(objs, policy)...).
		WithStatusSubresource(&gryviav1.GryviaGPUSharingPolicy{}).
		Build()
	r := &GryviaGPUSharingPolicyReconciler{Client: c, Scheme: scheme, Log: ctrl.Log.WithName("test")}
	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: policy.Name}}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	got := &gryviav1.GryviaGPUSharingPolicy{}
	if err := c.Get(context.Background(), types.NamespacedName{Name: policy.Name}, got); err != nil {
		t.Fatal(err)
	}
	return c, got
}

func nodeLabels(t *testing.T, c client.Client, name string) map[string]string {
	t.Helper()
	n := &corev1.Node{}
	if err := c.Get(context.Background(), types.NamespacedName{Name: name}, n); err != nil {
		t.Fatal(err)
	}
	return n.Labels
}

func wantLabel(t *testing.T, labels map[string]string, key, want string) {
	t.Helper()
	if got, ok := labels[key]; !ok || got != want {
		t.Errorf("label %s = %q (present=%v), want %q; labels: %v", key, got, ok, want, labels)
	}
}

// A time-slicing policy puts the label the NVIDIA GPU Operator's device plugin reads on the matching nodes.
func TestSharingTimeSlicingWritesNVIDIADevicePluginLabel(t *testing.T) {
	policy := &gryviav1.GryviaGPUSharingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "share-inference"},
		Spec: gryviav1.GryviaGPUSharingPolicySpec{
			Strategy:    "time-slicing",
			TimeSlicing: &gryviav1.GPUTimeSlicingConfig{Enabled: true, MaxPodsPerGPU: 4},
		},
	}
	c, got := reconcileSharing(t, policy,
		sharingGpuNode("gpu-1", "node-1", "A100", 2),
		&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}},
	)
	l := nodeLabels(t, c, "node-1")
	wantLabel(t, l, "nvidia.com/device-plugin.config", "gryvia-time-slicing")
	wantLabel(t, l, "gryvia.io/gpu-sharing", "time-slicing")
	wantLabel(t, l, "gryvia.io/max-pods-per-gpu", "4")
	if len(got.Status.AffectedNodes) != 1 || got.Status.AffectedNodes[0] != "node-1" || got.Status.TotalGPUs != 2 {
		t.Errorf("status: affected=%v total=%d, want [node-1] and 2", got.Status.AffectedNodes, got.Status.TotalGPUs)
	}
}

// A MIG policy sets nvidia.com/mig.config to its first profile, which must be a real mig-parted name.
func TestSharingMIGWritesNVIDIAMigConfigLabel(t *testing.T) {
	policy := &gryviav1.GryviaGPUSharingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "mig-small"},
		Spec: gryviav1.GryviaGPUSharingPolicySpec{
			Strategy: "mig",
			MIG:      &gryviav1.GPUMIGConfig{Enabled: true, Profiles: []gryviav1.GPUMIGProfile{{Name: "all-1g.10gb", Count: 7}}},
		},
	}
	c, _ := reconcileSharing(t, policy,
		sharingGpuNode("gpu-1", "node-1", "A100", 1),
		&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}},
	)
	l := nodeLabels(t, c, "node-1")
	wantLabel(t, l, "nvidia.com/mig.config", "all-1g.10gb")
	wantLabel(t, l, "gryvia.io/gpu-sharing", "mig")
	wantLabel(t, l, "gryvia.io/mig-all-1g.10gb", "7")
}

// Only nodes the policy's nodeSelector matches are labelled; others are left alone.
func TestSharingLeavesNonMatchingNodesAlone(t *testing.T) {
	policy := &gryviav1.GryviaGPUSharingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "a100-only"},
		Spec: gryviav1.GryviaGPUSharingPolicySpec{
			NodeSelector: map[string]string{"gryvia.io/gpu-type": "A100"},
			Strategy:     "time-slicing",
			TimeSlicing:  &gryviav1.GPUTimeSlicingConfig{Enabled: true, MaxPodsPerGPU: 2},
		},
	}
	c, _ := reconcileSharing(t, policy,
		sharingGpuNode("gpu-a", "node-a", "A100", 1),
		sharingGpuNode("gpu-h", "node-h", "H100", 1),
		&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-a"}},
		&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-h"}},
	)
	wantLabel(t, nodeLabels(t, c, "node-a"), "nvidia.com/device-plugin.config", "gryvia-time-slicing")
	if _, ok := nodeLabels(t, c, "node-h")["nvidia.com/device-plugin.config"]; ok {
		t.Error("the H100 node does not match the selector but was labelled")
	}
}

// A strategy whose own block is disabled writes nothing.
func TestSharingDisabledBlockWritesNoLabels(t *testing.T) {
	policy := &gryviav1.GryviaGPUSharingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "off"},
		Spec: gryviav1.GryviaGPUSharingPolicySpec{
			Strategy:    "time-slicing",
			TimeSlicing: &gryviav1.GPUTimeSlicingConfig{Enabled: false, MaxPodsPerGPU: 4},
		},
	}
	c, _ := reconcileSharing(t, policy,
		sharingGpuNode("gpu-1", "node-1", "A100", 1),
		&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}},
	)
	if l := nodeLabels(t, c, "node-1"); len(l) != 0 {
		t.Errorf("a disabled policy labelled the node: %v", l)
	}
}
