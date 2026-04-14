package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// Ensure metav1 import is used
var _ metav1.Condition

// --- FabricAutoTuner ---

func (in *FabricAutoTuner) DeepCopyInto(out *FabricAutoTuner) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

func (in *FabricAutoTuner) DeepCopy() *FabricAutoTuner {
	if in == nil {
		return nil
	}
	out := new(FabricAutoTuner)
	in.DeepCopyInto(out)
	return out
}

func (in *FabricAutoTuner) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

func (in *FabricAutoTunerList) DeepCopyInto(out *FabricAutoTunerList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]FabricAutoTuner, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

func (in *FabricAutoTunerList) DeepCopy() *FabricAutoTunerList {
	if in == nil {
		return nil
	}
	out := new(FabricAutoTunerList)
	in.DeepCopyInto(out)
	return out
}

func (in *FabricAutoTunerList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

func (in *FabricAutoTunerSpec) DeepCopyInto(out *FabricAutoTunerSpec) {
	*out = *in
	if in.ParameterSpace != nil {
		in, out := &in.ParameterSpace, &out.ParameterSpace
		*out = make([]ParameterSpec, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	out.Objective = in.Objective
	in.JobTemplate.DeepCopyInto(&out.JobTemplate)
	if in.ASHAConfig != nil {
		in, out := &in.ASHAConfig, &out.ASHAConfig
		*out = new(ASHAConfig)
		**out = **in
	}
}

func (in *FabricAutoTunerSpec) DeepCopy() *FabricAutoTunerSpec {
	if in == nil {
		return nil
	}
	out := new(FabricAutoTunerSpec)
	in.DeepCopyInto(out)
	return out
}

func (in *FabricAutoTunerStatus) DeepCopyInto(out *FabricAutoTunerStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]metav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Trials != nil {
		in, out := &in.Trials, &out.Trials
		*out = make([]TrialResult, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.BestTrial != nil {
		in, out := &in.BestTrial, &out.BestTrial
		*out = new(TrialResult)
		(*in).DeepCopyInto(*out)
	}
	if in.StartTime != nil {
		in, out := &in.StartTime, &out.StartTime
		*out = (*in).DeepCopy()
	}
	if in.CompletionTime != nil {
		in, out := &in.CompletionTime, &out.CompletionTime
		*out = (*in).DeepCopy()
	}
}

func (in *FabricAutoTunerStatus) DeepCopy() *FabricAutoTunerStatus {
	if in == nil {
		return nil
	}
	out := new(FabricAutoTunerStatus)
	in.DeepCopyInto(out)
	return out
}

func (in *ParameterSpec) DeepCopyInto(out *ParameterSpec) {
	*out = *in
	if in.Min != nil {
		in, out := &in.Min, &out.Min
		*out = new(float64)
		**out = **in
	}
	if in.Max != nil {
		in, out := &in.Max, &out.Max
		*out = new(float64)
		**out = **in
	}
	if in.Step != nil {
		in, out := &in.Step, &out.Step
		*out = new(int32)
		**out = **in
	}
	if in.Values != nil {
		in, out := &in.Values, &out.Values
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

func (in *ParameterSpec) DeepCopy() *ParameterSpec {
	if in == nil {
		return nil
	}
	out := new(ParameterSpec)
	in.DeepCopyInto(out)
	return out
}

func (in *ObjectiveSpec) DeepCopyInto(out *ObjectiveSpec) { *out = *in }
func (in *ObjectiveSpec) DeepCopy() *ObjectiveSpec {
	if in == nil {
		return nil
	}
	out := new(ObjectiveSpec)
	*out = *in
	return out
}

func (in *ASHAConfig) DeepCopyInto(out *ASHAConfig) { *out = *in }
func (in *ASHAConfig) DeepCopy() *ASHAConfig {
	if in == nil {
		return nil
	}
	out := new(ASHAConfig)
	*out = *in
	return out
}

func (in *TrialResult) DeepCopyInto(out *TrialResult) {
	*out = *in
	if in.Parameters != nil {
		in, out := &in.Parameters, &out.Parameters
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.MetricValue != nil {
		in, out := &in.MetricValue, &out.MetricValue
		*out = new(float64)
		**out = **in
	}
	if in.IntermediateMetrics != nil {
		in, out := &in.IntermediateMetrics, &out.IntermediateMetrics
		*out = make([]IntermediateMetric, len(*in))
		copy(*out, *in)
	}
	if in.StartTime != nil {
		in, out := &in.StartTime, &out.StartTime
		*out = (*in).DeepCopy()
	}
	if in.CompletionTime != nil {
		in, out := &in.CompletionTime, &out.CompletionTime
		*out = (*in).DeepCopy()
	}
}

func (in *TrialResult) DeepCopy() *TrialResult {
	if in == nil {
		return nil
	}
	out := new(TrialResult)
	in.DeepCopyInto(out)
	return out
}

func (in *IntermediateMetric) DeepCopyInto(out *IntermediateMetric) { *out = *in }
func (in *IntermediateMetric) DeepCopy() *IntermediateMetric {
	if in == nil {
		return nil
	}
	out := new(IntermediateMetric)
	*out = *in
	return out
}

// --- FabricWorkflow ---

func (in *FabricWorkflow) DeepCopyInto(out *FabricWorkflow) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

func (in *FabricWorkflow) DeepCopy() *FabricWorkflow {
	if in == nil {
		return nil
	}
	out := new(FabricWorkflow)
	in.DeepCopyInto(out)
	return out
}

func (in *FabricWorkflow) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

func (in *FabricWorkflowList) DeepCopyInto(out *FabricWorkflowList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]FabricWorkflow, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

func (in *FabricWorkflowList) DeepCopy() *FabricWorkflowList {
	if in == nil {
		return nil
	}
	out := new(FabricWorkflowList)
	in.DeepCopyInto(out)
	return out
}

func (in *FabricWorkflowList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

func (in *FabricWorkflowSpec) DeepCopyInto(out *FabricWorkflowSpec) {
	*out = *in
	if in.Steps != nil {
		in, out := &in.Steps, &out.Steps
		*out = make([]WorkflowStep, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Parameters != nil {
		in, out := &in.Parameters, &out.Parameters
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

func (in *FabricWorkflowSpec) DeepCopy() *FabricWorkflowSpec {
	if in == nil {
		return nil
	}
	out := new(FabricWorkflowSpec)
	in.DeepCopyInto(out)
	return out
}

func (in *FabricWorkflowStatus) DeepCopyInto(out *FabricWorkflowStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]metav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.StepStatuses != nil {
		in, out := &in.StepStatuses, &out.StepStatuses
		*out = make([]StepStatus, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.StartTime != nil {
		in, out := &in.StartTime, &out.StartTime
		*out = (*in).DeepCopy()
	}
	if in.CompletionTime != nil {
		in, out := &in.CompletionTime, &out.CompletionTime
		*out = (*in).DeepCopy()
	}
}

func (in *FabricWorkflowStatus) DeepCopy() *FabricWorkflowStatus {
	if in == nil {
		return nil
	}
	out := new(FabricWorkflowStatus)
	in.DeepCopyInto(out)
	return out
}

func (in *WorkflowStep) DeepCopyInto(out *WorkflowStep) {
	*out = *in
	if in.DependsOn != nil {
		in, out := &in.DependsOn, &out.DependsOn
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.JobTemplate != nil {
		in, out := &in.JobTemplate, &out.JobTemplate
		*out = new(FabricAIJobSpec)
		(*in).DeepCopyInto(*out)
	}
	if in.Script != nil {
		in, out := &in.Script, &out.Script
		*out = new(ScriptStep)
		(*in).DeepCopyInto(*out)
	}
	if in.Webhook != nil {
		in, out := &in.Webhook, &out.Webhook
		*out = new(WebhookStep)
		(*in).DeepCopyInto(*out)
	}
}

func (in *WorkflowStep) DeepCopy() *WorkflowStep {
	if in == nil {
		return nil
	}
	out := new(WorkflowStep)
	in.DeepCopyInto(out)
	return out
}

func (in *ScriptStep) DeepCopyInto(out *ScriptStep) {
	*out = *in
	if in.Command != nil {
		in, out := &in.Command, &out.Command
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Args != nil {
		in, out := &in.Args, &out.Args
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

func (in *ScriptStep) DeepCopy() *ScriptStep {
	if in == nil {
		return nil
	}
	out := new(ScriptStep)
	in.DeepCopyInto(out)
	return out
}

func (in *WebhookStep) DeepCopyInto(out *WebhookStep) {
	*out = *in
	if in.Headers != nil {
		in, out := &in.Headers, &out.Headers
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

func (in *WebhookStep) DeepCopy() *WebhookStep {
	if in == nil {
		return nil
	}
	out := new(WebhookStep)
	in.DeepCopyInto(out)
	return out
}

func (in *StepStatus) DeepCopyInto(out *StepStatus) {
	*out = *in
	if in.StartTime != nil {
		in, out := &in.StartTime, &out.StartTime
		*out = (*in).DeepCopy()
	}
	if in.CompletionTime != nil {
		in, out := &in.CompletionTime, &out.CompletionTime
		*out = (*in).DeepCopy()
	}
}

func (in *StepStatus) DeepCopy() *StepStatus {
	if in == nil {
		return nil
	}
	out := new(StepStatus)
	in.DeepCopyInto(out)
	return out
}

// --- FabricModelRegistry ---

func (in *FabricModelRegistry) DeepCopyInto(out *FabricModelRegistry) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

func (in *FabricModelRegistry) DeepCopy() *FabricModelRegistry {
	if in == nil {
		return nil
	}
	out := new(FabricModelRegistry)
	in.DeepCopyInto(out)
	return out
}

func (in *FabricModelRegistry) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

func (in *FabricModelRegistryList) DeepCopyInto(out *FabricModelRegistryList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]FabricModelRegistry, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

func (in *FabricModelRegistryList) DeepCopy() *FabricModelRegistryList {
	if in == nil {
		return nil
	}
	out := new(FabricModelRegistryList)
	in.DeepCopyInto(out)
	return out
}

func (in *FabricModelRegistryList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

func (in *FabricModelRegistrySpec) DeepCopyInto(out *FabricModelRegistrySpec) {
	*out = *in
	out.Source = in.Source
	out.Artifacts = in.Artifacts
	if in.Metadata != nil {
		in, out := &in.Metadata, &out.Metadata
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.ServingConfig != nil {
		in, out := &in.ServingConfig, &out.ServingConfig
		*out = new(ServingConfig)
		**out = **in
	}
}

func (in *FabricModelRegistrySpec) DeepCopy() *FabricModelRegistrySpec {
	if in == nil {
		return nil
	}
	out := new(FabricModelRegistrySpec)
	in.DeepCopyInto(out)
	return out
}

func (in *FabricModelRegistryStatus) DeepCopyInto(out *FabricModelRegistryStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]metav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.DeployedAt != nil {
		in, out := &in.DeployedAt, &out.DeployedAt
		*out = (*in).DeepCopy()
	}
	if in.RegisteredAt != nil {
		in, out := &in.RegisteredAt, &out.RegisteredAt
		*out = (*in).DeepCopy()
	}
}

func (in *FabricModelRegistryStatus) DeepCopy() *FabricModelRegistryStatus {
	if in == nil {
		return nil
	}
	out := new(FabricModelRegistryStatus)
	in.DeepCopyInto(out)
	return out
}

func (in *ModelSource) DeepCopyInto(out *ModelSource)     { *out = *in }
func (in *ModelSource) DeepCopy() *ModelSource             { o := new(ModelSource); *o = *in; return o }
func (in *ModelArtifacts) DeepCopyInto(out *ModelArtifacts) { *out = *in }
func (in *ModelArtifacts) DeepCopy() *ModelArtifacts       { o := new(ModelArtifacts); *o = *in; return o }
func (in *ServingConfig) DeepCopyInto(out *ServingConfig)   { *out = *in }
func (in *ServingConfig) DeepCopy() *ServingConfig         { o := new(ServingConfig); *o = *in; return o }

// --- FabricInferenceService ---

func (in *FabricInferenceService) DeepCopyInto(out *FabricInferenceService) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

func (in *FabricInferenceService) DeepCopy() *FabricInferenceService {
	if in == nil {
		return nil
	}
	out := new(FabricInferenceService)
	in.DeepCopyInto(out)
	return out
}

func (in *FabricInferenceService) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

func (in *FabricInferenceServiceList) DeepCopyInto(out *FabricInferenceServiceList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]FabricInferenceService, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

func (in *FabricInferenceServiceList) DeepCopy() *FabricInferenceServiceList {
	if in == nil {
		return nil
	}
	out := new(FabricInferenceServiceList)
	in.DeepCopyInto(out)
	return out
}

func (in *FabricInferenceServiceList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

func (in *FabricInferenceServiceSpec) DeepCopyInto(out *FabricInferenceServiceSpec) {
	*out = *in
	if in.Args != nil {
		in, out := &in.Args, &out.Args
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Autoscaling != nil {
		in, out := &in.Autoscaling, &out.Autoscaling
		*out = new(AutoscalingConfig)
		**out = **in
	}
	if in.Canary != nil {
		in, out := &in.Canary, &out.Canary
		*out = new(CanaryConfig)
		**out = **in
	}
	if in.HealthCheck != nil {
		in, out := &in.HealthCheck, &out.HealthCheck
		*out = new(HealthCheckConfig)
		**out = **in
	}
}

func (in *FabricInferenceServiceSpec) DeepCopy() *FabricInferenceServiceSpec {
	if in == nil {
		return nil
	}
	out := new(FabricInferenceServiceSpec)
	in.DeepCopyInto(out)
	return out
}

func (in *FabricInferenceServiceStatus) DeepCopyInto(out *FabricInferenceServiceStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]metav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.CanaryStatus != nil {
		in, out := &in.CanaryStatus, &out.CanaryStatus
		*out = new(CanaryStatus)
		(*in).DeepCopyInto(*out)
	}
	if in.LastHealthCheck != nil {
		in, out := &in.LastHealthCheck, &out.LastHealthCheck
		*out = (*in).DeepCopy()
	}
}

func (in *FabricInferenceServiceStatus) DeepCopy() *FabricInferenceServiceStatus {
	if in == nil {
		return nil
	}
	out := new(FabricInferenceServiceStatus)
	in.DeepCopyInto(out)
	return out
}

func (in *AutoscalingConfig) DeepCopyInto(out *AutoscalingConfig) { *out = *in }
func (in *AutoscalingConfig) DeepCopy() *AutoscalingConfig {
	o := new(AutoscalingConfig); *o = *in; return o
}
func (in *CanaryConfig) DeepCopyInto(out *CanaryConfig) { *out = *in }
func (in *CanaryConfig) DeepCopy() *CanaryConfig {
	o := new(CanaryConfig); *o = *in; return o
}
func (in *HealthCheckConfig) DeepCopyInto(out *HealthCheckConfig) { *out = *in }
func (in *HealthCheckConfig) DeepCopy() *HealthCheckConfig {
	o := new(HealthCheckConfig); *o = *in; return o
}

func (in *CanaryStatus) DeepCopyInto(out *CanaryStatus) {
	*out = *in
	if in.StartedAt != nil {
		in, out := &in.StartedAt, &out.StartedAt
		*out = (*in).DeepCopy()
	}
}

func (in *CanaryStatus) DeepCopy() *CanaryStatus {
	if in == nil {
		return nil
	}
	out := new(CanaryStatus)
	in.DeepCopyInto(out)
	return out
}

// --- FabricWorkspace ---

func (in *FabricWorkspace) DeepCopyInto(out *FabricWorkspace) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

func (in *FabricWorkspace) DeepCopy() *FabricWorkspace {
	if in == nil {
		return nil
	}
	out := new(FabricWorkspace)
	in.DeepCopyInto(out)
	return out
}

func (in *FabricWorkspace) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

func (in *FabricWorkspaceList) DeepCopyInto(out *FabricWorkspaceList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]FabricWorkspace, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

func (in *FabricWorkspaceList) DeepCopy() *FabricWorkspaceList {
	if in == nil {
		return nil
	}
	out := new(FabricWorkspaceList)
	in.DeepCopyInto(out)
	return out
}

func (in *FabricWorkspaceList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

func (in *FabricWorkspaceSpec) DeepCopyInto(out *FabricWorkspaceSpec) {
	*out = *in
	if in.Env != nil {
		in, out := &in.Env, &out.Env
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

func (in *FabricWorkspaceSpec) DeepCopy() *FabricWorkspaceSpec {
	if in == nil {
		return nil
	}
	out := new(FabricWorkspaceSpec)
	in.DeepCopyInto(out)
	return out
}

func (in *FabricWorkspaceStatus) DeepCopyInto(out *FabricWorkspaceStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]metav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.LastActivity != nil {
		in, out := &in.LastActivity, &out.LastActivity
		*out = (*in).DeepCopy()
	}
	if in.StartTime != nil {
		in, out := &in.StartTime, &out.StartTime
		*out = (*in).DeepCopy()
	}
}

func (in *FabricWorkspaceStatus) DeepCopy() *FabricWorkspaceStatus {
	if in == nil {
		return nil
	}
	out := new(FabricWorkspaceStatus)
	in.DeepCopyInto(out)
	return out
}
