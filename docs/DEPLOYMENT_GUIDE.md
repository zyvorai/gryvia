# TensorReaper Bare Metal Deployment Guide

Complete guide for deploying TensorReaper on bare metal GPU clusters.

## Quick Deploy (Single Server)

For quick deployment to any server with a Kubernetes cluster:

```bash
# One-command deploy
./scripts/deploy-remote.sh <host> <user> <password>

# Quick mode (rsync + apply only)
./scripts/deploy-remote.sh <host> <user> <password> --quick

# Uninstall
./scripts/deploy-remote.sh <host> <user> <password> --uninstall
```

This script handles: rsync, CRD installation, namespace creation, operator deployment, API gateway, and Web UI. See [Quick Start Guide](getting-started/quickstart.md) for details.

## Full Production Deployment

For multi-node GPU clusters with HA, RDMA, and parallel storage.

## Prerequisites

### Hardware Requirements

**Control Plane Nodes** (minimum 3 for HA):
- CPU: 8+ cores
- RAM: 16GB+
- Disk: 100GB+ SSD
- Network: 10Gbps+

**GPU Worker Nodes**:
- CPU: 32+ cores
- RAM: 256GB+ (512GB recommended for H100)
- GPUs: NVIDIA H100/A100/L40/V100/T4
- Disk: 500GB+ NVMe SSD
- Network: 100-400Gbps InfiniBand or RoCE
- RDMA-capable NICs (Mellanox ConnectX-6 or newer)

**Storage**:
- VAST Data / Weka / DDN / Lustre cluster
- RDMA connectivity
- 100GB/s+ aggregate throughput

### Software Requirements

- Ubuntu 22.04 LTS (recommended) or RHEL 8+
- Kernel 5.15+
- SSH access to all nodes
- Internet connectivity for package downloads

## Step 1: Prepare Infrastructure

### 1.1 Network Setup

**Management Network** (1GbE):
- Control plane: 10.0.1.100-10.0.1.110
- GPU nodes: 10.0.1.200-10.0.1.250

**RDMA Network** (100-400GbE InfiniBand/RoCE):
- GPU nodes: 192.168.100.10-192.168.100.250
- Storage: 192.168.100.1-192.168.100.9

### 1.2 Storage Setup

Mount VAST/Weka/DDN on all GPU nodes:
```bash
# Example for VAST
mkdir -p /mnt/vast
mount -t nfs -o rdma,port=20049 vast-vip:/tensorreaper /mnt/vast
```

Add to `/etc/fstab`:
```
vast-vip:/tensorreaper /mnt/vast nfs rdma,port=20049,hard,nointr 0 0
```

## Step 2: Configure Terraform

### 2.1 Copy Example Configuration

```bash
cd terraform/bare-metal
cp terraform.tfvars.example terraform.tfvars
```

### 2.2 Edit terraform.tfvars

```hcl
cluster_name = "tensorreaper-production"
control_plane_endpoint = "10.0.1.100"

control_nodes = [
  { name = "master-01", ip = "10.0.1.101" },
  { name = "master-02", ip = "10.0.1.102" },
  { name = "master-03", ip = "10.0.1.103" }
]

gpu_nodes = [
  {
    name          = "gpu-h100-01"
    ip            = "10.0.1.201"
    gpu_type      = "H100"
    gpu_count     = 8
    rdma_enabled  = true
    rdma_device   = "mlx5_0"
    storage_mount = "/mnt/vast"
  },
  # Add more nodes...
]

storage_backend   = "vast"
storage_endpoint  = "vast-vip.example.com"
rdma_subnet       = "192.168.100.0/24"
```

### 2.3 Generate Configuration

```bash
terraform init
terraform plan
terraform apply
```

This generates:
- Ansible inventory
- Kubernetes configs
- GPU node manifests
- Network/storage configs

## Step 3: Run Ansible Deployment

### 3.1 Install Ansible Dependencies

```bash
cd ../../ansible
ansible-galaxy collection install -r requirements.yaml
```

### 3.2 Test Connectivity

```bash
ansible all -i ../terraform/bare-metal/generated/inventory.ini -m ping
```

### 3.3 Deploy Cluster

Run the auto-generated script:
```bash
cd ../terraform/bare-metal/generated
./deploy.sh
```

Or run Ansible manually:
```bash
cd ../../ansible
ansible-playbook -i ../terraform/bare-metal/generated/inventory.ini playbooks/site.yaml
```

This will:
1. Configure all nodes (kernel tuning, packages)
2. Install NVIDIA drivers
3. Setup RDMA networking
4. Optimize GPU performance
5. Install Kubernetes
6. Deploy TensorReaper operators
7. Register GPU nodes

**Duration**: 30-60 minutes depending on cluster size

## Step 4: Verify Installation

### 4.1 Check Cluster

```bash
export KUBECONFIG=./generated/kubeconfig
kubectl get nodes
```

Expected output:
```
NAME           STATUS   ROLES           AGE   VERSION
master-01      Ready    control-plane   10m   v1.30.5
gpu-h100-01    Ready    <none>          8m    v1.30.5
gpu-h100-02    Ready    <none>          8m    v1.30.5
```

### 4.2 Verify GPU Nodes

```bash
kubectl get fabricgpunodes
```

Expected output:
```
NAME           NODE          GPU-TYPE   GPU-COUNT   RDMA   PHASE   AGE
gpu-h100-01    gpu-h100-01   H100       8           true   Ready   5m
gpu-h100-02    gpu-h100-02   H100       8           true   Ready   5m
```

### 4.3 Check Operators

```bash
kubectl get pods -n tensorreaper-system
```

Expected output:
```
NAME                                    READY   STATUS    RESTARTS   AGE
tensorreaper-gpu-operator-xxx             1/1     Running   0          5m
tensorreaper-ai-operator-xxx              1/1     Running   0          5m
nvidia-device-plugin-daemonset-xxx      1/1     Running   0          5m
dcgm-exporter-xxx                       1/1     Running   0          5m
```

### 4.4 Test GPU Access

```bash
kubectl apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: gpu-test
spec:
  containers:
  - name: cuda
    image: nvidia/cuda:12.2.0-base-ubuntu22.04
    command: ["nvidia-smi"]
  restartPolicy: Never
EOF

kubectl logs gpu-test
```

## Step 5: Configure Monitoring

### 5.1 Access Grafana

```bash
kubectl port-forward -n tensorreaper-system svc/tensorreaper-observability-grafana 3000:80
```

Open http://localhost:3000
- Username: admin
- Password: tensorreaper-admin

### 5.2 Import Dashboards

Pre-configured dashboards available:
- GPU Cluster Overview
- Training Job Performance
- GPU Health & Utilization
- Network Performance
- Storage I/O

## Step 6: Submit Test Job

```bash
kubectl apply -f ../../examples/training/simple-pytorch-training.yaml
kubectl get fabricaijob
kubectl logs -f $(kubectl get pod -l tensorreaper.ai/job=pytorch-simple-training -o name)
```

## Troubleshooting

### NVIDIA Drivers Not Loading

```bash
# On GPU node
sudo modprobe nvidia
sudo nvidia-smi

# Check kernel module
lsmod | grep nvidia
```

### RDMA Not Working

```bash
# Check InfiniBand status
ibstat
ibv_devices

# Test RDMA bandwidth
ib_write_bw
```

### Pods Not Scheduling

```bash
kubectl describe fabricaijob <job-name>
kubectl get events --sort-by='.lastTimestamp'
```

### GPU Not Detected

```bash
kubectl describe node <gpu-node-name>
# Look for nvidia.com/gpu resource
```

## Next Steps

1. Configure team quotas: `kubectl apply -f examples/quotas/`
2. Setup storage classes: `kubectl apply -f examples/storage/`
3. Run distributed training: `kubectl apply -f examples/distributed/`
4. Configure alerts: See `manifests/monitoring/alerts/`

## Production Checklist

- [ ] High availability control plane (3+ nodes)
- [ ] Backup etcd regularly
- [ ] Setup monitoring and alerting
- [ ] Configure network policies
- [ ] Implement RBAC
- [ ] Enable audit logging
- [ ] Setup log aggregation
- [ ] Configure automatic updates
- [ ] Test disaster recovery
- [ ] Document runbooks

## Support

For issues and questions:
- GitHub Issues: https://github.com/ssahani/tensorreaper/issues
- Documentation: https://tensorreaper.ai/docs
- Community: https://tensorreaper.ai/community
