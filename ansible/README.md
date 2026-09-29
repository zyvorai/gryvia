# Ansible playbooks: experimental, incomplete

These playbooks were a starting point for bare-metal provisioning. They are **not** a working one-command install
and have never been run in CI.

`playbooks/site.yaml` references roles that do not exist in this repository:
`kernel-tuning`, `container-runtime`, `kubernetes-control-plane`, `kubernetes-worker`, `gryvia-configure`.
The roles that do exist (`common`, `nvidia-drivers`, `rdma`, `gpu-optimization`, `gryvia-install`) are untested;
`nvidia-drivers` targets Ubuntu 22.04 only, does not handle reboots and does not make `nvidia` the default
container runtime. There is no Fabric Manager or OFED handling.

## What to use instead

| You have | Use |
|---|---|
| A fresh Ubuntu server with an NVIDIA GPU | `sudo ./scripts/install-k3s-gpu.sh server` (k3s, Gryvia and NVIDIA's GPU Operator) |
| An existing Kubernetes cluster | `helm install gryvia ... --set nvidia.enabled=true` |

See `website/docs/guides/GPU_NODES.md`.
