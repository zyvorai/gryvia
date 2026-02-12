# Operational Playbooks

Standard operating procedures and runbooks for KubeFabric operations.

## Table of Contents

1. [Emergency Response](#emergency-response)
2. [Performance Issues](#performance-issues)
3. [Cost Management](#cost-management)
4. [Capacity Planning](#capacity-planning)
5. [Maintenance Procedures](#maintenance-procedures)
6. [Incident Response](#incident-response)

---

## Emergency Response

### GPU Node Failure

**Symptoms:**
- Jobs failing with GPU errors
- Node marked as NotReady
- DCGM reporting hardware failures

**Response Steps:**

```bash
# 1. Identify failed node
kfctl health status cluster-gpu-health

# 2. Cordon node immediately
kubectl cordon gpu-node-05

# 3. List affected jobs
kubectl get fabricaijobs -o wide | grep gpu-node-05

# 4. Migrate running jobs
for job in $(kubectl get fabricaijobs -o name | grep running); do
  kfctl job migrate $job --target-node gpu-node-06
done

# 5. Drain node gracefully
kubectl drain gpu-node-05 --ignore-daemonsets --delete-emptydir-data

# 6. Run diagnostics
kfctl health diagnose --node gpu-node-05

# 7. Create incident ticket
kfctl incident create \
  --title "GPU node failure: gpu-node-05" \
  --severity high \
  --assign sre-team

# 8. Schedule maintenance
# If hardware replacement needed:
kfctl maintenance schedule gpu-node-05 \
  --action "Replace failed GPU" \
  --window "2024-01-22 02:00-06:00"
```

**Recovery:**

```bash
# After hardware replacement:
# 1. Uncordon node
kubectl uncordon gpu-node-05

# 2. Verify health
kfctl health check node gpu-node-05

# 3. Run test job
kfctl job test --node gpu-node-05 --gpu-count 8

# 4. Close incident
kfctl incident resolve <incident-id>
```

---

### Cluster at Capacity

**Symptoms:**
- Many jobs stuck in Pending
- Long queue times
- Budget still available

**Response Steps:**

```bash
# 1. Check current capacity
kfctl cluster status

# 2. View queue
kfctl queue status default

# 3. Identify bottleneck
kfctl capacity analyze

# 4. Options:

# Option A: Scale up (if auto-scaling enabled)
kfctl autoscale trigger --gpu-type A100-80G --count 16

# Option B: Optimize existing jobs
kfctl profile analyze-queue
# Shows jobs that can be right-sized or use different GPU types

# Option C: Enable GPU sharing for dev jobs
kfctl gpu-sharing enable --gpu-type T4 --max-pods 4

# Option D: Migrate low-priority jobs to spot
kfctl job migrate-to-spot --priority low --count 10

# 5. Communicate to users
kfctl announcement create \
  --title "Cluster at capacity" \
  --message "Long queue times expected. Consider using spot instances or T4 GPUs."
```

---

### Out of Budget

**Symptoms:**
- Jobs blocked with BudgetExceeded
- Alerts about budget consumption

**Response Steps:**

```bash
# 1. Check budget status
kfctl budget status --team ml-research

# 2. Analyze spending
kfctl cost analyze --team ml-research --breakdown

# 3. Identify cost drivers
kfctl cost top-jobs --team ml-research --top 10

# 4. Options:

# Option A: Request budget increase
kfctl budget request-increase \
  --team ml-research \
  --amount 10000 \
  --justification "Critical deadline"

# Option B: Optimize spending
# Cancel low-priority jobs
kfctl job cancel --priority low --team ml-research

# Enable spot instances
kfctl job migrate-to-spot --team ml-research

# Use smaller GPUs
kfctl recommend right-size --team ml-research

# 5. Set up alerts
kfctl budget alert create \
  --team ml-research \
  --threshold 80 \
  --notify team-lead@company.com
```

---

## Performance Issues

### Slow Training Job

**Diagnosis:**

```bash
# 1. Profile the job
kfctl profile training-job-42

# 2. Check GPU utilization
kfctl metrics gpu-utilization training-job-42

# 3. Check data loading
kfctl profile data-loading training-job-42

# 4. Check network
kfctl network metrics training-job-42

# 5. Get recommendations
kfctl optimize training-job-42
```

**Common Fixes:**

```yaml
# Fix 1: Enable mixed precision
spec:
  env:
    - name: PYTORCH_ENABLE_AMP
      value: "1"

# Fix 2: Increase data loader workers
spec:
  env:
    - name: NUM_WORKERS
      value: "16"  # Up from 4

# Fix 3: Optimize batch size
spec:
  env:
    - name: BATCH_SIZE
      value: "192"  # Up from 128

# Fix 4: Enable torch.compile
spec:
  command:
    - python
    - train.py
    - --compile=True
```

---

### Poor Multi-Node Scaling

**Diagnosis:**

```bash
# 1. Check network configuration
kfctl network nccl-check training-job-42

# 2. Test network bandwidth
kfctl network test bandwidth \
  --nodes gpu-node-01,gpu-node-02 \
  --gpus-per-node 8

# 3. Check topology
kfctl topology analyze training-job-42

# 4. View NCCL logs
kubectl logs training-job-42 | grep NCCL
```

**Common Fixes:**

```yaml
# Fix 1: Enable GPUDirect RDMA
spec:
  network:
    gpuDirectRequired: true

# Fix 2: Optimize NCCL settings
spec:
  env:
    - name: NCCL_SOCKET_IFNAME
      value: "ib0"
    - name: NCCL_NET_GDR_LEVEL
      value: "5"
    - name: NCCL_MIN_NRINGS
      value: "8"

# Fix 3: Use topology-aware placement
spec:
  placement:
    topologyAware: true
    preferNVLink: true
```

---

## Cost Management

### Monthly Cost Review

**Process:**

```bash
# 1. Generate monthly report
kfctl cost report --month 2024-01 --output report.pdf

# 2. Analyze by team
kfctl cost breakdown --group-by team

# 3. Identify waste
kfctl cost waste --last 30d

# 4. Get optimization recommendations
kfctl cost optimize --potential-savings

# 5. Compare to budget
kfctl budget compare --month 2024-01

# 6. Project next month
kfctl cost forecast --next-month
```

**Optimization Actions:**

```bash
# Enable spot instances for batch jobs
kfctl job migrate-to-spot --job-type batch --dry-run
kfctl job migrate-to-spot --job-type batch --confirm

# Enable MIG for development
kfctl gpu-sharing enable --gpu-type A100-80G --profile all-1g.10gb

# Right-size over-provisioned jobs
kfctl recommend right-size --execute

# Set up budget alerts
for team in ml-research cv-team nlp-team; do
  kfctl budget alert create \
    --team $team \
    --threshold 75,90,100 \
    --action warn,warn,block
done
```

---

## Capacity Planning

### Quarterly Capacity Review

**Process:**

```bash
# 1. Analyze current utilization
kfctl capacity analyze --last 90d

# 2. Forecast demand
kfctl capacity forecast --next-quarter

# 3. Identify gaps
kfctl capacity gaps --horizon 90d

# 4. Generate expansion plan
kfctl capacity plan --output expansion-plan.json

# 5. Estimate costs
kfctl capacity cost-estimate expansion-plan.json

# 6. Create presentation
kfctl capacity presentation --output capacity-review-q2.pdf
```

**Expansion Procedure:**

```bash
# 1. Request budget approval
kfctl budget request-capex \
  --amount 500000 \
  --justification "Q2 capacity expansion" \
  --attach expansion-plan.json

# 2. Order hardware
# (External procurement process)

# 3. Schedule installation
kfctl maintenance schedule-installation \
  --nodes 16 \
  --gpu-type A100-80G \
  --date 2024-04-01

# 4. Pre-configure
kfctl node preconfigure \
  --count 16 \
  --gpu-type A100-80G \
  --network infiniband-hdr200

# 5. Add to cluster
for node in gpu-node-{17..32}; do
  kfctl node add $node --validate
done

# 6. Verify
kfctl cluster validate

# 7. Announce
kfctl announcement create \
  --title "Capacity Expansion Complete" \
  --message "16 new A100-80G nodes available"
```

---

## Maintenance Procedures

### Planned Maintenance Window

**Pre-Maintenance:**

```bash
# 1. Announce maintenance (7 days before)
kfctl announcement create \
  --title "Scheduled Maintenance" \
  --message "Maintenance window: Jan 28, 02:00-06:00 UTC" \
  --send-email

# 2. Create maintenance window
kfctl maintenance create \
  --start "2024-01-28T02:00:00Z" \
  --duration 4h \
  --nodes gpu-node-{01..04}

# 3. Send reminders (24h before)
kfctl announcement remind maintenance-001

# 4. Verify no critical jobs scheduled
kfctl jobs list --during-maintenance maintenance-001
```

**During Maintenance:**

```bash
# 1. Enable maintenance mode
kfctl maintenance start maintenance-001

# 2. Cordon nodes
kfctl maintenance cordon maintenance-001

# 3. Drain gracefully
kfctl maintenance drain maintenance-001 --timeout 30m

# 4. Perform maintenance
# - Update firmware
# - Apply patches
# - Hardware upgrades
# - Network configuration

# 5. Validate nodes
for node in gpu-node-{01..04}; do
  kfctl health check node $node
  kfctl health test $node --full
done

# 6. Uncordon nodes
kfctl maintenance uncordon maintenance-001

# 7. End maintenance mode
kfctl maintenance complete maintenance-001
```

**Post-Maintenance:**

```bash
# 1. Verify cluster health
kfctl health status cluster-gpu-health

# 2. Run smoke tests
kfctl test smoke

# 3. Monitor for issues
kfctl monitor --window 2h

# 4. Send completion notice
kfctl announcement create \
  --title "Maintenance Complete" \
  --message "All systems operational"
```

---

### Rolling Updates

**Procedure:**

```bash
# 1. Plan rollout
kfctl upgrade plan --version 1.1.0

# 2. Create rollout
kfctl upgrade create \
  --version 1.1.0 \
  --strategy rolling \
  --max-unavailable 25%

# 3. Start rollout
kfctl upgrade start

# 4. Monitor progress
kfctl upgrade status

# 5. If issues detected
kfctl upgrade pause
# Fix issues
kfctl upgrade resume

# 6. Complete rollout
kfctl upgrade verify
```

---

## Incident Response

### Incident Management Workflow

**Severity Levels:**

- **P0 (Critical)**: Complete cluster outage, data loss
- **P1 (High)**: Major functionality impaired, SLA breach
- **P2 (Medium)**: Partial functionality impaired
- **P3 (Low)**: Minor issues, no immediate impact

**P0 Incident Response:**

```bash
# 1. Declare incident
kfctl incident create \
  --severity P0 \
  --title "Cluster outage" \
  --description "Complete cluster unavailable"

# 2. Notify stakeholders
kfctl incident notify \
  --channels slack,pagerduty,email \
  --recipients on-call,leadership

# 3. Assemble response team
kfctl incident assign \
  --incident-lead alice \
  --tech-lead bob \
  --comms-lead charlie

# 4. Create war room
kfctl incident war-room create

# 5. Update status every 15 minutes
kfctl incident update \
  --status "Investigating root cause" \
  --eta "30 minutes to diagnosis"

# 6. Implement fix
# ... resolution steps ...

# 7. Verify resolution
kfctl health status cluster-gpu-health
kfctl test smoke

# 8. Clear incident
kfctl incident resolve \
  --resolution "Restored from backup" \
  --duration "2h 15m"

# 9. Schedule post-mortem
kfctl incident post-mortem schedule \
  --date "2024-01-23 14:00" \
  --required alice,bob,charlie
```

**Post-Mortem Template:**

```markdown
# Incident Post-Mortem: [Title]

## Summary
- **Date**: 2024-01-21
- **Duration**: 2h 15m
- **Severity**: P0
- **Impact**: 150 jobs affected, $5,000 cost impact

## Timeline
- 10:30 - Initial alert
- 10:35 - Incident declared
- 10:45 - Root cause identified
- 12:00 - Fix implemented
- 12:45 - Verified resolution

## Root Cause
[Detailed analysis]

## Resolution
[What fixed it]

## Action Items
1. [ ] Improve monitoring for X
2. [ ] Add automated recovery for Y
3. [ ] Update runbook for Z

## Lessons Learned
- What went well
- What could be improved
- What we learned
```

---

## Checklists

### New Node Onboarding

```bash
☐ Physical installation complete
☐ Network cables connected (IB + Ethernet)
☐ Power verified (redundant PSUs)
☐ BIOS configured
☐ OS installed and patched
☐ GPU drivers installed
☐ NVIDIA Fabric Manager configured
☐ Kubernetes joined cluster
☐ Node labeled correctly
☐ Health checks passing
☐ Test job successful
☐ Added to monitoring
☐ Added to backup
☐ Documentation updated
```

### Job Troubleshooting

```bash
☐ Check job status: kfctl job status <name>
☐ View logs: kfctl job logs <name>
☐ Check events: kubectl describe fabricaijob <name>
☐ Verify resources available
☐ Check quota/budget
☐ Review node health
☐ Check network connectivity
☐ Verify image exists and accessible
☐ Check for resource conflicts
☐ Review scheduling decisions
```

---

## Emergency Contacts

```yaml
Severity: P0/P1
  Primary: On-call SRE (PagerDuty)
  Escalation: Engineering Manager
  Notify: VP Engineering

Severity: P2
  Primary: SRE Team (Slack #incidents)
  Escalation: On-call SRE

Severity: P3
  Primary: Support Team (Slack #support)
```

---

## Monitoring Alerts

### Critical Alerts

```yaml
ClusterDown:
  severity: P0
  response: "Execute emergency response playbook"

NodeFailure:
  severity: P1
  response: "Cordon, drain, diagnose per GPU node failure playbook"

BudgetExceeded:
  severity: P1
  response: "Execute budget management playbook"

DiskFull:
  severity: P1
  response: "Clean up logs, expand storage"

GPUError:
  severity: P1
  response: "Run diagnostics, potentially replace GPU"
```

---

## Useful Commands Reference

```bash
# Cluster health
kfctl health status cluster-gpu-health
kfctl cluster validate
kfctl node list --unhealthy

# Performance
kfctl profile <job-name>
kfctl metrics gpu-utilization
kfctl network metrics

# Cost
kfctl cost analyze
kfctl cost optimize
kfctl budget status

# Capacity
kfctl capacity analyze
kfctl capacity forecast
kfctl queue status

# Incidents
kfctl incident create
kfctl incident update
kfctl incident resolve

# Maintenance
kfctl maintenance create
kfctl maintenance start
kfctl maintenance complete
```

---

*For additional support, see [INTEGRATIONS.md](INTEGRATIONS.md) and [ADVANCED_FEATURES.md](ADVANCED_FEATURES.md)*
