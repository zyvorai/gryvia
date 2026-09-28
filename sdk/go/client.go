package gryvia

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gryviav1 "github.com/zyvorai/gryvia/operators/ai-operator/api/v1"
)

// GryviaClient provides typed methods for managing Gryvia resources.
type GryviaClient struct {
	client client.Client
}

// NewClient creates a new GryviaClient using the provided Kubernetes REST config.
func NewClient(cfg *rest.Config) (*GryviaClient, error) {
	scheme := runtime.NewScheme()
	if err := gryviav1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("failed to add Gryvia types to scheme: %w", err)
	}

	c, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	return &GryviaClient{client: c}, nil
}

// NewClientFromExisting wraps an existing controller-runtime client.
// The caller must ensure that Gryvia types are registered in the client's scheme.
func NewClientFromExisting(c client.Client) *GryviaClient {
	return &GryviaClient{client: c}
}

// --- FabricAIJob operations ---

// CreateJob creates a new FabricAIJob in the given namespace.
func (t *GryviaClient) CreateJob(ctx context.Context, job *FabricAIJob) error {
	return t.client.Create(ctx, job)
}

// GetJob retrieves a FabricAIJob by name and namespace.
func (t *GryviaClient) GetJob(ctx context.Context, namespace, name string) (*FabricAIJob, error) {
	job := &FabricAIJob{}
	err := t.client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, job)
	if err != nil {
		return nil, err
	}
	return job, nil
}

// ListJobs returns all FabricAIJobs in the given namespace.
func (t *GryviaClient) ListJobs(ctx context.Context, namespace string, opts ...client.ListOption) (*FabricAIJobList, error) {
	list := &FabricAIJobList{}
	allOpts := append([]client.ListOption{client.InNamespace(namespace)}, opts...)
	err := t.client.List(ctx, list, allOpts...)
	if err != nil {
		return nil, err
	}
	return list, nil
}

// DeleteJob deletes a FabricAIJob by name and namespace.
func (t *GryviaClient) DeleteJob(ctx context.Context, namespace, name string) error {
	job := &FabricAIJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	return t.client.Delete(ctx, job)
}

// UpdateJobStatus updates the status of a FabricAIJob.
func (t *GryviaClient) UpdateJobStatus(ctx context.Context, job *FabricAIJob) error {
	return t.client.Status().Update(ctx, job)
}

// WatchJob returns a watch.Interface that receives events for FabricAIJob changes.
// Note: This requires the client to support watching (e.g., a cached client from a Manager).
// For non-cached clients, consider using an informer instead.
func (t *GryviaClient) WatchJob(ctx context.Context, namespace string) (watch.Interface, error) {
	// controller-runtime's client.Client does not natively expose Watch.
	// Callers using a manager should use the informer cache.
	// This method documents the intended interface; a full implementation
	// would use client-go's typed client or informer factories.
	return nil, fmt.Errorf("Watch requires an informer-backed client; use manager's cache instead")
}

// --- FabricAutoTuner operations ---

// CreateAutoTuner creates a new FabricAutoTuner.
func (t *GryviaClient) CreateAutoTuner(ctx context.Context, tuner *FabricAutoTuner) error {
	return t.client.Create(ctx, tuner)
}

// GetAutoTuner retrieves a FabricAutoTuner by name and namespace.
func (t *GryviaClient) GetAutoTuner(ctx context.Context, namespace, name string) (*FabricAutoTuner, error) {
	tuner := &FabricAutoTuner{}
	err := t.client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, tuner)
	if err != nil {
		return nil, err
	}
	return tuner, nil
}

// ListAutoTuners returns all FabricAutoTuners in the given namespace.
func (t *GryviaClient) ListAutoTuners(ctx context.Context, namespace string, opts ...client.ListOption) (*FabricAutoTunerList, error) {
	list := &FabricAutoTunerList{}
	allOpts := append([]client.ListOption{client.InNamespace(namespace)}, opts...)
	err := t.client.List(ctx, list, allOpts...)
	if err != nil {
		return nil, err
	}
	return list, nil
}

// DeleteAutoTuner deletes a FabricAutoTuner by name and namespace.
func (t *GryviaClient) DeleteAutoTuner(ctx context.Context, namespace, name string) error {
	tuner := &FabricAutoTuner{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	return t.client.Delete(ctx, tuner)
}

// --- FabricWorkflow operations ---

// CreateWorkflow creates a new FabricWorkflow.
func (t *GryviaClient) CreateWorkflow(ctx context.Context, wf *FabricWorkflow) error {
	return t.client.Create(ctx, wf)
}

// GetWorkflow retrieves a FabricWorkflow by name and namespace.
func (t *GryviaClient) GetWorkflow(ctx context.Context, namespace, name string) (*FabricWorkflow, error) {
	wf := &FabricWorkflow{}
	err := t.client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, wf)
	if err != nil {
		return nil, err
	}
	return wf, nil
}

// ListWorkflows returns all FabricWorkflows in the given namespace.
func (t *GryviaClient) ListWorkflows(ctx context.Context, namespace string, opts ...client.ListOption) (*FabricWorkflowList, error) {
	list := &FabricWorkflowList{}
	allOpts := append([]client.ListOption{client.InNamespace(namespace)}, opts...)
	err := t.client.List(ctx, list, allOpts...)
	if err != nil {
		return nil, err
	}
	return list, nil
}

// DeleteWorkflow deletes a FabricWorkflow by name and namespace.
func (t *GryviaClient) DeleteWorkflow(ctx context.Context, namespace, name string) error {
	wf := &FabricWorkflow{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	return t.client.Delete(ctx, wf)
}

// --- FabricModelRegistry operations ---

// CreateModelRegistry creates a new FabricModelRegistry entry.
func (t *GryviaClient) CreateModelRegistry(ctx context.Context, model *FabricModelRegistry) error {
	return t.client.Create(ctx, model)
}

// GetModelRegistry retrieves a FabricModelRegistry by name and namespace.
func (t *GryviaClient) GetModelRegistry(ctx context.Context, namespace, name string) (*FabricModelRegistry, error) {
	model := &FabricModelRegistry{}
	err := t.client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, model)
	if err != nil {
		return nil, err
	}
	return model, nil
}

// ListModelRegistries returns all FabricModelRegistry entries in the given namespace.
func (t *GryviaClient) ListModelRegistries(ctx context.Context, namespace string, opts ...client.ListOption) (*FabricModelRegistryList, error) {
	list := &FabricModelRegistryList{}
	allOpts := append([]client.ListOption{client.InNamespace(namespace)}, opts...)
	err := t.client.List(ctx, list, allOpts...)
	if err != nil {
		return nil, err
	}
	return list, nil
}

// DeleteModelRegistry deletes a FabricModelRegistry by name and namespace.
func (t *GryviaClient) DeleteModelRegistry(ctx context.Context, namespace, name string) error {
	model := &FabricModelRegistry{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	return t.client.Delete(ctx, model)
}

// PromoteModel updates the stage of a model registry entry.
func (t *GryviaClient) PromoteModel(ctx context.Context, namespace, name string, stage gryviav1.ModelStage) error {
	model, err := t.GetModelRegistry(ctx, namespace, name)
	if err != nil {
		return err
	}
	model.Spec.Stage = stage
	return t.client.Update(ctx, model)
}

// --- FabricInferenceService operations ---

// CreateInferenceService creates a new FabricInferenceService.
func (t *GryviaClient) CreateInferenceService(ctx context.Context, svc *FabricInferenceService) error {
	return t.client.Create(ctx, svc)
}

// GetInferenceService retrieves a FabricInferenceService by name and namespace.
func (t *GryviaClient) GetInferenceService(ctx context.Context, namespace, name string) (*FabricInferenceService, error) {
	svc := &FabricInferenceService{}
	err := t.client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, svc)
	if err != nil {
		return nil, err
	}
	return svc, nil
}

// ListInferenceServices returns all FabricInferenceServices in the given namespace.
func (t *GryviaClient) ListInferenceServices(ctx context.Context, namespace string, opts ...client.ListOption) (*FabricInferenceServiceList, error) {
	list := &FabricInferenceServiceList{}
	allOpts := append([]client.ListOption{client.InNamespace(namespace)}, opts...)
	err := t.client.List(ctx, list, allOpts...)
	if err != nil {
		return nil, err
	}
	return list, nil
}

// DeleteInferenceService deletes a FabricInferenceService by name and namespace.
func (t *GryviaClient) DeleteInferenceService(ctx context.Context, namespace, name string) error {
	svc := &FabricInferenceService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	return t.client.Delete(ctx, svc)
}

// --- FabricWorkspace operations ---

// CreateWorkspace creates a new FabricWorkspace.
func (t *GryviaClient) CreateWorkspace(ctx context.Context, ws *FabricWorkspace) error {
	return t.client.Create(ctx, ws)
}

// GetWorkspace retrieves a FabricWorkspace by name and namespace.
func (t *GryviaClient) GetWorkspace(ctx context.Context, namespace, name string) (*FabricWorkspace, error) {
	ws := &FabricWorkspace{}
	err := t.client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, ws)
	if err != nil {
		return nil, err
	}
	return ws, nil
}

// ListWorkspaces returns all FabricWorkspaces in the given namespace.
func (t *GryviaClient) ListWorkspaces(ctx context.Context, namespace string, opts ...client.ListOption) (*FabricWorkspaceList, error) {
	list := &FabricWorkspaceList{}
	allOpts := append([]client.ListOption{client.InNamespace(namespace)}, opts...)
	err := t.client.List(ctx, list, allOpts...)
	if err != nil {
		return nil, err
	}
	return list, nil
}

// DeleteWorkspace deletes a FabricWorkspace by name and namespace.
func (t *GryviaClient) DeleteWorkspace(ctx context.Context, namespace, name string) error {
	ws := &FabricWorkspace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	return t.client.Delete(ctx, ws)
}

// PauseWorkspace sets the workspace to paused state, freeing GPU resources while retaining data.
func (t *GryviaClient) PauseWorkspace(ctx context.Context, namespace, name string) error {
	ws, err := t.GetWorkspace(ctx, namespace, name)
	if err != nil {
		return err
	}
	ws.Spec.Paused = true
	return t.client.Update(ctx, ws)
}

// ResumeWorkspace resumes a paused workspace, recreating the pod with the existing PVC.
func (t *GryviaClient) ResumeWorkspace(ctx context.Context, namespace, name string) error {
	ws, err := t.GetWorkspace(ctx, namespace, name)
	if err != nil {
		return err
	}
	ws.Spec.Paused = false
	return t.client.Update(ctx, ws)
}

// --- FabricGPUNode operations ---

// GetGPUNode retrieves a FabricGPUNode by name.
func (t *GryviaClient) GetGPUNode(ctx context.Context, name string) (*FabricGPUNode, error) {
	node := &FabricGPUNode{}
	err := t.client.Get(ctx, types.NamespacedName{Name: name}, node)
	if err != nil {
		return nil, err
	}
	return node, nil
}

// ListGPUNodes returns all FabricGPUNodes.
func (t *GryviaClient) ListGPUNodes(ctx context.Context, opts ...client.ListOption) (*FabricGPUNodeList, error) {
	list := &FabricGPUNodeList{}
	err := t.client.List(ctx, list, opts...)
	if err != nil {
		return nil, err
	}
	return list, nil
}

// --- FabricCheckpointGuard operations ---

// CreateCheckpointGuard creates a new FabricCheckpointGuard.
func (t *GryviaClient) CreateCheckpointGuard(ctx context.Context, guard *FabricCheckpointGuard) error {
	return t.client.Create(ctx, guard)
}

// GetCheckpointGuard retrieves a FabricCheckpointGuard by name and namespace.
func (t *GryviaClient) GetCheckpointGuard(ctx context.Context, namespace, name string) (*FabricCheckpointGuard, error) {
	guard := &FabricCheckpointGuard{}
	err := t.client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, guard)
	if err != nil {
		return nil, err
	}
	return guard, nil
}

// ListCheckpointGuards returns all FabricCheckpointGuards in the given namespace.
func (t *GryviaClient) ListCheckpointGuards(ctx context.Context, namespace string, opts ...client.ListOption) (*FabricCheckpointGuardList, error) {
	list := &FabricCheckpointGuardList{}
	allOpts := append([]client.ListOption{client.InNamespace(namespace)}, opts...)
	err := t.client.List(ctx, list, allOpts...)
	if err != nil {
		return nil, err
	}
	return list, nil
}

// DeleteCheckpointGuard deletes a FabricCheckpointGuard by name and namespace.
func (t *GryviaClient) DeleteCheckpointGuard(ctx context.Context, namespace, name string) error {
	guard := &FabricCheckpointGuard{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	return t.client.Delete(ctx, guard)
}
