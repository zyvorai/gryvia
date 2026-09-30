package main

import (
	"flag"
	"testing"
	"time"

	"github.com/zyvorai/gryvia/operators/ai-operator/controllers"
)

func TestMLFlagDefaultsKeepTheBuiltInImages(t *testing.T) {
	var o mlOptions
	fs := flag.NewFlagSet("t", flag.ContinueOnError)
	o.bind(fs)
	if err := fs.Parse(nil); err != nil {
		t.Fatal(err)
	}
	if !o.enabled || o.workspaceJupyterImage != controllers.DefaultJupyterImage || o.workspaceCodeImage != controllers.DefaultVSCodeImage {
		t.Errorf("workspace defaults: %+v", o)
	}
	if o.inferenceImageVLLM != "vllm/vllm-openai:latest" || o.inferenceImageTriton != "nvcr.io/nvidia/tritonserver:24.01-py3" ||
		o.inferenceImageTensorRT != "nvcr.io/nvidia/tritonserver:24.01-trtllm-python-py3" || o.inferenceImageTorchServe != "pytorch/torchserve:latest-gpu" {
		t.Errorf("inference defaults: %+v", o)
	}
	if o.autoServeGPUCount != 1 || o.workflowAllowWebhooks || o.canaryStartupGrace != 5*time.Minute || o.inferenceHealthPath != "" {
		t.Errorf("other defaults: %+v", o)
	}
}

func TestMLFlagOverrides(t *testing.T) {
	var o mlOptions
	fs := flag.NewFlagSet("t", flag.ContinueOnError)
	o.bind(fs)
	err := fs.Parse([]string{
		"--workspace-code-image=busybox:1", "--inference-image-vllm=nginx:1", "--inference-health-path=/",
		"--autoserve-default-gpu-count=0", "--workflow-allow-webhooks=true", "--enable-ml-controllers=false",
	})
	if err != nil {
		t.Fatal(err)
	}
	if o.enabled || o.workspaceCodeImage != "busybox:1" || o.inferenceImageVLLM != "nginx:1" || o.inferenceHealthPath != "/" ||
		o.autoServeGPUCount != 0 || !o.workflowAllowWebhooks {
		t.Errorf("overrides: %+v", o)
	}
}
