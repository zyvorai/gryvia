## Gryvia Examples

This directory contains example configurations for running AI workloads on Gryvia.

### Training Examples

#### Simple Single-GPU Training
```bash
kubectl apply -f training/simple-pytorch-training.yaml
kubectl get gryviaaijob pytorch-simple-training
kubectl logs -f $(kubectl get pod -l gryvia.io/job=pytorch-simple-training -o name)
```

#### Multi-GPU Distributed Training
```bash
# Create PVCs for data and checkpoints first
kubectl apply -f distributed/multi-gpu-training.yaml
kubectl get gryviaaijob distributed-llama-training
```

### Inference Examples

#### LLM Inference with vLLM
```bash
kubectl apply -f inference/llm-inference.yaml
kubectl get gryviaaijob llama-inference

# Port forward to access the API
kubectl port-forward svc/llama-inference-headless 8000:8000

# Test the inference endpoint
curl http://localhost:8000/v1/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llama-70b",
    "prompt": "Explain quantum computing in simple terms:",
    "max_tokens": 100
  }'
```

### GPU Node Configuration

#### Register a GPU Node
```bash
kubectl apply -f gpu-nodes/h100-node.yaml
kubectl get gryviagpunode
kubectl describe gryviagpunode gpu-h100-01
```

### Storage Configuration

#### Setup VAST Storage
```bash
kubectl apply -f storage/vast-storage.yaml
kubectl get gryviastorage
```

### Network Configuration

#### Setup RDMA Network
```bash
kubectl apply -f network/rdma-network.yaml
kubectl get gryvianetwork
```

### Quotas

#### Set Team GPU Quota
```bash
kubectl apply -f quotas/team-quota.yaml
kubectl get gryviaquota
```

## Best Practices

1. **Always specify GPU type** for predictable performance
2. **Use RDMA networking** for multi-GPU training
3. **Mount fast storage** for data-intensive workloads
4. **Set resource limits** to prevent resource exhaustion
5. **Use priorities** to manage job scheduling
6. **Enable retries** for fault tolerance

## Monitoring

Watch job progress:
```bash
kubectl get gryviaaijob -w
```

Check GPU utilization:
```bash
kubectl top node -l gryvia.io/gpu=true
```

View metrics in Grafana:
```bash
kubectl port-forward -n gryvia-system svc/gryvia-observability-grafana 3000:80
# Open http://localhost:3000
```
