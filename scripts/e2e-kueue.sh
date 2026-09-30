#!/usr/bin/env bash
# End-to-end checks of the Kueue integration (docs/kueue-integration.md) on a kind cluster that has Gryvia
# installed WITH Kueue and both operator flags on. Run by .github/workflows/e2e-kueue.yml, one scenario per step:
#
#   scripts/e2e-kueue.sh setup     # tenant q1 -> ClusterQueue/LocalQueue/flavors; fake "slot" resource on the node
#   scripts/e2e-kueue.sh queue     # two jobs that do not fit together: first Running, second Queued, then Succeeded
#   scripts/e2e-kueue.sh gang      # a gang bigger than the quota never starts partially (zero pods), then starts whole
#   scripts/e2e-kueue.sh preempt   # a higher-priority job preempts a running lower-priority one, which is requeued
#
# NOTHING HERE HAS BEEN RUN: the author had no cluster. Assumptions that could not be checked offline are marked
# "ASSUMPTION" and are the first things to look at if a step fails.
#
# Quota resource: kind has no GPUs and its CPU is too small to give a nominal quota that means something, so the
# quota operator runs with --kueue-quota-resources=example.com/slot and the node advertises 100 fake slots (the
# documented "extended resource" node status patch). A job pod asks for one slot and tiny cpu/memory, so the
# scheduler is never the bottleneck: only Kueue's quota is. Tenant q1 has concurrentGPUs: 2 = 2 slots.
set -euo pipefail

API=gryvia.io/v1alpha1
NS=tenant-q1
CQ=gryvia-q1
SLOT=example.com/slot

fail() { echo "E2E FAIL: $*" >&2; exit 1; }

wait_for() { # <seconds> <description> <command...>
  local secs="$1" what="$2"; shift 2
  local end=$((SECONDS + secs))
  until "$@" >/dev/null 2>&1; do
    (( SECONDS < end )) || fail "timed out after ${secs}s waiting for: $what"
    sleep 3
  done
  echo "ok: $what"
}

phase() { kubectl -n "$NS" get gryviaaijob "$1" -o jsonpath='{.status.phase}' 2>/dev/null; }
is_phase() { [[ "$(phase "$1")" == "$2" ]]; }
message() { kubectl -n "$NS" get gryviaaijob "$1" -o jsonpath='{.status.message}' 2>/dev/null; }
pod_count() { kubectl -n "$NS" get pods -l "gryvia.io/job=$1" -o name 2>/dev/null | wc -l | tr -d ' '; }
no_pods() { [[ "$(pod_count "$1")" == "0" ]]; }
job_suspended() { [[ "$(kubectl -n "$NS" get job "$1" -o jsonpath='{.spec.suspend}' 2>/dev/null)" == "true" ]]; }
job_unsuspended() { [[ "$(kubectl -n "$NS" get job "$1" -o jsonpath='{.spec.suspend}' 2>/dev/null)" == "false" ]]; }
gone() { ! kubectl -n "$NS" get "$1" "$2" >/dev/null 2>&1; }
msg_matches() { [[ "$(message "$1")" =~ $2 ]]; }

# The Kueue Workload owned by the batch Job of a GryviaAIJob (Kueue names it job-<name>-<hash>).
wl_field() { # <job> <jq expression on the workload>
  kubectl -n "$NS" get workloads -o json |
    jq -r --arg j "$1" "[.items[] | select(any(.metadata.ownerReferences[]?; .kind==\"Job\" and .name==\$j))][0] | $2"
}
cq_field() { kubectl get clusterqueue "$CQ" -o jsonpath="$1" 2>/dev/null; }

kq_job() { # <name> <nodes> <priority> <sleep-seconds>
  local name="$1" nodes="$2" prio="$3" secs="$4"
  cat <<YAML | kubectl apply -f - >/dev/null
apiVersion: $API
kind: GryviaAIJob
metadata: {name: $name, namespace: $NS}
spec:
  type: training
  image: busybox:1.36
  gpus: 0
  priority: $prio
  retryLimit: 0
  timeout: 20m
  distributed: {enabled: true, framework: pytorch, backend: gloo, nodes: $nodes}
  resources:
    requests: {cpu: 20m, memory: 16Mi, $SLOT: "1"}
    limits: {cpu: 200m, memory: 64Mi, $SLOT: "1"}
  command: ["sh", "-c"]
  args: ["echo rank=\$RANK; sleep $secs"]
YAML
}

cq_idle() { # nothing admitted or pending in the ClusterQueue
  [[ "$(cq_field '{.status.reservingWorkloads}')" =~ ^0?$ && "$(cq_field '{.status.pendingWorkloads}')" =~ ^0?$ ]]
}

scenario_setup() {
  # ASSUMPTION: a single kind node; the extended-resource capacity patch survives kubelet status updates
  # (it is the documented way to advertise a device-plugin-less resource).
  for n in $(kubectl get nodes -o name | sed 's#node/##'); do
    kubectl patch node "$n" --subresource=status --type=json \
      -p "[{\"op\":\"add\",\"path\":\"/status/capacity/example.com~1slot\",\"value\":\"100\"}]" >/dev/null
  done
  wait_for 60 "node advertises 100 $SLOT" bash -c "kubectl get nodes -o jsonpath='{.items[0].status.allocatable.example\\.com/slot}' | grep -qx 100"

  cat <<YAML | kubectl apply -f - >/dev/null
apiVersion: $API
kind: GryviaTenant
metadata: {name: q1}
spec:
  displayName: Kueue e2e
  quotas: {concurrentGPUs: 2}
YAML
  wait_for 120 "namespace $NS exists" kubectl get ns "$NS"
  wait_for 120 "ClusterQueue $CQ exists" kubectl get clusterqueue "$CQ"
  wait_for 120 "LocalQueue $NS/gryvia exists" kubectl -n "$NS" get localqueue gryvia
  wait_for 60 "ResourceFlavor gryvia-default exists" kubectl get resourceflavor gryvia-default
  wait_for 120 "ClusterQueue $CQ is Active" \
    bash -c "[[ \"\$(kubectl get clusterqueue $CQ -o jsonpath='{.status.conditions[?(@.type==\"Active\")].status}')\" == True ]]"

  # The objects are what docs/kueue-integration.md says.
  [[ "$(cq_field '{.spec.cohort}')" == "gryvia" ]] || fail "cohort != gryvia"
  [[ "$(cq_field '{.spec.namespaceSelector.matchLabels.kubernetes\.io/metadata\.name}')" == "$NS" ]] || fail "namespaceSelector"
  [[ "$(cq_field '{.spec.preemption.withinClusterQueue}')" == "LowerPriority" ]] || fail "preemption.withinClusterQueue"
  [[ "$(cq_field '{.spec.resourceGroups[0].flavors[0].resources[0].name}')" == "$SLOT" ]] || fail "quota resource != $SLOT"
  [[ "$(cq_field '{.spec.resourceGroups[0].flavors[0].resources[0].nominalQuota}')" == "2" ]] || fail "nominalQuota != 2"
  [[ "$(kubectl -n "$NS" get localqueue gryvia -o jsonpath='{.spec.clusterQueue}')" == "$CQ" ]] || fail "LocalQueue points elsewhere"
}

scenario_queue() {
  # Two 2-slot jobs against a 2-slot quota. No spec.queueName: the default LocalQueue of the tenant namespace is used.
  kq_job qa 2 0 75
  wait_for 300 "qa reaches Running" is_phase qa Running
  [[ "$(kubectl -n "$NS" get job qa -o jsonpath="{.metadata.labels.kueue\.x-k8s\.io/queue-name}")" == "gryvia" ]] || fail "qa: no default queue label"
  job_unsuspended qa || fail "qa: Kueue did not unsuspend the Job"
  [[ "$(pod_count qa)" == "2" ]] || fail "qa: expected 2 pods, got $(pod_count qa)"

  kq_job qb 2 0 10
  wait_for 120 "qb is Queued" is_phase qb Queued
  # ASSUMPTION: Kueue's pending message is non-empty free text; the operator prefixes it.
  wait_for 60 "qb message comes from Kueue" msg_matches qb '^(Queued by Kueue|Waiting for Kueue)'
  echo "qb message: $(message qb)"
  job_suspended qb || fail "qb: Job must stay suspended while queued"
  no_pods qb || fail "qb: pods exist while queued"
  [[ "$(wl_field qb '.status.conditions[]? | select(.type=="QuotaReserved") | .status')" == "False" ]] || fail "qb: Workload has quota reserved"
  wait_for 90 "ClusterQueue reports 1 pending workload" bash -c "[[ \"\$(kubectl get clusterqueue $CQ -o jsonpath='{.status.pendingWorkloads}')\" == 1 ]]"
  is_phase qa Running || fail "qa left Running while qb waited"

  # When qa finishes its quota is released and qb is admitted (fair, first-come admission).
  wait_for 300 "qa reaches Succeeded" is_phase qa Succeeded
  wait_for 300 "qb is admitted and Running or Succeeded" bash -c "p=\$(kubectl -n $NS get gryviaaijob qb -o jsonpath='{.status.phase}'); [[ \$p == Running || \$p == Succeeded ]]"
  wait_for 300 "qb reaches Succeeded" is_phase qb Succeeded
  kubectl -n "$NS" delete gryviaaijob qa qb --wait=true >/dev/null
  wait_for 120 "ClusterQueue idle" cq_idle
}

scenario_gang() {
  # 5 pods, quota 2: the whole gang must wait. Without Kueue the scheduler would happily start all five pods
  # (the node has 100 slots), so "zero pods" can only come from gang admission.
  kq_job gang 5 0 20
  wait_for 120 "gang is Queued" is_phase gang Queued
  for _ in 1 2 3 4 5 6 7 8; do
    no_pods gang || fail "gang: $(pod_count gang) of 5 pods exist while the gang cannot be admitted (must be all-or-nothing)"
    job_suspended gang || fail "gang: Job was unsuspended although it does not fit"
    is_phase gang Queued || fail "gang: phase $(phase gang), want Queued"
    sleep 5
  done
  echo "gang message: $(message gang)"

  # Grow the tenant quota to 5: the quota operator updates the ClusterQueue, Kueue admits the whole gang at once.
  kubectl patch gryviatenant q1 --type=merge -p '{"spec":{"quotas":{"concurrentGPUs":5}}}' >/dev/null
  wait_for 120 "ClusterQueue nominalQuota is 5" bash -c "[[ \"\$(kubectl get clusterqueue $CQ -o jsonpath='{.spec.resourceGroups[0].flavors[0].resources[0].nominalQuota}')\" == 5 ]]"
  wait_for 300 "gang is Running" is_phase gang Running
  [[ "$(pod_count gang)" == "5" ]] || fail "gang: expected all 5 pods, got $(pod_count gang)"
  wait_for 300 "gang reaches Succeeded" is_phase gang Succeeded
  kubectl -n "$NS" delete gryviaaijob gang --wait=true >/dev/null

  kubectl patch gryviatenant q1 --type=merge -p '{"spec":{"quotas":{"concurrentGPUs":2}}}' >/dev/null
  wait_for 120 "ClusterQueue nominalQuota is back to 2" bash -c "[[ \"\$(kubectl get clusterqueue $CQ -o jsonpath='{.spec.resourceGroups[0].flavors[0].resources[0].nominalQuota}')\" == 2 ]]"
  wait_for 120 "ClusterQueue idle" cq_idle
}

scenario_preempt() {
  kq_job low 2 10 600
  wait_for 300 "low reaches Running" is_phase low Running
  kq_job high 2 90 20

  # Priority classes: spec.priority 10 -> gryvia-priority-10, 90 -> gryvia-priority-90 (created by the ai-operator).
  wait_for 60 "WorkloadPriorityClass gryvia-priority-90 exists" kubectl get workloadpriorityclass gryvia-priority-90
  kubectl get workloadpriorityclass gryvia-priority-10 >/dev/null || fail "gryvia-priority-10 missing"

  # LowerPriority preemption inside the ClusterQueue evicts low; its Job is suspended again and it goes back to Queued.
  wait_for 240 "low is requeued (Queued)" is_phase low Queued
  # ASSUMPTION: the operator turns "was admitted, is suspended again" into this message whatever Kueue's conditions say.
  wait_for 30 "low message says it was evicted" msg_matches low '^Evicted by Kueue'
  echo "low message: $(message low)"
  wait_for 120 "low has no pods left" no_pods low
  job_suspended low || fail "low: Job must be suspended after eviction"
  [[ "$(wl_field high '.spec.priority')" == "90" ]] || fail "high workload priority = $(wl_field high '.spec.priority'), want 90"
  [[ "$(wl_field low '.spec.priority')" == "10" ]] || fail "low workload priority = $(wl_field low '.spec.priority'), want 10"

  wait_for 300 "high reaches Running or Succeeded" bash -c "p=\$(kubectl -n $NS get gryviaaijob high -o jsonpath='{.status.phase}'); [[ \$p == Running || \$p == Succeeded ]]"
  wait_for 300 "high reaches Succeeded" is_phase high Succeeded
  # Kueue requeues the evicted workload: with the quota free again low runs again (not terminal Preempted).
  wait_for 300 "low is admitted again and Running" is_phase low Running

  kubectl -n "$NS" annotate gryviaaijob low gryvia.io/cancel=true --overwrite >/dev/null
  wait_for 120 "low is Cancelled" is_phase low Cancelled
  kubectl -n "$NS" delete gryviaaijob high low --ignore-not-found --wait=true >/dev/null
}

case "${1:-}" in
  setup)   scenario_setup ;;
  queue)   scenario_queue ;;
  gang)    scenario_gang ;;
  preempt) scenario_preempt ;;
  *) echo "usage: $0 setup|queue|gang|preempt" >&2; exit 2 ;;
esac
echo "E2E OK: ${1}"
