#!/usr/bin/env bash
# Two-cluster MultiKueue check (docs/kueue-integration.md): a GryviaAIJob submitted on the MANAGER cluster is
# dispatched by Kueue to a WORKER cluster, runs there, and its status comes back. Both are kind clusters on the
# same docker network. Run by .github/workflows/e2e-multikueue.yml, one step each:
#
#   scripts/e2e-multikueue.sh worker      # worker: Kueue-ready namespace, flavor, ClusterQueue, LocalQueue, fake slots
#   scripts/e2e-multikueue.sh connect     # manager: worker kubeconfig Secret, MultiKueueCluster/Config, AdmissionCheck
#   (then scripts/e2e-kueue.sh setup: the tenant queues; they stay inactive until the check above is Active)
#   scripts/e2e-multikueue.sh verify      # the tenant ClusterQueue is Active and bound to the check
#   scripts/e2e-multikueue.sh dispatch    # the job runs on the worker, not on the manager, and finishes
#
# Scope: this proves Kueue's MultiKueue binding as Gryvia configures it (tenant ClusterQueue bound to the
# admission check with platformCompletion.kueueAdmissionCheck=multikueue) with CPU pods. It does not prove GPU
# placement, data replication, failover or cost aggregation across clusters, none of which Gryvia implements.
#
# ASSUMPTIONS to look at first when a step fails: the manager's node image supports Job managedBy (Kubernetes
# >= 1.32 beta); the worker API server is reachable from the manager pods as https://<worker>-control-plane:6443
# (docker DNS on the kind network); Kueue's MultiKueue feature gate is on (default since Kueue 0.9).
set -euo pipefail

MANAGER_CTX="${MANAGER_CTX:-kind-gryvia-demo}"
WORKER_NAME="${WORKER_NAME:-gryvia-worker}"
WORKER_CTX="kind-$WORKER_NAME"
NS=tenant-q1
SLOT=example.com/slot
API=gryvia.io/v1alpha1

fail() { echo "E2E FAIL: $*" >&2; exit 1; }
km() { kubectl --context "$MANAGER_CTX" "$@"; }
kw() { kubectl --context "$WORKER_CTX" "$@"; }

wait_for() { # <seconds> <description> <command...>
  local secs="$1" what="$2"; shift 2
  local end=$((SECONDS + secs))
  until "$@" >/dev/null 2>&1; do
    (( SECONDS < end )) || fail "timed out after ${secs}s waiting for: $what"
    sleep 3
  done
  echo "ok: $what"
}

jsonpath_is() { # <kubectl-fn> <expected> <args...>: jsonpath value equals expected
  local fn="$1" want="$2"; shift 2
  [[ "$("$fn" "$@" 2>/dev/null)" == "$want" ]]
}

scenario_worker() {
  for n in $(kw get nodes -o name | sed 's#node/##'); do
    kw patch node "$n" --subresource=status --type=json \
      -p "[{\"op\":\"add\",\"path\":\"/status/capacity/example.com~1slot\",\"value\":\"100\"}]" >/dev/null
  done
  kw create namespace "$NS" --dry-run=client -o yaml | kw apply -f - >/dev/null
  wait_for 120 "Kueue CRDs on the worker" kw get crd clusterqueues.kueue.x-k8s.io localqueues.kueue.x-k8s.io
  # The manager's queue name is "gryvia" in $NS (the default the ai-operator uses); the worker must have the same.
  for _ in $(seq 1 30); do
    if cat <<YAML | kw apply -f - >/dev/null 2>&1; then break; fi
apiVersion: kueue.x-k8s.io/v1beta1
kind: ResourceFlavor
metadata: {name: default}
YAML
    sleep 4
  done
  cat <<YAML | kw apply -f - >/dev/null
apiVersion: kueue.x-k8s.io/v1beta1
kind: ClusterQueue
metadata: {name: worker-cq}
spec:
  namespaceSelector: {}
  resourceGroups:
    - coveredResources: [cpu, memory, $SLOT]
      flavors:
        - name: default
          resources:
            - {name: cpu, nominalQuota: "4"}
            - {name: memory, nominalQuota: 4Gi}
            - {name: $SLOT, nominalQuota: "50"}
---
apiVersion: kueue.x-k8s.io/v1beta1
kind: LocalQueue
metadata: {name: gryvia, namespace: $NS}
spec: {clusterQueue: worker-cq}
YAML
  wait_for 120 "worker ClusterQueue is Active" \
    jsonpath_is kw True get clusterqueue worker-cq -o 'jsonpath={.status.conditions[?(@.type=="Active")].status}'
}

scenario_connect() {
  local kcfg
  kcfg="$(mktemp)"
  kind get kubeconfig --internal --name "$WORKER_NAME" > "$kcfg"
  # Kueue reads the Secret from its own namespace (the chart installs it in gryvia-system).
  km -n gryvia-system create secret generic worker-kubeconfig --from-file=kubeconfig="$kcfg" \
    --dry-run=client -o yaml | km apply -f - >/dev/null
  rm -f "$kcfg"
  cat <<YAML | km apply -f - >/dev/null
apiVersion: kueue.x-k8s.io/v1beta1
kind: MultiKueueCluster
metadata: {name: worker1}
spec:
  kubeConfig: {locationType: Secret, location: worker-kubeconfig}
---
apiVersion: kueue.x-k8s.io/v1beta1
kind: MultiKueueConfig
metadata: {name: gryvia-workers}
spec: {clusters: [worker1]}
---
apiVersion: kueue.x-k8s.io/v1beta1
kind: AdmissionCheck
metadata: {name: multikueue}
spec:
  controllerName: kueue.x-k8s.io/multikueue
  parameters: {apiGroup: kueue.x-k8s.io, kind: MultiKueueConfig, name: gryvia-workers}
YAML
  wait_for 180 "MultiKueueCluster worker1 is Active (manager reaches the worker)" \
    jsonpath_is km True get multikueuecluster worker1 -o 'jsonpath={.status.conditions[?(@.type=="Active")].status}'
  wait_for 120 "AdmissionCheck multikueue is Active" \
    jsonpath_is km True get admissioncheck multikueue -o 'jsonpath={.status.conditions[?(@.type=="Active")].status}'
}

# After e2e-kueue.sh setup: the tenant ClusterQueue exists. It stays inactive while the admission check it
# references does not exist or is not Active, which is why `connect` must run before `setup`.
scenario_verify() {
  wait_for 180 "tenant ClusterQueue gryvia-q1 is Active with the check bound" \
    jsonpath_is km True get clusterqueue gryvia-q1 -o 'jsonpath={.status.conditions[?(@.type=="Active")].status}'
  km get clusterqueue gryvia-q1 -o jsonpath='{.spec.admissionChecksStrategy}' | grep -q multikueue \
    || fail "gryvia-q1 does not reference the multikueue admission check (platformCompletion.kueueAdmissionCheck)"
}

phase() { km -n "$NS" get gryviaaijob "$1" -o jsonpath='{.status.phase}' 2>/dev/null; }
is_phase() { [[ "$(phase "$1")" == "$2" ]]; }

scenario_dispatch() {
  cat <<YAML | km apply -f - >/dev/null
apiVersion: $API
kind: GryviaAIJob
metadata: {name: remote1, namespace: $NS}
spec:
  type: training
  image: busybox:1.36
  gpus: 0
  retryLimit: 0
  timeout: 20m
  resources:
    requests: {cpu: 20m, memory: 16Mi, $SLOT: "1"}
    limits: {cpu: 200m, memory: 64Mi, $SLOT: "1"}
  command: ["sh", "-c"]
  args: ["echo ran-on-worker; hostname; sleep 20"]
YAML
  # Kueue mirrors the Job to the worker once the workload is admitted through the multikueue check.
  wait_for 300 "the Job appears on the worker cluster" kw -n "$NS" get job remote1
  wait_for 300 "a pod of the job runs on the worker" \
    bash -c "kubectl --context $WORKER_CTX -n $NS get pods -l job-name=remote1 -o jsonpath='{.items[*].status.phase}' | grep -Eq 'Running|Succeeded'"
  [[ "$(km -n "$NS" get pods -o name | grep -c remote1 || true)" == "0" ]] || fail "the manager cluster ran a pod of remote1; it must run on the worker only"
  wait_for 300 "the GryviaAIJob on the manager reaches Succeeded (status came back from the worker)" is_phase remote1 Succeeded
  kw -n "$NS" logs job/remote1 | grep -q ran-on-worker || fail "worker pod log missing"
  km -n "$NS" delete gryviaaijob remote1 --wait=true >/dev/null
}

case "${1:-}" in
  worker)   scenario_worker ;;
  connect)  scenario_connect ;;
  verify)   scenario_verify ;;
  dispatch) scenario_dispatch ;;
  *) echo "usage: $0 worker|connect|verify|dispatch" >&2; exit 2 ;;
esac
echo "E2E OK: ${1}"
