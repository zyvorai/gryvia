# Model factory: fine-tune and serve new open models automatically

A `GryviaModelWatch` polls the Hugging Face Hub. For each new model that passes its filters it runs a
`GryviaWorkflow`: download the weights to a shared PVC, LoRA fine-tune them on your data, evaluate, and register the
result in the `GryviaModelRegistry`. The registry's `promotionPolicy` moves the version to production only if its
evaluation score beats the version being served, and `servingConfig.serviceName` rolls it out as a canary of the
shared vLLM service, promoted after it stays healthy or rolled back.

> **Status.** The controllers are unit-tested and the control-plane flow (e2e with a stand-in hub and busybox steps) passed on a k3s cluster without GPUs.
> The scripts here are unit-tested without a GPU. Nothing has run on GPUs, with real model weights or with a real
> vLLM image. See [docs/model-factory.md](../../docs/model-factory.md).

## Files

- [model-watch.yaml](model-watch.yaml): the watch, its token Secret and the four-step workflow template
- [download.py](download.py): `snapshot_download` of one revision into `/models/cache`, skipped when already present
- [finetune_lora.py](finetune_lora.py): TRL/PEFT LoRA (or `--qlora`) fine-tune; merges the adapter and saves safetensors
- [evaluate.py](evaluate.py): lm-evaluation-harness tasks plus an optional exact-match JSONL; reports `score`
- [outputs.py](outputs.py): writes step outputs to the termination message the workflow reads
- [Dockerfile](Dockerfile) and [requirements.txt](requirements.txt): one image for all three steps
- [test_model_factory.py](test_model_factory.py): unit tests (no GPU, no network)

## Run it

```bash
docker build -t registry.example.com/gryvia-model-factory:1.0 examples/model-factory
docker push registry.example.com/gryvia-model-factory:1.0

helm upgrade gryvia helm/gryvia --reuse-values --set aiOperator.modelWatch.enabled=true

# PVCs "models" (ReadWriteMany) and "datasets" (with chat.jsonl and eval.jsonl) must exist in ml-team.
kubectl apply -f examples/model-factory/model-watch.yaml
gryvia models watch runs open-llms -n ml-team      # or: kubectl get gryviamodelwatch open-llms -n ml-team -o yaml
kubectl get gryviaworkflows,gryviamodelregistries,gryviainferenceservices -n ml-team
```

The first poll only records the models that already exist (`Baseline`); set `includeExisting: true` to fine-tune
them too. Each step's outputs are visible in `kubectl get gryviaworkflow <run> -o jsonpath='{.status.stepStatuses}'`.

The serving args assume the image's entrypoint is the vLLM OpenAI server (`--model`, `--port`); check the
entrypoint of the vLLM image you configure with `--inference-image-vllm`.

## Test the scripts

```bash
cd examples/model-factory && python3 test_model_factory.py
```
