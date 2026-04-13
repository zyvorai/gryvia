# KubeFabric CLI

Command-line interface for managing KubeFabric GPU clusters.

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
sudo cp target/release/kubefabric /usr/local/bin/
```

### Using Cargo

```bash
cargo install --path .
```

## Usage

### Submit a Training Job

```bash
# Submit job from YAML file
kubefabric submit -f job.yaml

# Submit and wait for completion
kubefabric submit -f job.yaml --wait

# Submit and follow logs
kubefabric submit -f job.yaml --logs
```

### List Resources

```bash
# List all jobs
kubefabric list jobs

# List all quotas
kubefabric list quotas

# List GPU nodes
kubefabric list nodes

# Output as JSON
kubefabric list jobs --output json
```

### Get Detailed Information

```bash
# Get job details
kubefabric get job my-training-job

# Get quota details
kubefabric get quota team-ml

# Get node details
kubefabric get node gpu-worker-01
```

### Monitor Job Status

```bash
# View job status
kubefabric status my-training-job

# View and follow logs
kubefabric logs my-training-job --follow

# View specific replica logs
kubefabric logs my-training-job --replica 0
```

The `logs` and `submit --logs` commands auto-detect the container name from the
pod spec (using the first container). This works with any job container name,
not just the default "trainer".

### Cancel Jobs

Cancelling a job patches its status to "Cancelled" rather than deleting the resource, preserving the job record for auditing.

```bash
# Cancel a single job
kubefabric cancel my-job

# Cancel multiple jobs
kubefabric cancel job1 job2 job3

# Skip confirmation
kubefabric cancel my-job --yes
```

### Cluster Overview

```bash
# View cluster status
kubefabric cluster

# Detailed view
kubefabric cluster --detailed

# Watch mode (refresh every 5 seconds, interval must be > 0)
kubefabric cluster --watch 5
```

### View Quotas

```bash
# List all team quotas
kubefabric quota

# View specific team quota
kubefabric quota --team ml-research

# Show budget details
kubefabric quota --team ml-research --budget
```

### Cost Analysis

The `--period` parameter is validated and only accepts `day`, `week`, or `month`.

```bash
# View monthly costs for all teams
kubefabric cost

# View costs for specific team
kubefabric cost --team ml-research

# Detailed breakdown
kubefabric cost --team ml-research --detailed
```

### Health Checks

```bash
# Check all components
kubefabric health

# Check specific component
kubefabric health gpu
kubefabric health storage
kubefabric health network
```

### Delete Resources

```bash
# Delete a job
kubefabric delete job my-training-job

# Delete a quota
kubefabric delete quota team-dev

# Delete storage or network resources (stub - not yet implemented)
kubefabric delete storage my-storage
kubefabric delete network my-network

# Skip confirmation
kubefabric delete job my-job --yes
```

### Validate YAML

Validates the YAML file structure and checks that `apiVersion` and `kind` match known KubeFabric types (e.g., `kubefabric.ai/v1` / `FabricAIJob`).

```bash
kubefabric validate -f job.yaml
```

## Global Options

```bash
# Use specific Kubernetes context
kubefabric --context production list jobs

# Use specific namespace
kubefabric -n ml-training list jobs

# Enable verbose logging
kubefabric --verbose submit -f job.yaml
```

## Examples

### Complete Workflow

```bash
# 1. Check cluster status
kubefabric cluster

# 2. View available quota
kubefabric quota --team ml-research --budget

# 3. Submit training job
kubefabric submit -f llm-training.yaml --wait

# 4. Monitor job status
kubefabric status llm-training

# 5. View logs
kubefabric logs llm-training --follow

# 6. Check cost impact
kubefabric cost --team ml-research
```

### Team Manager Workflow

```bash
# View team quota status
kubefabric quota --team computer-vision --budget

# List all running jobs
kubefabric list jobs | grep Running

# Check monthly spending
kubefabric cost --team computer-vision --period month

# Check if approaching budget
kubefabric quota --team computer-vision
```

## Output Formats

The CLI supports multiple output formats. The `--output` flag is validated and only accepts `table`, `json`, or `yaml`; any other value produces an error.

- **table** (default): Pretty-printed tables with colors
- **json**: Machine-readable JSON output
- **yaml**: YAML format

```bash
kubefabric list jobs --output table
kubefabric list jobs --output json
kubefabric list jobs --output yaml
```

## Configuration

The CLI uses your Kubernetes configuration (`~/.kube/config`) by default.

### Use Different Context

```bash
export KUBECONFIG=/path/to/kubeconfig
kubefabric --context staging list jobs
```

### Set Default Namespace

```bash
kubefabric -n production list jobs
```

## Building

### Development Build

```bash
cargo build
./target/debug/kubefabric --help
```

### Release Build (Optimized)

```bash
cargo build --release
./target/release/kubefabric --help
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
kubefabric --context my-cluster list jobs
```

### Permission Errors

Ensure your Kubernetes user has permissions to access KubeFabric CRDs:

```bash
kubectl auth can-i list fabricaijobs
kubectl auth can-i get fabricquotas
```

### CRD Not Found

Ensure KubeFabric CRDs are installed:

```bash
kubectl get crds | grep kubefabric
```

## Contributing

See the main [KubeFabric repository](https://github.com/ssahani/kube-fabric) for contribution guidelines.

## License

Apache 2.0
