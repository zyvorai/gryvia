# NVIDIA one-click install

Gryvia does not replace NVIDIA's operators. It turns them on and labels nodes so a
policy is enough.

## What one command does

```bash
sudo ./scripts/install-k3s-gpu.sh server --api-key '<secret>'
```

That installs k3s, Gryvia and the NVIDIA GPU Operator (driver, toolkit, device plugin, DCGM).
Not validated on real GPUs. See docs/gpu-validation.md.

## What you add for a training fabric

```bash
helm upgrade gryvia oci://ghcr.io/zyvorai/charts/gryvia \
  -n gryvia-system \
  --set nvidia.enabled=true \
  --set nvidia.driver.rdma.enabled=true \
  --set nvidia.gds.enabled=true \
  --set nvidia.mig.strategy=mixed \
  --set nvidia.migManager.enabled=true \
  --set nvidiaNetwork.enabled=true \
  --set nvidiaPlatform.timeSlicing.enabled=true \
  --set nvidia.devicePlugin.config.name=gryvia-time-slicing \
  --set nvidiaPlatform.validate.enabled=true
```

| Flag | NVIDIA component |
|---|---|
| `nvidia.enabled` | GPU Operator sub-chart |
| `nvidia.driver.rdma.enabled` | GPUDirect RDMA in the driver |
| `nvidia.gds.enabled` | GPUDirect Storage (`nvidia-fs`) |
| `nvidia.mig.strategy` / `migManager.enabled` | MIG manager |
| `nvidiaPlatform.timeSlicing` + `nvidia.devicePlugin.config.name` | ConfigMap, plus the ClusterPolicy setting that reads it; the policy then adds the node label `nvidia.com/device-plugin.config` |
| `nvidiaNetwork.enabled` | NVIDIA Network Operator (DOCA/OFED, SR-IOV, RDMA plugin) |
| `nvidiaPlatform.validate` | post-install `nvidia-smi` Job |

Network Operator does not create a NicClusterPolicy by itself. Follow NVIDIA's
NicClusterPolicy example after the operator is Running. Fabric Manager stays a
GPU Operator concern on NVSwitch nodes; this chart does not invent a second installer.

`GryviaGPUSharingPolicy` now writes NVIDIA's labels, not only `gryvia.io/*`:

- time-slicing: `nvidia.com/device-plugin.config=gryvia-time-slicing`
- mig: `nvidia.com/mig.config=<first profile name>` (must match a mig-parted profile)
