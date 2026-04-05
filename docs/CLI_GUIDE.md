# KubeFabric CLI Guide

The KubeFabric CLI provides a powerful command-line interface for managing GPU clusters, submitting jobs, monitoring quotas, and analyzing costs.

## Installation

### From Release Binary

```bash
# Download latest release
curl -LO https://github.com/ssahani/kube-fabric/releases/latest/download/kubefabric
chmod +x kubefabric
sudo mv kubefabric /usr/local/bin/
```

### From Source

```bash
cd cli
cargo build --release
sudo cp target/release/kubefabric /usr/local/bin/
```

### Verify Installation

```bash
kubefabric --version
kubefabric --help
```

## Quick Start

### 1. Submit a Training Job

```bash
# Create job YAML
cat > llm-training.yaml <<EOF
apiVersion: kubefabric.ai/v1
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
kubefabric submit -f llm-training.yaml

# Submit and wait for completion
kubefabric submit -f llm-training.yaml --wait
```

### 2. Monitor Cluster Status

```bash
# View cluster overview
kubefabric cluster

# Watch mode (refresh every 5 seconds)
kubefabric cluster --watch 5

# Detailed node information
kubefabric cluster --detailed
```

### 3. Check Team Quota

```bash
# View all team quotas
kubefabric quota

# Specific team with budget details
kubefabric quota --team ml-research --budget
```

### 4. Analyze Costs

```bash
# Monthly cost summary for all teams
kubefabric cost

# Specific team with details
kubefabric cost --team ml-research --detailed
```

## Common Workflows

### Data Scientist Workflow

```bash
# 1. Check available GPU quota
kubefabric quota --team my-team

# 2. View available GPU nodes
kubefabric list nodes

# 3. Submit training job
kubefabric submit -f my-experiment.yaml --wait

# 4. Monitor job progress
kubefabric status my-experiment

# 5. View logs
kubefabric logs my-experiment --follow

# 6. Check cost impact
kubefabric cost --team my-team
```

### Team Manager Workflow

```bash
# 1. View team quota and budget status
kubefabric quota --team computer-vision --budget

# 2. List running jobs
kubefabric list jobs

# 3. Check monthly spending
kubefabric cost --team computer-vision --period month

# 4. Cancel expensive job if needed
kubefabric cancel expensive-job

# 5. Monitor budget alerts
kubefabric quota --team computer-vision
```

### Cluster Admin Workflow

```bash
# 1. Check overall cluster health
kubefabric health

# 2. View all nodes and their status
kubefabric list nodes

# 3. Monitor all team quotas
kubefabric list quotas

# 4. View cost breakdown across teams
kubefabric cost

# 5. Watch cluster in real-time
kubefabric cluster --watch 10
```

## Command Reference

### Job Management

#### Submit Jobs

```bash
# Basic submission
kubefabric submit -f job.yaml

# Submit and wait for completion
kubefabric submit -f job.yaml --wait

# Submit and follow logs
kubefabric submit -f job.yaml --logs

# Use specific namespace
kubefabric -n production submit -f job.yaml
```

#### List Jobs

```bash
# List all jobs in current namespace
kubefabric list jobs

# All namespaces
kubefabric list jobs --all-namespaces

# JSON output
kubefabric list jobs --output json

# YAML output
kubefabric list jobs --output yaml
```

#### Job Status

```bash
# View detailed job status
kubefabric status my-training-job

# With log following
kubefabric status my-job --follow
```

#### View Logs

```bash
# View last 100 lines
kubefabric logs my-job

# Follow logs in real-time
kubefabric logs my-job --follow

# Show last 500 lines
kubefabric logs my-job --tail 500

# Specific replica for distributed jobs
kubefabric logs my-job --replica 0
```

#### Cancel Jobs

```bash
# Cancel single job
kubefabric cancel my-job

# Cancel multiple jobs
kubefabric cancel job1 job2 job3

# Skip confirmation prompt
kubefabric cancel my-job --yes
```

### Quota Management

#### View Quotas

```bash
# List all team quotas
kubefabric list quotas

# Specific team quota
kubefabric quota --team ml-research

# With budget details
kubefabric quota --team ml-research --budget
```

#### Quota Details

```bash
# Get full quota specification
kubefabric get quota team-ml

# YAML format
kubefabric get quota team-ml --output yaml

# JSON format
kubefabric get quota team-ml --output json
```

### Cost Analysis

#### View Costs

```bash
# All teams monthly costs
kubefabric cost

# Specific team
kubefabric cost --team ml-research

# Different time periods
kubefabric cost --period day
kubefabric cost --period week
kubefabric cost --period month

# Detailed breakdown
kubefabric cost --team ml-research --detailed
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
kubefabric cluster

# Detailed view with GPU metrics
kubefabric cluster --detailed

# Watch mode (refresh every 5 seconds)
kubefabric cluster --watch 5

# Refresh every 30 seconds
kubefabric cluster --watch 30
```

#### GPU Nodes

```bash
# List all GPU nodes
kubefabric list nodes

# Get node details
kubefabric get node gpu-worker-01

# JSON output
kubefabric get node gpu-worker-01 --output json
```

### Health Checks

```bash
# Check all components
kubefabric health

# Check specific components
kubefabric health gpu
kubefabric health storage
kubefabric health network
```

### Resource Management

#### Get Resources

```bash
# Get job details
kubefabric get job my-training-job

# Get quota details
kubefabric get quota team-ml

# Get node details
kubefabric get node gpu-worker-01

# Different output formats
kubefabric get job my-job --output yaml
kubefabric get job my-job --output json
```

#### Delete Resources

```bash
# Delete job
kubefabric delete job old-experiment

# Delete quota
kubefabric delete quota team-dev

# Skip confirmation
kubefabric delete job my-job --yes
```

#### Validate YAML

```bash
# Validate job YAML before submission
kubefabric validate -f job.yaml
```

## Advanced Usage

### Using Different Contexts

```bash
# Use specific Kubernetes context
kubefabric --context production list jobs

# Use staging context
kubefabric --context staging cluster
```

### Custom Namespaces

```bash
# Use specific namespace
kubefabric -n ml-production list jobs

# Override default namespace
kubefabric -n experiments submit -f job.yaml
```

### Verbose Logging

```bash
# Enable debug logging
kubefabric --verbose submit -f job.yaml

# Verbose output for troubleshooting
kubefabric -v list jobs
```

### Output Formats

The CLI supports three output formats:

```bash
# Pretty table (default)
kubefabric list jobs

# JSON (machine-readable)
kubefabric list jobs --output json | jq '.items[].status.phase'

# YAML
kubefabric list jobs --output yaml
```

## Examples

### Complete Training Workflow

```bash
#!/bin/bash
set -e

# Check quota
echo "Checking quota..."
kubefabric quota --team ml-research --budget

# Submit job
echo "Submitting job..."
kubefabric submit -f llm-training.yaml

# Wait a bit for job to start
sleep 10

# Monitor status
echo "Monitoring job..."
kubefabric status llm-training

# Follow logs
kubefabric logs llm-training --follow
```

### Monitor Multiple Jobs

```bash
#!/bin/bash

JOBS=("job1" "job2" "job3")

for job in "${JOBS[@]}"; do
    echo "=== $job ==="
    kubefabric status $job
    echo ""
done
```

### Cost Report Script

```bash
#!/bin/bash

# Generate monthly cost report
echo "KubeFabric Monthly Cost Report"
echo "=============================="
echo ""

kubefabric cost --period month

echo ""
echo "Per-Team Breakdown:"
echo "-------------------"

for team in ml-research computer-vision nlp; do
    echo ""
    echo "Team: $team"
    kubefabric cost --team $team
done
```

### Cluster Health Dashboard

```bash
#!/bin/bash

while true; do
    clear
    echo "KubeFabric Cluster Dashboard"
    echo "============================"
    echo ""

    kubefabric cluster

    echo ""
    kubefabric health

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
kubefabric --context my-cluster list jobs
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
kubectl get crds | grep kubefabric

# Expected output:
# fabricaijobs.kubefabric.ai
# fabricgpunodes.kubefabric.ai
# fabricnetworks.kubefabric.ai
# fabricquotas.kubefabric.ai
# fabricstorages.kubefabric.ai
```

### Debug Mode

```bash
# Enable verbose logging
kubefabric --verbose list jobs

# Check CLI version
kubefabric --version

# View help for specific command
kubefabric submit --help
```

## Tips and Best Practices

### 1. Use Watch Mode for Monitoring

```bash
# Watch cluster status
kubefabric cluster --watch 5

# Monitor job in terminal
watch -n 5 "kubefabric status my-job"
```

### 2. Combine with jq for Filtering

```bash
# Get all running jobs
kubefabric list jobs --output json | \
  jq -r '.items[] | select(.status.phase=="Running") | .metadata.name'

# Count jobs by status
kubefabric list jobs --output json | \
  jq -r '.items | group_by(.status.phase) | map({status: .[0].status.phase, count: length})'
```

### 3. Create Aliases

```bash
# Add to ~/.bashrc or ~/.zshrc
alias kf="kubefabric"
alias kfj="kubefabric list jobs"
alias kfq="kubefabric quota"
alias kfc="kubefabric cluster"

# Usage
kfj                    # List jobs
kfq --team my-team     # Check quota
kfc --watch 5          # Watch cluster
```

### 4. Use Tab Completion

```bash
# Generate completion script (future feature)
kubefabric completion bash > /etc/bash_completion.d/kubefabric
kubefabric completion zsh > ~/.zsh/completion/_kubefabric
```

### 5. Validate Before Submitting

```bash
# Always validate YAML
kubefabric validate -f job.yaml

# Then submit if valid
kubefabric submit -f job.yaml
```

## Integration with CI/CD

### GitLab CI Example

```yaml
submit_training:
  stage: train
  script:
    - kubefabric submit -f jobs/llm-training.yaml --wait
    - kubefabric logs llm-training
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
      - name: Install KubeFabric CLI
        run: |
          curl -LO https://github.com/ssahani/kube-fabric/releases/latest/download/kubefabric
          chmod +x kubefabric
      - name: Submit Job
        run: ./kubefabric submit -f job.yaml --wait
```

## Future Features

Coming soon:
- [ ] Interactive job creation wizard (`kubefabric create job`)
- [ ] Job templates library
- [ ] Shell completion (bash/zsh/fish)
- [ ] Job scheduling and queuing
- [ ] Multi-cluster support
- [ ] Real-time log streaming
- [ ] Job history and analytics

## Support

- **Documentation**: https://kube-fabric.readthedocs.io
- **Issues**: https://github.com/ssahani/kube-fabric/issues
- **Discussions**: https://github.com/ssahani/kube-fabric/discussions

## Contributing

See [CONTRIBUTING.md](../CONTRIBUTING.md) for development guidelines.
