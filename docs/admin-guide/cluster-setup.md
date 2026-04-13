# Cluster Setup Guide

Complete guide for setting up a production TensorReaper cluster.

## Prerequisites

### Hardware Requirements

**Control Plane:**
- 3 nodes for HA
- 8 CPU cores per node
- 32GB RAM per node
- 200GB SSD storage

**GPU Nodes:**
- NVIDIA GPUs (Tesla T4, A100, H100, etc.)
- NVIDIA driver 525+ installed
- 64GB+ RAM per node
- High-speed networking (InfiniBand/RoCE recommended)

**Storage:**
- Parallel filesystem (VAST Data, Weka, or similar)
- NFS for shared storage
- Local SSD for caching

### Software Requirements

- Kubernetes 1.30+
- Container runtime with GPU support (containerd + nvidia-container-runtime)
- Helm 3.0+
- kubectl

## Installation Methods

### Method 1: Helm Installation (Recommended)

```bash
# Add Helm repository
helm repo add tensorreaper https://ssahani.github.io/tensor-reaper
helm repo update

# Create namespace
kubectl create namespace tensorreaper

# Install with default values
helm install tensorreaper tensorreaper/tensorreaper \
  --namespace tensorreaper \
  --wait

# Or customize installation
helm install tensorreaper tensorreaper/tensorreaper \
  --namespace tensorreaper \
  --set gpuOperator.replicaCount=3 \
  --set highAvailability.enabled=true \
  --set monitoring.prometheus.enabled=true \
  --wait
```

### Method 2: Manual Installation

```bash
# Clone repository
git clone https://github.com/ssahani/tensor-reaper.git
cd tensor-reaper

# Install CRDs
kubectl apply -f manifests/crds/

# Install operators
kubectl apply -f manifests/deploy/

# Install monitoring
kubectl apply -f manifests/monitoring/

# Install RBAC
kubectl apply -f manifests/rbac/
```

### Method 3: Terraform + Ansible

```bash
# Navigate to infrastructure directory
cd terraform/baremetal

# Initialize Terraform
terraform init

# Plan deployment
terraform plan -var-file=production.tfvars

# Apply
terraform apply -var-file=production.tfvars

# Run Ansible playbooks
cd ../../ansible
ansible-playbook -i inventory/production cluster-setup.yml
```

## Configuration

### High Availability Setup

```yaml
# ha-values.yaml
highAvailability:
  enabled: true

gpuOperator:
  replicaCount: 3
  resources:
    requests:
      memory: 2Gi
      cpu: 1
    limits:
      memory: 4Gi
      cpu: 2

aiOperator:
  replicaCount: 3

storageOperator:
  replicaCount: 2

networkOperator:
  replicaCount: 2

quotaOperator:
  replicaCount: 3

postgresql:
  replicaCount: 3
  persistence:
    enabled: true
    size: 100Gi
```

```bash
helm upgrade tensorreaper tensorreaper/tensorreaper \
  -f ha-values.yaml \
  --namespace tensorreaper
```

### Storage Backend Configuration

**VAST Data CSI:**

```yaml
storage:
  vastData:
    enabled: true
    endpoint: "vast.example.com"
    vipPool: "vip-pool-1"
    storageClass:
      name: vast-sc
      reclaimPolicy: Retain
```

**Weka:**

```yaml
storage:
  weka:
    enabled: true
    endpoint: "weka.example.com"
    storageClass:
      name: weka-sc
```

**NFS:**

```yaml
storage:
  nfs:
    enabled: true
    server: "nfs.example.com"
    path: /exports/tensorreaper
```

### Network Configuration

**InfiniBand:**

```yaml
network:
  infiniband:
    enabled: true
    devicePlugin: mellanox
    sriovEnabled: true
    numVfs: 8
```

**RoCE:**

```yaml
network:
  roce:
    enabled: true
    priority: 3
    dscp: 26
```

### GPU Pricing Configuration

```yaml
# gpu-pricing.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: gpu-pricing
  namespace: tensorreaper
data:
  pricing.yaml: |
    gpuPricing:
      H100:
        hourlyRate: 30.00
        currency: USD
      A100-80G:
        hourlyRate: 24.00
        currency: USD
      A100-40G:
        hourlyRate: 12.00
        currency: USD
      V100:
        hourlyRate: 8.00
        currency: USD
      T4:
        hourlyRate: 3.00
        currency: USD
```

## Node Setup

### GPU Node Labels

```bash
# Label GPU nodes
kubectl label nodes gpu-node-1 tensorreaper.ai/gpu=true
kubectl label nodes gpu-node-1 tensorreaper.ai/gpu-type=A100-80G
kubectl label nodes gpu-node-1 tensorreaper.ai/gpu-count=8
kubectl label nodes gpu-node-1 tensorreaper.ai/nvlink=true
kubectl label nodes gpu-node-1 tensorreaper.ai/infiniband=true
```

### Register GPU Nodes

```yaml
# gpu-node-profile.yaml
apiVersion: tensorreaper.ai/v1
kind: FabricGPUNode
metadata:
  name: gpu-node-1
spec:
  nodeName: gpu-node-1
  gpuType: A100-80G
  gpuCount: 8
  gpuMemory: 80Gi
  totalMemory: 512Gi
  totalCPU: 64
  nvlink: true
  infiniband:
    enabled: true
    ports: 2
    speed: 200Gb
  pricing:
    hourlyRate: 192.00  # 24.00 * 8 GPUs
    currency: USD
  maintenance:
    schedule: "0 2 * * 0"  # Sunday 2 AM
```

### Node Taints (Optional)

```bash
# Taint GPU nodes to prevent non-GPU workloads
kubectl taint nodes gpu-node-1 tensorreaper.ai/gpu=true:NoSchedule

# Jobs will automatically add tolerations
```

## Networking Setup

### Multus CNI (for RDMA)

```bash
# Install Multus
kubectl apply -f https://raw.githubusercontent.com/k8snetworkplumbingwg/multus-cni/master/deployments/multus-daemonset.yml

# Create NetworkAttachmentDefinition
cat <<EOF | kubectl apply -f -
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: ib-network
  namespace: default
spec:
  config: '{
    "cniVersion": "0.3.1",
    "type": "ib-sriov",
    "deviceID": "0000:05:00.0",
    "ipam": {
      "type": "host-local",
      "subnet": "192.168.1.0/24",
      "rangeStart": "192.168.1.10",
      "rangeEnd": "192.168.1.250"
    }
  }'
EOF
```

### SR-IOV Network Device Plugin

```bash
# Install SR-IOV Network Device Plugin
kubectl apply -f https://raw.githubusercontent.com/k8snetworkplumbingwg/sriov-network-device-plugin/master/deployments/sriovdp-daemonset.yaml

# Configure SR-IOV
kubectl apply -f - <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: sriovdp-config
  namespace: kube-system
data:
  config.json: |
    {
      "resourceList": [{
        "resourceName": "infiniband",
        "selectors": {
          "vendors": ["15b3"],
          "devices": ["101b"],
          "drivers": ["mlx5_core"]
        }
      }]
    }
EOF
```

## Monitoring Setup

### Prometheus and Grafana

```yaml
monitoring:
  prometheus:
    enabled: true
    retention: 30d
    storageSize: 500Gi
    resources:
      requests:
        memory: 8Gi
        cpu: 4
      limits:
        memory: 16Gi
        cpu: 8

  grafana:
    enabled: true
    adminPassword: "change-me-in-production"
    persistence:
      enabled: true
      size: 10Gi
```

### NVIDIA DCGM Exporter

```bash
# Install DCGM Exporter
helm repo add gpu-helm-charts \
  https://nvidia.github.io/dcgm-exporter/helm-charts

helm install dcgm-exporter \
  gpu-helm-charts/dcgm-exporter \
  --namespace tensorreaper \
  --set serviceMonitor.enabled=true
```

## Security Setup

### TLS Certificates

```bash
# Install cert-manager
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.14.0/cert-manager.yaml

# Create ClusterIssuer
kubectl apply -f - <<EOF
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-prod
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: admin@example.com
    privateKeySecretRef:
      name: letsencrypt-prod
    solvers:
    - http01:
        ingress:
          class: nginx
EOF
```

### RBAC Configuration

```bash
# Apply RBAC policies
kubectl apply -f manifests/rbac/

# Create admin user
kubectl apply -f - <<EOF
apiVersion: v1
kind: ServiceAccount
metadata:
  name: tensorreaper-admin
  namespace: tensorreaper

---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: tensorreaper-admin
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: tensorreaper-platform-admin
subjects:
- kind: ServiceAccount
  name: tensorreaper-admin
  namespace: tensorreaper
EOF

# Get admin token
kubectl create token tensorreaper-admin -n tensorreaper
```

### Network Policies

```bash
# Apply network policies
kubectl apply -f manifests/security/network-policies.yaml
```

### Pod Security Policies

```bash
# Apply PSPs
kubectl apply -f manifests/security/pod-security-policies.yaml
```

## Database Setup

### PostgreSQL for Cost Tracking

```yaml
postgresql:
  enabled: true
  replicaCount: 3
  persistence:
    enabled: true
    size: 100Gi
  metrics:
    enabled: true
  resources:
    requests:
      memory: 4Gi
      cpu: 2
    limits:
      memory: 8Gi
      cpu: 4
```

### Database Schema

```sql
-- Applied automatically on first install
-- See manifests/deploy/database-schema.sql
```

## Verification

### Check Operators

```bash
# Check all operators are running
kubectl get pods -n tensorreaper

# Expected output:
# tensorreaper-gpu-operator-xxx       1/1   Running
# tensorreaper-ai-operator-xxx        1/1   Running
# tensorreaper-storage-operator-xxx   1/1   Running
# tensorreaper-network-operator-xxx   1/1   Running
# tensorreaper-quota-operator-xxx     1/1   Running
```

### Check CRDs

```bash
# Verify CRDs are installed
kubectl get crds | grep tensorreaper.ai

# Expected output:
# fabricaijobs.tensorreaper.ai
# fabricgpunodes.tensorreaper.ai
# fabricquotas.tensorreaper.ai
# fabricstorages.tensorreaper.ai
# fabricnetworks.tensorreaper.ai
```

### Test GPU Scheduling

```bash
# Submit test job
kubectl apply -f - <<EOF
apiVersion: tensorreaper.ai/v1
kind: FabricAIJob
metadata:
  name: gpu-test
spec:
  framework: pytorch
  resources:
    gpuType: A100-80G
    gpuCount: 1
  image: nvcr.io/nvidia/pytorch:24.01-py3
  command: ["python", "-c", "import torch; print(torch.cuda.is_available())"]
EOF

# Check job status
kubectl get fabricaijob gpu-test
tensorreaper logs gpu-test
```

### Run Diagnostics

```bash
# Run GPU diagnostics on all nodes
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: gpu-diagnostics
spec:
  selector:
    matchLabels:
      app: gpu-diagnostics
  template:
    metadata:
      labels:
        app: gpu-diagnostics
    spec:
      nodeSelector:
        tensorreaper.ai/gpu: "true"
      containers:
      - name: diagnostics
        image: nvidia/cuda:12.3.0-base-ubuntu22.04
        command: ["nvidia-smi"]
EOF

# View results
kubectl logs -l app=gpu-diagnostics
```

## Backup Configuration

```bash
# Configure automatic backups
kubectl apply -f - <<EOF
apiVersion: batch/v1
kind: CronJob
metadata:
  name: tensorreaper-backup
  namespace: tensorreaper
spec:
  schedule: "0 2 * * *"  # Daily at 2 AM
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: backup
            image: bitnami/kubectl:1.32.0
            command:
            - /bin/bash
            - -c
            - |
              /tools/backup-restore.sh backup --dir /backups
            volumeMounts:
            - name: backup-storage
              mountPath: /backups
            - name: tools
              mountPath: /tools
          volumes:
          - name: backup-storage
            persistentVolumeClaim:
              claimName: backup-pvc
          - name: tools
            configMap:
              name: backup-scripts
          restartPolicy: OnFailure
EOF
```

## Maintenance

### Cluster Health Checks

```bash
# Run health check script
./scripts/health-check.sh

# Or manually check
kubectl get nodes
kubectl get pods -n tensorreaper
kubectl get fabricgpunodes
tensorreaper cluster status
```

### Log Rotation

```yaml
# Configure log rotation
apiVersion: v1
kind: ConfigMap
metadata:
  name: fluent-bit-config
data:
  fluent-bit.conf: |
    [OUTPUT]
        Name es
        Match *
        Host elasticsearch
        Port 9200
        Index tensorreaper
        Type  _doc
```

### Update GPU Drivers

```bash
# Drain node
kubectl drain gpu-node-1 --ignore-daemonsets

# Update drivers (on node)
sudo apt update
sudo apt install nvidia-driver-535

# Reboot node
sudo reboot

# Uncordon node
kubectl uncordon gpu-node-1
```

## Scaling

### Adding GPU Nodes

```bash
# 1. Provision new node
# 2. Install NVIDIA drivers
# 3. Join to Kubernetes cluster
# 4. Label node
kubectl label nodes gpu-node-5 tensorreaper.ai/gpu=true
kubectl label nodes gpu-node-5 tensorreaper.ai/gpu-type=H100

# 5. Register node
kubectl apply -f gpu-node-5-profile.yaml
```

### Scaling Operators

```bash
# Scale operators
kubectl scale deployment tensorreaper-gpu-operator -n tensorreaper --replicas=5

# Or use Helm
helm upgrade tensorreaper tensorreaper/tensorreaper \
  --set gpuOperator.replicaCount=5 \
  --namespace tensorreaper
```

## Troubleshooting

### Operator Not Starting

```bash
# Check logs
kubectl logs -n tensorreaper deployment/tensorreaper-gpu-operator

# Check events
kubectl get events -n tensorreaper --sort-by='.lastTimestamp'

# Describe deployment
kubectl describe deployment -n tensorreaper tensorreaper-gpu-operator
```

### GPU Not Detected

```bash
# Check NVIDIA drivers
nvidia-smi

# Check device plugin
kubectl logs -n kube-system -l name=nvidia-device-plugin-ds

# Check node labels
kubectl get nodes --show-labels | grep gpu
```

### Jobs Not Scheduling

```bash
# Check GPU availability
tensorreaper cluster nodes

# Check quotas
tensorreaper quota list

# Check job events
kubectl describe fabricaijob <job-name>
```

## Performance Tuning

### Kernel Parameters

```bash
# /etc/sysctl.conf
net.core.rmem_max = 134217728
net.core.wmem_max = 134217728
net.ipv4.tcp_rmem = 4096 87380 67108864
net.ipv4.tcp_wmem = 4096 65536 67108864
vm.swappiness = 10
```

### GPU Persistence Mode

```bash
# Enable on all nodes
sudo nvidia-smi -pm 1

# Make persistent across reboots
sudo systemctl enable nvidia-persistenced
```

## Support

- Setup Issues: https://github.com/ssahani/tensor-reaper/issues
- Slack: #tensorreaper-support
- Documentation: https://github.com/ssahani/tensor-reaper/docs
