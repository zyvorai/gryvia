# KubeFabric Tools

Collection of diagnostic and utility tools for KubeFabric platform.

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
                         KUBEFABRIC COST REPORT
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

### 3. Cluster Health Check

Coming soon: Comprehensive cluster health monitoring.

### 4. Resource Optimizer

Coming soon: Recommendations for resource allocation optimization.

### 5. Job Migration Tool

Coming soon: Migrate jobs between clusters.

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
docker build -t kubefabric-tools -f tools/Dockerfile tools/

# Run cost calculator
docker run -v ~/.kube:/root/.kube kubefabric-tools \
  python3 cost-calculator.py

# Run GPU diagnostics (on GPU node)
docker run --gpus all kubefabric-tools \
  bash gpu-diagnostics.sh
```

## Kubernetes Integration

### Deploy as CronJob

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: daily-cost-report
  namespace: kubefabric
spec:
  schedule: "0 9 * * *"  # Daily at 9 AM
  jobTemplate:
    spec:
      template:
        spec:
          serviceAccountName: kubefabric-tools
          containers:
          - name: cost-calculator
            image: kubefabric-tools:latest
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
  namespace: kubefabric
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
        kubefabric.io/gpu: "true"
      hostPID: true
      containers:
      - name: diagnostics
        image: kubefabric-tools:latest
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
  -n kubefabric
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

- Issues: https://github.com/ssahani/kube-fabric/issues
- Documentation: https://github.com/ssahani/kube-fabric/docs
- Discussions: https://github.com/ssahani/kube-fabric/discussions
