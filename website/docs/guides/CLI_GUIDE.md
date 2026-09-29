# Gryvia CLI Guide

The Gryvia CLI provides a command-line interface for managing GPU clusters, submitting jobs, monitoring quotas, and analyzing costs.

Run `gryvia <command> --help` for the authoritative list of options for any command; this guide may lag behind the CLI.

## Installation

### From Release Binary

```bash
# Download latest release
curl -LO https://github.com/zyvorai/gryvia/releases/latest/download/gryvia
chmod +x gryvia
sudo mv gryvia /usr/local/bin/
```

### From Source

```bash
cd cli
cargo build --release
sudo cp target/release/gryvia /usr/local/bin/
```

### Verify Installation

```bash
gryvia --version
gryvia --help
```

## Platform status

`gryvia status` with no argument reports on the whole platform, in the style of `cilium status`: one line per
component with an `OK`, `Warning`, `Error` or `disabled` marker, then workload readiness, cluster totals, image
versions, any errors, and a per-node table. It reads only the Kubernetes API, so it needs no agent on the nodes.

```bash
gryvia status
```

Example output (from the CLI's own snapshot tests):

```text
    ______      Gryvia:       OK
   /      \     Operators:    OK  gpu, ai, quota
  /   G    \    API gateway:  OK  2/2 ready
  \        /    Dashboard:    OK  2/2 ready
   \______/     GPU nodes:    OK  2/2 ready
                GPU add-ons:  OK  dcgm-exporter 2/2, nvidia-device-plugin 2/2
                Collectors:   disabled  not installed

Deployment gryvia-gpu-operator          Desired: 1, Ready: 1/1, Available: 1/1
Deployment gryvia-ai-operator           Desired: 1, Ready: 1/1, Available: 1/1
Deployment gryvia-quota-operator        Desired: 1, Ready: 1/1, Available: 1/1
Deployment gryvia-api-gateway           Desired: 2, Ready: 2/2, Available: 2/2
Deployment gryvia-ui                    Desired: 2, Ready: 2/2, Available: 2/2
DaemonSet  gryvia-dcgm-exporter         Desired: 2, Ready: 2/2, Available: 2/2
DaemonSet  gryvia-nvidia-device-plugin  Desired: 2, Ready: 2/2, Available: 2/2

Cluster Pods:   5/5 running
Jobs:           1 running, 2 pending, 3 completed, 0 failed
Namespace:      gryvia-system

Image versions
  gryvia-ai-operator           ghcr.io/zyvorai/gryvia-ai-operator:1.0.0
  gryvia-api-gateway           ghcr.io/zyvorai/gryvia-api-gateway:1.0.0
  gryvia-dcgm-exporter         ghcr.io/zyvorai/gryvia-dcgm-exporter:1.0.0
  gryvia-gpu-operator          ghcr.io/zyvorai/gryvia-gpu-operator:1.0.0
  gryvia-nvidia-device-plugin  ghcr.io/zyvorai/gryvia-nvidia-device-plugin:1.0.0
  gryvia-quota-operator        ghcr.io/zyvorai/gryvia-quota-operator:1.0.0
  gryvia-ui                    ghcr.io/zyvorai/gryvia-ui:1.0.0

Nodes
  NODE    STATE  GPUS      DRIVER              GPU HEALTH  DCGM  PLUGIN  COLLECTOR  PODS
  node-a  Ready  8 x H100  550.54 / CUDA 12.4  8/8         ✓     ✓       -          3
  node-b  Ready  8 x H100  550.54 / CUDA 12.4  8/8         ✓     ✓       -          2
```

The per-node table has one row per Kubernetes node (plus a row for any `GryviaGpuNode` whose node does not exist):
readiness, GPUs, driver and CUDA versions, how many GPUs report healthy, whether the DCGM exporter, NVIDIA device
plugin and collector pods run on that node (`✓` running, `✗` present but not ready, `-` none), pod count, and a note
when the node is cordoned or in maintenance. When something is wrong the platform line turns `Warning` or `Error`
and the problems are listed:

```text
    ______      Gryvia:       Warning
   /      \     Operators:    OK  gpu, ai
  /   G    \    API gateway:  Warning  1/2 ready
  \        /    Dashboard:    OK  2/2 ready
   \______/     GPU nodes:    Warning  2/3 ready
                GPU add-ons:  OK  dcgm-exporter 2/2, nvidia-device-plugin 2/2
                Collectors:   disabled  not installed

Deployment gryvia-gpu-operator          Desired: 1, Ready: 1/1, Available: 1/1
Deployment gryvia-ai-operator           Desired: 1, Ready: 1/1, Available: 1/1
Deployment gryvia-api-gateway           Desired: 2, Ready: 1/2, Available: 1/2
Deployment gryvia-ui                    Desired: 2, Ready: 2/2, Available: 2/2
DaemonSet  gryvia-dcgm-exporter         Desired: 2, Ready: 2/2, Available: 2/2
DaemonSet  gryvia-nvidia-device-plugin  Desired: 2, Ready: 2/2, Available: 2/2

Cluster Pods:   5/6 running
Jobs:           1 running, 2 pending, 3 completed, 0 failed
Namespace:      gryvia-system

Image versions
  gryvia-ai-operator           ghcr.io/zyvorai/gryvia-ai-operator:1.0.0
  gryvia-api-gateway           ghcr.io/zyvorai/gryvia-api-gateway:1.0.0
  gryvia-dcgm-exporter         ghcr.io/zyvorai/gryvia-dcgm-exporter:1.0.0
  gryvia-gpu-operator          ghcr.io/zyvorai/gryvia-gpu-operator:1.0.0
  gryvia-nvidia-device-plugin  ghcr.io/zyvorai/gryvia-nvidia-device-plugin:1.0.0
  gryvia-ui                    ghcr.io/zyvorai/gryvia-ui:1.0.0

Warnings
  ! could not list jobs: forbidden

Errors
  ✗ pod gryvia-quota-operator-q1: ImagePullBackOff

Nodes
  NODE    STATE         GPUS      DRIVER              GPU HEALTH  DCGM  PLUGIN  COLLECTOR  PODS  NOTES
  ghost   no such node  8 x H100  550.54 / CUDA 12.4  Failed      -     -       -          0
  node-a  Ready         8 x H100  550.54 / CUDA 12.4  8/8         ✓     ✓       -          3     driver update
  node-b  NotReady      8 x H100  550.54 / CUDA 12.4  6/8         ✓     ✓       -          3
```

Only the required components (operators, API gateway, dashboard) decide the result: `gryvia status` exits with
code 1 when one of them is in `Error`. GPU nodes, GPU add-ons and collectors are reported but never fail the
command, so a cluster without GPUs (for example the kind demo) still reports healthy.

Useful flags:

```bash
gryvia status --brief                       # OK, or the failing components; exit code 1 on failure
gryvia status --wait --wait-timeout 5m      # block until the platform is OK (for scripts and CI)
gryvia status --node gpu-node-01            # only that node in the per-node table
gryvia status -o json                       # the same data as JSON (or -o yaml)
gryvia status --platform-namespace my-ns    # when Gryvia is not installed in gryvia-system
gryvia status my-job                        # with a job name it shows that job instead
```

## Global options, colors and output formats

`gryvia --help` lists the commands in groups (Workloads, Cluster, Operations, Observe, Utilities) and every
command's `--help` ends with examples. Help and output are colored when stdout is a terminal.

| Option | Effect |
|--------|--------|
| `--no-color` | Turn colors off. The `NO_COLOR` environment variable does the same. |
| `CLICOLOR_FORCE=1` | Keep colors when piping (for example into `less -R`). |
| `-o, --output table\|json\|yaml` | Output format of commands that print data (`GRYVIA_OUTPUT` sets the default where supported). |
| `--context`, `-n/--namespace` | Kubernetes context and namespace. |

Piped output has no color codes, so `gryvia list jobs | grep Running` and `gryvia status -o json | jq` work as
expected. Errors are printed to stderr with a short hint, for example when no kubeconfig is found.

```bash
gryvia --no-color status
NO_COLOR=1 gryvia cluster --detailed
```

## Shell completion and versions

```bash
gryvia completion bash > /etc/bash_completion.d/gryvia
gryvia completion zsh > ~/.zsh/completions/_gryvia
gryvia completion fish > ~/.config/fish/completions/gryvia.fish
```

`gryvia version` prints the client version and the image of every workload in the platform namespace;
`gryvia version --client` needs no cluster.

## Quick Start

### 1. Submit a Training Job

```bash
# Create job YAML
cat > llm-training.yaml <<EOF
apiVersion: gryvia.io/v1alpha1
kind: GryviaAIJob
metadata:
  name: llm-training
spec:
  framework: pytorch
  resources:
    gpuType: H100
    gpuCount: 8
  image: nvcr.io/nvidia/pytorch:24.01-py3
  command: ["python", "train.py"]
EOF

# Submit job
gryvia submit -f llm-training.yaml

# Submit and wait for completion
gryvia submit -f llm-training.yaml --wait
```

### 2. Monitor Cluster Status

```bash
# View cluster overview
gryvia cluster

# Watch mode (refresh every 5 seconds)
gryvia cluster --watch 5

# Detailed node information
gryvia cluster --detailed
```

### 3. Check Team Quota

```bash
# View all team quotas
gryvia quota

# Specific team with budget details
gryvia quota ml-research --budget
```

### 4. Analyze Costs

```bash
# Monthly cost summary for all teams
gryvia cost

# Specific team with details
gryvia cost ml-research --detailed
```

## Common Workflows

### Data Scientist Workflow

```bash
# 1. Check available GPU quota
gryvia quota my-team

# 2. View available GPU nodes
gryvia list nodes

# 3. Submit training job
gryvia submit -f my-experiment.yaml --wait

# 4. Monitor job progress
gryvia status my-experiment

# 5. View logs
gryvia logs my-experiment --follow

# 6. Check cost impact
gryvia cost my-team
```

### Team Manager Workflow

```bash
# 1. View team quota and budget status
gryvia quota computer-vision --budget

# 2. List running jobs
gryvia list jobs

# 3. Check monthly spending
gryvia cost computer-vision --period month

# 4. Cancel expensive job if needed (sets status to "Cancelled", preserves job record)
gryvia cancel expensive-job

# 5. Monitor budget alerts
gryvia quota computer-vision
```

### Cluster Admin Workflow

```bash
# 1. Check overall cluster health
gryvia health

# 2. View all nodes and their status
gryvia list nodes

# 3. Monitor all team quotas
gryvia list quotas

# 4. View cost breakdown across teams
gryvia cost

# 5. Watch cluster in real-time
gryvia cluster --watch 10
```

## Command Reference

### Job Management

#### Submit Jobs

```bash
# Basic submission
gryvia submit -f job.yaml

# Submit and wait for completion
gryvia submit -f job.yaml --wait

# Submit and follow logs
gryvia submit -f job.yaml --logs

# Use specific namespace
gryvia -n production submit -f job.yaml
```

#### List Jobs

```bash
# List all jobs in current namespace
gryvia list jobs

# All namespaces
gryvia list jobs --all-namespaces

# JSON output
gryvia list jobs --output json

# YAML output
gryvia list jobs --output yaml
```

#### Job Status

```bash
# View detailed job status
gryvia status my-training-job

# With log following
gryvia status my-job --follow
```

#### View Logs

```bash
# View last 100 lines
gryvia logs my-job

# Follow logs in real-time
gryvia logs my-job --follow

# Show last 500 lines
gryvia logs my-job --tail 500

# Specific replica for distributed jobs
gryvia logs my-job --replica 0
```

#### Cancel Jobs

Cancelling a job patches its status to "Cancelled" rather than deleting the resource. The job record is preserved for auditing and cost tracking.

```bash
# Cancel single job (sets status to "Cancelled")
gryvia cancel my-job

# Cancel multiple jobs
gryvia cancel job1 job2 job3

# Skip confirmation prompt
gryvia cancel my-job --yes
```

### Quota Management

#### View Quotas

```bash
# List all team quotas
gryvia list quotas

# Specific team quota
gryvia quota ml-research

# With budget details
gryvia quota ml-research --budget
```

#### Quota Details

```bash
# Get full quota specification
gryvia get quota team-ml

# YAML format
gryvia get quota team-ml --output yaml

# JSON format
gryvia get quota team-ml --output json
```

### Cost Analysis

#### View Costs

The `--period` parameter is validated and only accepts `day`, `week`, or `month`; any other value produces an error.

```bash
# All teams monthly costs
gryvia cost

# Specific team
gryvia cost ml-research

# Different time periods
gryvia cost --period day
gryvia cost --period week
gryvia cost --period month

# Detailed breakdown
gryvia cost ml-research --detailed
```

#### Budget Alerts

The CLI highlights budget warnings:
- 🟢 Green: < 75% of budget used
- 🟡 Yellow: 75-99% of budget used
- 🔴 Red: ≥100% of budget used

### Cluster Overview

#### Cluster Status

```bash
# Basic cluster overview
gryvia cluster

# Detailed view with GPU metrics
gryvia cluster --detailed

# Watch mode (refresh every 5 seconds, interval must be > 0)
gryvia cluster --watch 5

# Refresh every 30 seconds
gryvia cluster --watch 30
```

#### GPU Nodes

```bash
# List all GPU nodes
gryvia list nodes

# Get node details
gryvia get node gpu-worker-01

# JSON output
gryvia get node gpu-worker-01 --output json
```

### Queue Management

```bash
# View job queue (pending/queued/scheduling jobs)
gryvia queue

# Show a single queue by name
gryvia queue my-experiment

# Watch mode (refresh every 5 seconds)
gryvia queue --watch 5
```

### Interactive Creation

```bash
# Interactive job creation wizard
gryvia create job

# Interactive quota creation wizard
gryvia create quota
```

The wizard prompts for framework, GPU type/count, image, distributed config, and more.

### Health Checks

```bash
# Check all components
gryvia health

# Check specific components
gryvia health gpu
gryvia health storage
gryvia health network
```

### Resource Management

#### Get Resources

```bash
# Get job details
gryvia get job my-training-job

# Get quota details
gryvia get quota team-ml

# Get node details
gryvia get node gpu-worker-01

# Different output formats
gryvia get job my-job --output yaml
gryvia get job my-job --output json
```

#### Delete Resources

```bash
# Delete job
gryvia delete job old-experiment

# Delete quota
gryvia delete quota team-dev

# Delete storage
gryvia delete storage my-storage

# Delete network
gryvia delete network my-network

# Skip confirmation
gryvia delete job my-job --yes
```

#### Validate YAML

Validates the YAML file structure and checks that `apiVersion` and `kind` match known Gryvia types (e.g., `gryvia.io/v1alpha1` / `GryviaAIJob`, `GryviaQuota`, `GryviaGpuNode`, etc.).

```bash
# Validate job YAML before submission
gryvia validate job.yaml
```

### GPU Capacity

`gryvia capacity` is a read-only snapshot computed from cluster objects. Supply is the sum of
`GryviaGpuNode` `spec.gpuCount` per `spec.gpuType`; allocation is the GPUs held by Running jobs; demand is the
GPUs requested by Pending, Queued and Scheduling jobs (the same phase grouping the gateway uses, where
Succeeded and Completed both mean completed). Per type it reports free GPUs, the pending shortfall (demand
that free GPUs of that type cannot cover) and the headroom left after pending demand.

```bash
# Capacity per GPU type
gryvia capacity

# One GPU type only (case-insensitive)
gryvia capacity --gpu-type H100

# Machine-readable
gryvia capacity --output json
```

Notes on what the numbers mean:

- Jobs that do not name a GPU type (or set it to `any`) cannot be attributed to one type. They are counted only
  in the cluster-wide totals, shortfall and headroom, and are excluded when `--gpu-type` is set.
- A job type that no node registers appears as its own row with 0 total GPUs.
- If running jobs hold more GPUs than nodes provide (for example after a node was removed), free is shown as 0
  and the row is flagged with the excess.
- It does not forecast growth or estimate purchases; it counts GPUs as they are right now.

### Node Maintenance

`gryvia maintenance` wraps the cordon workflow. It marks nodes with two annotations,
`gryvia.io/maintenance` (RFC 3339 start time) and `gryvia.io/maintenance-reason`, so that `list` can show who is
out of service and for how long. The node names are Kubernetes node names.

```bash
# Cordon a node and record why
gryvia maintenance start gpu-node-05 --reason "replace failed PSU"

# Cordon and also evict the pods on it (asks for confirmation; add --yes to skip)
gryvia maintenance start gpu-node-05 --reason "firmware update" --drain

# Show nodes currently marked, with age and reason
gryvia maintenance list

# Uncordon and remove both annotations
gryvia maintenance end gpu-node-05
```

`--drain` only uses the Kubernetes Eviction API, so PodDisruptionBudgets are honoured. It skips DaemonSet pods,
mirror (static) pods and pods that already finished or are terminating. A pod whose eviction is refused (HTTP 429,
usually a PodDisruptionBudget) is reported as blocked and left running; the command then exits with an error and
the node stays cordoned. Pods are never deleted directly. Running `start` again on a node that is already marked
keeps the original start time, which makes retrying a blocked drain safe.

## Tenants, catalog and usage

These commands cover the [GPU as a Service](./GPU_AS_A_SERVICE.md) flow: a provider publishes SKUs, creates tenants and reads metered usage. They talk to the Kubernetes API directly, like the other commands, and need the `gryvia.io/v1alpha1` CRDs installed.

### `gryvia catalog` (alias `skus`)

Lists the GPU SKUs on offer (GryviaGpuSku): GPU type, GPUs per unit, rate per hour, spot discount and whether the SKU is enabled, with a total line. Output formats: `table`, `json`, `yaml`.

```bash
gryvia catalog
gryvia skus -o json
```

### `gryvia tenant`

A tenant is a cluster-scoped GryviaTenant. Its workloads run in the namespace `tenant-<name>`, so the name must be lowercase letters, digits and `-`, at most 56 characters.

```bash
gryvia tenant list
gryvia tenant get acme
gryvia tenant create acme --display-name "Acme Corp" --allowed-sku h100-8x --allowed-sku a100 --max-gpus 16 --isolated true
gryvia tenant delete acme --yes
```

- `list` and `get` accept `-o table|json|yaml`.
- `create` accepts `--display-name`, repeatable `--allowed-sku` (default: all enabled SKUs), `--max-gpus` (maximum concurrent GPUs) and `--isolated true|false` (network isolation from other tenants). It applies the object, so running it again updates the tenant.
- `delete` asks for confirmation unless `--yes` is given. Deleting a tenant can remove its namespace and everything in it.

### `gryvia usage`

Aggregates the metered usage records (GryviaUsageRecord) into GPU hours, cost and the number of distinct jobs per tenant, SKU or day, with a total row.

```bash
gryvia usage
gryvia usage --tenant acme --from 2026-09-01 --to 2026-09-30
gryvia usage --group-by sku -o json
gryvia usage --group-by day -o csv
```

- `--tenant` restricts to one tenant.
- `--from` and `--to` filter on each record's start time. A date-only `--to` includes that whole day (UTC). RFC 3339 timestamps are also accepted.
- `--group-by tenant|sku|day` (default `tenant`).
- `-o table|json|yaml|csv`. CSV has a header row and a final `total` row, and cells starting with `=`, `+`, `-` or `@` are prefixed with `'` so spreadsheets do not run them as formulas. When records use different currencies the currency shows as `MIXED`.
- Costs are estimates: job run time times the SKU rate. No invoices or payments are processed.

## Network Intelligence Commands

### Network Overview

```bash
# Network health overview
gryvia network status
```

### Network Tracing

`gryvia network trace` takes the target service as a positional argument. The capture level is `l3`, `l4` or `l7` (default `l7`), and the duration defaults to `2m`.

```bash
# Trace a service for 5 minutes
gryvia network trace training-service --duration 5m

# Trace at L4 in a specific namespace
gryvia network trace training-service --level l4 --trace-namespace ml-research --duration 2m

# Stream results live
gryvia network trace training-service --follow
```

### Flow Analysis

```bash
# Recent flows in a namespace
gryvia network flows --flow-namespace ml-research

# Flows for a specific service over the last hour
gryvia network flows --service training-service --last 1h

# Export flows as JSON
gryvia network flows --flow-namespace ml-research --output json
```

### Service Graph

```bash
# View ASCII service dependency graph
gryvia network graph --graph-namespace ml-research

# JSON output
gryvia network graph --graph-namespace ml-research --format json
```

### Network Policy Management

```bash
# List flow policies
gryvia network policy list

# Show auto-generated policy suggestions
gryvia network policy suggest --policy-namespace ml-research

# Apply a suggested policy by name
gryvia network policy apply suggestion-name --policy-namespace ml-research
```

### Anomaly Detection

```bash
# View detected network anomalies
gryvia network anomalies

# Filter by severity (critical, high, medium, low)
gryvia network anomalies --severity critical

# Filter by service, as JSON
gryvia network anomalies --service training-service --output json
```

For full documentation on network intelligence features, see [Network Intelligence Guide](NETWORK_INTELLIGENCE.md).

## Security Commands

```bash
# View security alerts
gryvia security alerts

# Filter by severity
gryvia security alerts --severity critical

# Filter by alert type (escape, mining, exfiltration, privesc)
gryvia security alerts --alert-type mining

# View security status overview
gryvia security status

# List security policies
gryvia security policy list

# Create a security policy
gryvia security policy create my-policy \
  --namespaces ml-research,ml-platform --rules escape,mining --auto-block
```

## GPU Network Analysis Commands

```bash
# View NCCL communication metrics for a training job
gryvia gpu nccl --job llm-distributed-training

# View GPU memory transfer stats for a node
gryvia gpu memory --node gpu-node-01

# View RDMA statistics for a node
gryvia gpu rdma --node gpu-node-01

# Training insights for a job
gryvia gpu training --job llm-distributed-training
```

For full documentation on GPU-level network analysis, see [Network Intelligence Guide](NETWORK_INTELLIGENCE.md#gpu-programs).

## Advanced Usage

### Using Different Contexts

```bash
# Use specific Kubernetes context
gryvia --context production list jobs

# Use staging context
gryvia --context staging cluster
```

### Custom Namespaces

```bash
# Use specific namespace
gryvia -n ml-production list jobs

# Override default namespace
gryvia -n experiments submit -f job.yaml
```

### Verbose Logging

```bash
# Enable debug logging
gryvia --verbose submit -f job.yaml

# Verbose output for troubleshooting
gryvia -v list jobs
```

### Output Formats

The CLI supports three output formats: `table`, `json`, and `yaml`. The `--output` flag is validated and any other value produces an error.

```bash
# Pretty table (default)
gryvia list jobs

# JSON (machine-readable)
gryvia list jobs --output json | jq '.items[].status.phase'

# YAML
gryvia list jobs --output yaml
```

## Examples

### Complete Training Workflow

```bash
#!/bin/bash
set -e

# Check quota
echo "Checking quota..."
gryvia quota ml-research --budget

# Submit job
echo "Submitting job..."
gryvia submit -f llm-training.yaml

# Wait a bit for job to start
sleep 10

# Monitor status
echo "Monitoring job..."
gryvia status llm-training

# Follow logs
gryvia logs llm-training --follow
```

### Monitor Multiple Jobs

```bash
#!/bin/bash

JOBS=("job1" "job2" "job3")

for job in "${JOBS[@]}"; do
    echo "=== $job ==="
    gryvia status $job
    echo ""
done
```

### Cost Report Script

```bash
#!/bin/bash

# Generate monthly cost report
echo "Gryvia Monthly Cost Report"
echo "=============================="
echo ""

gryvia cost --period month

echo ""
echo "Per-Team Breakdown:"
echo "-------------------"

for team in ml-research computer-vision nlp; do
    echo ""
    echo "Team: $team"
    gryvia cost $team
done
```

### Cluster Health Dashboard

```bash
#!/bin/bash

while true; do
    clear
    echo "Gryvia Cluster Dashboard"
    echo "============================"
    echo ""

    gryvia cluster

    echo ""
    gryvia health

    sleep 30
done
```

## Troubleshooting

### Connection Issues

```bash
# Verify Kubernetes connection
kubectl cluster-info

# Check current context
kubectl config current-context

# List available contexts
kubectl config get-contexts

# Use specific context
gryvia --context my-cluster list jobs
```

### Permission Errors

```bash
# Check if you can access CRDs
kubectl auth can-i list gryviaaijobs
kubectl auth can-i get gryviaquotas

# View your permissions
kubectl auth can-i --list
```

### CRD Not Found

```bash
# Verify CRDs are installed
kubectl get crds | grep gryvia

# Expected output:
# gryviaaijobs.gryvia.io
# gryviagpunodes.gryvia.io
# gryvianetworks.gryvia.io
# gryviaquotas.gryvia.io
# gryviastorages.gryvia.io
```

### Debug Mode

```bash
# Enable verbose logging
gryvia --verbose list jobs

# Check CLI version
gryvia --version

# View help for specific command
gryvia submit --help
```

## Tips and Best Practices

### 1. Use Watch Mode for Monitoring

```bash
# Watch cluster status
gryvia cluster --watch 5

# Monitor job in terminal
watch -n 5 "gryvia status my-job"
```

### 2. Combine with jq for Filtering

```bash
# Get all running jobs
gryvia list jobs --output json | \
  jq -r '.items[] | select(.status.phase=="Running") | .metadata.name'

# Count jobs by status
gryvia list jobs --output json | \
  jq -r '.items | group_by(.status.phase) | map({status: .[0].status.phase, count: length})'
```

### 3. Create Aliases

```bash
# Add to ~/.bashrc or ~/.zshrc
alias kf="gryvia"
alias kfj="gryvia list jobs"
alias kfq="gryvia quota"
alias kfc="gryvia cluster"

# Usage
kfj                    # List jobs
kfq my-team            # Check quota
kfc --watch 5          # Watch cluster
```

### 4. Validate Before Submitting

```bash
# Always validate YAML
gryvia validate job.yaml

# Then submit if valid
gryvia submit -f job.yaml
```

## Integration with CI/CD

### GitLab CI Example

```yaml
submit_training:
  stage: train
  script:
    - gryvia submit -f jobs/llm-training.yaml --wait
    - gryvia logs llm-training
  only:
    - main
```

### GitHub Actions Example

```yaml
name: Submit Training Job
on:
  push:
    branches: [main]

jobs:
  train:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      - name: Install Gryvia CLI
        run: |
          curl -LO https://github.com/zyvorai/gryvia/releases/latest/download/gryvia
          chmod +x gryvia
      - name: Submit Job
        run: ./gryvia submit -f job.yaml --wait
```

## Recently Implemented

- [x] Interactive job/quota creation wizard (`gryvia create job`, `gryvia create quota`)
- [x] Real-time log streaming (`gryvia logs --follow`)
- [x] Job queue monitoring (`gryvia queue`, `gryvia queue --watch 5`)
- [x] Storage and network deletion (`gryvia delete storage/network`)
- [x] Storage and network health checks (`gryvia health storage`, `gryvia health network`)
- [x] Detailed cluster view with per-GPU metrics (`gryvia cluster --detailed`)
- [x] Status follow mode (`gryvia status --follow`)
- [x] Submit with log following (`gryvia submit --logs`)
- [x] Cost detailed breakdown (`gryvia cost --detailed`)
- [x] GPU capacity report (`gryvia capacity`)
- [x] Node maintenance marking and draining (`gryvia maintenance start/end/list`)

## Future Features

Coming soon:
- [ ] Job templates library
- [ ] Shell completion (bash/zsh/fish)
- [ ] Multi-cluster support
- [ ] Job history and analytics

## Support

- **Documentation**: https://gryvia.readthedocs.io
- **Issues**: https://github.com/zyvorai/gryvia/issues
- **Discussions**: https://github.com/zyvorai/gryvia/discussions

## Contributing

See [CONTRIBUTING.md](https://github.com/zyvorai/gryvia/blob/main/CONTRIBUTING.md) for development guidelines.
