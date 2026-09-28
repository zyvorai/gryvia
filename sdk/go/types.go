// Package gryvia provides a typed Go client for Gryvia resources.
//
// This package re-exports all API types from the ai-operator for convenient
// use by external consumers without directly depending on the operator module.
package gryvia

import (
	gryviav1 "github.com/zyvorai/gryvia/operators/ai-operator/api/v1"
)

// --- FabricAIJob re-exports ---

type FabricAIJob = gryviav1.FabricAIJob
type FabricAIJobList = gryviav1.FabricAIJobList
type FabricAIJobSpec = gryviav1.FabricAIJobSpec
type FabricAIJobStatus = gryviav1.FabricAIJobStatus
type DistributedConfig = gryviav1.DistributedConfig
type JobMetrics = gryviav1.JobMetrics

// --- FabricAutoTuner re-exports ---

type FabricAutoTuner = gryviav1.FabricAutoTuner
type FabricAutoTunerList = gryviav1.FabricAutoTunerList
type FabricAutoTunerSpec = gryviav1.FabricAutoTunerSpec
type FabricAutoTunerStatus = gryviav1.FabricAutoTunerStatus
type ParameterSpec = gryviav1.ParameterSpec
type ObjectiveSpec = gryviav1.ObjectiveSpec
type ASHAConfig = gryviav1.ASHAConfig
type TrialResult = gryviav1.TrialResult
type IntermediateMetric = gryviav1.IntermediateMetric

// Search algorithm constants
const (
	SearchAlgorithmGrid     = gryviav1.SearchAlgorithmGrid
	SearchAlgorithmRandom   = gryviav1.SearchAlgorithmRandom
	SearchAlgorithmBayesian = gryviav1.SearchAlgorithmBayesian
	SearchAlgorithmASHA     = gryviav1.SearchAlgorithmASHA
)

// Objective direction constants
const (
	ObjectiveMinimize = gryviav1.ObjectiveMinimize
	ObjectiveMaximize = gryviav1.ObjectiveMaximize
)

// Parameter type constants
const (
	ParameterTypeFloat       = gryviav1.ParameterTypeFloat
	ParameterTypeInt         = gryviav1.ParameterTypeInt
	ParameterTypeCategorical = gryviav1.ParameterTypeCategorical
)

// --- FabricWorkflow re-exports ---

type FabricWorkflow = gryviav1.FabricWorkflow
type FabricWorkflowList = gryviav1.FabricWorkflowList
type FabricWorkflowSpec = gryviav1.FabricWorkflowSpec
type FabricWorkflowStatus = gryviav1.FabricWorkflowStatus
type WorkflowStep = gryviav1.WorkflowStep
type ScriptStep = gryviav1.ScriptStep
type WebhookStep = gryviav1.WebhookStep
type StepStatus = gryviav1.StepStatus

// Step type constants
const (
	StepTypeJob     = gryviav1.StepTypeJob
	StepTypeScript  = gryviav1.StepTypeScript
	StepTypeWebhook = gryviav1.StepTypeWebhook
)

// --- FabricModelRegistry re-exports ---

type FabricModelRegistry = gryviav1.FabricModelRegistry
type FabricModelRegistryList = gryviav1.FabricModelRegistryList
type FabricModelRegistrySpec = gryviav1.FabricModelRegistrySpec
type FabricModelRegistryStatus = gryviav1.FabricModelRegistryStatus
type ModelSource = gryviav1.ModelSource
type ModelArtifacts = gryviav1.ModelArtifacts
type ServingConfig = gryviav1.ServingConfig

// Model stage constants
const (
	ModelStageDev        = gryviav1.ModelStageDev
	ModelStageStaging    = gryviav1.ModelStageStaging
	ModelStageProduction = gryviav1.ModelStageProduction
	ModelStageArchived   = gryviav1.ModelStageArchived
)

// --- FabricInferenceService re-exports ---

type FabricInferenceService = gryviav1.FabricInferenceService
type FabricInferenceServiceList = gryviav1.FabricInferenceServiceList
type FabricInferenceServiceSpec = gryviav1.FabricInferenceServiceSpec
type FabricInferenceServiceStatus = gryviav1.FabricInferenceServiceStatus
type AutoscalingConfig = gryviav1.AutoscalingConfig
type CanaryConfig = gryviav1.CanaryConfig
type HealthCheckConfig = gryviav1.HealthCheckConfig
type CanaryStatus = gryviav1.CanaryStatus

// Inference backend constants
const (
	BackendTriton      = gryviav1.BackendTriton
	BackendVLLM        = gryviav1.BackendVLLM
	BackendTensorRTLLM = gryviav1.BackendTensorRTLLM
	BackendTorchServe  = gryviav1.BackendTorchServe
)

// --- FabricWorkspace re-exports ---

type FabricWorkspace = gryviav1.FabricWorkspace
type FabricWorkspaceList = gryviav1.FabricWorkspaceList
type FabricWorkspaceSpec = gryviav1.FabricWorkspaceSpec
type FabricWorkspaceStatus = gryviav1.FabricWorkspaceStatus

// Workspace type constants
const (
	WorkspaceTypeJupyter = gryviav1.WorkspaceTypeJupyter
	WorkspaceTypeVSCode  = gryviav1.WorkspaceTypeVSCode
)

// --- FabricGPUNode re-exports ---

type FabricGPUNode = gryviav1.FabricGpuNode
type FabricGPUNodeList = gryviav1.FabricGpuNodeList

// --- FabricCheckpointGuard re-exports ---

type FabricCheckpointGuard = gryviav1.FabricCheckpointGuard
type FabricCheckpointGuardList = gryviav1.FabricCheckpointGuardList

// --- GroupVersion ---

var GroupVersion = gryviav1.GroupVersion
var AddToScheme = gryviav1.AddToScheme
