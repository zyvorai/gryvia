# ML Workflow Examples

Example YAML manifests for Gryvia ML workflow CRDs.

> **Status.** None of these five kinds (`GryviaAutoTuner`, `GryviaWorkflow`, `GryviaModelRegistry`, `GryviaInferenceService`, `GryviaWorkspace`) has a registered controller. The CRDs and gateway/dashboard CRUD exist, so the manifests apply and can be listed, but nothing reconciles them. Treat these as schema examples.

## Examples

- **[hyperparameter-tuning.yaml](hyperparameter-tuning.yaml)** - `GryviaAutoTuner` running Bayesian HPO for ResNet with 50 trials
- **[training-pipeline.yaml](training-pipeline.yaml)** - `GryviaWorkflow` with a 4-step DAG: preprocess, train, evaluate, deploy
- **[model-registry.yaml](model-registry.yaml)** - `GryviaModelRegistry` registering a trained model with artifacts and metadata
- **[inference-service.yaml](inference-service.yaml)** - `GryviaInferenceService` deploying vLLM with canary traffic splitting (90/10)
- **[gpu-workspace.yaml](gpu-workspace.yaml)** - `GryviaWorkspace` creating a JupyterLab environment with 2 A100 GPUs

## Usage

```bash
kubectl apply -f hyperparameter-tuning.yaml
kubectl apply -f training-pipeline.yaml
kubectl apply -f model-registry.yaml
kubectl apply -f inference-service.yaml
kubectl apply -f gpu-workspace.yaml
```
