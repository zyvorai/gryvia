## TensorReaper Complete Deployment Guide

This guide walks through deploying a complete TensorReaper cluster with all six operators on bare metal infrastructure.

## Prerequisites

### Hardware Requirements

**GPU Nodes (minimum 4 nodes):**
- CPU: 2x AMD EPYC or Intel Xeon (96+ cores)
- RAM: 1TB DDR5 ECC
- GPU: 8x NVIDIA H100/A100 per node
- Network: 2x 400Gb/s InfiniBand NDR adapters
- Storage: 2TB NVMe for OS, connected to parallel filesystem

**Storage (VAST/Weka/DDN):**
- 500TB+ usable capacity
- 100GB/s+ aggregate bandwidth
- NFS/NVMe-oF connectivity

**Network Infrastructure:**
- InfiniBand NDR switches (400Gb/s)
- Subnet manager (OpenSM or vendor equivalent)
- 10/25/100Gb/s Ethernet for management

### Software Requirements

- Ubuntu 22.04 LTS (on all nodes)
- Kubernetes 1.30+ cluster
- Helm 3.12+
- kubectl 1.30+
- NVIDIA Driver 535+ with CUDA 12.2+
- NVIDIA OFED 24.01+ (for InfiniBand)

## Step-by-Step Deployment

### 1. Provision Bare Metal Infrastructure

```bash
cd terraform/bare-metal

# Configure your cluster
cp terraform.tfvars.example terraform.tfvars
vim terraform.tfvars  # Edit cluster configuration

# Generate Ansible inventory and configs
terraform apply

# Output files created:
# - inventory.ini
# - kubeadm-config.yaml
# - node configs
```

### 2. Deploy Kubernetes and Dependencies

```bash
cd ../../ansible

# Run full deployment
ansible-playbook -i ../terraform/bare-metal/inventory.ini playbooks/site.yaml

# This installs:
# - Kubernetes cluster (kubeadm)
# - NVIDIA drivers and CUDA
# - RDMA/InfiniBand drivers
# - Container runtime (containerd)
# - CNI (Calico)
# - Multus CNI
```

Deployment takes ~30 minutes. Verify:

```bash
kubectl get nodes
# All nodes should be Ready with correct labels

kubectl get nodes -o json | jq '.items[].status.capacity'
# Should show nvidia.com/gpu resources
```

### 3. Install TensorReaper Operators

#### Option A: Helm Installation (Recommended)

```bash
cd helm/tensorreaper-core

# Install all operators at once
helm install tensorreaper . \
  --namespace tensorreaper-system \
  --create-namespace \
  --set storageOperator.enabled=true \
  --set networkOperator.enabled=true \
  --set quotaOperator.enabled=true

# Verify installation
kubectl get pods -n tensorreaper-system
```

Expected output:
```
NAME                                READY   STATUS    RESTARTS   AGE
gpu-operator-6d8f9b7c5d-x9k2m      1/1     Running   0          2m
ai-operator-7b9f8c6d4e-y8l3n       1/1     Running   0          2m
storage-operator-8c7d9f5e6g-z9m4o  1/1     Running   0          2m
network-operator-9d8e0g6f7h-a0n5p  1/1     Running   0          2m
quota-operator-0e9f1h7g8i-b1o6q    1/1     Running   0          2m
net-intel-operator-1f0g2i8h9j-c2p7r 1/1   Running   0          2m
nvidia-device-plugin-daemonset-... 8/8     Running   0          2m
dcgm-exporter-...                  8/8     Running   0          2m
```

#### Option B: Manual Installation

```bash
# Apply CRDs
kubectl apply -f crds/

# Deploy operators individually
kubectl apply -f operators/gpu-operator/config/
kubectl apply -f operators/ai-operator/config/
kubectl apply -f operators/storage-operator/config/
kubectl apply -f operators/network-operator/config/
kubectl apply -f operators/quota-operator/config/

# Deploy Network Intelligence Operator
kubectl apply -f operators/network-intelligence/config/

# Deploy eBPF Collector DaemonSet
kubectl apply -f collector/deploy/daemonset.yaml
```

### 4. Configure Storage Backend

#### VAST Data Configuration

```bash
# Create storage credentials
kubectl create secret generic vast-credentials \
  -n tensorreaper-system \
  --from-literal=username=admin \
  --from-literal=password=your-vast-password

# Deploy VAST storage
kubectl apply -f - <<EOF
apiVersion: tensorreaper.ai/v1
kind: FabricStorage
metadata:
  name: vast-production
spec:
  backendType: vast
  endpoint: vast-mgmt.example.com
  capacity: 500Ti
  nodeSelector:
    tensorreaper.ai/storage: "true"
  credentials:
    secretName: vast-credentials
    secretNamespace: tensorreaper-system
EOF

# Wait for CSI driver deployment
kubectl wait --for=condition=Ready fabricstorage/vast-production --timeout=300s

# Verify CSI driver
kubectl get pods -n kube-system | grep vast-csi
kubectl get storageclass vast-production
```

### 5. Configure RDMA Network

```bash
# Label nodes with RDMA capability
kubectl label nodes gpu-worker-{01..04} tensorreaper.ai/rdma=true

# Deploy RDMA network
kubectl apply -f - <<EOF
apiVersion: tensorreaper.ai/v1
kind: FabricNetwork
metadata:
  name: rdma-training
spec:
  networkType: rdma
  mtu: 9000
  nodeSelector:
    tensorreaper.ai/rdma: "true"
  rdma:
    mode: infiniband
    devices: [mlx5_0, mlx5_1]
    subnet: 10.100.0.0/16
    gateway: 10.100.0.1
EOF

# Wait for RDMA device plugin
kubectl wait --for=condition=Ready fabricnetwork/rdma-training --timeout=300s

# Verify RDMA resources
kubectl get nodes -o json | jq '.items[].status.allocatable' | grep rdma
```

### 6. Register GPU Nodes

```bash
# GPU nodes are auto-discovered by the GPU operator
# Verify FabricGpuNode resources were created
kubectl get fabricgpunodes

# Expected output:
# NAME            GPU TYPE   COUNT   STATUS    RDMA      NVLINK
# gpu-worker-01   H100       8       Healthy   Enabled   Enabled
# gpu-worker-02   H100       8       Healthy   Enabled   Enabled
# gpu-worker-03   H100       8       Healthy   Enabled   Enabled
# gpu-worker-04   H100       8       Healthy   Enabled   Enabled
```

### 7. Create Team Quotas

```bash
# Create namespaces
kubectl create namespace ml-training
kubectl create namespace cv-training
kubectl create namespace nlp-training

# Deploy quotas
kubectl apply -f examples/quota/team-ml-quota.yaml
kubectl apply -f examples/quota/team-cv-quota.yaml
kubectl apply -f examples/quota/team-nlp-quota.yaml

# Verify quotas
kubectl get fabricquotas
```

### 8. Deploy Monitoring Stack

```bash
cd helm/observability

# Install Prometheus and Grafana
helm install monitoring . \
  --namespace monitoring \
  --create-namespace

# Wait for pods
kubectl wait --for=condition=Ready pods --all -n monitoring --timeout=300s

# Get Grafana password
kubectl get secret -n monitoring monitoring-grafana \
  -o jsonpath='{.data.admin-password}' | base64 -d

# Port-forward to access Grafana
kubectl port-forward -n monitoring svc/monitoring-grafana 3000:80
# Open http://localhost:3000 (admin/<password>)
```

### 9. Run Test Workload

```bash
# Submit distributed training job
kubectl apply -f - <<EOF
apiVersion: tensorreaper.ai/v1
kind: FabricAIJob
metadata:
  name: test-training
  namespace: ml-training
spec:
  framework: pytorch
  distributed:
    enabled: true
    strategy: ddp
    nodes: 2         # 2 nodes
    gpusPerNode: 8   # 8 GPUs per node = 16 GPUs total

  resources:
    gpuType: H100
    gpuCount: 8
    memory: 512Gi
    cpu: 96

  image: nvcr.io/nvidia/pytorch:24.01-py3

  command:
    - python
    - -m
    - torch.distributed.run
    - --nproc_per_node=8
    - --nnodes=2
    - test_script.py

  storage:
    - name: data
      storageClassName: vast-production
      size: 1Ti
      mountPath: /data

  network:
    rdma: rdma-training
EOF

# Monitor job
kubectl get fabricaijob -n ml-training test-training -w

# Check pods
kubectl get pods -n ml-training -l job-name=test-training

# View logs
kubectl logs -n ml-training test-training-0
```

## Verification Checklist

### ✅ Operators Running

```bash
kubectl get pods -n tensorreaper-system
# All 6 operators should be Running
```

### ✅ GPUs Discovered

```bash
kubectl get fabricgpunodes
# Should show all GPU nodes with correct count
```

### ✅ Storage Ready

```bash
kubectl get fabricstorage
# STATUS should be Ready

kubectl get storageclass
# Should include vast-production (or your backend)
```

### ✅ RDMA Configured

```bash
kubectl get fabricnetwork
# STATUS should be Ready

kubectl get pods -n kube-system | grep rdma-device-plugin
# DaemonSet pods should be Running

# Test RDMA from a pod
kubectl run -it --rm rdma-test \
  --image=nvcr.io/nvidia/pytorch:24.01-py3 \
  --overrides='{"metadata":{"annotations":{"k8s.v1.cni.cncf.io/networks":"rdma-training"}}}' \
  -- ibv_devinfo
```

### ✅ Quotas Active

```bash
kubectl get fabricquotas
# All quotas should show Phase: Active
```

### ✅ Monitoring Working

```bash
# Check Prometheus targets
kubectl port-forward -n monitoring svc/prometheus-server 9090:80
# Open http://localhost:9090/targets
# All TensorReaper targets should be UP

# Check GPU metrics in Grafana
# Dashboard: "TensorReaper - GPU Overview"
```

## Production Configuration

### Enable High Availability

```bash
# Update Helm values
helm upgrade tensorreaper ./helm/tensorreaper-core \
  --namespace tensorreaper-system \
  --set ha.enabled=true \
  --set gpuOperator.replicas=3 \
  --set aiOperator.replicas=3 \
  --set storageOperator.replicas=2 \
  --set networkOperator.replicas=2 \
  --set quotaOperator.replicas=2 \
  --set networkIntelligenceOperator.replicas=2
```

### Configure GPU Pricing

Edit quota operator ConfigMap:

```bash
kubectl create configmap gpu-pricing \
  -n tensorreaper-system \
  --from-literal=H100=8.00 \
  --from-literal=A100-80G=4.00 \
  --from-literal=L40=2.50 \
  -o yaml --dry-run=client | kubectl apply -f -
```

### Set Up Backup

```bash
# Install Velero for cluster backups
velero install \
  --provider aws \
  --bucket tensorreaper-backups \
  --backup-location-config region=us-west-2

# Schedule daily CRD backups
velero schedule create daily-backup \
  --schedule="0 2 * * *" \
  --include-namespaces tensorreaper-system,ml-training,cv-training,nlp-training
```

## Troubleshooting

### GPU Not Detected

```bash
# Check NVIDIA driver
nvidia-smi

# Check device plugin logs
kubectl logs -n kube-system -l app=nvidia-device-plugin

# Verify node labels
kubectl get nodes -o jsonpath='{.items[*].metadata.labels}' | grep nvidia
```

### Storage PVC Stuck in Pending

```bash
# Check StorageClass
kubectl get storageclass

# Check CSI driver pods
kubectl get pods -n kube-system | grep csi

# Describe PVC for events
kubectl describe pvc <pvc-name>
```

### RDMA Not Working

```bash
# Check InfiniBand status on node
ibstat

# Verify RDMA device plugin
kubectl get pods -n kube-system | grep rdma-device-plugin

# Check NetworkAttachmentDefinition
kubectl get network-attachment-definitions
```

### Job Not Scheduling

```bash
# Check quota
kubectl get fabricquota -o yaml

# Check job status
kubectl describe fabricaijob <job-name>

# Check scheduler logs
kubectl logs -n tensorreaper-system -l app=ai-operator
```

## Performance Tuning

### GPU Optimization

```bash
# Set persistence mode (done by Ansible already)
nvidia-smi -pm 1

# Set GPU clocks to max
nvidia-smi -ac 1593,2100  # For H100

# Disable ECC for max performance (optional)
nvidia-smi -e 0
```

### NCCL Tuning

Add to job env vars:

```yaml
env:
  - name: NCCL_IB_HCA
    value: "mlx5_0,mlx5_1"
  - name: NCCL_IB_GID_INDEX
    value: "3"
  - name: NCCL_IB_TC
    value: "106"
  - name: NCCL_NET_GDR_LEVEL
    value: "5"
  - name: NCCL_SOCKET_IFNAME
    value: "eth0"
```

### Storage Performance

```bash
# Enable client-side caching (VAST)
mount -o remount,actimeo=120 /mnt/vast

# Increase read-ahead
echo 16384 > /sys/block/sda/queue/read_ahead_kb
```

## Scaling

### Add GPU Nodes

```bash
# Provision new hardware with Terraform
cd terraform/bare-metal
vim terraform.tfvars  # Add new nodes
terraform apply

# Deploy to new nodes
cd ../../ansible
ansible-playbook -i ../terraform/bare-metal/inventory.ini \
  playbooks/add-node.yaml --limit new-nodes

# Join to cluster
# Nodes auto-register with GPU operator
```

### Increase Storage Capacity

```bash
# Update FabricStorage
kubectl edit fabricstorage vast-production
# Change capacity: 1Pi

# Expand existing PVCs
kubectl patch pvc training-data \
  -p '{"spec":{"resources":{"requests":{"storage":"100Ti"}}}}'
```

## Next Steps

- [ ] Set up CI/CD pipelines for model training
- [ ] Configure auto-scaling policies
- [ ] Implement custom scheduling policies
- [ ] Set up centralized logging (ELK/Loki)
- [ ] Configure alerting (PagerDuty, Slack)
- [ ] Enable cost tracking and chargeback
- [ ] Deploy model registry (MLflow, Weights & Biases)
- [ ] Set up experiment tracking

## Support

- GitHub Issues: https://github.com/ssahani/TensorReaper/issues
- Documentation: https://tensor-reaper.readthedocs.io
- Slack: #tensor-reaper

## Cost Estimate

For a 32x H100 cluster (4 nodes x 8 GPUs):

- **Hardware**: ~$800k (one-time)
- **Power**: ~$10k/month (64kW @ $0.10/kWh)
- **Cooling**: ~$5k/month
- **Network**: ~$50k (one-time for IB switches)
- **Storage**: ~$200k (500TB VAST cluster)

**Total TCO (3 years)**: ~$1.6M
**Per GPU-hour**: ~$2.30

Compare to cloud (H100 @ $8/hour): **72% savings**
