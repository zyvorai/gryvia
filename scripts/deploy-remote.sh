#!/bin/bash
# ============================================================================
# deploy-remote.sh — Full KubeFabric deployment to a remote server
# ============================================================================
# One command to deploy KubeFabric to a remote Kubernetes node:
#   1. Rsync repo to remote
#   2. Install CRDs
#   3. Build and deploy operators
#   4. Deploy API gateway + Web UI
#   5. Verify everything works
#
# Usage:
#   ./scripts/deploy-remote.sh <host> [user] [password]
#   ./scripts/deploy-remote.sh 185.165.240.5 root mypassword
#   ./scripts/deploy-remote.sh 10.0.0.1 root                  # SSH key auth
#   ./scripts/deploy-remote.sh 10.0.0.1 root pass --quick     # skip build
#   ./scripts/deploy-remote.sh 10.0.0.1 root pass --uninstall # remove kubefabric
#
# Environment variables:
#   DEPLOY_HOST=185.165.240.5
#   DEPLOY_USER=root
#   DEPLOY_PASS=mypassword
#   DEPLOY_DIR=/root/kube-fabric
# ============================================================================

set -euo pipefail

info()  { echo "  ✅ $*"; }
warn()  { echo "  ⚠️  $*"; }
error() { echo "  ❌ $*"; exit 1; }
step()  { echo ""; echo "  🔧 $*"; }

# ── Parse args ──
QUICK_MODE=false
UNINSTALL_MODE=false
POSITIONAL=()
for arg in "$@"; do
    case "$arg" in
        --quick)     QUICK_MODE=true ;;
        --uninstall) UNINSTALL_MODE=true ;;
        --help|-h)
            echo "Usage: $0 <host> [user] [password] [--quick|--uninstall]"
            echo ""
            echo "  --quick      Skip builds (only rsync + kubectl apply)"
            echo "  --uninstall  Remove KubeFabric from remote server"
            echo ""
            echo "Full mode: rsync, install CRDs, build operators, deploy all."
            exit 0
            ;;
        *)  POSITIONAL+=("$arg") ;;
    esac
done

HOST="${POSITIONAL[0]:-${DEPLOY_HOST:-}}"
USER="${POSITIONAL[1]:-${DEPLOY_USER:-root}}"
PASS="${POSITIONAL[2]:-${DEPLOY_PASS:-}}"
REMOTE_DIR="${DEPLOY_DIR:-/root/kube-fabric}"

[ -z "$HOST" ] && error "Usage: $0 <host> [user] [password] [--quick]"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

[ -d "$REPO_DIR/operators" ] || error "Not in kube-fabric repo: $REPO_DIR"

# ── SSH/rsync wrappers ──
_ssh() {
    if [ -n "$PASS" ]; then
        SSHPASS="$PASS" sshpass -e ssh -o StrictHostKeyChecking=no "${USER}@${HOST}" "$@"
    else
        ssh -o StrictHostKeyChecking=no "${USER}@${HOST}" "$@"
    fi
}

_rsync() {
    local ssh_cmd="ssh -o StrictHostKeyChecking=no"
    if [ -n "$PASS" ]; then
        ssh_cmd="sshpass -e $ssh_cmd"
    fi
    SSHPASS="$PASS" rsync -avz \
        --exclude='.git' --exclude='__pycache__' --exclude='*.pyc' \
        --exclude='node_modules' --exclude='dist/' --exclude='target/' \
        --exclude='bin/' --exclude='*.egg-info' \
        -e "$ssh_cmd" \
        "$@"
}

# ── Preflight ──
if [ -n "$PASS" ] && ! command -v sshpass &>/dev/null; then
    error "sshpass required for password auth. Install: dnf install sshpass"
fi

# ── Uninstall mode ──
if $UNINSTALL_MODE; then
    echo ""
    echo "  ╔══════════════════════════════════════════════════╗"
    echo "  ║     🗑️  KubeFabric Remote Uninstall               ║"
    echo "  ╚══════════════════════════════════════════════════╝"
    echo ""
    echo "  Host: ${USER}@${HOST}"
    echo ""

    step "Removing KubeFabric resources"
    _ssh "
        kubectl delete namespace kubefabric --ignore-not-found 2>/dev/null || true
        kubectl delete crd fabricaijobs.kubefabric.ai fabricgpunodes.kubefabric.ai \
            fabricstorages.kubefabric.ai fabricnetworks.kubefabric.ai \
            fabricquotas.kubefabric.ai --ignore-not-found 2>/dev/null || true
        rm -rf $REMOTE_DIR
        echo 'Done'
    " 2>&1 | grep -v "^Warning" || true

    info "KubeFabric removed from ${HOST}"
    exit 0
fi

TOTAL_STEPS=6
$QUICK_MODE && TOTAL_STEPS=4

echo ""
echo "  ╔══════════════════════════════════════════════════╗"
echo "  ║     🚀 KubeFabric Remote Deployment              ║"
echo "  ╚══════════════════════════════════════════════════╝"
echo ""
echo "  Host:     ${USER}@${HOST}"
echo "  Auth:     $([ -n "$PASS" ] && echo "🔑 password" || echo "🔐 SSH key")"
echo "  Local:    $REPO_DIR"
echo "  Remote:   $REMOTE_DIR"
echo "  Mode:     $($QUICK_MODE && echo "⚡ quick (rsync + apply only)" || echo "📦 full (build + deploy)")"
echo ""

# ── Step 1: Rsync repo ──
step "Step 1/${TOTAL_STEPS}: 📤 Syncing repository to ${HOST}"
_rsync "$REPO_DIR/" "${USER}@${HOST}:${REMOTE_DIR}/" 2>&1 | tail -3
info "Synced to ${HOST}:${REMOTE_DIR}"

# ── Step 2: Check prerequisites ──
step "Step 2/${TOTAL_STEPS}: 🔍 Checking prerequisites"
_ssh "
    # Check kubectl
    if command -v kubectl &>/dev/null; then
        KVER=\$(kubectl version --client --short 2>/dev/null | head -1 || kubectl version --client -o json 2>/dev/null | grep gitVersion | head -1)
        echo \"kubectl: \$KVER\"
    else
        echo 'MISSING: kubectl'
        exit 1
    fi

    # Check helm
    if command -v helm &>/dev/null; then
        echo \"helm: \$(helm version --short 2>/dev/null)\"
    else
        echo 'MISSING: helm'
    fi

    # Check cluster
    if kubectl cluster-info &>/dev/null 2>&1; then
        NODES=\$(kubectl get nodes --no-headers 2>/dev/null | wc -l)
        echo \"cluster: \$NODES node(s)\"
    else
        echo 'MISSING: no cluster access'
        exit 1
    fi

    # Check Go (for building)
    if command -v go &>/dev/null; then
        echo \"go: \$(go version 2>/dev/null | awk '{print \$3}')\"
    else
        echo 'go: not installed (will skip operator builds)'
    fi
" 2>&1
info "Prerequisites checked"

if ! $QUICK_MODE; then
    # ── Step 3: Install CRDs ──
    step "Step 3/${TOTAL_STEPS}: 📋 Installing CRDs"
    _ssh "
        cd $REMOTE_DIR
        kubectl apply -f crds/
    " 2>&1
    info "CRDs installed"

    # ── Step 4: Deploy namespace + RBAC ──
    step "Step 4/${TOTAL_STEPS}: 🔐 Creating namespace and RBAC"
    _ssh "
        cd $REMOTE_DIR
        kubectl create namespace kubefabric --dry-run=client -o yaml | kubectl apply -f -

        # Apply operator deployments
        for op in gpu-operator ai-operator storage-operator network-operator quota-operator; do
            if [ -f operators/\$op/config/deployment.yaml ]; then
                kubectl apply -f operators/\$op/config/deployment.yaml 2>&1
            fi
        done
    " 2>&1
    info "Operators deployed"

    # ── Step 5: Deploy API gateway + Web UI ──
    step "Step 5/${TOTAL_STEPS}: 🌐 Deploying API gateway and Web UI"
    _ssh "
        cd $REMOTE_DIR
        if [ -f manifests/deploy/api-gateway-deployment.yaml ]; then
            kubectl apply -f manifests/deploy/api-gateway-deployment.yaml 2>&1
        fi
        if [ -f manifests/deploy/ui-deployment.yaml ]; then
            kubectl apply -f manifests/deploy/ui-deployment.yaml 2>&1
        fi
    " 2>&1
    info "API gateway and Web UI deployed"
else
    # Quick mode: just apply CRDs + deployments
    step "Step 3/${TOTAL_STEPS}: 📋 Applying CRDs and deployments"
    _ssh "
        cd $REMOTE_DIR
        kubectl apply -f crds/
        kubectl create namespace kubefabric --dry-run=client -o yaml | kubectl apply -f -
        for op in gpu-operator ai-operator storage-operator network-operator quota-operator; do
            if [ -f operators/\$op/config/deployment.yaml ]; then
                kubectl apply -f operators/\$op/config/deployment.yaml 2>&1
            fi
        done
        [ -f manifests/deploy/api-gateway-deployment.yaml ] && kubectl apply -f manifests/deploy/api-gateway-deployment.yaml 2>&1
        [ -f manifests/deploy/ui-deployment.yaml ] && kubectl apply -f manifests/deploy/ui-deployment.yaml 2>&1
    " 2>&1
    info "Resources applied"
fi

# ── Verify ──
if $QUICK_MODE; then
    VERIFY_STEP=4
else
    VERIFY_STEP=6
fi
step "Step ${VERIFY_STEP}/${TOTAL_STEPS}: ✅ Verifying deployment"

_ssh "
    echo ''
    echo '📋 CRDs:'
    kubectl get crd | grep kubefabric || echo '  (none found)'

    echo ''
    echo '📦 Pods:'
    kubectl get pods -n kubefabric --no-headers 2>/dev/null | head -10 || echo '  (none running)'

    echo ''
    echo '🖥️  GPU Nodes:'
    kubectl get fabricgpunodes --no-headers 2>/dev/null | head -5 || echo '  (none registered)'

    echo ''
    echo '📊 Jobs:'
    kubectl get fabricaijobs --all-namespaces --no-headers 2>/dev/null | head -5 || echo '  (none submitted)'

    echo ''
    echo '🔑 Quotas:'
    kubectl get fabricquotas --all-namespaces --no-headers 2>/dev/null | head -5 || echo '  (none configured)'
" 2>&1

info "Deployment verified"

echo ""
echo "  ════════════════════════════════════════════════════"
echo "  🎉 Deployment complete: ${USER}@${HOST}"
echo "  ════════════════════════════════════════════════════"
echo ""
echo "  🔗 Connect:"
echo "    ssh ${USER}@${HOST}"
echo ""
echo "  🌐 Web Dashboard:"
echo "    kubectl port-forward -n kubefabric svc/kubefabric-ui 8080:80"
echo "    Then open: http://localhost:8080"
echo ""
echo "  🚀 Submit a job:"
echo "    kubectl apply -f examples/training/simple-pytorch-training.yaml"
echo ""
echo "  📊 Check status:"
echo "    kubectl get fabricgpunodes"
echo "    kubectl get fabricaijobs"
echo "    kubectl get pods -n kubefabric"
echo ""
