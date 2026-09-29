# Gryvia Storage Operator

The Storage Operator manages parallel filesystem integrations for AI workloads in Gryvia. It automates CSI driver deployment and storage configuration for enterprise-grade parallel filesystems.

## Supported Storage Backends

- **VAST Data** - Disaggregated shared everything architecture
- **WekaFS** - High-performance parallel filesystem
- **DDN EXAScaler** - Lustre-based parallel filesystem
- **Lustre** - Open-source parallel filesystem
- **Ceph** - Software-defined storage

## Features

- **Automated CSI Driver Installation** - Deploys and configures CSI drivers for each backend
- **Dynamic StorageClass Creation** - Auto-creates storage classes based on backend type
- **Health Monitoring** - Continuous health checks of storage endpoints
- **Multi-Backend Support** - Run multiple storage backends simultaneously
- **Node Selection** - Target specific nodes for storage client installation

## Architecture

```
GryviaStorage CR
       ↓
Storage Operator
       ↓
  ┌────┴────┬────────┬─────────┐
  ↓         ↓        ↓         ↓
VAST CSI  Weka CSI  DDN CSI  Ceph CSI
  ↓         ↓        ↓         ↓
StorageClass (ReadWriteMany PVCs)
```

## Installation

```bash
# Apply CRD
kubectl apply -f crds/gryviastorage.yaml

# Deploy operator
kubectl apply -f operators/storage-operator/config/

# Verify deployment
kubectl get pods -n gryvia-system -l app=storage-operator
```

## Usage

### VAST Data Example

```yaml
apiVersion: gryvia.io/v1
kind: GryviaStorage
metadata:
  name: vast-production
spec:
  backendType: vast
  endpoint: vast-mgmt.example.com
  capacity: 100Ti
  nodeSelector:
    gryvia.io/storage: "true"
  credentials:
    secretName: vast-credentials
    secretNamespace: gryvia
```

### Create PVC

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: training-data
spec:
  accessModes:
    - ReadWriteMany
  storageClassName: vast-production
  resources:
    requests:
      storage: 10Ti
```

## Implementation Details

### Reconciliation Behavior

- The operator sets the phase to `Configuring` only when it is empty (initial creation), not on every reconciliation cycle. This prevents unnecessary status churn during steady-state operation.
- `ensureStorageClass` properly returns errors for non-NotFound API failures, ensuring transient errors are not silently ignored.

### VAST CSI Driver

The operator deploys:
- ServiceAccount with RBAC permissions
- CSI Controller Deployment (1 replica)
- CSI Node DaemonSet (on all storage nodes)
- StorageClass with RWX access mode and `Retain` reclaim policy (default)

**Files:**
- `controllers/gryviastorage_controller.go` - Main reconciliation loop
- `pkg/vast/vast.go` - VAST-specific CSI deployment (~300 LOC)

### Health Monitoring

The operator performs health checks every 5 minutes:
- VAST: `https://<endpoint>/api/health`
- Weka: `https://<endpoint>/api/v2/healthcheck`
- DDN: `https://<endpoint>/api/health`

## Build

```bash
make build          # Build binary
make docker-build   # Build container
make deploy         # Deploy to cluster
```

## Development

```bash
# Run locally
make run

# Run tests
make test

# Generate manifests
controller-gen rbac:roleName=storage-operator paths="./..." output:rbac:artifacts:config=config/
```

## RBAC Permissions

The operator requires:
- `persistentvolumes`, `persistentvolumeclaims`: Full CRUD
- `storageclasses`, `csidrivers`: Full CRUD
- `deployments`, `daemonsets`: Full CRUD for CSI components
- `clusterroles`, `clusterrolebindings`: For CSI RBAC setup

## Troubleshooting

**CSI Driver Not Starting**
```bash
# Check operator logs
kubectl logs -n gryvia-system -l app=storage-operator

# Check CSI controller
kubectl logs -n kube-system -l app=vast-csi-controller

# Verify storage endpoint
curl -k https://<endpoint>/api/health
```

**PVC Stuck in Pending**
```bash
# Check StorageClass
kubectl get storageclass

# Check CSI driver registration
kubectl get csidrivers

# Check node plugin
kubectl get pods -n kube-system -l app=vast-csi-node
```

## Performance Tuning

For optimal AI workload performance:

1. **Enable Direct I/O**: Mount with `direct_io` option
2. **Increase Read-Ahead**: `readahead_kb=16384`
3. **Use NVMe-oF**: For lowest latency access
4. **Enable Client-Side Caching**: For repeated data access

## Roadmap

- [ ] Storage quota management per namespace
- [ ] Performance metrics collection (IOPS, bandwidth)
- [ ] Automated data migration between backends
- [ ] Storage tiering policies
- [ ] Volume snapshot/clone automation
