# TensorReaper Tools

Collection of diagnostic and utility tools for TensorReaper platform.

## Tools

### 1. GPU Diagnostics (gpu-diagnostics.sh)

Comprehensive GPU health and performance checking.

**Usage:**
```bash
# Run on GPU node
./tools/gpu-diagnostics.sh

# Run in Kubernetes pod
kubectl exec -it <gpu-pod> -- /tools/gpu-diagnostics.sh
```

**Checks:**
- NVIDIA driver installation
- GPU health status (temperature, power, utilization)
- CUDA installation
- GPU topology
- NCCL installation
- NVLink status
- GPU memory errors
- Quick performance benchmark

**Output:**
- Console report with color-coded status
- Detailed report saved to `/tmp/gpu_diagnostics_<hostname>.txt`

### 2. Cost Calculator (cost-calculator.py)

Analyzes job history and provides cost projections.

**Usage:**
```bash
# Analyze last 30 days
python3 tools/cost-calculator.py

# Analyze last 7 days
python3 tools/cost-calculator.py --days 7

# Filter by team
python3 tools/cost-calculator.py --team ml-research

# Save to JSON
python3 tools/cost-calculator.py --output report.json

# Specific namespace
python3 tools/cost-calculator.py --namespace production
```

**Reports:**
- Total cost summary
- Monthly projections
- Cost by team
- Cost by GPU type
- Cost by project
- Top 20 most expensive jobs

**Example Output:**
```
================================================================================
                         TENSORREAPER COST REPORT
================================================================================

📊 SUMMARY
--------------------------------------------------------------------------------
Total Cost (last 30 days):    $45,234.50
Total Jobs:                   342
Average Cost per Job:         $132.26

📅 MONTHLY PROJECTION
--------------------------------------------------------------------------------
Current Month (MTD):          $18,234.50
Projected Month Total:        $54,703.50
Days Elapsed / Remaining:     10 / 20

👥 COST BY TEAM
--------------------------------------------------------------------------------
Team                                 Cost       Jobs      Avg/Job
--------------------------------------------------------------------------------
ml-research                   $25,432.10        156     $163.03
computer-vision               $12,345.60         98     $126.00
nlp                           $7,456.80          88     $84.74
```

### 3. Backup and Restore (backup-restore.sh)

Complete backup and restore solution for TensorReaper resources.

**Usage:**
```bash
# Create backup
./tools/backup-restore.sh backup

# Create backup in custom location
./tools/backup-restore.sh backup --dir /backups

# List backups
./tools/backup-restore.sh list

# Verify backup
./tools/backup-restore.sh verify --file /backups/tensorreaper-20240101-120000.tar.gz

# Restore from backup
./tools/backup-restore.sh restore --file /backups/tensorreaper-20240101-120000.tar.gz
```

**What Gets Backed Up:**
- All CRDs
- GPU Nodes configuration
- AI Jobs
- Quotas
- Storage configurations
- Network configurations
- ConfigMaps
- Secrets (encrypted)
- RBAC policies

**Backup Format:**
- Compressed tarball (.tar.gz)
- SHA256 checksum
- Metadata (timestamp, version, etc.)

### 4. Upgrade Tool (upgrade.sh)

Safe TensorReaper version upgrades with automatic backup.

**Usage:**
```bash
# Upgrade to latest version
./tools/upgrade.sh

# Upgrade to specific version
./tools/upgrade.sh --version 1.1.0

# Upgrade specific namespace
./tools/upgrade.sh --version 1.1.0 --namespace production

# Skip backup (not recommended)
./tools/upgrade.sh --skip-backup

# Dry run (show what would be upgraded)
./tools/upgrade.sh --dry-run
```

**Upgrade Process:**
1. Pre-upgrade checks (running jobs, pod health)
2. Automatic backup
3. CRD upgrades
4. Operator upgrades (rolling deployment)
5. Web UI/API Gateway upgrades
6. Post-upgrade verification

**Supports:**
- Helm deployments
- Manual deployments
- Zero-downtime upgrades
- Automatic rollback on failure

### 5. GPU Profiler (profiler.py)

Analyzes GPU utilization and provides optimization recommendations.

**Usage:**
```bash
# Upgrade to latest version
./tools/upgrade.sh

# Upgrade to specific version
./tools/upgrade.sh --version 1.1.0

# Upgrade specific namespace
./tools/upgrade.sh --version 1.1.0 --namespace production

# Skip backup (not recommended)
./tools/upgrade.sh --skip-backup

# Dry run (show what would be upgraded)
./tools/upgrade.sh --dry-run
```

**Upgrade Process:**
1. Pre-upgrade checks (running jobs, pod health)
2. Automatic backup
3. CRD upgrades
4. Operator upgrades (rolling deployment)
5. Web UI/API Gateway upgrades
6. Post-upgrade verification

**Supports:**
- Helm deployments
- Manual deployments
- Zero-downtime upgrades
- Automatic rollback on failure

### 5. GPU Profiler (profiler.py)

Analyzes GPU utilization and provides optimization recommendations.

**Usage:**
```bash
# Profile specific job
python3 tools/profiler.py --job pytorch-training \
  --gpu-type A100-80G \
  --gpu-count 8

# Profile all jobs
python3 tools/profiler.py --all

# Profile jobs in specific namespace
python3 tools/profiler.py --all --namespace ml-research

# Save results to JSON
python3 tools/profiler.py --job my-job --output report.json
```

**Analysis:**
- GPU utilization patterns
- Memory usage efficiency
- Compute vs memory bottlenecks
- Cost optimization opportunities
- Performance recommendations

**Recommendations:**
- Batch size adjustments
- GPU type optimization
- Data loading improvements
- Memory optimization techniques
- Cost-saving opportunities

**Example Output:**
```
╔════════════════════════════════════════════════════════════════╗
║          TensorReaper GPU Profiler                              ║
╚════════════════════════════════════════════════════════════════╝

📊 UTILIZATION SUMMARY
──────────────────────────────────────────────────────────────────
Efficiency Score:       45.5/100
GPU Utilization:        42.3%
GPU Memory Usage:       28.7%
Runtime:                4.25 hours

⚠️  ISSUES DETECTED
──────────────────────────────────────────────────────────────────
[HIGH] Low GPU utilization: 42.3%
[MEDIUM] Low GPU memory usage: 28.7%

💡 RECOMMENDATIONS
──────────────────────────────────────────────────────────────────
[HIGH] Increase Batch Size
  Category: performance
  Low GPU utilization indicates underutilization. Increase batch size.
  Action: Try increasing batch size by 50-100%
  Expected Impact: 30-50% improvement in GPU utilization

[MEDIUM] Use Smaller GPU Type
  Category: cost
  Memory usage is only 28.7%. You may be overpaying for GPU capacity.
  Action: Consider switching to A100-40G instead of A100-80G
  Expected Impact: 40-50% cost reduction

💰 COST ANALYSIS
──────────────────────────────────────────────────────────────────
Current Cost:           $816.00

Potential Savings:
  • Use Smaller GPU Type
    $408.00 (50.0%)
  • Improve GPU utilization to 80%+
    $204.00 (25.0%)

Total Potential Savings: $612.00
```

## Installation

### Local Installation

```bash
# Install dependencies for cost calculator
pip install kubernetes

# Make scripts executable
chmod +x tools/*.sh
chmod +x tools/*.py
```

### Container Usage

```bash
# Build tools container
docker build -t tensorreaper-tools -f tools/Dockerfile tools/

# Run cost calculator
docker run -v ~/.kube:/root/.kube tensorreaper-tools \
  python3 cost-calculator.py

# Run GPU diagnostics (on GPU node)
docker run --gpus all tensorreaper-tools \
  bash gpu-diagnostics.sh
```

## Kubernetes Integration

### Deploy as CronJob

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: daily-cost-report
  namespace: tensorreaper
spec:
  schedule: "0 9 * * *"  # Daily at 9 AM
  jobTemplate:
    spec:
      template:
        spec:
          serviceAccountName: tensorreaper-tools
          containers:
          - name: cost-calculator
            image: tensorreaper-tools:1.0.0
            command:
              - python3
              - /tools/cost-calculator.py
              - --output
              - /reports/cost-report.json
            volumeMounts:
              - name: reports
                mountPath: /reports
          volumes:
            - name: reports
              persistentVolumeClaim:
                claimName: cost-reports-pvc
          restartPolicy: OnFailure
```

### Deploy as DaemonSet (GPU Diagnostics)

```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: gpu-diagnostics
  namespace: tensorreaper
spec:
  selector:
    matchLabels:
      app: gpu-diagnostics
  template:
    metadata:
      labels:
        app: gpu-diagnostics
    spec:
      nodeSelector:
        tensorreaper.ai/gpu: "true"
      hostPID: true
      containers:
      - name: diagnostics
        image: tensorreaper-tools:1.0.0
        command:
          - /bin/bash
          - -c
          - |
            while true; do
              /tools/gpu-diagnostics.sh > /var/log/gpu-diagnostics.log
              sleep 3600
            done
        volumeMounts:
          - name: nvidia
            mountPath: /usr/local/nvidia
          - name: logs
            mountPath: /var/log
        securityContext:
          privileged: true
      volumes:
        - name: nvidia
          hostPath:
            path: /usr/local/nvidia
        - name: logs
          hostPath:
            path: /var/log
```

## Troubleshooting

### Cost Calculator Issues

**Problem**: "No module named 'kubernetes'"
```bash
pip install kubernetes
```

**Problem**: "Unauthorized" error
```bash
# Ensure kubeconfig is accessible
export KUBECONFIG=~/.kube/config

# Or copy into cluster
kubectl create secret generic kubeconfig \
  --from-file=config=$HOME/.kube/config \
  -n tensorreaper
```

### GPU Diagnostics Issues

**Problem**: "nvidia-smi: command not found"
```bash
# Install NVIDIA drivers first
# Or run on a node with GPU drivers installed
```

**Problem**: "Permission denied"
```bash
# Run with appropriate permissions
sudo ./gpu-diagnostics.sh

# Or in Kubernetes with privileged pod
```

## Best Practices

### Cost Reporting

1. **Regular Reports**: Run daily cost reports
2. **Team Attribution**: Ensure all jobs have team labels
3. **Budget Alerts**: Set up alerts when costs exceed thresholds
4. **Trend Analysis**: Track costs over time

### GPU Monitoring

1. **Scheduled Checks**: Run diagnostics hourly
2. **Alert on Issues**: Integrate with monitoring system
3. **Temperature Monitoring**: Alert on high temperatures
4. **Error Tracking**: Monitor ECC errors

### Automation

1. **CronJobs**: Automate regular tasks
2. **Alerting**: Integrate with Prometheus/Alertmanager
3. **Dashboards**: Visualize metrics in Grafana
4. **Reporting**: Send reports via email/Slack

## Contributing

To add new tools:

1. Create tool in `tools/` directory
2. Add documentation to this README
3. Create Dockerfile if needed
4. Add examples
5. Submit pull request

## Support

- Issues: https://github.com/ssahani/TensorReaper/issues
- Documentation: https://github.com/ssahani/TensorReaper/docs
- Discussions: https://github.com/ssahani/TensorReaper/discussions
