# Frequently Asked Questions

Common questions about Gryvia and GPU infrastructure management.

## General

### What is Gryvia?

Gryvia is an open, Kubernetes-native GPU compute platform for AI/ML infrastructure. It provides:
- GPU resource management and scheduling
- Cost optimization and budget controls
- Multi-tenancy with quotas
- Advanced features like job dependencies, auto-scaling, and health monitoring

### Who should use Gryvia?

- **ML Engineers**: Submit training jobs, manage experiments
- **Platform Teams**: Manage GPU infrastructure at scale
- **Finance**: Track and optimize GPU compute costs
- **Executives**: Get insights and forecasts on GPU utilization

### How is Gryvia different from Kubeflow?

| Feature | Gryvia | Kubeflow |
|---------|-----------|----------|
| Focus | GPU infrastructure management | ML pipelines |
| Multi-tenancy | Built-in with quotas | Basic |
| Cost management | Advanced | None |
| GPU sharing | MIG, fractional, time-slicing | Limited |
| Scheduling | 13 policies + ML-driven | Basic |
| Reservations | Yes | No |

**Use together**: Gryvia for infrastructure, Kubeflow for ML pipelines.

---

## Getting Started

### How do I submit my first job?

```bash
# Install CLI
curl -sSL https://gryvia.io/install.sh | bash

# Submit job
kfctl submit job.yaml --gpu-type A100-80G --gpu-count 8

# Check status
kfctl job status my-job

# View logs
kfctl job logs my-job
```

### What GPU types are supported?

All NVIDIA GPUs:
- **H100**: Latest, highest performance
- **A100-80G / A100-40G**: Most popular for training
- **V100**: Good for medium workloads
- **T4**: Great for inference and development
- **MIG instances**: Fractional GPUs (1g.10gb, 2g.20gb, 3g.40gb, 7g.80gb)

### Can I use my existing Kubernetes cluster?

Yes! Gryvia is deployed on Kubernetes:

```bash
# Add Helm repo
helm repo add gryvia https://zyvorai.github.io/gryvia/charts

# Install
helm install gryvia gryvia/gryvia \
  --namespace gryvia-system \
  --create-namespace
```

---

## Resource Management

### How do quotas work?

Quotas limit resource usage per team:

```yaml
quotas:
  gpuHours:
    monthly: 2000  # 2000 GPU hours per month
  costUSD:
    monthly: 50000  # $50k per month
  concurrentGPUs: 128  # Max 128 GPUs at once
```

When quota exceeded, new jobs are blocked.

### Can I request more quota?

Yes:

```bash
kfctl quota request-increase \
  --team ml-research \
  --amount 10000 \
  --reason "Critical deadline"
```

Requires approval from admin/finance.

### What happens if I exceed my budget?

Depends on enforcement policy:
- **Warn**: Email alert, jobs continue
- **Block**: New jobs blocked
- **Throttle**: Jobs run at lower priority

Configure per budget:

```yaml
enforcement:
  enabled: true
  action: block  # or warn, throttle
  gracePeriod: 2h
```

---

## Cost Optimization

### How can I reduce costs?

**Top 5 ways to save:**

1. **Use Spot Instances** (40-70% savings)
   ```bash
   kfctl job submit --spot
   ```

2. **Enable MIG for Dev/Inference** (86% savings)
   ```bash
   kfctl gpu-sharing enable --profile all-1g.10gb
   ```

3. **Right-Size Resources**
   ```bash
   kfctl profile my-job
   kfctl optimize my-job
   ```

4. **Use Reservations** (15-25% discount)
   ```bash
   kfctl reservation create --duration 30d --exclusive
   ```

5. **Enable Auto-Scaling** (eliminate idle)
   ```bash
   kfctl autoscale enable
   ```

### How much can I save with spot instances?

**Example:**
- On-demand: 32x A100 = $768/hour × 4 hours = $3,072
- Spot: 32x A100 = $461/hour × 4 hours = $1,844
- **Savings: $1,228 (40%)**

Spot instances can be interrupted, so enable checkpointing:

```yaml
checkpointing:
  enabled: true
  frequency: 10m
```

### What is MIG and when should I use it?

**MIG** (Multi-Instance GPU) splits A100/H100 into smaller instances:

**Use MIG for:**
- Development (1g.10gb = $3.43/hr vs $24/hr for full GPU)
- Inference serving
- Small models
- Multiple concurrent jobs

**Don't use MIG for:**
- Large-scale distributed training
- Workloads needing >40GB memory
- Maximum performance requirements

---

## Job Management

### How do I retry failed jobs?

Automatic retry with policy:

```yaml
spec:
  retryPolicy:
    maxRetries: 3
    backoff:
      type: exponential
      initialDelay: 1m
```

Or manual:

```bash
kfctl job retry my-job
```

### Can I checkpoint and resume jobs?

Yes:

```yaml
spec:
  checkpointing:
    enabled: true
    frequency: 10m
    path: /checkpoints

  command:
    - python
    - train.py
    - --checkpoint-dir=/checkpoints
    - --auto-resume
```

Job automatically resumes from last checkpoint on:
- Spot interruption
- Preemption
- Failure

### How do I run distributed training?

```yaml
spec:
  framework: pytorch
  distributed:
    enabled: true
    strategy: ddp
    nodes: 4         # Number of nodes
    gpusPerNode: 8   # GPUs per node

  resources:
    gpuType: A100-80G
    gpuCount: 8  # GPUs per node
```

Gryvia handles:
- Node selection via the GPU-aware scheduler
- `WORLD_SIZE` environment variable (automatically set to `nodes * gpusPerNode`)
- `MASTER_ADDR` and `MASTER_PORT` for rendezvous
- NCCL configuration (including RDMA settings when network is `rdma`)
- Rank assignment via StatefulSet ordinal indices

---

## Performance

### My training is slow. How do I optimize?

```bash
# 1. Profile the job
kfctl profile my-job

# 2. Get recommendations
kfctl optimize my-job
```

**Common fixes:**
- Enable mixed precision (2-3x speedup)
- Increase batch size
- Optimize data loading
- Enable torch.compile
- Use GPUDirect RDMA

See [OPERATIONAL_PLAYBOOKS.md](OPERATIONAL_PLAYBOOKS.md#performance-issues) for details.

### How do I enable GPUDirect RDMA?

```yaml
spec:
  network:
    gpuDirectRequired: true

  env:
    - name: NCCL_NET_GDR_LEVEL
      value: "5"
```

Requires InfiniBand or RoCE network.

### What is good GPU utilization?

**Targets:**
- **Training**: >85% GPU utilization
- **Inference**: >70% (varies by latency requirements)
- **Development**: >50% (intermittent usage expected)

Check with:

```bash
kfctl metrics gpu-utilization my-job
```

---

## Multi-Tenancy

### How do I create a team?

```bash
kfctl tenant create ml-research \
  --display-name "ML Research Team" \
  --quota-gpus 128 \
  --quota-cost 50000
```

### How do I add team members?

```bash
kfctl tenant add-member ml-research alice --role admin
kfctl tenant add-member ml-research bob --role member
kfctl tenant add-member ml-research charlie --role viewer
```

**Roles:**
- **Admin**: Manage team, budgets, members
- **Member**: Submit jobs, view team resources
- **Viewer**: Read-only access

### Can teams share GPUs?

Yes, configure sharing policy:

```yaml
spec:
  strategy: time-slicing
  timeSlicing:
    maxPodsPerGPU: 4
  qos:
    isolationLevel: memory  # Memory isolated between tenants
```

---

## Security & Compliance

### Is Gryvia SOC2 compliant?

Yes, with audit trail enabled:

```yaml
apiVersion: gryvia.io/v1alpha1
kind: GryviaAudit
spec:
  compliance:
    frameworks: [SOC2, ISO27001]
  retention:
    duration: 7y
  events:
    # All required events tracked
```

Features:
- 7-year audit retention
- Comprehensive event logging
- Automated compliance reports
- Anomaly detection

### How is data encrypted?

- **At rest**: AES-256 encryption (storage layer)
- **In transit**: TLS 1.3 for all API calls
- **Secrets**: Kubernetes secrets (optionally Vault)
- **Volumes**: Encrypted by storage provider

### Can I use Gryvia for HIPAA workloads?

Yes:

```yaml
spec:
  compliance:
    frameworks: [HIPAA]
    dataClassification: restricted
  retention:
    duration: 6y
```

Requirements:
- Enable audit trail
- Configure network isolation
- Use encrypted storage
- Enable access controls

---

## Advanced Features

### What are job hooks?

Hooks execute actions at job lifecycle events:

```yaml
apiVersion: gryvia.io/v1alpha1
kind: GryviaJobHook
spec:
  trigger: post-completion
  action:
    type: webhook
    webhook:
      url: https://api.example.com/notify
```

**Use cases:**
- Upload model to registry
- Send Slack notification
- Trigger downstream jobs
- Clean up resources

### How does auto-scaling work?

Queue-based auto-scaling:

```yaml
spec:
  scaleUpPolicy:
    pendingJobs: 10  # Scale up if >10 pending
    queueTimeMinutes: 30  # Or queue time >30min
    increment: 2  # Add 2 nodes at a time

  scaleDownPolicy:
    idleTimeMinutes: 15  # Scale down after 15min idle
    decrement: 1
```

### Can I reserve GPUs in advance?

Yes:

```bash
kfctl reservation create paper-deadline \
  --gpu-type H100 \
  --gpu-count 64 \
  --start "2024-02-01 00:00" \
  --end "2024-02-05 23:59" \
  --exclusive
```

Benefits:
- Guaranteed availability
- 15-25% discount
- No queue time
- SLA guarantees

---

## Troubleshooting

### My job is stuck in Pending

The scheduler reports why no nodes matched when scheduling fails. Check the
job's conditions for a message like:
`no nodes meet the job requirements (gpuType="H100", gpus=8, network="rdma", 12 nodes evaluated)`

**Check:**

```bash
# 1. Why is it pending? (look at conditions for scheduler error details)
kubectl describe gryviaaijob my-job

# 2. Check capacity
kfctl cluster status

# 3. Check quota
kfctl quota status --team my-team

# 4. Check budget
kfctl budget status --team my-team

# 5. View queue position
kfctl queue status
```

### Jobs keep failing

```bash
# 1. View logs
kfctl job logs my-job --tail 100

# 2. Check events
kubectl describe gryviaaijob my-job

# 3. Common issues:
# - OOM: Increase memory or use larger GPU
# - Image pull error: Check image name and credentials
# - Quota exceeded: Request more quota
# - GPU error: Check node health
```

### How do I get support?

1. **Documentation**: https://gryvia.io/docs
2. **GitHub Issues**: https://github.com/zyvorai/gryvia/issues
3. **Slack**: #gryvia-users
4. **Email**: support@gryvia.io (Enterprise only)

---

## Best Practices

### Job Submission

```yaml
# ✓ Good
spec:
  priorityClassName: normal  # Set appropriate priority
  retryPolicy:
    maxRetries: 3
  checkpointing:
    enabled: true
  resources:
    gpuType: A100-80G
    gpuCount: 8
    memory: 512Gi  # Realistic memory
    cpu: 64

# ✗ Bad
spec:
  priorityClassName: high  # Don't abuse high priority
  resources:
    gpuType: H100  # Don't always use most expensive
    gpuCount: 128  # Don't over-request
    memory: 2Ti  # Don't over-provision
```

### Cost Management

1. **Enable budgets**: Set monthly limits
2. **Monitor regularly**: Review weekly
3. **Use spot when possible**: 40-70% savings
4. **Right-size resources**: Profile first
5. **Enable auto-scaling**: No idle resources

### Performance

1. **Profile before optimizing**: Data beats guessing
2. **Enable mixed precision**: Easy 2x speedup
3. **Optimize data loading**: Often the bottleneck
4. **Use topology-aware placement**: For multi-node
5. **Monitor continuously**: Catch regressions early

---

## Migration

### From Slurm

```bash
# Export Slurm jobs
squeue -u $USER -o "%i,%j,%N,%p" > slurm-jobs.csv

# Convert to Gryvia
kfctl import slurm slurm-jobs.csv

# Or manually:
srun --gres=gpu:8 python train.py  # Slurm
kfctl submit job.yaml --gpu-count 8  # Gryvia
```

### From Kubernetes Jobs

```bash
# Migrate existing K8s jobs
kfctl import kubernetes job.yaml

# Or use directly:
kubectl apply -f gryviaaijob.yaml
```

---

## Limits and Quotas

### System Limits

- Max GPUs per job: 512
- Max concurrent jobs per user: 1000
- Max job duration: 30 days
- Max checkpoint size: 1TB
- Max dataset size: 10TB

### API Rate Limits

- Job submissions: 100/minute
- Status queries: 1000/minute
- Metrics queries: 500/minute

### Resource Limits

Depends on cluster size and quotas. Contact admin for increases.

---

## Glossary

- **MIG**: Multi-Instance GPU
- **GPUDirect**: NVIDIA peer-to-peer GPU communication
- **NCCL**: NVIDIA Collective Communications Library
- **DDP**: Distributed Data Parallel (PyTorch)
- **SLA**: Service Level Agreement
- **QoS**: Quality of Service
- **RDMA**: Remote Direct Memory Access

---

*Still have questions? See [ADVANCED_FEATURES.md](ADVANCED_FEATURES.md) or ask in Slack #gryvia-users*
