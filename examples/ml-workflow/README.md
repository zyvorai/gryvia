# ML Workflow Examples

Example YAML manifests for TensorReaper ML workflow CRDs.

## Examples

- **[hyperparameter-tuning.yaml](hyperparameter-tuning.yaml)** - `FabricAutoTuner` running Bayesian HPO for ResNet with 50 trials
- **[training-pipeline.yaml](training-pipeline.yaml)** - `FabricWorkflow` with a 4-step DAG: preprocess, train, evaluate, deploy
- **[model-registry.yaml](model-registry.yaml)** - `FabricModelRegistry` registering a trained model with artifacts and metadata
- **[inference-service.yaml](inference-service.yaml)** - `FabricInferenceService` deploying vLLM with canary traffic splitting (90/10)
- **[gpu-workspace.yaml](gpu-workspace.yaml)** - `FabricWorkspace` creating a JupyterLab environment with 2 A100 GPUs

## Usage

```bash
kubectl apply -f hyperparameter-tuning.yaml
kubectl apply -f training-pipeline.yaml
kubectl apply -f model-registry.yaml
kubectl apply -f inference-service.yaml
kubectl apply -f gpu-workspace.yaml
```
