# GPU validation checklist

The GPU path (NVIDIA GPU Operator sub-chart, `install-k3s-gpu.sh`, auto-registration) is covered by unit tests, chart
rendering, dry-run tests and a GPU-less k3s job in CI. **It has not been exercised on real GPU hardware.** Run this on
a machine with an NVIDIA GPU before relying on it, and report what differs.

Use a fresh Ubuntu 22.04 or 24.04 server with one NVIDIA GPU, no NVIDIA driver installed, Secure Boot off.

## Run it as a script

Steps 2 to 5 (and the optional RDMA part) are automated by `scripts/validate-gpu.sh`. Run it after step 1, from the
repository root, on the machine that has the cluster credentials (for RDMA checks, on the GPU node itself):

```bash
export KUBECONFIG=/etc/rancher/k3s/k3s.yaml
./scripts/validate-gpu.sh --report gpu-validation-report.json            # add --host-driver for step 6 machines
./scripts/validate-gpu.sh --rdma --collector-url http://<collector>:9090 \
  --collector-token-file /path/to/api-token                              # adds the RDMA/NIC checks
./scripts/validate-gpu.sh --dry-run                                      # print the commands, run nothing
```

It waits for each component (default budget 900 s per check, `--timeout`), prints a table, writes the JSON report and
exits non-zero if any check failed. It creates one pod (`gryvia-validate-smi`) and submits the example job, and deletes
both afterwards unless you pass `--keep`. **The script itself has only been tested against a fake `kubectl`
(`scripts/tests/validate-gpu.test.sh`); it has not run on a GPU node.** If it misbehaves on real hardware, that is a
finding: report it together with the output.

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

## Smoke job and evidence file

For a short, scripted record that the device plugin scheduled a pod on the GPU, without the full checklist above:

```bash
kubectl apply -f examples/validation/gpu-smoke.yaml     # ConfigMap and Job gryvia-gpu-smoke in gryvia-system, requests nvidia.com/gpu: 1
kubectl -n gryvia-system wait --for=condition=complete job/gryvia-gpu-smoke --timeout=300s
./scripts/record-gpu-validation.sh report.txt           # NS=<namespace> if not gryvia-system
```

The job runs `nvidia-smi -L` and prints the GPU name, driver version and memory. `record-gpu-validation.sh` writes one
text file with the nodes, each node's allocatable `nvidia.com/gpu`, the `GryviaGpuNode` objects, and the job's YAML and
log. It is evidence that the GPU path scheduled a pod, not a benchmark: it checks no RDMA, NCCL, DCGM or NVSwitch, and it
has not been run on a GPU. Attach the report to whatever claims the GPU path is validated.

## Report

Paste the script's summary table and `gpu-validation-report.json` into an issue, plus the GPU model, driver version
chosen, kernel, and the pod logs of whichever step failed. The report looks like this (the values below are an
illustration of the format, not a real run):

```json
{
  "schema": "gryvia.validate-gpu/v1",
  "result": "pass",
  "started": "2026-01-01T10:00:00Z",
  "finished": "2026-01-01T10:12:30Z",
  "context": "default",
  "namespace": "gryvia-system",
  "node": "gpu-node-1",
  "checks": [
    {"name": "nvidia.clusterpolicy", "status": "pass", "detail": "ClusterPolicy state ready", "started": "...", "finished": "..."},
    {"name": "pod.nvidia-smi", "status": "pass", "detail": "| NVIDIA-SMI 550.54 Driver Version: 550.54 ...", "started": "...", "finished": "..."}
  ]
}
```

| Field | Meaning |
|---|---|
| `result` | `pass`, `fail` (any check failed; the script exits 1) or `dry-run` |
| `checks[].name` | `preflight.kubectl`, `preflight.context`, `preflight.namespace`, `nvidia.clusterpolicy`, `nvidia.pods.{driver,toolkit,device-plugin}`, `node.allocatable`, `pod.nvidia-smi`, `gryvia.gpunode`, `gryvia.job`, and with `--rdma` `rdma.sysfs`, `rdma.collector` |
| `checks[].status` | `pass`, `fail`, or `skip` (not applicable: `--host-driver`, `--skip-job`, no GPU node found, earlier preflight failure, no collector URL) |
| `checks[].detail` | what was observed, or why it failed or was skipped |
| `checks[].started` / `finished` | UTC timestamps |

Step 6 (host-driver mode) and the installer itself (step 1) are not scripted here.

## Next: the AI features

Once a GPU job runs, [GPU-cluster runbook](gpu-ai-runbook.md) takes one small model through the AI features that need
GPUs: the default ML images, the model factory (fine-tune, AWQ quantization, evaluation), a vLLM canary and its
rollback, and vLLM behind the LLM gateway, RAG and agents.
