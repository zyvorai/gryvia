# Complete Gryvia Setup Example

This directory contains a complete example setup for Gryvia with all components configured.

## Prerequisites

- Kubernetes cluster with GPU nodes
- kubectl configured
- Helm 3.x installed
- 3+ GPU nodes with NVIDIA drivers installed

> **Status: partly stale, use with care.** The only manifest in this directory is `production-deployment.yaml` (storage, network, GPU node, quotas, two jobs and a PVC). The other files that the original walkthrough and `deploy.sh` refer to (`gpu-nodes/`, `storage-config.yaml`, `network-config.yaml`, `quotas/`, `jobs/`, `example-job.yaml`) do not exist, and `deploy.sh` applies per-operator `config/deployment.yaml` files that exist only for the storage, network and quota operators (not gpu-operator or ai-operator), so **`deploy.sh` does not work as written**. Nothing here has been run on real GPU, VAST or InfiniBand hardware. The supported install path is the Helm chart (see the root README and `scripts/install.sh`, or `scripts/install-k3s-gpu.sh` for a single GPU server).

## Quick Start

```bash
# 1. Install Gryvia with the Helm chart (see the root README for the exact command)
helm install gryvia ./helm/gryvia --namespace gryvia-system --create-namespace \
  --set auth.apiKey='a-long-random-secret'

# 2. Verify installation
kubectl get pods -n gryvia-system

# 3. Review and edit the example resources, then apply them.
#    The storage, network and GPU node entries point at example endpoints and hardware.
kubectl apply -f production-deployment.yaml

# 4. Check job status
kubectl get gryviaaijobs -n default
```

## What's Included

`production-deployment.yaml` contains, as one multi-document file:

- **Storage**: a `GryviaStorage` for a VAST Data backend (example endpoint, 500Ti)
- **Network**: a `GryviaNetwork` for RDMA/InfiniBand
- **GPU node**: a `GryviaGpuNode` declaration
- **Quotas**: two `GryviaQuota` objects with budgets
- **Jobs**: two example `GryviaAIJob` objects and a PVC
- **Monitoring**: not included here; see `monitoring/` and the observability Helm chart

The endpoints, hardware and capacities are placeholders. The step-by-step section that used to describe separate per-component directories has been removed because those files do not exist.

## Accessing the System

### Web UI

```bash
kubectl port-forward -n gryvia-system svc/gryvia-ui 8443:443
```

Open https://localhost:8443 (self-signed certificate; accept the browser warning)

### CLI

```bash
# Install CLI
curl -L https://github.com/zyvorai/gryvia/releases/latest/download/gryvia-linux-amd64 -o gryvia
chmod +x gryvia
sudo mv gryvia /usr/local/bin/

# Check cluster status
gryvia cluster

# List jobs
gryvia list jobs

# Get quota information
gryvia quota
```

### Grafana Dashboards

```bash
kubectl port-forward -n gryvia-system svc/prometheus-grafana 3000:80   # service name depends on how you installed Grafana
```

Open http://localhost:3000
- Username: admin
- Password: (get from secret)

```bash
kubectl get secret -n gryvia-system prometheus-grafana -o jsonpath="{.data.admin-password}" | base64 -d
```

## Example Workflows

### Submit a Training Job

```yaml
apiVersion: gryvia.io/v1alpha1
kind: GryviaAIJob
metadata:
  name: llama-training
  namespace: default
  labels:
    team: nlp
spec:
  type: training
  distributed:
    framework: pytorch
    enabled: true
    nodes: 1
    gpusPerNode: 8
  gpus: 8
  gpuType: H100
  resources:
    requests:
      cpu: "64"
      memory: 512Gi
  image: nvcr.io/nvidia/pytorch:24.01-py3
  command:
    - torchrun
    - --nproc_per_node=8
    - train.py
    - --model=llama-70b
    - --batch-size=4
  env:
    - name: NCCL_DEBUG
      value: INFO
```

```bash
kubectl apply -f llama-training.yaml
gryvia status llama-training
gryvia logs llama-training
```

### Monitor GPU Utilization

```bash
# Real-time GPU metrics
watch kubectl get gryviagpunodes -o custom-columns=NAME:.metadata.name,TYPE:.spec.gpuType,GPUS:.spec.gpuCount,PHASE:.status.phase

# Detailed node metrics
kubectl describe gryviagpunode gpu-node-01
```

### Check Team Quota Usage

```bash
# Via CLI
gryvia quota

# Via kubectl
kubectl get gryviaquotas

# Detailed quota info
kubectl describe gryviaquota team-ml-research
```

## Troubleshooting

### Operators not starting

```bash
# Check operator logs
kubectl logs -n gryvia-system -l app=gryvia-gpu-operator --tail=50

# Check RBAC
kubectl auth can-i --list --as=system:serviceaccount:gryvia:gryvia-gpu-operator
```

### Jobs stuck in Pending

```bash
# Check quota limits
kubectl get gryviaquotas

# Check GPU availability
kubectl get gryviagpunodes

# Check job events
kubectl describe gryviaaijob <job-name>
```

### Storage not mounting

```bash
# Check storage operator
kubectl logs -n gryvia-system -l app=gryvia-storage-operator

# Check CSI driver
kubectl get pods -n gryvia-system | grep vast-csi

# Verify storage backend
kubectl get gryviastorage
```

## Cleanup

```bash
# Delete all jobs
kubectl delete gryviaaijobs --all

# Delete example resources
kubectl delete -f .

# Uninstall Gryvia
helm uninstall gryvia -n gryvia-system
kubectl delete namespace gryvia-system
```

## Production Considerations

1. **Security**
   - Replace credential placeholders in `production-deployment.yaml` (marked `<REPLACE_WITH_*>`) with real values or use a secrets manager (e.g., Vault, Sealed Secrets)
   - Enable RBAC for team isolation
   - Use network policies
   - Enable audit logging
   - Set up authentication (OIDC, LDAP)
   - Pin all container images to specific version tags (never use `:latest`)

2. **High Availability**
   - Run 3+ operator replicas
   - Use pod anti-affinity
   - Configure leader election
   - Set up backup/restore

3. **Monitoring**
   - Configure Alertmanager
   - Set up Slack/PagerDuty notifications
   - Enable audit logs
   - Monitor GPU health continuously

4. **Capacity Planning**
   - Monitor queue depths
   - Track quota utilization
   - Plan for peak usage
   - Budget for growth

5. **Cost Optimization**
   - Set appropriate quotas
   - Use spot instances where applicable
   - Monitor idle resources
   - Implement auto-scaling policies

## Support

- Documentation: https://github.com/zyvorai/gryvia
- Issues: https://github.com/zyvorai/gryvia/issues
- Discussions: https://github.com/zyvorai/gryvia/discussions
