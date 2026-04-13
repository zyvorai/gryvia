# TensorReaper CLI Guide

The TensorReaper CLI provides a powerful command-line interface for managing GPU clusters, submitting jobs, monitoring quotas, and analyzing costs.

## Installation

### From Release Binary

```bash
# Download latest release
curl -LO https://github.com/ssahani/TensorReaper/releases/latest/download/tensorreaper
chmod +x tensorreaper
sudo mv tensorreaper /usr/local/bin/
```

### From Source

```bash
cd cli
cargo build --release
sudo cp target/release/tensorreaper /usr/local/bin/
```

### Verify Installation

```bash
tensorreaper --version
tensorreaper --help
```

## Quick Start

### 1. Submit a Training Job

```bash
# Create job YAML
cat > llm-training.yaml <<EOF
apiVersion: tensorreaper.ai/v1
kind: FabricAIJob
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
tensorreaper submit -f llm-training.yaml

# Submit and wait for completion
tensorreaper submit -f llm-training.yaml --wait
```

### 2. Monitor Cluster Status

```bash
# View cluster overview
tensorreaper cluster

# Watch mode (refresh every 5 seconds)
tensorreaper cluster --watch 5

# Detailed node information
tensorreaper cluster --detailed
```

### 3. Check Team Quota

```bash
# View all team quotas
tensorreaper quota

# Specific team with budget details
tensorreaper quota --team ml-research --budget
```

### 4. Analyze Costs

```bash
# Monthly cost summary for all teams
tensorreaper cost

# Specific team with details
tensorreaper cost --team ml-research --detailed
```

## Common Workflows

### Data Scientist Workflow

```bash
# 1. Check available GPU quota
tensorreaper quota --team my-team

# 2. View available GPU nodes
tensorreaper list nodes

# 3. Submit training job
tensorreaper submit -f my-experiment.yaml --wait

# 4. Monitor job progress
tensorreaper status my-experiment

# 5. View logs
tensorreaper logs my-experiment --follow

# 6. Check cost impact
tensorreaper cost --team my-team
```

### Team Manager Workflow

```bash
# 1. View team quota and budget status
tensorreaper quota --team computer-vision --budget

# 2. List running jobs
tensorreaper list jobs

# 3. Check monthly spending
tensorreaper cost --team computer-vision --period month

# 4. Cancel expensive job if needed (sets status to "Cancelled", preserves job record)
tensorreaper cancel expensive-job

# 5. Monitor budget alerts
tensorreaper quota --team computer-vision
```

### Cluster Admin Workflow

```bash
# 1. Check overall cluster health
tensorreaper health

# 2. View all nodes and their status
tensorreaper list nodes

# 3. Monitor all team quotas
tensorreaper list quotas

# 4. View cost breakdown across teams
tensorreaper cost

# 5. Watch cluster in real-time
tensorreaper cluster --watch 10
```

## Command Reference

### Job Management

#### Submit Jobs

```bash
# Basic submission
tensorreaper submit -f job.yaml

# Submit and wait for completion
tensorreaper submit -f job.yaml --wait

# Submit and follow logs
tensorreaper submit -f job.yaml --logs

# Use specific namespace
tensorreaper -n production submit -f job.yaml
```

#### List Jobs

```bash
# List all jobs in current namespace
tensorreaper list jobs

# All namespaces
tensorreaper list jobs --all-namespaces

# JSON output
tensorreaper list jobs --output json

# YAML output
tensorreaper list jobs --output yaml
```

#### Job Status

```bash
# View detailed job status
tensorreaper status my-training-job

# With log following
tensorreaper status my-job --follow
```

#### View Logs

```bash
# View last 100 lines
tensorreaper logs my-job

# Follow logs in real-time
tensorreaper logs my-job --follow

# Show last 500 lines
tensorreaper logs my-job --tail 500

# Specific replica for distributed jobs
tensorreaper logs my-job --replica 0
```

#### Cancel Jobs

Cancelling a job patches its status to "Cancelled" rather than deleting the resource. The job record is preserved for auditing and cost tracking.

```bash
# Cancel single job (sets status to "Cancelled")
tensorreaper cancel my-job

# Cancel multiple jobs
tensorreaper cancel job1 job2 job3

# Skip confirmation prompt
tensorreaper cancel my-job --yes
```

### Quota Management

#### View Quotas

```bash
# List all team quotas
tensorreaper list quotas

# Specific team quota
tensorreaper quota --team ml-research

# With budget details
tensorreaper quota --team ml-research --budget
```

#### Quota Details

```bash
# Get full quota specification
tensorreaper get quota team-ml

# YAML format
tensorreaper get quota team-ml --output yaml

# JSON format
tensorreaper get quota team-ml --output json
```

### Cost Analysis

#### View Costs

The `--period` parameter is validated and only accepts `day`, `week`, or `month`; any other value produces an error.

```bash
# All teams monthly costs
tensorreaper cost

# Specific team
tensorreaper cost --team ml-research

# Different time periods
tensorreaper cost --period day
tensorreaper cost --period week
tensorreaper cost --period month

# Detailed breakdown
tensorreaper cost --team ml-research --detailed
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
tensorreaper cluster

# Detailed view with GPU metrics
tensorreaper cluster --detailed

# Watch mode (refresh every 5 seconds, interval must be > 0)
tensorreaper cluster --watch 5

# Refresh every 30 seconds
tensorreaper cluster --watch 30
```

#### GPU Nodes

```bash
# List all GPU nodes
tensorreaper list nodes

# Get node details
tensorreaper get node gpu-worker-01

# JSON output
tensorreaper get node gpu-worker-01 --output json
```

### Queue Management

```bash
# View job queue (pending/queued/scheduling jobs)
tensorreaper queue

# Filter by name
tensorreaper queue --name my-experiment

# Watch mode (refresh every 5 seconds)
tensorreaper queue --watch 5
```

### Interactive Creation

```bash
# Interactive job creation wizard
tensorreaper create job

# Interactive quota creation wizard
tensorreaper create quota
```

The wizard prompts for framework, GPU type/count, image, distributed config, and more.

### Health Checks

```bash
# Check all components
tensorreaper health

# Check specific components
tensorreaper health gpu
tensorreaper health storage
tensorreaper health network
```

### Resource Management

#### Get Resources

```bash
# Get job details
tensorreaper get job my-training-job

# Get quota details
tensorreaper get quota team-ml

# Get node details
tensorreaper get node gpu-worker-01

# Different output formats
tensorreaper get job my-job --output yaml
tensorreaper get job my-job --output json
```

#### Delete Resources

```bash
# Delete job
tensorreaper delete job old-experiment

# Delete quota
tensorreaper delete quota team-dev

# Delete storage
tensorreaper delete storage my-storage

# Delete network
tensorreaper delete network my-network

# Skip confirmation
tensorreaper delete job my-job --yes
```

#### Validate YAML

Validates the YAML file structure and checks that `apiVersion` and `kind` match known TensorReaper types (e.g., `tensorreaper.ai/v1` / `FabricAIJob`, `FabricQuota`, `FabricGpuNode`, etc.).

```bash
# Validate job YAML before submission
tensorreaper validate -f job.yaml
```

## Advanced Usage

### Using Different Contexts

```bash
# Use specific Kubernetes context
tensorreaper --context production list jobs

# Use staging context
tensorreaper --context staging cluster
```

### Custom Namespaces

```bash
# Use specific namespace
tensorreaper -n ml-production list jobs

# Override default namespace
tensorreaper -n experiments submit -f job.yaml
```

### Verbose Logging

```bash
# Enable debug logging
tensorreaper --verbose submit -f job.yaml

# Verbose output for troubleshooting
tensorreaper -v list jobs
```

### Output Formats

The CLI supports three output formats: `table`, `json`, and `yaml`. The `--output` flag is validated and any other value produces an error.

```bash
# Pretty table (default)
tensorreaper list jobs

# JSON (machine-readable)
tensorreaper list jobs --output json | jq '.items[].status.phase'

# YAML
tensorreaper list jobs --output yaml
```

## Examples

### Complete Training Workflow

```bash
#!/bin/bash
set -e

# Check quota
echo "Checking quota..."
tensorreaper quota --team ml-research --budget

# Submit job
echo "Submitting job..."
tensorreaper submit -f llm-training.yaml

# Wait a bit for job to start
sleep 10

# Monitor status
echo "Monitoring job..."
tensorreaper status llm-training

# Follow logs
tensorreaper logs llm-training --follow
```

### Monitor Multiple Jobs

```bash
#!/bin/bash

JOBS=("job1" "job2" "job3")

for job in "${JOBS[@]}"; do
    echo "=== $job ==="
    tensorreaper status $job
    echo ""
done
```

### Cost Report Script

```bash
#!/bin/bash

# Generate monthly cost report
echo "TensorReaper Monthly Cost Report"
echo "=============================="
echo ""

tensorreaper cost --period month

echo ""
echo "Per-Team Breakdown:"
echo "-------------------"

for team in ml-research computer-vision nlp; do
    echo ""
    echo "Team: $team"
    tensorreaper cost --team $team
done
```

### Cluster Health Dashboard

```bash
#!/bin/bash

while true; do
    clear
    echo "TensorReaper Cluster Dashboard"
    echo "============================"
    echo ""

    tensorreaper cluster

    echo ""
    tensorreaper health

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
tensorreaper --context my-cluster list jobs
```

### Permission Errors

```bash
# Check if you can access CRDs
kubectl auth can-i list fabricaijobs
kubectl auth can-i get fabricquotas

# View your permissions
kubectl auth can-i --list
```

### CRD Not Found

```bash
# Verify CRDs are installed
kubectl get crds | grep tensorreaper

# Expected output:
# fabricaijobs.tensorreaper.ai
# fabricgpunodes.tensorreaper.ai
# fabricnetworks.tensorreaper.ai
# fabricquotas.tensorreaper.ai
# fabricstorages.tensorreaper.ai
```

### Debug Mode

```bash
# Enable verbose logging
tensorreaper --verbose list jobs

# Check CLI version
tensorreaper --version

# View help for specific command
tensorreaper submit --help
```

## Tips and Best Practices

### 1. Use Watch Mode for Monitoring

```bash
# Watch cluster status
tensorreaper cluster --watch 5

# Monitor job in terminal
watch -n 5 "tensorreaper status my-job"
```

### 2. Combine with jq for Filtering

```bash
# Get all running jobs
tensorreaper list jobs --output json | \
  jq -r '.items[] | select(.status.phase=="Running") | .metadata.name'

# Count jobs by status
tensorreaper list jobs --output json | \
  jq -r '.items | group_by(.status.phase) | map({status: .[0].status.phase, count: length})'
```

### 3. Create Aliases

```bash
# Add to ~/.bashrc or ~/.zshrc
alias kf="tensorreaper"
alias kfj="tensorreaper list jobs"
alias kfq="tensorreaper quota"
alias kfc="tensorreaper cluster"

# Usage
kfj                    # List jobs
kfq --team my-team     # Check quota
kfc --watch 5          # Watch cluster
```

### 4. Use Tab Completion

```bash
# Generate completion script (future feature)
tensorreaper completion bash > /etc/bash_completion.d/tensorreaper
tensorreaper completion zsh > ~/.zsh/completion/_tensorreaper
```

### 5. Validate Before Submitting

```bash
# Always validate YAML
tensorreaper validate -f job.yaml

# Then submit if valid
tensorreaper submit -f job.yaml
```

## Integration with CI/CD

### GitLab CI Example

```yaml
submit_training:
  stage: train
  script:
    - tensorreaper submit -f jobs/llm-training.yaml --wait
    - tensorreaper logs llm-training
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
      - name: Install TensorReaper CLI
        run: |
          curl -LO https://github.com/ssahani/TensorReaper/releases/latest/download/tensorreaper
          chmod +x tensorreaper
      - name: Submit Job
        run: ./tensorreaper submit -f job.yaml --wait
```

## Recently Implemented

- [x] Interactive job/quota creation wizard (`tensorreaper create job`, `tensorreaper create quota`)
- [x] Real-time log streaming (`tensorreaper logs --follow`)
- [x] Job queue monitoring (`tensorreaper queue`, `tensorreaper queue --watch 5`)
- [x] Storage and network deletion (`tensorreaper delete storage/network`)
- [x] Storage and network health checks (`tensorreaper health storage`, `tensorreaper health network`)
- [x] Detailed cluster view with per-GPU metrics (`tensorreaper cluster --detailed`)
- [x] Status follow mode (`tensorreaper status --follow`)
- [x] Submit with log following (`tensorreaper submit --logs`)
- [x] Cost detailed breakdown (`tensorreaper cost --detailed`)

## Future Features

Coming soon:
- [ ] Job templates library
- [ ] Shell completion (bash/zsh/fish)
- [ ] Multi-cluster support
- [ ] Job history and analytics

## Support

- **Documentation**: https://tensor-reaper.readthedocs.io
- **Issues**: https://github.com/ssahani/TensorReaper/issues
- **Discussions**: https://github.com/ssahani/TensorReaper/discussions

## Contributing

See [CONTRIBUTING.md](../CONTRIBUTING.md) for development guidelines.
