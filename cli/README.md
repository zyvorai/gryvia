# Gryvia CLI

Command-line interface for managing Gryvia GPU clusters.

## Features

- **Job Management**: Submit, list, monitor, and cancel AI training jobs
- **Quota Monitoring**: View team quotas and budget status
- **Cluster Overview**: Real-time cluster status and GPU utilization
- **Cost Analysis**: Track spending and budget usage
- **Health Checks**: Monitor GPU, storage, and network health
- **Beautiful Output**: Colored tables and status indicators

## Installation

### From Source

```bash
cd cli
cargo build --release
sudo cp target/release/gryvia /usr/local/bin/
```

### Using Cargo

```bash
cargo install --path .
```

## Usage

### Submit a Training Job

```bash
# Submit job from YAML file
gryvia submit -f job.yaml

# Submit and wait for completion
gryvia submit -f job.yaml --wait

# Submit and follow logs
gryvia submit -f job.yaml --logs
```

### List Resources

```bash
# List all jobs
gryvia list jobs

# List all quotas
gryvia list quotas

# List GPU nodes
gryvia list nodes

# Output as JSON
gryvia list jobs --output json
```

### Get Detailed Information

```bash
# Get job details
gryvia get job my-training-job

# Get quota details
gryvia get quota team-ml

# Get node details
gryvia get node gpu-worker-01
```

### Monitor Job Status

```bash
# View job status
gryvia status my-training-job

# View and follow logs
gryvia logs my-training-job --follow

# View specific replica logs
gryvia logs my-training-job --replica 0
```

The `logs` and `submit --logs` commands auto-detect the container name from the
pod spec (using the first container). This works with any job container name,
not just the default "trainer".

### Cancel Jobs

Cancelling a job patches its status to "Cancelled" rather than deleting the resource, preserving the job record for auditing.

```bash
# Cancel a single job
gryvia cancel my-job

# Cancel multiple jobs
gryvia cancel job1 job2 job3

# Skip confirmation
gryvia cancel my-job --yes
```

### Cluster Overview

```bash
# View cluster status
gryvia cluster

# Detailed view
gryvia cluster --detailed

# Watch mode (refresh every 5 seconds, interval must be > 0)
gryvia cluster --watch 5
```

### View Quotas

```bash
# List all team quotas
gryvia quota

# View specific team quota
gryvia quota ml-research

# Show budget details
gryvia quota ml-research --budget
```

### Cost Analysis

The `--period` parameter is validated and only accepts `day`, `week`, or `month`.

```bash
# View monthly costs for all teams
gryvia cost

# View costs for specific team
gryvia cost ml-research

# Detailed breakdown
gryvia cost ml-research --detailed
```

### Health Checks

```bash
# Check all components
gryvia health

# Check specific component
gryvia health gpu
gryvia health storage
gryvia health network
```

### Delete Resources

```bash
# Delete a job
gryvia delete job my-training-job

# Delete a quota
gryvia delete quota team-dev

# Delete storage or network resources (stub - not yet implemented)
gryvia delete storage my-storage
gryvia delete network my-network

# Skip confirmation
gryvia delete job my-job --yes
```

### Validate YAML

Validates the YAML file structure and checks that `apiVersion` and `kind` match known Gryvia types (e.g., `gryvia.io/v1` / `GryviaAIJob`).

```bash
gryvia validate job.yaml
```

## Global Options

```bash
# Use specific Kubernetes context
gryvia --context production list jobs

# Use specific namespace
gryvia -n ml-training list jobs

# Enable verbose logging
gryvia --verbose submit -f job.yaml
```

## Examples

### Complete Workflow

```bash
# 1. Check cluster status
gryvia cluster

# 2. View available quota
gryvia quota ml-research --budget

# 3. Submit training job
gryvia submit -f llm-training.yaml --wait

# 4. Monitor job status
gryvia status llm-training

# 5. View logs
gryvia logs llm-training --follow

# 6. Check cost impact
gryvia cost ml-research
```

### Team Manager Workflow

```bash
# View team quota status
gryvia quota computer-vision --budget

# List all running jobs
gryvia list jobs | grep Running

# Check monthly spending
gryvia cost computer-vision --period month

# Check if approaching budget
gryvia quota computer-vision
```

## Output Formats

The CLI supports multiple output formats. The `--output` flag is validated and only accepts `table`, `json`, or `yaml`; any other value produces an error.

- **table** (default): Pretty-printed tables with colors
- **json**: Machine-readable JSON output
- **yaml**: YAML format

```bash
gryvia list jobs --output table
gryvia list jobs --output json
gryvia list jobs --output yaml
```

## Configuration

The CLI uses your Kubernetes configuration (`~/.kube/config`) by default.

### Use Different Context

```bash
export KUBECONFIG=/path/to/kubeconfig
gryvia --context staging list jobs
```

### Set Default Namespace

```bash
gryvia -n production list jobs
```

## Building

### Development Build

```bash
cargo build
./target/debug/gryvia --help
```

### Release Build (Optimized)

```bash
cargo build --release
./target/release/gryvia --help
```

### Run Tests

```bash
cargo test
```

## Command Reference

| Command | Description |
|---------|-------------|
| `submit` | Submit a job from YAML file |
| `list` | List resources (jobs, quotas, nodes) |
| `get` | Get detailed resource information |
| `delete` | Delete a resource (supports job, quota, storage, network) |
| `status` | Show job status |
| `logs` | View job logs |
| `cancel` | Cancel running jobs (patches status to "Cancelled") |
| `cluster` | Show cluster overview |
| `quota` | View team quotas |
| `cost` | Show cost analysis |
| `queue` | View job queue status |
| `create` | Interactive resource creation |
| `validate` | Validate YAML files |
| `health` | Check cluster health |

## Technical Notes

- GPU count fields use `u32` (unsigned 32-bit integer) since counts cannot be negative.
- The tokio runtime uses specific feature flags (`rt-multi-thread`, `macros`, `time`, etc.) rather than the `full` feature set, reducing binary size.
- Job names are validated against Kubernetes DNS-1123 subdomain rules: lowercase alphanumeric and hyphens only, max 253 characters, must not start or end with a hyphen.

## Troubleshooting

### Connection Issues

```bash
# Verify Kubernetes connection
kubectl cluster-info

# Check context
kubectl config current-context

# Use specific context
gryvia --context my-cluster list jobs
```

### Permission Errors

Ensure your Kubernetes user has permissions to access Gryvia CRDs:

```bash
kubectl auth can-i list gryviaaijobs
kubectl auth can-i get gryviaquotas
```

### CRD Not Found

Ensure Gryvia CRDs are installed:

```bash
kubectl get crds | grep gryvia
```

## Contributing

See the main [Gryvia repository](https://github.com/zyvorai/gryvia) for contribution guidelines.

## License

Apache 2.0
