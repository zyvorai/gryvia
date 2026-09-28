# ML Workflows Guide

Complete guide to Gryvia's machine learning workflow capabilities, including hyperparameter tuning, DAG-based pipelines, model registry, inference serving, and interactive workspaces.

## Table of Contents

1. [Hyperparameter Tuning (FabricAutoTuner)](#hyperparameter-tuning-fabricautotuner)
2. [DAG-Based Pipelines (FabricWorkflow)](#dag-based-pipelines-fabricworkflow)
3. [Model Registry (FabricModelRegistry)](#model-registry-fabricmodelregistry)
4. [Inference Serving (FabricInferenceService)](#inference-serving-fabricinferenceservice)
5. [Interactive Workspaces (FabricWorkspace)](#interactive-workspaces-fabricworkspace)

---

## Hyperparameter Tuning (FabricAutoTuner)

Automated hyperparameter optimization with multiple search strategies and early stopping.

### Overview

FabricAutoTuner provides a Kubernetes-native hyperparameter tuning system that integrates directly with Gryvia's GPU scheduling and quota management. It supports four search strategies:

- **Grid Search**: Exhaustive search over all parameter combinations. Best for small, discrete parameter spaces.
- **Random Search**: Random sampling from parameter distributions. Effective for large parameter spaces where not all parameters are equally important.
- **Bayesian (TPE)**: Tree-structured Parzen Estimator for intelligent, model-guided search. Converges faster than random search by learning from prior trials.
- **ASHA Early Stopping**: Asynchronous Successive Halving Algorithm. Aggressively prunes underperforming trials to save GPU resources.

### Example

```yaml
apiVersion: gryvia.io/v1
kind: FabricAutoTuner
metadata:
  name: resnet-hpo
  namespace: ml-research
spec:
  # Search strategy: grid, random, bayesian, asha
  algorithm: bayesian

  # Optimization objective
  objective:
    metric: val_accuracy
    goal: maximize

  # Trial budget
  maxTrials: 100
  parallelism: 4

  # Parameter search space
  parameterSpace:
    - name: learning_rate
      type: float
      min: 0.0001
      max: 0.1
      scale: log

    - name: batch_size
      type: int
      values: [16, 32, 64, 128, 256]

    - name: optimizer
      type: categorical
      values: ["adam", "sgd", "adamw"]

    - name: weight_decay
      type: float
      min: 0.0
      max: 0.1

    - name: num_layers
      type: int
      min: 2
      max: 8

  # Early stopping configuration (for ASHA)
  earlyStop:
    metric: val_loss
    patience: 10
    minDelta: 0.001

  # Base trial template
  trialTemplate:
    framework: pytorch
    image: nvcr.io/nvidia/pytorch:24.01-py3
    resources:
      gpuType: A100-80G
      gpuCount: 1
    command:
      - python
      - train.py
      - --lr={{ .learning_rate }}
      - --batch-size={{ .batch_size }}
      - --optimizer={{ .optimizer }}
      - --weight-decay={{ .weight_decay }}
      - --num-layers={{ .num_layers }}

  # Resume from prior study
  resumeFrom: ""

  # Cost guardrails
  budget:
    maxCostUSD: 500
    maxTotalGPUHours: 200
```

### CLI Usage

```bash
# Create a tuning job interactively
gryvia create job --tuner \
  --algorithm bayesian \
  --max-trials 100 \
  --parallelism 4 \
  --objective "val_accuracy:maximize"

# Submit from YAML
gryvia submit -f resnet-hpo.yaml

# Monitor tuning progress
gryvia status resnet-hpo

# View trial results sorted by objective
gryvia get autotuner resnet-hpo --trials

# Get best trial parameters
gryvia get autotuner resnet-hpo --best

# Cancel tuning early (preserves completed trial results)
gryvia cancel resnet-hpo
```

### Status Tracking

The FabricAutoTuner status reports trial progress and the current best result:

```yaml
status:
  phase: Running
  completedTrials: 42
  runningTrials: 4
  failedTrials: 2
  bestTrial:
    name: resnet-hpo-trial-37
    parameters:
      learning_rate: 0.0023
      batch_size: 64
      optimizer: adamw
      weight_decay: 0.01
      num_layers: 5
    metrics:
      val_accuracy: 0.9412
```

---

## DAG-Based Pipelines (FabricWorkflow)

Multi-step ML pipelines with dependency management, conditional execution, and fan-out/fan-in patterns.

### Overview

FabricWorkflow lets you define complex ML pipelines as directed acyclic graphs (DAGs). Each step in the workflow can:

- Depend on one or more upstream steps
- Run conditionally based on upstream results
- Retry independently on failure
- Use different GPU types and resource configurations
- Pass artifacts between steps

### Example

```yaml
apiVersion: gryvia.io/v1
kind: FabricWorkflow
metadata:
  name: llm-training-pipeline
  namespace: ml-research
spec:
  # Global defaults for all steps
  defaults:
    image: nvcr.io/nvidia/pytorch:24.01-py3
    retries: 2
    retryDelay: 5m

  steps:
    - name: data-preprocessing
      image: python:3.11
      command: ["python", "preprocess.py", "--dataset", "openwebtext"]
      resources:
        cpu: 16
        memory: 64Gi
      outputs:
        artifacts:
          - name: processed-data
            path: /output/data

    - name: tokenizer-training
      dependsOn: [data-preprocessing]
      command: ["python", "train_tokenizer.py"]
      resources:
        cpu: 32
        memory: 128Gi
      inputs:
        artifacts:
          - name: processed-data
            from: data-preprocessing
      outputs:
        artifacts:
          - name: tokenizer
            path: /output/tokenizer

    - name: model-training
      dependsOn: [tokenizer-training]
      command: ["torchrun", "--nproc_per_node=8", "train.py"]
      resources:
        gpuType: H100
        gpuCount: 8
        memory: 512Gi
      retries: 3
      retryDelay: 10m
      checkpointing:
        enabled: true
        frequency: 15m
      inputs:
        artifacts:
          - name: processed-data
            from: data-preprocessing
          - name: tokenizer
            from: tokenizer-training

    - name: evaluation
      dependsOn: [model-training]
      command: ["python", "evaluate.py"]
      resources:
        gpuType: A100-80G
        gpuCount: 1
      conditions:
        - type: exitCode
          step: model-training
          operator: equals
          value: "0"

    - name: benchmark-mmlu
      dependsOn: [model-training]
      command: ["python", "benchmark.py", "--suite", "mmlu"]
      resources:
        gpuType: A100-80G
        gpuCount: 1

    - name: benchmark-humaneval
      dependsOn: [model-training]
      command: ["python", "benchmark.py", "--suite", "humaneval"]
      resources:
        gpuType: A100-80G
        gpuCount: 1

    - name: publish-results
      dependsOn: [evaluation, benchmark-mmlu, benchmark-humaneval]
      command: ["python", "publish.py"]
      resources:
        cpu: 4
        memory: 8Gi
      conditions:
        - type: allSucceeded
          steps: [evaluation, benchmark-mmlu, benchmark-humaneval]
```

### Fan-Out / Fan-In Patterns

The workflow above demonstrates a fan-out/fan-in pattern:

```
data-preprocessing
       |
tokenizer-training
       |
 model-training
    /  |  \            <-- fan-out
   /   |   \
eval  mmlu  humaneval
   \   |   /
    \  |  /            <-- fan-in
 publish-results
```

Fan-out runs multiple steps in parallel after a single upstream step completes. Fan-in waits for all parallel branches to finish before proceeding.

### Conditional Execution

Steps can run conditionally based on upstream results:

```yaml
- name: deploy-to-staging
  dependsOn: [evaluation]
  conditions:
    # Only deploy if accuracy exceeds threshold
    - type: metric
      step: evaluation
      metric: accuracy
      operator: greaterThan
      value: "0.90"

    # And training completed successfully
    - type: exitCode
      step: model-training
      operator: equals
      value: "0"
```

### CLI Usage

```bash
# Submit a workflow
gryvia submit -f llm-pipeline.yaml

# View workflow DAG status
gryvia status llm-training-pipeline

# View logs for a specific step
gryvia logs llm-training-pipeline --step model-training --follow

# Retry a failed step (re-runs from that step forward)
gryvia retry llm-training-pipeline --step evaluation

# Cancel entire workflow
gryvia cancel llm-training-pipeline
```

---

## Model Registry (FabricModelRegistry)

Versioned model storage with stage-based promotion and automated deployment.

### Overview

FabricModelRegistry provides a Kubernetes-native model registry for tracking trained models through their lifecycle. Models progress through three stages:

1. **dev** -- Initial stage after training. Used for experimentation and evaluation.
2. **staging** -- Promoted for integration testing and validation against production data.
3. **production** -- Approved for live serving. Promotion to production can trigger automatic deployment via FabricInferenceService.

### Example

```yaml
apiVersion: gryvia.io/v1
kind: FabricModelRegistry
metadata:
  name: llama-3-fine-tuned
  namespace: ml-research
spec:
  modelName: llama-3-fine-tuned
  version: "2.1.0"

  # Model artifacts
  artifacts:
    modelPath: s3://models/llama-3-ft/v2.1.0/
    format: safetensors
    framework: pytorch
    size: 14Gi

  # Current lifecycle stage
  stage: staging

  # Metadata
  metadata:
    description: "LLaMA 3 fine-tuned on domain-specific data"
    author: alice
    team: ml-research
    tags:
      - llm
      - fine-tuned
      - domain-specific

  # Training provenance
  source:
    trainingJob: llm-training-pipeline
    dataset: domain-corpus-v3
    framework: pytorch
    baseModel: meta-llama/llama-3-8b

  # Evaluation metrics
  metrics:
    accuracy: 0.942
    f1Score: 0.938
    perplexity: 4.21
    mmluScore: 0.71
    latencyP99ms: 45

  # Auto-deploy when promoted to production
  autoDeploy:
    enabled: true
    inferenceServiceRef: llama-3-serving
    canary:
      enabled: true
      initialWeight: 10
      stepWeight: 20
      stepInterval: 10m
      successThreshold:
        latencyP99ms: 100
        errorRate: 0.01

  # Approval workflow for production promotion
  approval:
    required: true
    approvers:
      - alice
      - bob
    minApprovals: 1
```

### Stage Promotion

```bash
# Register a new model version
gryvia submit -f model-registry.yaml

# List all versions of a model
gryvia get modelregistry --model llama-3-fine-tuned

# Promote to staging
gryvia promote model llama-3-fine-tuned --version 2.1.0 --stage staging

# Promote to production (triggers auto-deploy if configured)
gryvia promote model llama-3-fine-tuned --version 2.1.0 --stage production

# Rollback to a previous version
gryvia promote model llama-3-fine-tuned --version 2.0.0 --stage production

# View model details and metrics
gryvia get modelregistry llama-3-fine-tuned --output yaml
```

### Auto-Deploy on Production Promotion

When a model is promoted to `production` with `autoDeploy.enabled: true`, Gryvia automatically:

1. Creates or updates the referenced FabricInferenceService
2. Starts a canary rollout with the configured weight schedule
3. Monitors latency and error rate against success thresholds
4. Completes the rollout or triggers auto-rollback on threshold violation

---

## Inference Serving (FabricInferenceService)

Production model serving with multiple backends, canary deployments, and auto-rollback.

### Overview

FabricInferenceService deploys trained models as scalable inference endpoints. It supports four serving backends:

| Backend | Best For | Features |
|---------|----------|----------|
| **Triton** | Multi-framework, ensemble models | Dynamic batching, model ensemble, concurrent model execution |
| **vLLM** | Large language models | PagedAttention, continuous batching, tensor parallelism |
| **TensorRT-LLM** | Optimized LLM inference | INT8/FP8 quantization, inflight batching, KV cache optimization |
| **TorchServe** | PyTorch models | Custom handlers, model versioning, metrics |

### Example

```yaml
apiVersion: gryvia.io/v1
kind: FabricInferenceService
metadata:
  name: llama-3-serving
  namespace: ml-production
spec:
  # Model reference from registry
  modelRef:
    name: llama-3-fine-tuned
    version: "2.1.0"

  # Serving backend
  backend: vllm

  # Backend-specific configuration
  backendConfig:
    maxModelLen: 8192
    tensorParallelSize: 2
    quantization: awq
    gpuMemoryUtilization: 0.90
    maxBatchSize: 64

  # Replica configuration
  replicas:
    min: 2
    max: 10
    target:
      requestsPerSecond: 100
      gpuUtilization: 80

  # Resource requirements per replica
  resources:
    gpuType: A100-80G
    gpuCount: 2
    memory: 128Gi
    cpu: 16

  # Canary deployment configuration
  canary:
    enabled: true
    # Traffic weight for the canary version
    initialWeight: 10
    # Increment weight by this amount each step
    stepWeight: 20
    # Time between weight increases
    stepInterval: 10m
    # Rollback if these thresholds are exceeded
    successThreshold:
      latencyP99ms: 100
      errorRate: 0.01
      minRequests: 1000

  # Auto-rollback on failure
  rollback:
    automatic: true
    onLatencyExceeded: true
    onErrorRateExceeded: true
    onHealthCheckFailed: true

  # Health checks
  healthCheck:
    path: /health
    intervalSeconds: 10
    timeoutSeconds: 5
    failureThreshold: 3

  # Autoscaling
  autoscaling:
    enabled: true
    metric: requests_per_second
    targetValue: 100
    scaleUpStabilization: 60s
    scaleDownStabilization: 300s
```

### Canary Deployment Flow

When a new model version is deployed, the canary rollout proceeds as follows:

1. New version deployed with `initialWeight` (10%) of traffic
2. Health checks and success thresholds are monitored
3. If thresholds pass, traffic weight increases by `stepWeight` (20%) every `stepInterval` (10m)
4. Rollout: 10% -> 30% -> 50% -> 70% -> 90% -> 100%
5. If any threshold is violated, traffic automatically reverts to the previous version

### CLI Usage

```bash
# Deploy an inference service
gryvia submit -f inference-service.yaml

# View serving status
gryvia get inferenceservice llama-3-serving

# Check canary rollout progress
gryvia status llama-3-serving --canary

# Manually promote canary to full traffic
gryvia promote inference llama-3-serving --weight 100

# Rollback to previous version
gryvia rollback inference llama-3-serving

# Scale replicas manually
gryvia scale inference llama-3-serving --replicas 5

# View inference metrics
gryvia metrics inference llama-3-serving
```

---

## Interactive Workspaces (FabricWorkspace)

Managed interactive development environments with GPU access, persistent storage, and idle management.

### Overview

FabricWorkspace provides on-demand interactive environments for data scientists and ML engineers. Supported environment types:

- **Jupyter** -- JupyterLab with pre-installed ML frameworks and GPU drivers
- **VS Code** -- Code Server (VS Code in browser) with full extension support
- **Custom** -- Any container image with a web-based IDE

Workspaces include persistent storage, automatic idle detection, and pause/resume to save GPU costs when not in use.

### Example

```yaml
apiVersion: gryvia.io/v1
kind: FabricWorkspace
metadata:
  name: research-notebook
  namespace: ml-research
spec:
  # Environment type: jupyter, vscode, custom
  type: jupyter

  # GPU resources
  gpuCount: 1
  gpuType: A100-80G

  # CPU and memory
  cpu: 8
  memory: 32Gi

  # Persistent storage
  storage:
    homePVC: researcher-home
    homeSize: 50Gi
    datasetPVC: shared-datasets
    scratchSize: 100Gi

  # Pre-installed packages
  packages:
    pip:
      - torch==2.2.0
      - transformers==4.37.0
      - datasets==2.16.0
      - wandb
    conda:
      - cudatoolkit=12.1

  # Idle management
  idleTimeout: 2h
  idleAction: pause   # pause | terminate

  # Environment variables
  env:
    - name: WANDB_PROJECT
      value: my-research
    - name: HF_TOKEN
      valueFrom:
        secretKeyRef:
          name: hf-credentials
          key: token

  # Image override (optional)
  image: nvcr.io/nvidia/pytorch:24.01-py3

  # Git integration
  git:
    repo: https://github.com/org/ml-experiments.git
    branch: main
    autoClone: true
```

### Pause and Resume

Workspaces can be paused to release GPU resources while preserving storage state. This is triggered automatically after the idle timeout or manually via CLI:

```bash
# Create a workspace
gryvia submit -f workspace.yaml

# Get workspace URL
gryvia get workspace research-notebook
# Output includes: URL: https://research-notebook.gryvia.example.com

# Pause workspace (releases GPU, preserves storage)
gryvia pause workspace research-notebook

# Resume workspace (re-acquires GPU, restores state)
gryvia resume workspace research-notebook

# Terminate workspace
gryvia delete workspace research-notebook

# List all workspaces with status
gryvia list workspaces
```

### Idle Detection

When a workspace is idle (no terminal activity, no running cells, no active file edits) for the configured `idleTimeout`, Gryvia automatically:

1. Sends a notification to the user (browser notification and email)
2. Waits 5 minutes for activity
3. Executes the configured `idleAction` (pause or terminate)

Paused workspaces can be instantly resumed. Terminated workspaces lose all non-persistent state.

### Workspace Status

```yaml
status:
  phase: Running    # Pending | Running | Paused | Terminating
  url: https://research-notebook.gryvia.example.com
  startTime: "2024-01-15T10:30:00Z"
  lastActivity: "2024-01-15T14:22:00Z"
  idleTime: 8m
  gpuUtilization: 45.2
  costAccumulated: 12.50
```

---

## Best Practices

### Hyperparameter Tuning

- Start with random search to identify promising regions, then switch to Bayesian for refinement.
- Use ASHA early stopping for large trial budgets to avoid wasting GPU time on poor configurations.
- Set cost guardrails (`budget.maxCostUSD`) to prevent runaway experiments.

### Pipelines

- Enable checkpointing on long-running training steps so retries resume from the last checkpoint.
- Use conditional execution to skip expensive steps when upstream quality is insufficient.
- Keep data preprocessing steps on CPU-only to avoid wasting GPU resources.

### Model Registry

- Tag models with training metadata (dataset, base model, hyperparameters) for reproducibility.
- Require approvals for production promotion in regulated environments.
- Use auto-deploy with canary rollouts to reduce risk of serving regressions.

### Inference Serving

- Start with conservative canary weights (5-10%) and short step intervals.
- Set `minRequests` in success thresholds to avoid promoting on insufficient traffic.
- Enable auto-rollback for all production deployments.

### Workspaces

- Set idle timeouts to avoid accumulating GPU costs during meetings or overnight.
- Use `pause` instead of `terminate` for idle action to preserve your working state.
- Mount shared dataset PVCs read-only to avoid accidental modification.

---

## Support

- **Documentation**: https://gryvia.io/docs
- **Issues**: https://github.com/zyvorai/gryvia/issues
- **Discussions**: https://github.com/zyvorai/gryvia/discussions
- **Slack**: #gryvia-users

---

*Gryvia - Enterprise GPU Infrastructure Management*
