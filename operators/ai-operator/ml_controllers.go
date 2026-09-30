package main

import (
	"flag"
	"time"

	gryviav1 "github.com/zyvorai/gryvia/operators/ai-operator/api/v1"
	"github.com/zyvorai/gryvia/operators/ai-operator/controllers"
)

// mlOptions are the flags of the ML controllers: GryviaWorkspace, GryviaInferenceService, GryviaModelRegistry,
// GryviaWorkflow and GryviaAutoTuner. Image defaults can be replaced per cluster here and per object with
// spec.image; the built-in defaults are large GPU images that are unverified here (see docs/ml-controllers.md).
type mlOptions struct {
	enabled bool

	workspaceJupyterImage string
	workspaceCodeImage    string

	inferenceImageVLLM       string
	inferenceImageTriton     string
	inferenceImageTensorRT   string
	inferenceImageTorchServe string
	inferenceHealthPath      string
	canaryStartupGrace       time.Duration
	inferenceGatewayRouting  bool
	autoServeGPUCount        int

	workflowMaxParallelSteps int
	workflowAllowWebhooks    bool

	tunerMaxTrials      int
	tunerMaxParallelism int
}

func (o *mlOptions) bind(fs *flag.FlagSet) {
	fs.BoolVar(&o.enabled, "enable-ml-controllers", true,
		"Run the GryviaWorkspace, GryviaInferenceService, GryviaModelRegistry, GryviaWorkflow and GryviaAutoTuner controllers.")
	fs.StringVar(&o.workspaceJupyterImage, "workspace-jupyter-image", controllers.DefaultJupyterImage,
		"Image of jupyter workspaces whose spec.image is empty.")
	fs.StringVar(&o.workspaceCodeImage, "workspace-code-image", controllers.DefaultVSCodeImage,
		"Image of vscode workspaces whose spec.image is empty.")
	fs.StringVar(&o.inferenceImageVLLM, "inference-image-vllm", controllers.DefaultInferenceImages[gryviav1.BackendVLLM],
		"Image of vllm inference services whose spec.image is empty.")
	fs.StringVar(&o.inferenceImageTriton, "inference-image-triton", controllers.DefaultInferenceImages[gryviav1.BackendTriton],
		"Image of triton inference services whose spec.image is empty.")
	fs.StringVar(&o.inferenceImageTensorRT, "inference-image-tensorrt-llm", controllers.DefaultInferenceImages[gryviav1.BackendTensorRTLLM],
		"Image of tensorrt-llm inference services whose spec.image is empty.")
	fs.StringVar(&o.inferenceImageTorchServe, "inference-image-torchserve", controllers.DefaultInferenceImages[gryviav1.BackendTorchServe],
		"Image of torchserve inference services whose spec.image is empty.")
	fs.StringVar(&o.inferenceHealthPath, "inference-health-path", "",
		"HTTP path of the inference readiness/liveness probes for every backend (default: per backend, e.g. /health for vllm). spec.healthCheck.path still wins.")
	fs.BoolVar(&o.inferenceGatewayRouting, "inference-gateway-routing", false, "Manage opt-in Gateway API HTTPRoutes for inference canaries. Requires Gateway API v1 and a Gateway controller.")
	fs.DurationVar(&o.canaryStartupGrace, "inference-canary-startup-grace", 5*time.Minute,
		"How long a new canary may take to become ready before health checks count it as failing.")
	fs.IntVar(&o.autoServeGPUCount, "autoserve-default-gpu-count", 1,
		"GPUs per replica of a service auto-created by a GryviaModelRegistry whose servingConfig sets no gpuCount (0 serves on CPU).")
	fs.IntVar(&o.workflowMaxParallelSteps, "workflow-max-parallel-steps", controllers.DefaultWorkflowMaxParallelSteps,
		"Most steps of one GryviaWorkflow that run at the same time.")
	fs.BoolVar(&o.workflowAllowWebhooks, "workflow-allow-webhooks", false,
		"Run webhook steps of GryviaWorkflow (the operator makes the HTTP call). Off by default: a webhook step lets whoever can create a workflow make the operator send requests inside the cluster network.")
	fs.IntVar(&o.tunerMaxTrials, "tuner-max-trials", controllers.DefaultMaxTrialsCap,
		"Largest spec.maxTrials a GryviaAutoTuner may ask for; larger tuners are marked Failed.")
	fs.IntVar(&o.tunerMaxParallelism, "tuner-max-parallelism", controllers.DefaultMaxParallelismCap,
		"Most trials of one GryviaAutoTuner that run at the same time (lowers spec.parallelism).")
}
