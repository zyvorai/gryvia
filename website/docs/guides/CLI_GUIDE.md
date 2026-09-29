# Gryvia CLI Guide

The Gryvia CLI provides a powerful command-line interface for managing GPU clusters, submitting jobs, monitoring quotas, and analyzing costs.

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

## Quick Start

### 1. Submit a Training Job

```bash
# Create job YAML
cat > llm-training.yaml <<EOF
apiVersion: gryvia.io/v1
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
gryvia quota --team ml-research --budget
```

### 4. Analyze Costs

```bash
# Monthly cost summary for all teams
gryvia cost

# Specific team with details
gryvia cost --team ml-research --detailed
```

## Common Workflows

### Data Scientist Workflow

```bash
# 1. Check available GPU quota
gryvia quota --team my-team

# 2. View available GPU nodes
gryvia list nodes

# 3. Submit training job
gryvia submit -f my-experiment.yaml --wait

# 4. Monitor job progress
gryvia status my-experiment

# 5. View logs
gryvia logs my-experiment --follow

# 6. Check cost impact
gryvia cost --team my-team
```

### Team Manager Workflow

```bash
# 1. View team quota and budget status
gryvia quota --team computer-vision --budget

# 2. List running jobs
gryvia list jobs

# 3. Check monthly spending
gryvia cost --team computer-vision --period month

# 4. Cancel expensive job if needed (sets status to "Cancelled", preserves job record)
gryvia cancel expensive-job

# 5. Monitor budget alerts
gryvia quota --team computer-vision
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
gryvia quota --team ml-research

# With budget details
gryvia quota --team ml-research --budget
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
gryvia cost --team ml-research

# Different time periods
gryvia cost --period day
gryvia cost --period week
gryvia cost --period month

# Detailed breakdown
gryvia cost --team ml-research --detailed
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

# Filter by name
gryvia queue --name my-experiment

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

Validates the YAML file structure and checks that `apiVersion` and `kind` match known Gryvia types (e.g., `gryvia.io/v1` / `GryviaAIJob`, `GryviaQuota`, `GryviaGpuNode`, etc.).

```bash
# Validate job YAML before submission
gryvia validate -f job.yaml
```

## Network Intelligence Commands

### Network Tracing

```bash
# Start a live trace session for a pod
gryvia network trace --pod training-worker-0 --duration 5m

# Trace with protocol and port filters
gryvia network trace --pod training-worker-0 \
  --protocol TCP --port 29500 --duration 2m

# Trace across a namespace
gryvia network trace --namespace ml-research --duration 1m

# View results from a completed trace session
gryvia network trace --session debug-training-latency --results
```

### Flow Analysis

```bash
# View real-time flows for a namespace
gryvia network flows --namespace ml-research

# Top flows by volume
gryvia network flows --top 20 --sort bytes

# Flows for a specific pod
gryvia network flows --pod training-worker-0

# Export flows as JSON
gryvia network flows --namespace ml-research --output json
```

### Service Graph

```bash
# View ASCII service dependency graph
gryvia network graph --namespace ml-research

# Multiple namespaces
gryvia network graph --namespace ml-research,ml-platform,storage

# Export as DOT format for Graphviz
gryvia network graph --namespace ml-research --output dot > graph.dot

# Include external endpoints
gryvia network graph --namespace ml-research --external
```

### Network Policy Management

```bash
# List active flow policies
gryvia network policy list

# Apply a flow policy
gryvia network policy apply -f flow-policy.yaml

# Audit mode (log but don't enforce)
gryvia network policy audit --namespace ml-research

# View autopolicy suggestions
gryvia network policy suggestions

# Accept an autopolicy suggestion
gryvia network policy accept suggestion-name

# View blocked traffic
gryvia network policy blocked --namespace ml-research
```

### Anomaly Detection

```bash
# View active network anomalies
gryvia network anomalies

# Filter by severity
gryvia network anomalies --severity critical

# View anomaly details
gryvia network anomalies --name latency-degradation --details

# Acknowledge an anomaly
gryvia network anomalies ack latency-degradation

# View anomaly history
gryvia network anomalies --history --days 7
```

For full documentation on network intelligence features, see [Network Intelligence Guide](NETWORK_INTELLIGENCE.md).

## Security Commands

```bash
# View security alerts
gryvia security alerts

# Filter by severity
gryvia security alerts --severity critical

# View security policy status
gryvia security status

# Apply a security policy
gryvia security policy apply -f security-policy.yaml

# View blocked threats
gryvia security alerts --type blocked

# Export security report
gryvia security alerts --output json --days 30 > security-report.json
```

## GPU Network Analysis Commands

```bash
# View NCCL communication metrics for a training job
gryvia gpu nccl --job llm-distributed-training

# Monitor GPU memory usage per pod
gryvia gpu memory --namespace ml-research

# View RDMA statistics for a node
gryvia gpu rdma --node gpu-node-01

# Training communication analysis
gryvia gpu training --job llm-distributed-training

# Straggler detection
gryvia gpu training --job llm-distributed-training --stragglers

# Gradient compression analysis
gryvia gpu training --job llm-distributed-training --gradients
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
gryvia quota --team ml-research --budget

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
    gryvia cost --team $team
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
kfq --team my-team     # Check quota
kfc --watch 5          # Watch cluster
```

### 4. Use Tab Completion

```bash
# Generate completion script (future feature)
gryvia completion bash > /etc/bash_completion.d/gryvia
gryvia completion zsh > ~/.zsh/completion/_gryvia
```

### 5. Validate Before Submitting

```bash
# Always validate YAML
gryvia validate -f job.yaml

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
