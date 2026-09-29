# ML Workflows Guide

Gryvia's machine learning workflow kinds: hyperparameter tuning, DAG pipelines, model registry, inference serving and interactive workspaces, plus the training-side kinds that do have a running controller.

## Status: read this first

The five kinds this guide is mostly about are **not running behaviour today**:

| Kind | CRD | Gateway/dashboard CRUD | Controller registered |
|------|-----|------------------------|-----------------------|
| `GryviaAutoTuner` | yes | create, list, delete, list trials | no |
| `GryviaWorkflow` | yes | create, list, delete | no |
| `GryviaModelRegistry` | yes | list, get, promote (patches `spec.stage` only) | no |
| `GryviaInferenceService` | yes | create, list, delete | no |
| `GryviaWorkspace` | yes | create, list, delete, pause and resume | no |

You can `kubectl apply` these manifests and the gateway and dashboard can create and list them, but no operator reconciles them: no trials are launched, no DAG steps run, no model is served, no notebook pod is created, and `status` stays empty. Controller-style code for several of them exists under `operators/ai-operator/controllers/` (and a search library in `operators/ai-operator/pkg/tuner`) but is not registered in the operator's `main.go`. The sections below describe the schema that exists and the behaviour the kinds are designed for; the behaviour parts are marked as design. All YAML uses the real schema (`crds/`) and is checked in CI; the examples in `examples/ml-workflow/` are the source models.

The API routes are in the [API reference](../developer-guide/api-reference.md). See the [CRD reference](../reference/crds.md) for the Controller column across all kinds.

## Table of Contents

0. [What runs today](#what-runs-today)
1. [Hyperparameter Tuning (GryviaAutoTuner)](#hyperparameter-tuning-gryviaautotuner)
2. [DAG-Based Pipelines (GryviaWorkflow)](#dag-based-pipelines-gryviaworkflow)
3. [Model Registry (GryviaModelRegistry)](#model-registry-gryviamodelregistry)
4. [Inference Serving (GryviaInferenceService)](#inference-serving-gryviainferenceservice)
5. [Interactive Workspaces (GryviaWorkspace)](#interactive-workspaces-gryviaworkspace)

---

## What runs today

The ai-operator registers controllers for these training-related kinds (examples in `examples/training/`):

- `GryviaAIJob`: creates the StatefulSet, Service and PVC for a job (see [Scheduling](SCHEDULING.md#what-runs-today) and the [job guide](../user-guide/jobs.md)).
- `GryviaCheckpointGuard`: checkpoint protection for a job.
- `GryviaTrainingTimeMachine`: checkpoint history and forking experiments from checkpoints.
- `GryviaLiveExperiment`: compares experiment runs using metrics parsed from job logs (metric patterns are configurable).
- `GryviaTrainingProfiler`: training profiling.
- `GryviaModelLineage`: records provenance of a model.

The gpu-operator adds `GryviaGpuMemoryOptimizer`. All of these are exercised by Go unit tests against fake clients; none has been verified end to end with real GPU training runs.

---

## Hyperparameter Tuning (GryviaAutoTuner)

Status: CRD and gateway/dashboard CRUD only. No controller launches trials.

### Overview

`GryviaAutoTuner` describes a study: a search algorithm, an objective, a parameter space and a `GryviaAIJob` spec (`jobTemplate`) to run for each trial. The schema names four `searchAlgorithm` values: `grid`, `random`, `bayesian` and `asha` (with `ashaConfig` for `maxEpochs`, `minResource` and `reductionFactor`). A search library exists in `operators/ai-operator/pkg/tuner/search.go`, but the controller that would use it is not registered, so nothing generates trials from the spec. Do not rely on any convergence claim for these strategies; none has been measured here.

### Example (schema-valid)

```yaml
apiVersion: gryvia.io/v1alpha1
kind: GryviaAutoTuner
metadata:
  name: resnet-hpo
  namespace: ml-team
spec:
  searchAlgorithm: bayesian
  objective:
    metricName: val_accuracy
    direction: maximize
  maxTrials: 50
  parallelism: 4
  earlyStoppingRounds: 10
  parameterSpace:
    - name: learning_rate
      type: float
      min: 0.0001
      max: 0.1
      scale: log
    - name: batch_size
      type: int
      values: ["32", "64", "128", "256"]
    - name: optimizer
      type: categorical
      values: ["adam", "sgd", "adamw"]
  jobTemplate:
    type: training
    model: resnet50
    image: training/resnet:latest
    gpus: 1
    gpuType: A100
    resources:
      requests:
        memory: "16Gi"
        cpu: "4"
```

Required spec fields: `searchAlgorithm`, `objective`, `maxTrials`, `parameterSpace`, `jobTemplate`. There is no `budget`, `resumeFrom` or `trialTemplate` field; earlier versions of this guide showed them, and they are rejected by the schema.

### Working with it today

```bash
# Create, list and inspect through kubectl (the gryvia CLI does not manage tuners)
kubectl apply -f resnet-hpo.yaml
kubectl get gryviaautotuners -n ml-team
kubectl get gryviaautotuner resnet-hpo -n ml-team -o yaml
```

`gryvia submit` is for `GryviaAIJob` files, not for these kinds.

### Status fields

The schema defines `status.phase`, `trialsCompleted`, `trialsRunning`, `trialsFailed`, `bestTrial` (name, jobName, parameters, metricValue, ...) and `trials`. They would be filled by a controller; today they stay empty.

---

## DAG-Based Pipelines (GryviaWorkflow)

Status: CRD and gateway/dashboard CRUD only. No controller executes steps.

### Overview

A `GryviaWorkflow` lists `steps`. Each step has a `name`, an optional `dependsOn` list, a `type` (the schema has `jobTemplate` for a job step, `script` for a container script and `webhook` for an HTTP call), `timeoutSeconds`, `retries`, `retryBackoffSeconds` and a free-form `condition` string. Workflow-level `parameters` are a string map. There is no artifact passing, no per-step checkpointing and no structured condition language in the schema. The dependency ordering, fan-out/fan-in, retries and conditions are what a workflow controller would implement; none is running.

### Example (schema-valid)

```yaml
apiVersion: gryvia.io/v1alpha1
kind: GryviaWorkflow
metadata:
  name: image-classifier-pipeline
  namespace: ml-team
spec:
  parameters:
    dataset: "imagenet-1k"
    model_name: "resnet50-v2"
  steps:
    - name: preprocess
      type: job
      timeoutSeconds: 3600
      retries: 2
      jobTemplate:
        type: training
        model: data-pipeline
        image: ml-pipelines/preprocess:1.4.0
        gpus: 0
        command: ["python"]
        args: ["preprocess.py", "--dataset=imagenet-1k", "--output=/data/processed"]
    - name: train
      type: job
      dependsOn: [preprocess]
      timeoutSeconds: 86400
      retries: 1
      jobTemplate:
        type: training
        model: resnet50
        image: training/pytorch-train:2.1.0
        gpus: 4
        gpuType: A100
        network: rdma
        distributed:
          enabled: true
          framework: pytorch
          nodes: 2
          gpusPerNode: 4
          backend: nccl
        command: ["torchrun"]
        args: ["--nproc_per_node=4", "--nnodes=2", "train.py", "--data=/data/processed"]
    - name: evaluate
      type: job
      dependsOn: [train]
      jobTemplate:
        type: evaluation
        model: resnet50
        image: training/pytorch-eval:2.1.0
        gpus: 1
        gpuType: A100
        command: ["python"]
        args: ["evaluate.py"]
```

### Fan-out and fan-in

Several steps that list the same `dependsOn` parent are the intended fan-out, and a step listing several parents is the fan-in:

```text
train -> evaluate ---\
train -> benchmark ---> publish
```

Design only until a workflow controller exists.

### Working with it today

```bash
kubectl apply -f pipeline.yaml
kubectl get gryviaworkflows -n ml-team
```

To get a pipeline running now, run the steps as separate `GryviaAIJob` objects yourself or use an external workflow engine.

---

## Model Registry (GryviaModelRegistry)

Status: CRD, list/get and a promote route in the gateway. No controller acts on the object.

### Overview

`GryviaModelRegistry` records a model version: `modelName`, `version`, `artifacts` (`s3Path` or `pvcName`/`subPath`, `format`, `sizeBytes`), `stage`, `source` (`jobRef`, `tunerRef` or `workflowRef`), free-form string `metadata`, and optional `autoServe` with a `servingConfig` (`backend`, `replicas`, `gpuCount`, `gpuType`). The dashboard and gateway treat `stage` as dev, staging and production. Promotion through the gateway (`POST /api/models/{name}/promote`) patches `spec.stage` and validates the transition; it does not deploy anything. `autoServe` would be acted on by a controller that does not exist yet, so promoting to production does not create an inference service. There is no approval workflow, canary block or metrics block on this kind.

### Example (schema-valid)

```yaml
apiVersion: gryvia.io/v1alpha1
kind: GryviaModelRegistry
metadata:
  name: resnet50-v2-1-0
  namespace: ml-team
spec:
  modelName: resnet50-v2
  version: "1.0.0"
  source:
    workflowRef: image-classifier-pipeline
  artifacts:
    s3Path: "s3://ml-models/resnet50-v2/v1.0.0/"
    format: pytorch
    sizeBytes: 102400000
  stage: staging
  description: "ResNet50 v2 trained on ImageNet-1K"
  metadata:
    framework: "pytorch-2.1.0"
    dataset: "imagenet-1k"
    top1_accuracy: "0.785"
  autoServe: false
```

### Stage changes

```bash
kubectl get gryviamodelregistries -n ml-team

# What the gateway does on promote: patch spec.stage
kubectl patch gryviamodelregistry resnet50-v2-1-0 -n ml-team \
  --type merge -p '{"spec":{"stage":"production"}}'
```

The status fields (`phase`, `servingEndpoint`, `inferenceServiceName`, ...) are defined by the CRD but not populated. Rollback is re-applying an earlier manifest; nothing automates it.

---

## Inference Serving (GryviaInferenceService)

Status: CRD and gateway/dashboard CRUD only. Nothing creates a Deployment, Service or canary. For serving today, run your own Deployment or use a `GryviaAIJob` with `type: inference`.

### Overview

`GryviaInferenceService` declares `modelRef` (a string naming a registry entry), `backend`, `replicas`, `gpuCount`, `gpuType`, `image`, `args`, `servicePort`, `autoscaling` (`minReplicas`, `maxReplicas`, `targetGPUUtilization`, `targetRequestsPerSecond`), `canary` (`enabled`, `weight`, `modelVersion`, `autoPromote`, `promoteAfterSeconds`) and `healthCheck` (`path`, `intervalSeconds`, `failureThreshold`, `autoRollback`).

Backends the design has in mind are Triton, vLLM, TensorRT-LLM and TorchServe; the schema takes `backend` as a free string. Feature lists for each backend (dynamic batching, PagedAttention, quantization and so on) belong to those projects, not to Gryvia, and Gryvia does not configure them beyond passing `args` to the container.

### Example (schema-valid)

```yaml
apiVersion: gryvia.io/v1alpha1
kind: GryviaInferenceService
metadata:
  name: llama-70b-serving
  namespace: ml-serving
spec:
  modelRef: llama-70b-v2-0
  backend: vllm
  replicas: 3
  gpuCount: 4
  gpuType: A100
  image: vllm/vllm-openai:0.3.0
  args:
    - "--tensor-parallel-size=4"
    - "--max-model-len=4096"
    - "--gpu-memory-utilization=0.90"
  servicePort: 8080
  autoscaling:
    enabled: true
    minReplicas: 2
    maxReplicas: 8
    targetGPUUtilization: 75
    targetRequestsPerSecond: 100
  canary:
    enabled: true
    weight: 10
    modelVersion: llama-70b-v2-1
    autoPromote: true
    promoteAfterSeconds: 3600
  healthCheck:
    path: /health
    intervalSeconds: 30
    failureThreshold: 3
    autoRollback: true
```

### Intended canary behaviour (design)

A serving controller would route `canary.weight` percent of traffic to `canary.modelVersion`, promote after `promoteAfterSeconds` if healthy, and roll back after `failureThreshold` failed health checks. The step-wise weight schedule and latency or error-rate thresholds that earlier versions of this guide described are not in the schema. No such rollout runs today.

### Working with it today

```bash
kubectl apply -f inference-service.yaml
kubectl get gryviainferenceservices -n ml-serving
```

---

## Interactive Workspaces (GryviaWorkspace)

Status: CRD and gateway/dashboard CRUD (including pause and resume routes that toggle `spec.paused`). No controller creates a pod, PVC or URL.

### Overview

`GryviaWorkspace` declares an environment `type` (required, for example `jupyter`), `gpuCount`, `gpuType`, `image`, `storage` (a size string) and `storageClassName`, CPU and memory requests and limits (`cpuRequest`, `cpuLimit`, `memRequest`, `memLimit`), `idleTimeoutMinutes`, `maxLifetimeHours`, `paused` and `env` (a string map). Idle detection, pause, automatic termination, package installation and git cloning would be workspace-controller behaviour and are not implemented; the earlier `packages`, `git`, `idleAction` and structured `storage` blocks are not in the schema.

### Example (schema-valid)

```yaml
apiVersion: gryvia.io/v1alpha1
kind: GryviaWorkspace
metadata:
  name: research-notebook
  namespace: ml-team
spec:
  type: jupyter
  gpuCount: 2
  gpuType: A100-80G
  image: ml-images/jupyterlab-cuda:12.1-pytorch2.1
  storage: "100Gi"
  storageClassName: fast-ssd
  cpuRequest: "8"
  cpuLimit: "16"
  memRequest: "32Gi"
  memLimit: "64Gi"
  idleTimeoutMinutes: 60
  maxLifetimeHours: 72
  env:
    WANDB_PROJECT: "ml-research"
    HF_HOME: "/workspace/.cache/huggingface"
```

### Working with it today

```bash
kubectl apply -f workspace.yaml
kubectl get gryviaworkspaces -n ml-team

# Sets spec.paused; with no controller this only changes the field
kubectl patch gryviaworkspace research-notebook -n ml-team \
  --type merge -p '{"spec":{"paused":true}}'

kubectl delete gryviaworkspace research-notebook -n ml-team
```

Status fields (`phase`, `url`, `podName`, `lastActivity`) are defined but not populated.

---

## Practical guidance

These apply to running training today.

- Write checkpoints to a mounted volume from your own training code, and see `GryviaCheckpointGuard` for the operator-side checkpoint handling.
- Set `spec.storage` on a `GryviaAIJob` to get a PVC mounted at `/data`.
- CPU-only work: the admission webhook rejects `gpus: 0` on a `GryviaAIJob` (the `gpus: 0` in the workflow example above only satisfies the schema), so run CPU-only steps as plain Kubernetes Jobs.
- Add `team` and project labels to jobs so cost and usage reports can group them.

The tuning, pipeline, registry, serving and workspace practices from earlier versions of this guide (early stopping budgets, canary weights, idle timeouts) presuppose controllers that do not exist and have been removed.

---

## Support

- **Issues**: https://github.com/zyvorai/gryvia/issues
- **Discussions**: https://github.com/zyvorai/gryvia/discussions
