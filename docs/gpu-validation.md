# GPU validation checklist

The GPU path (NVIDIA GPU Operator sub-chart, `install-k3s-gpu.sh`, auto-registration) is covered by unit tests, chart
rendering, dry-run tests and a GPU-less k3s job in CI. **It has not been exercised on real GPU hardware.** Run this on
a machine with an NVIDIA GPU before relying on it, and report what differs.

Use a fresh Ubuntu 22.04 or 24.04 server with one NVIDIA GPU, no NVIDIA driver installed, Secure Boot off.

## 1. Install

```bash
sudo ./scripts/install-k3s-gpu.sh server --api-key '<secret>'
```

If it asks for a reboot (nouveau was loaded), reboot and run it again.

## 2. NVIDIA components come up (5-15 minutes; the driver container compiles against the kernel)

```bash
export KUBECONFIG=/etc/rancher/k3s/k3s.yaml
kubectl -n gryvia-system get pods                                  # gpu-operator pods: all Running/Completed
kubectl -n gryvia-system get clusterpolicy                         # status: ready
kubectl get node -o json | jq '.items[0].status.allocatable["nvidia.com/gpu"]'   # "1" (or your GPU count)
kubectl get node --show-labels | tr ',' '\n' | grep nvidia.com/gpu # gpu.present=true, gpu.product=..., gpu.count=...
```

If the driver pod loops, check `kubectl -n gryvia-system logs -l app=nvidia-driver-daemonset` (kernel headers, Secure Boot).

## 3. A pod sees the GPU

```bash
kubectl run smi --rm -it --restart=Never --image=nvidia/cuda:12.4.1-base-ubuntu22.04 \
  --overrides='{"spec":{"containers":[{"name":"smi","image":"nvidia/cuda:12.4.1-base-ubuntu22.04","command":["nvidia-smi"],"resources":{"limits":{"nvidia.com/gpu":1}}}]}}'
```

## 4. Gryvia registers and reports the node

```bash
kubectl get gryviagpunodes                      # one object named after the node, label gryvia.io/auto-registered=true
kubectl get gryviagpunode <node> -o yaml        # phase Ready, driverVersion/cudaVersion set, gpuStatus filled from DCGM
gryvia status                                   # node listed with GPU health, driver, DaemonSet pods
```

Expected: phase `Ready`. If it stays `WaitingForDrivers`, allocatable `nvidia.com/gpu` is still 0: go back to step 2.

## 5. A Gryvia job runs on the GPU

```bash
gryvia submit --file examples/training/simple-pytorch-training.yaml
gryvia queue
```

## 6. Host-driver mode

On a machine that already has a working driver (`nvidia-smi` works), the installer should detect it and set
`nvidia.driver.enabled=false`. Confirm with `helm -n gryvia-system get values gryvia`.

## Report

Note the GPU model, driver version chosen, kernel, which step failed and the relevant pod logs.
