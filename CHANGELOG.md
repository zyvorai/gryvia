# Changelog

All notable changes to Gryvia are recorded here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
and the project aims to follow [Semantic Versioning](https://semver.org/) once it leaves alpha.

## [Unreleased]

### Added
- Network flows can come from [Netra](https://github.com/zyvorai/netra): set `apiGateway.netra.url` and `/api/network/flows` returns Netra's recent flows (falls back to service-graph edges when it is unset or unreachable).
- Workflows and auto tuners can be deleted from the dashboard and the gateway (`DELETE /api/workflows/{name}`, `DELETE /api/tuners/{name}`).
- GPU nodes prepare themselves: the chart can bundle NVIDIA's GPU Operator (`nvidia.enabled=true`: driver, container toolkit, device plugin, DCGM, node feature discovery), and `scripts/install-k3s-gpu.sh` takes a fresh Ubuntu server to k3s + Gryvia + GPU support (`agent` mode joins more nodes). CI runs the script on a real k3s (without a GPU) and dry-run tests for the GPU branches. See `website/docs/guides/GPU_NODES.md` and `docs/gpu-validation.md`.
- The GPU operator registers a `GryviaGpuNode` for every node labelled by NVIDIA GPU feature discovery (`--auto-register`, chart value `gpuOperator.autoRegister`) and reads readiness, driver and CUDA versions from the node's labels and allocatable GPUs; per-GPU numbers come from the DCGM exporter on that node.
- CLI in the style of `cilium`: colored `--help` with commands in groups, examples on every command, aliases, `--no-color`/`NO_COLOR`, shell completions (`gryvia completion`), `gryvia version`, concise errors with hints, and one output look (aligned tables with OK/Warning/Error markers) across all commands.
- `gryvia status` reports the whole platform like `cilium status`: a component banner, workload readiness, cluster totals, image versions, errors and a per-node table (GPU health, driver, DaemonSet pods, maintenance). `--brief`, `--wait`, `--node`, `-o json|yaml`; exits 1 when a required component fails. `-o json|yaml` is also available on `cluster`, `quota`, `queue` and `maintenance list`.
- Helm chart `helm/gryvia` installs the operators, API gateway and dashboard in one release, with optional
  Ingress, cert-manager TLS, PodDisruptionBudgets and a NetworkPolicy.
- Tag-driven release workflow: multi-arch images on ghcr.io (mirrored to Docker Hub), signed with cosign,
  OCI Helm chart and CLI binaries.
- `scripts/install.sh` and `scripts/kind-demo.sh` (a GPU-free demo with fictional data on kind).
- `POST /api/auth/login` on the gateway with rate limiting, a `usingDefaultKey` flag on `/api/auth/me`, and a
  dashboard warning while the default lab key is in use. `auth.apiKey=""` generates a random key.
- Dashboard: job logs, events and pods; searchable, sortable, paginated tables with URL state; create forms for
  traces and security policies; structured editors for tuner parameter spaces and workflow steps.
- Accessibility pass: skip link, focus management, modal focus trapping, table captions, text alternatives for
  the service graph, heat map and charts.
- Open tabs recover automatically after a redeploy.

### Changed
- The GPU operator no longer uses NVML: it runs without cgo on a distroless static image. It previously read GPU data inside its own pod, so it reported the wrong node and normally could not load the driver. `spec.drivers` is informational; nothing consumes it.
- The bundled NVIDIA device plugin and DCGM exporter DaemonSets only run on nodes labelled `nvidia.com/gpu.present=true` (configurable) and are skipped when the GPU Operator sub-chart is enabled. Previously they ran, and crashed, on every node.
- Docs no longer claim the Terraform/Ansible flow installs Kubernetes, Calico, Multus or GPU drivers; those directories are marked experimental and incomplete.
- **Breaking:** the API version is now `gryvia.io/v1alpha1` (it was `v1`), matching the project's alpha status. Objects created under `v1` are not converted: export them, change `apiVersion`, and re-apply. `scripts/rename-version.sh` documents the change; `deploy-remote.sh` recreates CRDs that still store `v1` when they hold no objects.
- **Breaking:** API kinds renamed from `Fabric*` to `Gryvia*` (for example `FabricAIJob` is now `GryviaAIJob`).
  Recreate existing objects under the new kinds; `scripts/rename-kinds.sh` documents the mapping.
- The dashboard follows netra's design system and defaults to the system light or dark setting.
- Status vocabulary unified across pages (`Succeeded` counts as completed, `Scheduling` as pending).

### Fixed
- The release rehearsal showed the eBPF collector image has never built (its kernel programs do not compile). It is no longer published and is off by default in the network-intelligence chart; documented as experimental.
- Costs are labelled as spend to date; empty states name the operator or collector that feeds the data instead
  of showing reassuring zeros.
- Rate limits count per client address instead of per proxy pod.

### Security
- Dashboard sessions are signed, expiring tokens issued by `/api/auth/login`; the API key is no longer stored in the browser.
- The dashboard bundle no longer contains a login credential; the gateway validates sign-in.
- The chart runs no root containers: the TLS Secret is mounted with `fsGroup` instead of a root init container.
- Login attempts are constant-time compared, delayed on failure and rate limited per client address.
