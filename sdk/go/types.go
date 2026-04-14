// Package tensorreaper provides a typed Go client for TensorReaper resources.
//
// This package re-exports all API types from the ai-operator for convenient
// use by external consumers without directly depending on the operator module.
package tensorreaper

import (
	tensorreaperv1 "github.com/ssahani/TensorReaper/operators/ai-operator/api/v1"
)

// --- FabricAIJob re-exports ---

type FabricAIJob = tensorreaperv1.FabricAIJob
type FabricAIJobList = tensorreaperv1.FabricAIJobList
type FabricAIJobSpec = tensorreaperv1.FabricAIJobSpec
type FabricAIJobStatus = tensorreaperv1.FabricAIJobStatus
type DistributedConfig = tensorreaperv1.DistributedConfig
type JobMetrics = tensorreaperv1.JobMetrics

// --- FabricAutoTuner re-exports ---

type FabricAutoTuner = tensorreaperv1.FabricAutoTuner
type FabricAutoTunerList = tensorreaperv1.FabricAutoTunerList
type FabricAutoTunerSpec = tensorreaperv1.FabricAutoTunerSpec
type FabricAutoTunerStatus = tensorreaperv1.FabricAutoTunerStatus
type ParameterSpec = tensorreaperv1.ParameterSpec
type ObjectiveSpec = tensorreaperv1.ObjectiveSpec
type ASHAConfig = tensorreaperv1.ASHAConfig
type TrialResult = tensorreaperv1.TrialResult
type IntermediateMetric = tensorreaperv1.IntermediateMetric

// Search algorithm constants
const (
	SearchAlgorithmGrid     = tensorreaperv1.SearchAlgorithmGrid
	SearchAlgorithmRandom   = tensorreaperv1.SearchAlgorithmRandom
	SearchAlgorithmBayesian = tensorreaperv1.SearchAlgorithmBayesian
	SearchAlgorithmASHA     = tensorreaperv1.SearchAlgorithmASHA
)

// Objective direction constants
const (
	ObjectiveMinimize = tensorreaperv1.ObjectiveMinimize
	ObjectiveMaximize = tensorreaperv1.ObjectiveMaximize
)

// Parameter type constants
const (
	ParameterTypeFloat       = tensorreaperv1.ParameterTypeFloat
	ParameterTypeInt         = tensorreaperv1.ParameterTypeInt
	ParameterTypeCategorical = tensorreaperv1.ParameterTypeCategorical
)

// --- FabricWorkflow re-exports ---

type FabricWorkflow = tensorreaperv1.FabricWorkflow
type FabricWorkflowList = tensorreaperv1.FabricWorkflowList
type FabricWorkflowSpec = tensorreaperv1.FabricWorkflowSpec
type FabricWorkflowStatus = tensorreaperv1.FabricWorkflowStatus
type WorkflowStep = tensorreaperv1.WorkflowStep
type ScriptStep = tensorreaperv1.ScriptStep
type WebhookStep = tensorreaperv1.WebhookStep
type StepStatus = tensorreaperv1.StepStatus

// Step type constants
const (
	StepTypeJob     = tensorreaperv1.StepTypeJob
	StepTypeScript  = tensorreaperv1.StepTypeScript
	StepTypeWebhook = tensorreaperv1.StepTypeWebhook
)

// --- FabricModelRegistry re-exports ---

type FabricModelRegistry = tensorreaperv1.FabricModelRegistry
type FabricModelRegistryList = tensorreaperv1.FabricModelRegistryList
type FabricModelRegistrySpec = tensorreaperv1.FabricModelRegistrySpec
type FabricModelRegistryStatus = tensorreaperv1.FabricModelRegistryStatus
type ModelSource = tensorreaperv1.ModelSource
type ModelArtifacts = tensorreaperv1.ModelArtifacts
type ServingConfig = tensorreaperv1.ServingConfig

// Model stage constants
const (
	ModelStageDev        = tensorreaperv1.ModelStageDev
	ModelStageStaging    = tensorreaperv1.ModelStageStaging
	ModelStageProduction = tensorreaperv1.ModelStageProduction
	ModelStageArchived   = tensorreaperv1.ModelStageArchived
)

// --- FabricInferenceService re-exports ---

type FabricInferenceService = tensorreaperv1.FabricInferenceService
type FabricInferenceServiceList = tensorreaperv1.FabricInferenceServiceList
type FabricInferenceServiceSpec = tensorreaperv1.FabricInferenceServiceSpec
type FabricInferenceServiceStatus = tensorreaperv1.FabricInferenceServiceStatus
type AutoscalingConfig = tensorreaperv1.AutoscalingConfig
type CanaryConfig = tensorreaperv1.CanaryConfig
type HealthCheckConfig = tensorreaperv1.HealthCheckConfig
type CanaryStatus = tensorreaperv1.CanaryStatus

// Inference backend constants
const (
	BackendTriton      = tensorreaperv1.BackendTriton
	BackendVLLM        = tensorreaperv1.BackendVLLM
	BackendTensorRTLLM = tensorreaperv1.BackendTensorRTLLM
	BackendTorchServe  = tensorreaperv1.BackendTorchServe
)

// --- FabricWorkspace re-exports ---

type FabricWorkspace = tensorreaperv1.FabricWorkspace
type FabricWorkspaceList = tensorreaperv1.FabricWorkspaceList
type FabricWorkspaceSpec = tensorreaperv1.FabricWorkspaceSpec
type FabricWorkspaceStatus = tensorreaperv1.FabricWorkspaceStatus

// Workspace type constants
const (
	WorkspaceTypeJupyter = tensorreaperv1.WorkspaceTypeJupyter
	WorkspaceTypeVSCode  = tensorreaperv1.WorkspaceTypeVSCode
)

// --- FabricGPUNode re-exports ---

type FabricGPUNode = tensorreaperv1.FabricGpuNode
type FabricGPUNodeList = tensorreaperv1.FabricGpuNodeList

// --- FabricCheckpointGuard re-exports ---

type FabricCheckpointGuard = tensorreaperv1.FabricCheckpointGuard
type FabricCheckpointGuardList = tensorreaperv1.FabricCheckpointGuardList

// --- GroupVersion ---

var GroupVersion = tensorreaperv1.GroupVersion
var AddToScheme = tensorreaperv1.AddToScheme
