# Job Templates

Pre-configured templates for common ML workloads.

## Available Templates

### Training Templates
- [PyTorch DDP Training](#pytorch-ddp-training)
- [TensorFlow Distributed](#tensorflow-distributed)
- [JAX Multi-Host](#jax-multi-host)
- [DeepSpeed Training](#deepspeed-training)

### Fine-Tuning Templates
- [LoRA Fine-Tuning](#lora-fine-tuning)
- [Full Fine-Tuning](#full-fine-tuning)
- [RLHF Training](#rlhf-training)

### Inference Templates
- [vLLM Inference Server](#vllm-inference)
- [TensorRT-LLM](#tensorrt-llm)
- [Triton Inference Server](#triton-inference)

### Data Processing Templates
- [Dataset Preprocessing](#dataset-preprocessing)
- [Data Validation](#data-validation)

## Usage

### Using kfctl

```bash
# List available templates
kfctl templates list

# Create job from template
kfctl submit --template pytorch-ddp \
  --param model=llama-7b \
  --param dataset=openwebtext \
  --param gpu-count=8

# View template
kfctl templates show pytorch-ddp
```

### Using kubectl

```bash
# Apply template directly
kubectl apply -f examples/templates/pytorch-ddp.yaml

# Customize with kustomize
kubectl apply -k examples/templates/pytorch-ddp/
```

## PyTorch DDP Training

Distributed data parallel training with PyTorch.

**Template:** `pytorch-ddp.yaml`

```yaml
apiVersion: kubefabric.io/v1
kind: FabricAIJob
metadata:
  name: pytorch-ddp-template
spec:
  framework: pytorch
  distributed:
    enabled: true
    strategy: ddp
    worldSize: ${GPU_COUNT}
  resources:
    gpuType: ${GPU_TYPE:-A100-80G}
    gpuCount: ${GPU_COUNT:-8}
    memory: ${MEMORY:-512Gi}
    cpu: ${CPU:-64}
  image: nvcr.io/nvidia/pytorch:24.01-py3
  command:
    - torchrun
    - --nproc_per_node=${GPU_COUNT}
    - --nnodes=1
    - train.py
    - --model=${MODEL}
    - --data=${DATASET}
    - --batch-size=${BATCH_SIZE:-32}
    - --lr=${LEARNING_RATE:-3e-4}
  env:
    - name: NCCL_DEBUG
      value: "INFO"
    - name: MLFLOW_TRACKING_URI
      value: "http://mlflow:5000"
```

**Parameters:**
- `MODEL`: Model architecture (e.g., llama-7b, gpt-2)
- `DATASET`: Dataset path or name
- `GPU_COUNT`: Number of GPUs (default: 8)
- `GPU_TYPE`: GPU type (default: A100-80G)
- `BATCH_SIZE`: Batch size per GPU (default: 32)
- `LEARNING_RATE`: Learning rate (default: 3e-4)

**Example:**

```bash
kfctl submit --template pytorch-ddp \
  --param model=llama-13b \
  --param dataset=/data/openwebtext \
  --param gpu-count=16 \
  --param batch-size=16
```

## TensorFlow Distributed

Multi-worker distributed training with TensorFlow.

**Template:** `tensorflow-distributed.yaml`

```yaml
apiVersion: kubefabric.io/v1
kind: FabricAIJob
metadata:
  name: tensorflow-distributed-template
spec:
  framework: tensorflow
  distributed:
    enabled: true
    strategy: multiworker
    workers: ${NUM_WORKERS:-4}
  resources:
    gpuType: ${GPU_TYPE:-A100-40G}
    gpuCount: ${GPU_COUNT:-1}
    memory: ${MEMORY:-64Gi}
    cpu: ${CPU:-16}
  image: tensorflow/tensorflow:2.15.0-gpu
  command:
    - python
    - train.py
    - --distribution_strategy=MultiWorkerMirroredStrategy
  env:
    - name: TF_CONFIG
      value: auto
```

## DeepSpeed Training

Large model training with DeepSpeed ZeRO optimization.

**Template:** `deepspeed-training.yaml`

```yaml
apiVersion: kubefabric.io/v1
kind: FabricAIJob
metadata:
  name: deepspeed-training-template
spec:
  framework: pytorch
  distributed:
    enabled: true
    strategy: deepspeed
    worldSize: ${GPU_COUNT}
  resources:
    gpuType: ${GPU_TYPE:-A100-80G}
    gpuCount: ${GPU_COUNT:-16}
    memory: ${MEMORY:-1Ti}
    cpu: ${CPU:-128}
  image: deepspeed/deepspeed:latest
  command:
    - deepspeed
    - --num_gpus=${GPU_COUNT}
    - train.py
    - --deepspeed
    - --deepspeed_config=/configs/ds_config.json
  volumeMounts:
    - name: deepspeed-config
      mountPath: /configs
  volumes:
    - name: deepspeed-config
      configMap:
        name: deepspeed-config
```

**DeepSpeed Config:**

```json
{
  "train_batch_size": 256,
  "gradient_accumulation_steps": 4,
  "optimizer": {
    "type": "AdamW",
    "params": {
      "lr": 3e-4,
      "betas": [0.9, 0.95],
      "eps": 1e-8,
      "weight_decay": 0.1
    }
  },
  "fp16": {
    "enabled": true,
    "loss_scale": 0,
    "initial_scale_power": 16
  },
  "zero_optimization": {
    "stage": 3,
    "offload_optimizer": {
      "device": "cpu",
      "pin_memory": true
    },
    "offload_param": {
      "device": "cpu",
      "pin_memory": true
    },
    "overlap_comm": true,
    "contiguous_gradients": true,
    "reduce_bucket_size": 5e8,
    "stage3_prefetch_bucket_size": 5e8,
    "stage3_param_persistence_threshold": 1e6
  }
}
```

## LoRA Fine-Tuning

Parameter-efficient fine-tuning with LoRA.

**Template:** `lora-finetuning.yaml`

```yaml
apiVersion: kubefabric.io/v1
kind: FabricAIJob
metadata:
  name: lora-finetuning-template
spec:
  framework: pytorch
  resources:
    gpuType: ${GPU_TYPE:-A100-40G}
    gpuCount: ${GPU_COUNT:-4}
    memory: ${MEMORY:-256Gi}
    cpu: ${CPU:-32}
  image: nvcr.io/nvidia/pytorch:24.01-py3
  command:
    - python
    - finetune.py
    - --base_model=${BASE_MODEL}
    - --dataset=${DATASET}
    - --lora_r=${LORA_R:-8}
    - --lora_alpha=${LORA_ALPHA:-16}
    - --lora_dropout=${LORA_DROPOUT:-0.05}
    - --batch_size=${BATCH_SIZE:-16}
    - --micro_batch_size=${MICRO_BATCH_SIZE:-4}
    - --num_epochs=${NUM_EPOCHS:-3}
    - --learning_rate=${LEARNING_RATE:-3e-4}
  env:
    - name: MLFLOW_TRACKING_URI
      value: "http://mlflow:5000"
```

**Parameters:**
- `BASE_MODEL`: Base model to fine-tune (e.g., llama-7b)
- `DATASET`: Training dataset
- `LORA_R`: LoRA rank (default: 8)
- `LORA_ALPHA`: LoRA alpha (default: 16)
- `LORA_DROPOUT`: Dropout rate (default: 0.05)

## vLLM Inference

High-throughput LLM inference server with vLLM.

**Template:** `vllm-inference.yaml`

```yaml
apiVersion: kubefabric.io/v1
kind: FabricAIJob
metadata:
  name: vllm-inference-template
spec:
  framework: pytorch
  resources:
    gpuType: ${GPU_TYPE:-A100-80G}
    gpuCount: ${GPU_COUNT:-2}
    memory: ${MEMORY:-128Gi}
    cpu: ${CPU:-32}
  image: vllm/vllm-openai:latest
  command:
    - python
    - -m
    - vllm.entrypoints.openai.api_server
    - --model=${MODEL}
    - --tensor-parallel-size=${GPU_COUNT}
    - --max-model-len=${MAX_MODEL_LEN:-4096}
    - --gpu-memory-utilization=${GPU_MEMORY_UTIL:-0.90}
  ports:
    - containerPort: 8000
      name: http
  livenessProbe:
    httpGet:
      path: /health
      port: 8000
    initialDelaySeconds: 60
    periodSeconds: 10
  readinessProbe:
    httpGet:
      path: /health
      port: 8000
    initialDelaySeconds: 30
    periodSeconds: 5
```

**Service:**

```yaml
apiVersion: v1
kind: Service
metadata:
  name: vllm-inference
spec:
  selector:
    job-name: vllm-inference-template
  ports:
    - port: 8000
      targetPort: 8000
```

**Usage:**

```bash
# Deploy inference server
kfctl submit --template vllm-inference \
  --param model=meta-llama/Llama-2-13b-hf \
  --param gpu-count=2

# Test inference
curl http://vllm-inference:8000/v1/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "meta-llama/Llama-2-13b-hf",
    "prompt": "Once upon a time",
    "max_tokens": 100
  }'
```

## Dataset Preprocessing

Large-scale data preprocessing job.

**Template:** `dataset-preprocessing.yaml`

```yaml
apiVersion: kubefabric.io/v1
kind: FabricAIJob
metadata:
  name: dataset-preprocessing-template
spec:
  framework: pytorch
  resources:
    gpuType: ${GPU_TYPE:-T4}
    gpuCount: ${GPU_COUNT:-1}
    memory: ${MEMORY:-64Gi}
    cpu: ${CPU:-32}
  image: nvcr.io/nvidia/pytorch:24.01-py3
  command:
    - python
    - preprocess.py
    - --input=${INPUT_PATH}
    - --output=${OUTPUT_PATH}
    - --num-workers=${NUM_WORKERS:-16}
    - --batch-size=${BATCH_SIZE:-1000}
  volumeMounts:
    - name: input-data
      mountPath: /input
    - name: output-data
      mountPath: /output
```

## Creating Custom Templates

### 1. Define Template

```yaml
# my-template.yaml
apiVersion: kubefabric.io/v1
kind: FabricJobTemplate
metadata:
  name: my-custom-template
spec:
  description: "Custom training template"
  parameters:
    - name: model
      description: "Model name"
      required: true
    - name: gpu-count
      description: "Number of GPUs"
      default: "8"
      type: integer
  template:
    # ... job spec
```

### 2. Register Template

```bash
kubectl apply -f my-template.yaml
```

### 3. Use Template

```bash
kfctl submit --template my-custom-template \
  --param model=custom-model \
  --param gpu-count=16
```

## Best Practices

1. **Resource Requests**: Always set appropriate resource requests
2. **MLflow Integration**: Enable experiment tracking
3. **Health Probes**: Add liveness/readiness probes for services
4. **Volume Mounts**: Use PVCs for data persistence
5. **Environment Variables**: Use ConfigMaps for configuration
6. **Secrets**: Store credentials in Kubernetes Secrets
7. **Cost Optimization**: Choose appropriate GPU types
8. **Monitoring**: Add Prometheus metrics

## Support

- Template Issues: https://github.com/ssahani/kube-fabric/issues
- Custom Templates: https://github.com/ssahani/kube-fabric/discussions
