# GPU reset agent

**Status: unit-tested with fake clients and a fake command runner; dry-run is the default and the only mode CI exercises.** `nvidia-smi --gpu-reset` has never been run by this agent on a real GPU, in a container, or against a GPU Operator driver container. Treat the execute path as unverified.

## What it is

An optional DaemonSet (`gpuOperator.resetAgent.enabled`, off by default) on nodes labelled `nvidia.com/gpu.present=true`. One agent acts on its own node only (`NODE_NAME`).

You ask for a reset by annotating the node:

```bash
kubectl cordon gpu-node-1
kubectl drain gpu-node-1 --ignore-daemonsets --delete-emptydir-data   # or let the health check quarantine/drain it
kubectl annotate node gpu-node-1 gryvia.io/gpu-reset-request=reset-2026-10-02-a gryvia.io/gpu-reset-gpus=0,1
kubectl get node gpu-node-1 -o jsonpath='{.metadata.annotations.gryvia\.io/gpu-reset-result}'
```

`gryvia.io/gpu-reset-gpus` is optional (empty means all GPUs). The agent acts only when the node is cordoned **and** no non-terminated pod other than DaemonSet/static pods requests `nvidia.com/gpu`; otherwise it records `Blocked` with the reason and re-checks every poll. A new request id is a new request; an id that was handled (`Done`, `Failed`, `DryRun`, `Invalid`) is never retried automatically, so a failed reset is not hammered. Results are JSON in `gryvia.io/gpu-reset-result` (`id`, `state`, `message`, `at`) and the handled id is in `gryvia.io/gpu-reset-observed`.

Safety properties, all covered by unit tests: dry-run by default and in dry-run nothing is executed; GPU indices are parsed as integers 0-63 (no shell, no free-form arguments, the only command is `nvidia-smi --gpu-reset [-i list]`); the request id is restricted to 63 characters of `[A-Za-z0-9._-]`; the agent never cordons, drains, uncordons or removes taints; it never touches another node.

## Turning on real resets

```yaml
gpuOperator:
  resetAgent:
    enabled: true
    execute: true
    nvidiaSmi: /run/nvidia/driver/usr/bin/nvidia-smi   # a path the container can run
    driverRoot: /run/nvidia/driver                     # hostPath providing it, mounted read-only
```

The container is then `privileged`, because a GPU reset needs device access. The agent image is distroless and contains no `nvidia-smi`; whether the host's binary runs from a mounted driver root (it needs its libraries too) depends on how the driver is installed, so test on one node first. Helm refuses `execute` without `nvidiaSmi`.

## Limits

- Not wired to the health check: nothing creates the request annotation automatically. A person (or your own automation) does, after the existing quarantine and PDB-respecting drain.
- No driver reload, no reboot, no automatic uncordon, no retry policy, no check that the GPU came back healthy (read the `gryvia.io/gpu-reset-result` and your health check afterwards).
- A reset can fail while something else (a monitoring agent, MIG manager, persistence mode) holds the GPU; the failure text is recorded, not interpreted.
- Unverified: container privilege model across distros, interaction with the NVIDIA GPU Operator driver container, MIG-enabled GPUs, NVSwitch/Fabric Manager systems (resetting one GPU in an NVSwitch domain may need the whole domain).
