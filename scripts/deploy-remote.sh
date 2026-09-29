#!/usr/bin/env bash
# Gryvia — remote deploy (SSH + rsync + on-host image build + Helm), modeled on
# netra's scripts/deploy-remote.sh.
#
# Deploys to a single-node k3s (or any cluster the remote user can reach):
#   1. rsync the repo to ~/.deployments/gryvia on the host
#   2. build the gpu-operator, ai-operator, api-gateway and ui images with podman
#      and import them into k3s's containerd
#   3. install the CRDs, then the gryvia-core Helm chart (operators)
#   4. deploy the API gateway and web UI (both HTTPS, self-signed certificates minted by
#      an init container in each pod), expose the UI on NodePort 32443
#   5. wait for every rollout, then smoke-test the UI, the API and a custom resource
#
# Usage:
#   ./scripts/deploy-remote.sh user@10.0.1.5
#   ./scripts/deploy-remote.sh 10.0.1.5 user
#   ./scripts/deploy-remote.sh user@10.0.1.5 --quick        # skip image builds
#   ./scripts/deploy-remote.sh user@10.0.1.5 --verify-only  # only run the checks
#   ./scripts/deploy-remote.sh user@10.0.1.5 --uninstall
#   ./scripts/deploy-remote.sh user@10.0.1.5 --dry-run      # print the remote script
#
# Environment:
#   GRYVIA_API_KEY          Gateway bearer / dashboard password (default: Admin@321, so
#                           you can sign in as admin / Admin@321). Set your own for
#                           anything reachable from an untrusted network. Written to
#                           ~/.gryvia/api-key on the host (mode 600).
#   GRYVIA_REMOTE_SUBDIR    checkout dir relative to the remote $HOME
#                           (default: .deployments/gryvia)
#   GRYVIA_UI_NODEPORT      NodePort for the web UI (default: 32443)
#   GRYVIA_DEPLOY_*         see scripts/lib/deploy-guards.sh (disk guard, timeouts)
#
# The kubeconfig is read from k3s with sudo into ~/.kube/gryvia-k3s.yaml and used
# only by this script; no shell startup files or cluster settings are changed.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

MODE="deploy"
DRY_RUN=false
POSITIONAL=()
SSH_OPTS=(-o StrictHostKeyChecking=accept-new -o ServerAliveInterval=30)

usage() { sed -n '2,27p' "$0" | sed 's/^# \{0,1\}//'; exit 0; }

while [[ $# -gt 0 ]]; do
  case "$1" in
    -h|--help) usage ;;
    --quick) MODE="quick"; shift ;;
    --verify-only) MODE="verify"; shift ;;
    --uninstall) MODE="uninstall"; shift ;;
    --dry-run) DRY_RUN=true; shift ;;
    -*) echo "unknown flag: $1" >&2; exit 2 ;;
    *) POSITIONAL+=("$1"); shift ;;
  esac
done

TARGET=""
if [[ ${#POSITIONAL[@]} -eq 1 ]]; then
  TARGET="${POSITIONAL[0]}"
elif [[ ${#POSITIONAL[@]} -eq 2 ]]; then
  # HOST USER, or user@host followed by anything
  if [[ "${POSITIONAL[0]}" == *@* ]]; then
    TARGET="${POSITIONAL[0]}"
  elif [[ "${POSITIONAL[1]}" == *@* ]]; then
    TARGET="${POSITIONAL[1]}"
  else
    TARGET="${POSITIONAL[1]}@${POSITIONAL[0]}"
  fi
else
  echo "usage: $0 user@host [--quick|--verify-only|--uninstall|--dry-run]" >&2
  echo "   or: $0 HOST USER [...]" >&2
  exit 2
fi

VERSION="$(sed -n 's/^appVersion: *"\{0,1\}\([^"]*\)"\{0,1\}$/\1/p' "${ROOT}/helm/gryvia-core/Chart.yaml")"
[[ -n "$VERSION" ]] || { echo "cannot read appVersion from helm/gryvia-core/Chart.yaml" >&2; exit 1; }
UI_NODEPORT="${GRYVIA_UI_NODEPORT:-32443}"
TARGET_HOST="${TARGET#*@}"
if [[ "$TARGET_HOST" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then TARGET_SAN="IP:${TARGET_HOST}"; else TARGET_SAN="DNS:${TARGET_HOST}"; fi
API_KEY_LOCAL="${GRYVIA_API_KEY:-}"
REMOTE_SUBDIR="${GRYVIA_REMOTE_SUBDIR:-.deployments/gryvia}"

log() { printf '[gryvia-deploy] %s\n' "$*"; }
ssh_host() { ssh "${SSH_OPTS[@]}" "$TARGET" "$@"; }

# Kubeconfig for this script only, and PATH. Runs on the host.
preamble() {
  cat <<'EOF'
mkdir -p "$HOME/.kube"
if [[ ! -r "$HOME/.kube/gryvia-k3s.yaml" ]] || [[ /etc/rancher/k3s/k3s.yaml -nt "$HOME/.kube/gryvia-k3s.yaml" ]]; then
  sudo cat /etc/rancher/k3s/k3s.yaml > "$HOME/.kube/gryvia-k3s.yaml"
  chmod 600 "$HOME/.kube/gryvia-k3s.yaml"
fi
export KUBECONFIG="$HOME/.kube/gryvia-k3s.yaml"
export PATH="/usr/local/bin:/usr/bin:$PATH"
EOF
}

if [[ "$MODE" == "uninstall" ]]; then
  remote_script=$(cat <<EOF
set -uo pipefail
$(preamble)
helm -n gryvia-system uninstall gryvia-core 2>/dev/null || true
kubectl -n gryvia-system delete deployment gryvia-api-gateway gryvia-ui --ignore-not-found
kubectl -n gryvia-system delete service gryvia-api-gateway gryvia-ui --ignore-not-found
kubectl delete clusterrole,clusterrolebinding gryvia-api-gateway gryvia-ui --ignore-not-found
kubectl delete namespace gryvia-system --ignore-not-found
echo "Gryvia removed. CRDs and ~/${REMOTE_SUBDIR} are kept; delete them by hand to purge data."
EOF
)
  if $DRY_RUN; then echo "$remote_script"; exit 0; fi
  ssh_host 'bash -s' <<<"$remote_script"
  exit 0
fi

# Smoke test, run on the host against the NodePort and the cluster. The certificate is
# self-signed, so curl runs with -k.
smoke_script=$(cat <<EOF
set -uo pipefail
$(preamble)
KEY="\$(cat "\$HOME/.gryvia/api-key" 2>/dev/null || true)"
BASE="https://127.0.0.1:${UI_NODEPORT}"
fail=0
check() { # check <name> <command...>
  local name="\$1"; shift
  if "\$@" >/dev/null 2>&1; then echo "  PASS  \$name"; else echo "  FAIL  \$name"; fail=1; fi
}
authed() { curl -sfk -H "Authorization: Bearer \$KEY" "\$@"; }
unauth_rejected() { local c; c="\$(curl -sk -o /dev/null -w '%{http_code}' "\$BASE/api/cluster/stats")"; [[ "\$c" == 401 || "\$c" == 403 ]]; }
echo "Gryvia smoke test"
check "namespace exists" kubectl get ns gryvia-system
check "CRD gryviaaijobs.gryvia.io established" kubectl wait --for=condition=Established crd/gryviaaijobs.gryvia.io --timeout=30s
for d in gryvia-core-gpu-operator gryvia-core-ai-operator gryvia-api-gateway gryvia-ui; do
  check "deployment \$d available" kubectl -n gryvia-system wait --for=condition=Available deployment/\$d --timeout=60s
done
check "UI serves index" curl -sfk "\$BASE/"
check "API rejects a request without a key" unauth_rejected
check "API accepts the key (cluster stats)" authed "\$BASE/api/cluster/stats"
check "auth/me identifies admin" bash -c "curl -sfk -H 'Authorization: Bearer \$KEY' '\$BASE/api/auth/me' | grep -q '\\"name\\": *\\"admin\\"'"
bad_bearer_rejected() { local c; c="\$(curl -sk -o /dev/null -w '%{http_code}' -H 'Authorization: Bearer not-the-key' "\$BASE/api/auth/me")"; [[ "\$c" == 401 || "\$c" == 403 ]]; }
check "auth/me rejects a wrong bearer" bad_bearer_rejected
check "API lists jobs" authed "\$BASE/api/jobs"
check "API lists nodes" authed "\$BASE/api/nodes"
# Every read endpoint the dashboard calls must answer 200 with JSON.
for path in cluster/stats jobs quotas nodes metrics/gpu metrics/costs \\
    network/flows network/policies network/insights network/graph network/anomalies network/costs network/traces \\
    security/alerts security/policies ai/training/insight ai/training/nccl gpu/memory \\
    workspaces models inference workflows tuners; do
  check "GET /api/\$path" bash -c "curl -sfk -H 'Authorization: Bearer \$KEY' '\$BASE/api/\$path' | python3 -c 'import sys,json; json.load(sys.stdin)'"
done
# Custom resource: accepted by the API server and visible through the Gryvia API.
kubectl delete gryviaquota gryvia-smoke --ignore-not-found >/dev/null 2>&1 || true
if kubectl apply -f "\$HOME/${REMOTE_SUBDIR}/scripts/lib/smoke-quota.yaml" >/dev/null 2>&1; then
  check "GryviaQuota visible through the API" bash -c "curl -sfk -H 'Authorization: Bearer \$KEY' '\$BASE/api/quotas' | grep -q gryvia-smoke"
  kubectl delete gryviaquota gryvia-smoke --ignore-not-found >/dev/null 2>&1 || true
else
  echo "  FAIL  apply GryviaQuota"; fail=1
fi
echo
kubectl -n gryvia-system get pods
if [[ \$fail -eq 0 ]]; then echo "ALL CHECKS PASSED"; else echo "SOME CHECKS FAILED"; fi
exit \$fail
EOF
)

if [[ "$MODE" == "verify" ]]; then
  if $DRY_RUN; then echo "$smoke_script"; exit 0; fi
  ssh_host 'bash -s' <<<"$smoke_script"
  exit $?
fi

REMOTE_HOME="$(ssh_host 'printf %s "$HOME"')"
REMOTE_DIR="${REMOTE_HOME}/${REMOTE_SUBDIR}"

log "sync → ${TARGET}:${REMOTE_DIR}"
if ! $DRY_RUN; then
  ssh_host "mkdir -p ${REMOTE_DIR}"
  rsync -az --delete -e "ssh ${SSH_OPTS[*]}" \
    --exclude '.git' --exclude 'node_modules' --exclude 'dist' --exclude 'target' \
    --exclude 'build' --exclude '.docusaurus' --exclude '.DS_Store' --exclude 'bin' \
    "${ROOT}/" "${TARGET}:${REMOTE_DIR}/"
fi

remote_script=$(cat <<EOF
set -euo pipefail
cd ${REMOTE_DIR}
$(preamble)
MODE="${MODE}"
VERSION="${VERSION}"
UI_NODEPORT="${UI_NODEPORT}"
REG="ghcr.io/zyvorai"

source scripts/lib/deploy-guards.sh
deploy_disk_guard /

# API key (dashboard password for user "admin"): explicit env, else the lab default.
mkdir -p "\$HOME/.gryvia"
API_KEY="${API_KEY_LOCAL:-Admin@321}"
printf '%s\n' "\$API_KEY" > "\$HOME/.gryvia/api-key"
chmod 600 "\$HOME/.gryvia/api-key"

if command -v podman >/dev/null 2>&1; then RT=podman
elif command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1; then RT=docker
else RT=""; fi

# name:build context:Dockerfile
IMAGES=(
  "gpu-operator:operators/gpu-operator:operators/gpu-operator/Dockerfile"
  "ai-operator:operators/ai-operator:operators/ai-operator/Dockerfile"
  "api-gateway:services/api-gateway:services/api-gateway/Dockerfile"
  "ui:.:docker/Dockerfile.ui"
)

if [[ "\$MODE" != "quick" ]]; then
  [[ -n "\$RT" ]] || { echo "podman or a working docker is required to build images" >&2; exit 1; }
  for spec in "\${IMAGES[@]}"; do
    IFS=: read -r name ctx df <<<"\$spec"
    echo "Building \$REG/gryvia-\$name:\$VERSION ..."
    \$RT build -f "\$df" -t "\$REG/gryvia-\$name:\$VERSION" "\$ctx"
  done
  # Build all, then import all: an imported image no pod uses yet is what kubelet's
  # image GC collects, and each build pushes the disk toward its threshold.
  for spec in "\${IMAGES[@]}"; do
    IFS=: read -r name _ _ <<<"\$spec"
    deploy_import_image "\$REG/gryvia-\$name:\$VERSION"
  done
fi

echo "Installing CRDs ..."
# A CRD's scope cannot be changed in place. If the installed scope differs from the
# generated one, recreate the CRD, but only when it has no objects (deleting a CRD
# deletes them); otherwise stop and say so.
for f in crds/*.yaml; do
  crd="\$(sed -n 's/^  name: \\(.*\\.gryvia\\.io\\)\$/\\1/p' "\$f" | head -1)"
  want="\$(sed -n 's/^  scope: //p' "\$f" | head -1)"
  have="\$(kubectl get crd "\$crd" -o jsonpath='{.spec.scope}' 2>/dev/null || true)"
  if [[ -n "\$have" && "\$have" != "\$want" ]]; then
    n="\$(kubectl get "\$crd" -A --no-headers 2>/dev/null | wc -l | tr -d ' ')"
    if [[ "\$n" != "0" ]]; then
      echo "CRD \$crd is \$have-scoped but should be \$want and has \$n object(s); back them up, delete them and the CRD, then redeploy" >&2
      exit 1
    fi
    echo "Recreating \$crd (scope \$have -> \$want, no objects)"
    kubectl delete crd "\$crd"
  fi
done
# The kinds were renamed Fabric* -> Gryvia*. Remove the legacy CRDs, but only when they hold
# no objects (deleting a CRD deletes them); otherwise leave them and say how to migrate.
for legacy in \$(kubectl get crd -o name 2>/dev/null | sed -n 's#^customresourcedefinition.apiextensions.k8s.io/\\(fabric.*\\.gryvia\\.io\\)\$#\\1#p'); do
  n="\$(kubectl get "\$legacy" -A --no-headers 2>/dev/null | wc -l | tr -d ' ')"
  if [[ "\$n" == "0" ]]; then
    echo "Removing legacy CRD \$legacy (no objects)"
    kubectl delete crd "\$legacy"
  else
    echo "WARNING: legacy CRD \$legacy still has \$n object(s); re-create them under the new Gryvia* kind and delete the CRD by hand" >&2
  fi
done
kubectl apply --server-side --force-conflicts -f crds/

echo "Installing gryvia-core (operators) ..."
helm upgrade --install gryvia-core ./helm/gryvia-core \\
  --namespace gryvia-system --create-namespace \\
  --set namespace.create=false \\
  --set crds.install=false \\
  --set monitoring.enabled=false \\
  --set nvidiaDevicePlugin.enabled=false \\
  --set dcgmExporter.enabled=false \\
  --set gpuOperator.image.repository="\$REG/gryvia-gpu-operator" --set gpuOperator.image.tag="\$VERSION" \\
  --set aiOperator.image.repository="\$REG/gryvia-ai-operator" --set aiOperator.image.tag="\$VERSION" \\
  --wait --timeout 300s

echo "Deploying API gateway and web UI ..."
# One self-signed certificate for the UI and the gateway, shared by every pod through a Secret so
# restarts and replicas serve the same certificate (a browser exception then keeps working).
# Created once; delete the gryvia-tls Secret to rotate it.
if ! kubectl -n gryvia-system get secret gryvia-tls >/dev/null 2>&1; then
  TLS_DIR="\$(mktemp -d)"
  HOST_IP="\$(hostname -I | awk '{print \$1}')"
  openssl req -x509 -nodes -days 3650 -newkey ec -pkeyopt ec_paramgen_curve:prime256v1 \\
    -keyout "\$TLS_DIR/tls.key" -out "\$TLS_DIR/tls.crt" -subj "/CN=gryvia/O=Gryvia" \\
    -addext "subjectAltName=DNS:gryvia,DNS:gryvia-ui,DNS:gryvia-ui.gryvia-system.svc,DNS:gryvia-api-gateway,DNS:gryvia-api-gateway.gryvia-system.svc,DNS:localhost,IP:127.0.0.1,IP:\$HOST_IP,${TARGET_SAN}"
  kubectl -n gryvia-system create secret tls gryvia-tls --cert="\$TLS_DIR/tls.crt" --key="\$TLS_DIR/tls.key"
  rm -rf "\$TLS_DIR"
fi
kubectl -n gryvia-system create secret generic gryvia-api-key \\
  --from-literal=GRYVIA_API_KEY="\$API_KEY" --dry-run=client -o yaml | kubectl apply -f -
sed "s#image: gryvia/api-gateway:.*#image: \$REG/gryvia-api-gateway:\$VERSION#" manifests/deploy/api-gateway-deployment.yaml | kubectl apply -f -
sed "s#image: gryvia/ui:.*#image: \$REG/gryvia-ui:\$VERSION#" manifests/deploy/ui-deployment.yaml | kubectl apply -f -
kubectl -n gryvia-system set env deployment/gryvia-api-gateway --from=secret/gryvia-api-key
kubectl -n gryvia-system patch svc gryvia-ui --type merge \\
  -p "{\"spec\":{\"type\":\"NodePort\",\"ports\":[{\"name\":\"https\",\"port\":443,\"targetPort\":\"https\",\"protocol\":\"TCP\",\"nodePort\":\$UI_NODEPORT}]}}"

# A fixed tag never changes the pod template, so restart to pick up freshly imported images.
# image-name:deployment:label selector
WORKLOADS=(
  "gpu-operator:gryvia-core-gpu-operator:app.kubernetes.io/component=gpu-operator"
  "ai-operator:gryvia-core-ai-operator:app.kubernetes.io/component=ai-operator"
  "api-gateway:gryvia-api-gateway:app=gryvia-api-gateway"
  "ui:gryvia-ui:app=gryvia-ui"
)
for spec in "\${WORKLOADS[@]}"; do
  IFS=: read -r img dep sel <<<"\$spec"
  ref="\$REG/gryvia-\$img:\$VERSION"
  deploy_ensure_image "\$ref"
  kubectl -n gryvia-system rollout restart deployment/\$dep
  deploy_wait_ready deployment/\$dep "\$sel" "\$ref"
done

echo "GRYVIA_UI=https://\$(hostname -I | awk '{print \$1}'):\$UI_NODEPORT"
echo "Sign in as: admin / \$API_KEY  (stored in ~/.gryvia/api-key on the host)"
if [[ "\$API_KEY" == "Admin@321" ]]; then
  echo "WARNING: this is the default lab credential. Set GRYVIA_API_KEY for anything reachable from an untrusted network."
fi
EOF
)

if $DRY_RUN; then
  log "dry-run remote script:"
  echo "$remote_script"
  exit 0
fi

ssh_host 'bash -s' <<<"$remote_script"
log "deployed; running smoke test"
ssh_host 'bash -s' <<<"$smoke_script"
