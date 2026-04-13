# Job Management Guide

Complete guide to submitting and managing AI workloads with TensorReaper.

## Job Basics

### Creating a Job

```yaml
apiVersion: tensorreaper.ai/v1
kind: FabricAIJob
metadata:
  name: my-training-job
  namespace: default
  labels:
    team: ml-research
    project: llama-training
spec:
  framework: pytorch
  resources:
    gpuType: A100-80G
    gpuCount: 8
    memory: 512Gi
    cpu: 64
  image: nvcr.io/nvidia/pytorch:24.01-py3
  command:
    - python
    - train.py
    - --model=llama-7b
    - --data=/data/openwebtext
```

### Submitting Jobs

**Using kubectl:**
```bash
kubectl apply -f job.yaml
```

**Using tensorreaper:**
```bash
# Submit from file
tensorreaper submit job.yaml

# Submit with parameters
tensorreaper submit --template pytorch-ddp \
  --param model=llama-7b \
  --param gpu-count=8

# Submit and wait
tensorreaper submit job.yaml --wait
```

## Job Lifecycle

### States

1. **Pending**: Job is queued, waiting for resources
2. **Running**: Job is executing
3. **Succeeded**: Job completed successfully
4. **Failed**: Job terminated with error
5. **Cancelled**: Job was manually cancelled

### Monitoring Jobs

```bash
# List all jobs
tensorreaper list

# Get job status
tensorreaper status my-training-job

# Watch job progress
watch tensorreaper status my-training-job

# Get detailed info
kubectl describe fabricaijob my-training-job
```

### Viewing Logs

```bash
# Stream logs
tensorreaper logs my-training-job

# Follow logs
tensorreaper logs my-training-job -f

# Get logs from specific replica
tensorreaper logs my-training-job --replica 0

# Save logs to file
tensorreaper logs my-training-job > training.log
```

### Cancelling Jobs

```bash
# Cancel job
tensorreaper cancel my-training-job

# Delete job
kubectl delete fabricaijob my-training-job
```

## Distributed Training

### PyTorch DDP

```yaml
spec:
  framework: pytorch
  distributed:
    enabled: true
    strategy: ddp
    gpusPerNode: 8
  resources:
    gpuType: A100-80G
    gpuCount: 8
  command:
    - torchrun
    - --nproc_per_node=8
    - --nnodes=1
    - train.py
```

### Multi-Node Training

```yaml
spec:
  framework: pytorch
  distributed:
    enabled: true
    strategy: ddp
    nodes: 4          # 4 nodes
    gpusPerNode: 8    # 8 GPUs per node = 32 total
  resources:
    gpuType: A100-80G
    gpuCount: 8
  command:
    - torchrun
    - --nproc_per_node=8
    - --nnodes=4
    - --node_rank=$NODE_RANK
    - --master_addr=$MASTER_ADDR
    - --master_port=$MASTER_PORT
    - train.py
```

TensorReaper automatically sets `WORLD_SIZE` to `nodes * gpusPerNode` (e.g., 32
for the example above), along with `MASTER_ADDR`, `MASTER_PORT`, and NCCL
environment variables.

### DeepSpeed

```yaml
spec:
  framework: pytorch
  distributed:
    enabled: true
    strategy: deepspeed
    nodes: 2
    gpusPerNode: 8
  resources:
    gpuType: A100-80G
    gpuCount: 8
  command:
    - deepspeed
    - --num_gpus=8
    - train.py
    - --deepspeed
    - --deepspeed_config=ds_config.json
```

## Resource Configuration

### GPU Selection

```yaml
resources:
  # Specific GPU type
  gpuType: A100-80G
  gpuCount: 8

  # Or use selectors
  gpuSelector:
    memory: ">=40GB"
    computeCapability: ">=8.0"
```

### Memory and CPU

```yaml
resources:
  memory: 512Gi  # Total memory
  cpu: 64        # Total CPU cores

  # Or per-GPU
  memoryPerGPU: 64Gi
  cpuPerGPU: 8
```

### Storage

```yaml
volumeMounts:
  - name: dataset
    mountPath: /data
  - name: checkpoints
    mountPath: /checkpoints

volumes:
  - name: dataset
    persistentVolumeClaim:
      claimName: shared-dataset
  - name: checkpoints
    persistentVolumeClaim:
      claimName: my-checkpoints
```

## Environment Configuration

### Environment Variables

```yaml
env:
  - name: NCCL_DEBUG
    value: "INFO"
  - name: MLFLOW_TRACKING_URI
    value: "http://mlflow:5000"
  - name: WANDB_API_KEY
    valueFrom:
      secretKeyRef:
        name: wandb-secret
        key: api-key
```

### Secrets

```yaml
envFrom:
  - secretRef:
      name: training-secrets
```

## Job Templates

### Using Templates

```bash
# List templates
tensorreaper templates list

# View template
tensorreaper templates show pytorch-ddp

# Submit from template
tensorreaper submit --template pytorch-ddp \
  --param model=llama-7b \
  --param dataset=/data/openwebtext \
  --param gpu-count=8 \
  --param batch-size=32
```

### Common Templates

- `pytorch-ddp`: PyTorch distributed training
- `tensorflow-distributed`: TensorFlow multi-worker
- `deepspeed-training`: DeepSpeed ZeRO optimization
- `lora-finetuning`: Parameter-efficient fine-tuning
- `vllm-inference`: High-throughput inference

## Advanced Features

### Checkpointing

```yaml
spec:
  checkpointing:
    enabled: true
    interval: 3600  # Save every hour
    path: /checkpoints
    maxCheckpoints: 5
```

### Auto-restart

```yaml
spec:
  restartPolicy: OnFailure
  backoffLimit: 3
```

### Preemption

```yaml
spec:
  priority: high  # high, medium, low
  preemptible: false
```

### Spot Instances

```yaml
spec:
  spot:
    enabled: true
    maxPrice: 15.0  # Max $/hour per GPU
```

## Cost Management

### Cost Estimation

```bash
# Estimate job cost
tensorreaper cost estimate --job my-training-job

# Track running costs
tensorreaper cost --job my-training-job
```

### Budget Alerts

```yaml
metadata:
  annotations:
    tensorreaper.ai/budget-alert: "100.00"
    tensorreaper.ai/alert-email: "team@company.com"
```

## Performance Optimization

### Profile Jobs

```bash
# Profile GPU utilization
python3 tools/profiler.py --job my-training-job \
  --gpu-type A100-80G \
  --gpu-count 8

# Get recommendations
tensorreaper profile my-training-job
```

### Common Optimizations

1. **Increase Batch Size**: Improve GPU utilization
2. **Mixed Precision**: Enable AMP for 2-3x speedup
3. **Gradient Accumulation**: Simulate larger batches
4. **Data Loading**: Increase num_workers
5. **Distributed Training**: Scale to multiple GPUs

## Troubleshooting

### Job Stuck in Pending

```bash
# Check scheduling events
kubectl describe fabricaijob my-training-job

# Check GPU availability
tensorreaper cluster nodes

# Check quota
tensorreaper quota my-team
```

### Job Failed

```bash
# Check logs
tensorreaper logs my-training-job

# Check events
kubectl get events --field-selector involvedObject.name=my-training-job

# Describe job
kubectl describe fabricaijob my-training-job
```

### Out of Memory

```bash
# Reduce batch size
# Enable gradient checkpointing
# Use gradient accumulation
# Switch to larger GPU type
```

### Low GPU Utilization

```bash
# Profile job
python3 tools/profiler.py --job my-training-job

# Common fixes:
# - Increase batch size
# - Increase num_workers in DataLoader
# - Use prefetching
# - Enable pin_memory
```

## Best Practices

1. **Resource Requests**: Set appropriate resource requests
2. **Labels**: Add team and project labels
3. **MLflow**: Enable experiment tracking
4. **Checkpointing**: Save checkpoints regularly
5. **Monitoring**: Monitor GPU utilization
6. **Cost Tracking**: Track job costs
7. **Documentation**: Document job parameters
8. **Version Control**: Version control job configs

## Examples

See [Job Examples](../../examples/jobs/) for complete examples:
- PyTorch distributed training
- TensorFlow multi-worker
- DeepSpeed ZeRO
- LoRA fine-tuning
- vLLM inference

## Support

- Job Issues: https://github.com/ssahani/tensor-reaper/issues
- Optimization Help: Use profiler tool
- Documentation: https://github.com/ssahani/tensor-reaper/docs
