# Operational Playbooks

Standard operating procedures and runbooks for Gryvia operations.

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
gryvia health status cluster-gpu-health

# 2. Cordon node immediately
kubectl cordon gpu-node-05

# 3. List affected jobs
kubectl get fabricaijobs -o wide | grep gpu-node-05

# 4. Migrate running jobs
for job in $(kubectl get fabricaijobs -o name | grep running); do
  gryvia job migrate $job --target-node gpu-node-06
done

# 5. Drain node gracefully
kubectl drain gpu-node-05 --ignore-daemonsets --delete-emptydir-data

# 6. Run diagnostics
gryvia health diagnose --node gpu-node-05

# 7. Create incident ticket
gryvia incident create \
  --title "GPU node failure: gpu-node-05" \
  --severity high \
  --assign sre-team

# 8. Schedule maintenance
# If hardware replacement needed:
gryvia maintenance schedule gpu-node-05 \
  --action "Replace failed GPU" \
  --window "2024-01-22 02:00-06:00"
```

**Recovery:**

```bash
# After hardware replacement:
# 1. Uncordon node
kubectl uncordon gpu-node-05

# 2. Verify health
gryvia health check node gpu-node-05

# 3. Run test job
gryvia job test --node gpu-node-05 --gpu-count 8

# 4. Close incident
gryvia incident resolve <incident-id>
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
gryvia cluster status

# 2. View queue
gryvia queue status default

# 3. Identify bottleneck
gryvia capacity analyze

# 4. Options:

# Option A: Scale up (if auto-scaling enabled)
gryvia autoscale trigger --gpu-type A100-80G --count 16

# Option B: Optimize existing jobs
gryvia profile analyze-queue
# Shows jobs that can be right-sized or use different GPU types

# Option C: Enable GPU sharing for dev jobs
gryvia gpu-sharing enable --gpu-type T4 --max-pods 4

# Option D: Migrate low-priority jobs to spot
gryvia job migrate-to-spot --priority low --count 10

# 5. Communicate to users
gryvia announcement create \
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
gryvia budget status --team ml-research

# 2. Analyze spending
gryvia cost analyze --team ml-research --breakdown

# 3. Identify cost drivers
gryvia cost top-jobs --team ml-research --top 10

# 4. Options:

# Option A: Request budget increase
gryvia budget request-increase \
  --team ml-research \
  --amount 10000 \
  --justification "Critical deadline"

# Option B: Optimize spending
# Cancel low-priority jobs
gryvia job cancel --priority low --team ml-research

# Enable spot instances
gryvia job migrate-to-spot --team ml-research

# Use smaller GPUs
gryvia recommend right-size --team ml-research

# 5. Set up alerts
gryvia budget alert create \
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
gryvia profile training-job-42

# 2. Check GPU utilization
gryvia metrics gpu-utilization training-job-42

# 3. Check data loading
gryvia profile data-loading training-job-42

# 4. Check network
gryvia network metrics training-job-42

# 5. Get recommendations
gryvia optimize training-job-42
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
gryvia network nccl-check training-job-42

# 2. Test network bandwidth
gryvia network test bandwidth \
  --nodes gpu-node-01,gpu-node-02 \
  --gpus-per-node 8

# 3. Check topology
gryvia topology analyze training-job-42

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
gryvia cost report --month 2024-01 --output report.pdf

# 2. Analyze by team
gryvia cost breakdown --group-by team

# 3. Identify waste
gryvia cost waste --last 30d

# 4. Get optimization recommendations
gryvia cost optimize --potential-savings

# 5. Compare to budget
gryvia budget compare --month 2024-01

# 6. Project next month
gryvia cost forecast --next-month
```

**Optimization Actions:**

```bash
# Enable spot instances for batch jobs
gryvia job migrate-to-spot --job-type batch --dry-run
gryvia job migrate-to-spot --job-type batch --confirm

# Enable MIG for development
gryvia gpu-sharing enable --gpu-type A100-80G --profile all-1g.10gb

# Right-size over-provisioned jobs
gryvia recommend right-size --execute

# Set up budget alerts
for team in ml-research cv-team nlp-team; do
  gryvia budget alert create \
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
gryvia capacity analyze --last 90d

# 2. Forecast demand
gryvia capacity forecast --next-quarter

# 3. Identify gaps
gryvia capacity gaps --horizon 90d

# 4. Generate expansion plan
gryvia capacity plan --output expansion-plan.json

# 5. Estimate costs
gryvia capacity cost-estimate expansion-plan.json

# 6. Create presentation
gryvia capacity presentation --output capacity-review-q2.pdf
```

**Expansion Procedure:**

```bash
# 1. Request budget approval
gryvia budget request-capex \
  --amount 500000 \
  --justification "Q2 capacity expansion" \
  --attach expansion-plan.json

# 2. Order hardware
# (External procurement process)

# 3. Schedule installation
gryvia maintenance schedule-installation \
  --nodes 16 \
  --gpu-type A100-80G \
  --date 2024-04-01

# 4. Pre-configure
gryvia node preconfigure \
  --count 16 \
  --gpu-type A100-80G \
  --network infiniband-hdr200

# 5. Add to cluster
for node in gpu-node-{17..32}; do
  gryvia node add $node --validate
done

# 6. Verify
gryvia cluster validate

# 7. Announce
gryvia announcement create \
  --title "Capacity Expansion Complete" \
  --message "16 new A100-80G nodes available"
```

---

## Maintenance Procedures

### Planned Maintenance Window

**Pre-Maintenance:**

```bash
# 1. Announce maintenance (7 days before)
gryvia announcement create \
  --title "Scheduled Maintenance" \
  --message "Maintenance window: Jan 28, 02:00-06:00 UTC" \
  --send-email

# 2. Create maintenance window
gryvia maintenance create \
  --start "2024-01-28T02:00:00Z" \
  --duration 4h \
  --nodes gpu-node-{01..04}

# 3. Send reminders (24h before)
gryvia announcement remind maintenance-001

# 4. Verify no critical jobs scheduled
gryvia jobs list --during-maintenance maintenance-001
```

**During Maintenance:**

```bash
# 1. Enable maintenance mode
gryvia maintenance start maintenance-001

# 2. Cordon nodes
gryvia maintenance cordon maintenance-001

# 3. Drain gracefully
gryvia maintenance drain maintenance-001 --timeout 30m

# 4. Perform maintenance
# - Update firmware
# - Apply patches
# - Hardware upgrades
# - Network configuration

# 5. Validate nodes
for node in gpu-node-{01..04}; do
  gryvia health check node $node
  gryvia health test $node --full
done

# 6. Uncordon nodes
gryvia maintenance uncordon maintenance-001

# 7. End maintenance mode
gryvia maintenance complete maintenance-001
```

**Post-Maintenance:**

```bash
# 1. Verify cluster health
gryvia health status cluster-gpu-health

# 2. Run smoke tests
gryvia test smoke

# 3. Monitor for issues
gryvia monitor --window 2h

# 4. Send completion notice
gryvia announcement create \
  --title "Maintenance Complete" \
  --message "All systems operational"
```

---

### Rolling Updates

**Procedure:**

```bash
# 1. Plan rollout
gryvia upgrade plan --version 1.1.0

# 2. Create rollout
gryvia upgrade create \
  --version 1.1.0 \
  --strategy rolling \
  --max-unavailable 25%

# 3. Start rollout
gryvia upgrade start

# 4. Monitor progress
gryvia upgrade status

# 5. If issues detected
gryvia upgrade pause
# Fix issues
gryvia upgrade resume

# 6. Complete rollout
gryvia upgrade verify
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
gryvia incident create \
  --severity P0 \
  --title "Cluster outage" \
  --description "Complete cluster unavailable"

# 2. Notify stakeholders
gryvia incident notify \
  --channels slack,pagerduty,email \
  --recipients on-call,leadership

# 3. Assemble response team
gryvia incident assign \
  --incident-lead alice \
  --tech-lead bob \
  --comms-lead charlie

# 4. Create war room
gryvia incident war-room create

# 5. Update status every 15 minutes
gryvia incident update \
  --status "Investigating root cause" \
  --eta "30 minutes to diagnosis"

# 6. Implement fix
# ... resolution steps ...

# 7. Verify resolution
gryvia health status cluster-gpu-health
gryvia test smoke

# 8. Clear incident
gryvia incident resolve \
  --resolution "Restored from backup" \
  --duration "2h 15m"

# 9. Schedule post-mortem
gryvia incident post-mortem schedule \
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
☐ Check job status: gryvia job status <name>
☐ View logs: gryvia job logs <name>
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
gryvia health status cluster-gpu-health
gryvia cluster validate
gryvia node list --unhealthy

# Performance
gryvia profile <job-name>
gryvia metrics gpu-utilization
gryvia network metrics

# Cost
gryvia cost analyze
gryvia cost optimize
gryvia budget status

# Capacity
gryvia capacity analyze
gryvia capacity forecast
gryvia queue status

# Incidents
gryvia incident create
gryvia incident update
gryvia incident resolve

# Maintenance
gryvia maintenance create
gryvia maintenance start
gryvia maintenance complete
```

---

*For additional support, see [INTEGRATIONS.md](INTEGRATIONS.md) and [ADVANCED_FEATURES.md](ADVANCED_FEATURES.md)*
