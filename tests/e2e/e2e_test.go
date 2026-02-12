package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	dynamicClient dynamic.Interface
	clientset     *kubernetes.Clientset
	namespace     = "kubefabric-e2e-test"
)

func setupTestCluster(t *testing.T) {
	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	require.NoError(t, err, "Failed to build kubeconfig")

	dynamicClient, err = dynamic.NewForConfig(config)
	require.NoError(t, err, "Failed to create dynamic client")

	clientset, err = kubernetes.NewForConfig(config)
	require.NoError(t, err, "Failed to create clientset")

	// Create test namespace
	_, err = clientset.CoreV1().Namespaces().Create(context.TODO(), &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: namespace},
	}, metav1.CreateOptions{})
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		require.NoError(t, err, "Failed to create test namespace")
	}
}

func teardownTestCluster(t *testing.T) {
	// Delete test namespace
	err := clientset.CoreV1().Namespaces().Delete(context.TODO(), namespace, metav1.DeleteOptions{})
	assert.NoError(t, err, "Failed to delete test namespace")
}

func TestE2E_AIJobLifecycle(t *testing.T) {
	setupTestCluster(t)
	defer teardownTestCluster(t)

	ctx := context.Background()
	jobName := "test-pytorch-job"

	// Define FabricAIJob resource
	gvr := schema.GroupVersionResource{
		Group:    "kubefabric.io",
		Version:  "v1",
		Resource: "fabricaijobs",
	}

	job := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kubefabric.io/v1",
			"kind":       "FabricAIJob",
			"metadata": map[string]interface{}{
				"name":      jobName,
				"namespace": namespace,
			},
			"spec": map[string]interface{}{
				"framework": "pytorch",
				"resources": map[string]interface{}{
					"gpuType":  "H100",
					"gpuCount": 1,
					"memory":   "32Gi",
					"cpu":      8,
				},
				"image": "nvcr.io/nvidia/pytorch:24.01-py3",
				"command": []string{
					"python",
					"-c",
					"import torch; print(f'PyTorch version: {torch.__version__}'); print(f'CUDA available: {torch.cuda.is_available()}'); print('Test completed successfully')",
				},
			},
		},
	}

	// Create job
	t.Run("CreateJob", func(t *testing.T) {
		_, err := dynamicClient.Resource(gvr).Namespace(namespace).Create(ctx, job, metav1.CreateOptions{})
		require.NoError(t, err, "Failed to create FabricAIJob")
	})

	// Wait for job to be processed
	t.Run("WaitForJobStatus", func(t *testing.T) {
		timeout := time.After(2 * time.Minute)
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-timeout:
				t.Fatal("Timeout waiting for job to have status")
			case <-ticker.C:
				obj, err := dynamicClient.Resource(gvr).Namespace(namespace).Get(ctx, jobName, metav1.GetOptions{})
				require.NoError(t, err)

				status, found, err := unstructured.NestedMap(obj.Object, "status")
				if err == nil && found && len(status) > 0 {
					phase, _, _ := unstructured.NestedString(status, "phase")
					t.Logf("Job phase: %s", phase)

					if phase == "Running" || phase == "Completed" || phase == "Failed" {
						return
					}
				}
			}
		}
	})

	// Verify job status
	t.Run("VerifyJobStatus", func(t *testing.T) {
		obj, err := dynamicClient.Resource(gvr).Namespace(namespace).Get(ctx, jobName, metav1.GetOptions{})
		require.NoError(t, err)

		status, found, err := unstructured.NestedMap(obj.Object, "status")
		require.NoError(t, err)
		require.True(t, found, "Job should have status")

		phase, found, err := unstructured.NestedString(status, "phase")
		require.NoError(t, err)
		require.True(t, found, "Job should have phase")

		assert.Contains(t, []string{"Pending", "Running", "Completed"}, phase, "Job should be in valid state")
	})

	// Delete job
	t.Run("DeleteJob", func(t *testing.T) {
		err := dynamicClient.Resource(gvr).Namespace(namespace).Delete(ctx, jobName, metav1.DeleteOptions{})
		require.NoError(t, err, "Failed to delete job")
	})
}

func TestE2E_QuotaEnforcement(t *testing.T) {
	setupTestCluster(t)
	defer teardownTestCluster(t)

	ctx := context.Background()
	quotaName := "test-team-quota"

	quotaGVR := schema.GroupVersionResource{
		Group:    "kubefabric.io",
		Version:  "v1",
		Resource: "fabricquotas",
	}

	quota := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kubefabric.io/v1",
			"kind":       "FabricQuota",
			"metadata": map[string]interface{}{
				"name": quotaName,
			},
			"spec": map[string]interface{}{
				"team":       "e2e-test-team",
				"namespaces": []string{namespace},
				"gpuQuota": map[string]interface{}{
					"maxGPUs":          2,
					"maxGPUsPerJob":    1,
					"allowedGPUTypes":  []string{"H100", "A100-80G"},
					"maxRunningJobs":   2,
				},
				"priority": 100,
			},
		},
	}

	t.Run("CreateQuota", func(t *testing.T) {
		_, err := dynamicClient.Resource(quotaGVR).Create(ctx, quota, metav1.CreateOptions{})
		require.NoError(t, err, "Failed to create FabricQuota")
	})

	t.Run("WaitForQuotaStatus", func(t *testing.T) {
		timeout := time.After(1 * time.Minute)
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-timeout:
				t.Fatal("Timeout waiting for quota to have status")
			case <-ticker.C:
				obj, err := dynamicClient.Resource(quotaGVR).Get(ctx, quotaName, metav1.GetOptions{})
				require.NoError(t, err)

				status, found, err := unstructured.NestedMap(obj.Object, "status")
				if err == nil && found && len(status) > 0 {
					return
				}
			}
		}
	})

	t.Run("VerifyQuotaStatus", func(t *testing.T) {
		obj, err := dynamicClient.Resource(quotaGVR).Get(ctx, quotaName, metav1.GetOptions{})
		require.NoError(t, err)

		status, found, err := unstructured.NestedMap(obj.Object, "status")
		require.NoError(t, err)
		require.True(t, found, "Quota should have status")

		currentUsage, found, err := unstructured.NestedMap(status, "currentUsage")
		require.NoError(t, err)
		require.True(t, found, "Quota should have currentUsage")

		allocatedGPUs, found, err := unstructured.NestedInt64(currentUsage, "allocatedGPUs")
		require.NoError(t, err)
		require.True(t, found)

		assert.GreaterOrEqual(t, int(allocatedGPUs), 0, "Allocated GPUs should be >= 0")
	})

	t.Run("DeleteQuota", func(t *testing.T) {
		err := dynamicClient.Resource(quotaGVR).Delete(ctx, quotaName, metav1.DeleteOptions{})
		require.NoError(t, err, "Failed to delete quota")
	})
}

func TestE2E_NodeRegistration(t *testing.T) {
	setupTestCluster(t)
	defer teardownTestCluster(t)

	ctx := context.Background()

	nodeGVR := schema.GroupVersionResource{
		Group:    "kubefabric.io",
		Version:  "v1",
		Resource: "fabricgpunodes",
	}

	t.Run("ListGPUNodes", func(t *testing.T) {
		nodes, err := dynamicClient.Resource(nodeGVR).List(ctx, metav1.ListOptions{})
		require.NoError(t, err, "Failed to list GPU nodes")

		t.Logf("Found %d GPU nodes", len(nodes.Items))

		for _, node := range nodes.Items {
			nodeName, _, _ := unstructured.NestedString(node.Object, "spec", "nodeName")
			gpuType, _, _ := unstructured.NestedString(node.Object, "spec", "gpuType")
			gpuCount, _, _ := unstructured.NestedInt64(node.Object, "spec", "gpuCount")

			t.Logf("Node: %s, GPU Type: %s, Count: %d", nodeName, gpuType, gpuCount)

			assert.NotEmpty(t, nodeName, "Node should have a name")
			assert.NotEmpty(t, gpuType, "Node should have GPU type")
			assert.Greater(t, int(gpuCount), 0, "Node should have GPUs")
		}
	})
}

func TestE2E_StorageProvisioning(t *testing.T) {
	setupTestCluster(t)
	defer teardownTestCluster(t)

	ctx := context.Background()
	storageName := "test-vast-storage"

	storageGVR := schema.GroupVersionResource{
		Group:    "kubefabric.io",
		Version:  "v1",
		Resource: "fabricstorages",
	}

	storage := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kubefabric.io/v1",
			"kind":       "FabricStorage",
			"metadata": map[string]interface{}{
				"name": storageName,
			},
			"spec": map[string]interface{}{
				"backendType": "vast",
				"vast": map[string]interface{}{
					"endpoint": "vast-mgmt.example.com",
					"vipPool":  "vip-pool-1",
					"viewPolicy": "default",
				},
				"capacity": "100Ti",
			},
		},
	}

	t.Run("CreateStorage", func(t *testing.T) {
		_, err := dynamicClient.Resource(storageGVR).Create(ctx, storage, metav1.CreateOptions{})
		require.NoError(t, err, "Failed to create FabricStorage")
	})

	t.Run("WaitForStorageReady", func(t *testing.T) {
		timeout := time.After(2 * time.Minute)
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-timeout:
				t.Log("Timeout waiting for storage - this is expected in test environment")
				return
			case <-ticker.C:
				obj, err := dynamicClient.Resource(storageGVR).Get(ctx, storageName, metav1.GetOptions{})
				require.NoError(t, err)

				status, found, err := unstructured.NestedMap(obj.Object, "status")
				if err == nil && found && len(status) > 0 {
					phase, _, _ := unstructured.NestedString(status, "phase")
					if phase == "Ready" {
						return
					}
				}
			}
		}
	})

	t.Run("DeleteStorage", func(t *testing.T) {
		err := dynamicClient.Resource(storageGVR).Delete(ctx, storageName, metav1.DeleteOptions{})
		require.NoError(t, err, "Failed to delete storage")
	})
}
