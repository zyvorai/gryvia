# KubeFabric Monitoring & Observability

Comprehensive monitoring setup for KubeFabric with Prometheus metrics, alerts, and Grafana dashboards.

## Overview

This monitoring stack provides complete visibility into:
- **GPU Health**: Temperature, utilization, memory, power
- **Job Status**: Running, pending, completed, failed jobs
- **Quota Tracking**: Team GPU allocations and job limits
- **Budget Monitoring**: Spending, projections, alerts
- **Storage Performance**: Latency, throughput, health
- **Network Status**: RDMA devices, SR-IOV VFs, errors
- **Operator Health**: Reconciliation performance, errors

## Components

### Prometheus Metrics

**Exposed by operators via `/metrics` endpoint:**

#### GPU Metrics
```
kubefabric_gpu_count                      # Total GPUs per node
kubefabric_gpu_utilization_percent        # GPU utilization (0-100)
kubefabric_gpu_temperature_celsius        # GPU temperature
kubefabric_gpu_memory_used_bytes          # GPU memory used
kubefabric_gpu_memory_total_bytes         # GPU memory total
kubefabric_gpu_power_watts                # GPU power draw
kubefabric_gpu_health_status              # 1=healthy, 0=unhealthy
```

#### Job Metrics
```
kubefabric_job_running                    # Running jobs count
kubefabric_job_pending                    # Pending jobs count
kubefabric_job_queued                     # Queued jobs count
kubefabric_job_completed_total            # Total completed jobs (counter)
kubefabric_job_failed_total               # Total failed jobs (counter)
kubefabric_job_duration_seconds           # Job duration histogram
kubefabric_job_pending_duration_seconds   # Time in pending state
```

#### Quota Metrics
```
kubefabric_quota_gpus_allocated           # GPUs allocated to team
kubefabric_quota_gpus_max                 # Max GPUs for team
kubefabric_quota_running_jobs             # Running jobs for team
kubefabric_quota_queued_jobs              # Queued jobs for team
kubefabric_quota_gpu_hours                # GPU hours consumed
```

#### Budget Metrics
```
kubefabric_budget_spent_month             # Spent this month ($)
kubefabric_budget_monthly_limit           # Monthly budget limit ($)
kubefabric_budget_remaining               # Remaining budget ($)
kubefabric_budget_percent_used            # Budget % used
kubefabric_budget_projected_spend         # Projected month-end spend ($)
kubefabric_budget_alert_threshold         # Alert threshold %
```

#### Storage Metrics
```
kubefabric_storage_health_status          # 1=healthy, 0=unhealthy
kubefabric_storage_latency_ms             # Storage latency
kubefabric_storage_throughput_mbps        # Storage throughput
```

#### Network Metrics
```
kubefabric_rdma_device_status             # 1=up, 0=down
kubefabric_network_errors_total           # Network errors (counter)
kubefabric_sriov_vf_available             # Available SR-IOV VFs
```

#### Operator Metrics
```
kubefabric_operator_errors_total          # Operator errors (counter)
kubefabric_operator_reconcile_duration_seconds  # Reconciliation time
```

### Alert Rules

**40+ Prometheus alerts organized by category:**

1. **GPU Alerts** (7 rules)
   - High temperature (>85°C warning, >90°C critical)
   - Low utilization (<20% for 30m)
   - High memory usage (>95%)
   - GPU unhealthy
   - Node down

2. **Job Alerts** (4 rules)
   - Job failed
   - Job stuck pending (>30min)
   - High queue depth (>10 jobs)
   - Low completion rate

3. **Quota Alerts** (6 rules)
   - Quota almost exceeded (>90%)
   - Quota exceeded
   - Budget alert threshold reached
   - Budget exceeded
   - Projected budget overrun

4. **Storage Alerts** (3 rules)
   - Backend unhealthy
   - High latency (>100ms)
   - Low throughput (<1GB/s)

5. **Network Alerts** (3 rules)
   - RDMA device down
   - High error rate
   - SR-IOV VFs exhausted

6. **Operator Alerts** (3 rules)
   - Operator down
   - High error rate
   - Slow reconciliation

### Grafana Dashboards

**4 comprehensive dashboards:**

#### 1. Cluster Overview (`kubefabric-overview.json`)
- GPU cluster summary stats
- GPU utilization by node
- GPU temperature trends
- GPU memory usage
- GPU power consumption
- Jobs by status (pie chart)
- Job completion rate
- Average job duration

#### 2. Team Quotas & Budgets (`kubefabric-quotas.json`)
- GPU quota usage by team (bar gauge)
- Budget usage by team (bar gauge)
- GPU allocation details (table)
- Budget status (table)
- GPU hours by team
- Budget burn rate
- Running jobs by team
- Queue depth by team

#### 3. GPU Metrics (`kubefabric-gpus.json`)
- GPU health status
- Average GPU utilization gauge
- Peak GPU temperature gauge
- Total GPU power draw
- GPU utilization heatmap
- Per-GPU detailed metrics (table)
- Temperature distribution
- Power consumption trends

#### 4. Cost Analysis (`kubefabric-costs.json`)
- Total monthly spending
- Budget remaining
- Budget utilization rate
- Projected overspend
- Team spending breakdown (pie chart)
- Budget vs actual (bar chart)
- Daily spending trend
- GPU cost per hour by team
- Cost per GPU type (table)
- Monthly cost projection
- GPU hours consumed
- Cost efficiency ($/GPU hour)
- Savings vs cloud comparison

## Installation

### 1. Install Prometheus Operator

```bash
kubectl create namespace monitoring

helm install prometheus-operator prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --set prometheus.prometheusSpec.serviceMonitorSelectorNilUsesHelmValues=false
```

### 2. Apply KubeFabric Monitoring Configuration

```bash
# Alert rules
kubectl apply -f monitoring/prometheus-rules.yaml

# ServiceMonitors for metric scraping
kubectl apply -f monitoring/servicemonitor.yaml
```

### 3. Import Grafana Dashboards

```bash
# Via Grafana UI: Import each dashboard JSON
# Or via ConfigMap:

kubectl create configmap kubefabric-dashboards \
  --from-file=monitoring/grafana-dashboards/ \
  -n monitoring

# Label for auto-discovery
kubectl label configmap kubefabric-dashboards \
  grafana_dashboard=1 \
  -n monitoring
```

### 4. Verify Metrics

```bash
# Port-forward Prometheus
kubectl port-forward -n monitoring svc/prometheus-operated 9090:9090

# Open http://localhost:9090
# Query: kubefabric_gpu_count

# Port-forward Grafana
kubectl port-forward -n monitoring svc/prometheus-grafana 3000:80

# Open http://localhost:3000
# Default credentials: admin/prom-operator
```

## Usage

### View Dashboards

1. **Cluster Overview**: Real-time GPU and job metrics
2. **Quotas**: Team resource allocation and budgets
3. **GPU Metrics**: Detailed per-GPU monitoring
4. **Costs**: Budget tracking and spending analysis

### Check Alerts

```bash
# Via Prometheus UI
http://localhost:9090/alerts

# Via CLI
kubectl get prometheusrules -n monitoring
```

### Query Metrics

```promql
# Average GPU utilization
avg(kubefabric_gpu_utilization_percent)

# Team GPU allocation percentage
(kubefabric_quota_gpus_allocated / kubefabric_quota_gpus_max) * 100

# Budget utilization rate
kubefabric_budget_percent_used

# Job completion rate
rate(kubefabric_job_completed_total[5m])
```

## Alert Routing

### Slack Integration

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: alertmanager-slack
  namespace: monitoring
stringData:
  slack_api_url: https://hooks.slack.com/services/YOUR/SLACK/WEBHOOK
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: alertmanager-config
  namespace: monitoring
data:
  alertmanager.yml: |
    route:
      group_by: ['alertname', 'cluster']
      receiver: 'slack-notifications'
      routes:
      - match:
          severity: critical
        receiver: 'slack-critical'

    receivers:
    - name: 'slack-notifications'
      slack_configs:
      - api_url_file: /etc/alertmanager/secrets/slack_api_url
        channel: '#kubefabric-alerts'
        title: 'KubeFabric Alert'
        text: '{{ .CommonAnnotations.summary }}'

    - name: 'slack-critical'
      slack_configs:
      - api_url_file: /etc/alertmanager/secrets/slack_api_url
        channel: '#kubefabric-critical'
        title: 'CRITICAL: KubeFabric Alert'
        text: '{{ .CommonAnnotations.summary }}'
```

### Email Integration

```yaml
receivers:
- name: 'email'
  email_configs:
  - to: 'team@example.com'
    from: 'kubefabric@example.com'
    smarthost: 'smtp.example.com:587'
    auth_username: 'kubefabric'
    auth_password: 'password'
```

## Retention

### Prometheus Retention

```yaml
# In Prometheus Operator values
prometheus:
  prometheusSpec:
    retention: 30d
    retentionSize: "50GB"
    storageSpec:
      volumeClaimTemplate:
        spec:
          resources:
            requests:
              storage: 100Gi
```

### Long-term Storage (Thanos)

```bash
# Optional: Deploy Thanos for long-term metrics storage
helm install thanos bitnami/thanos \
  --set query.enabled=true \
  --set bucketweb.enabled=true \
  --set compactor.enabled=true \
  --set storegateway.enabled=true
```

## Troubleshooting

### Metrics Not Appearing

```bash
# Check ServiceMonitor
kubectl get servicemonitor -n monitoring

# Check Prometheus targets
http://localhost:9090/targets

# Check operator metrics endpoint
kubectl port-forward -n kubefabric-system svc/gpu-operator-metrics 8080:8080
curl http://localhost:8080/metrics
```

### Alerts Not Firing

```bash
# Check PrometheusRule
kubectl get prometheusrule -n monitoring

# View Prometheus rules
http://localhost:9090/rules

# Check AlertManager
kubectl port-forward -n monitoring svc/alertmanager-operated 9093:9093
http://localhost:9093
```

### Dashboard Not Loading

```bash
# Check ConfigMap
kubectl get configmap kubefabric-dashboards -n monitoring

# Verify Grafana can read it
kubectl logs -n monitoring -l app.kubernetes.io/name=grafana | grep dashboard
```

## Performance Tuning

### High Cardinality

```yaml
# Limit label cardinality in metrics
kubefabric_gpu_utilization_percent{node="worker-01", gpu_id="0"}
# vs
kubefabric_gpu_utilization_percent{node="worker-01", gpu_id="0", uuid="GPU-xyz..."} # Too many labels
```

### Scrape Intervals

```yaml
# Adjust based on needs
- interval: 15s  # GPU metrics (fast changing)
- interval: 30s  # Operator metrics (moderate)
- interval: 60s  # Budget metrics (slow changing)
```

## Best Practices

1. **Set Up Alerts Early**: Configure Slack/email before production
2. **Dashboard Rotation**: Cycle through dashboards on displays
3. **Regular Review**: Check alerts weekly, tune thresholds
4. **Backup Dashboards**: Export JSON periodically
5. **Document Runbooks**: Link alerts to troubleshooting docs
6. **Test Alerts**: Trigger test alerts to verify routing

## Custom Metrics

To add custom metrics to operators, see:
- `operators/*/pkg/metrics/` (to be implemented)
- Prometheus client library documentation
- Controller-runtime metrics integration

## References

- [Prometheus Documentation](https://prometheus.io/docs/)
- [Grafana Dashboards](https://grafana.com/docs/grafana/latest/dashboards/)
- [DCGM Exporter](https://github.com/NVIDIA/dcgm-exporter)
- [Prometheus Operator](https://github.com/prometheus-operator/prometheus-operator)
