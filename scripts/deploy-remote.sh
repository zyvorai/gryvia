#!/bin/bash
# ============================================================================
# deploy-remote.sh — Full TensorReaper deployment to a remote server
# ============================================================================
# One command to deploy TensorReaper to a remote Kubernetes node:
#   1. Rsync repo to remote
#   2. Install CRDs
#   3. Build and deploy operators
#   4. Deploy API gateway + Web UI
#   5. Optionally configure HTTPS with auto self-signed cert
#   6. Verify everything works
#
# Usage:
#   ./scripts/deploy-remote.sh <host> [user] [password]
#   ./scripts/deploy-remote.sh 185.165.240.5 root mypassword
#   ./scripts/deploy-remote.sh 10.0.0.1 root                  # SSH key auth
#   ./scripts/deploy-remote.sh 10.0.0.1 root pass --quick     # skip build
#   ./scripts/deploy-remote.sh 10.0.0.1 root pass --https     # enable HTTPS
#   ./scripts/deploy-remote.sh 10.0.0.1 root pass --uninstall # remove tensorreaper
#
# Environment variables:
#   DEPLOY_HOST=185.165.240.5
#   DEPLOY_USER=root
#   DEPLOY_PASS=mypassword
#   DEPLOY_DIR=/root/tensor-reaper
#   TENSORREAPER_HTTPS_PORT=30443
# ============================================================================

set -euo pipefail

info()  { echo "  ✅ $*"; }
warn()  { echo "  ⚠️  $*"; }
error() { echo "  ❌ $*"; exit 1; }
step()  { echo ""; echo "  🔧 $*"; }

# ── Parse args ──
QUICK_MODE=false
UNINSTALL_MODE=false
HTTPS_MODE=false
POSITIONAL=()
for arg in "$@"; do
    case "$arg" in
        --quick)     QUICK_MODE=true ;;
        --uninstall) UNINSTALL_MODE=true ;;
        --https)     HTTPS_MODE=true ;;
        --help|-h)
            echo "Usage: $0 <host> [user] [password] [--quick|--uninstall|--https]"
            echo ""
            echo "  --quick      Skip builds (only rsync + kubectl apply)"
            echo "  --uninstall  Remove TensorReaper from remote server"
            echo "  --https      Enable HTTPS with auto self-signed TLS certificate"
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
REMOTE_DIR="${DEPLOY_DIR:-/root/tensor-reaper}"
HTTPS_PORT="${TENSORREAPER_HTTPS_PORT:-30443}"

[ -z "$HOST" ] && error "Usage: $0 <host> [user] [password] [--quick|--https]"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

[ -d "$REPO_DIR/operators" ] || error "Not in tensor-reaper repo: $REPO_DIR"

# ── SSH/rsync wrappers ──
# NOTE: StrictHostKeyChecking=accept-new requires OpenSSH 7.6+ (2017-10-03).
# It accepts host keys on first connection but rejects changed keys (MITM protection).
_ssh() {
    if [ -n "$PASS" ]; then
        # WARNING: Password authentication is less secure than SSH key auth.
        # Consider using ssh-copy-id to set up key-based authentication.
        SSHPASS="$PASS" sshpass -e ssh -o StrictHostKeyChecking=accept-new "${USER}@${HOST}" "$@"
    else
        ssh -o StrictHostKeyChecking=accept-new "${USER}@${HOST}" "$@"
    fi
}

_rsync() {
    local ssh_cmd="ssh -o StrictHostKeyChecking=accept-new"
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
    echo "  ║     🗑️  TensorReaper Remote Uninstall               ║"
    echo "  ╚══════════════════════════════════════════════════╝"
    echo ""
    echo "  Host: ${USER}@${HOST}"
    echo ""

    step "Removing TensorReaper resources"
    _ssh "
        kubectl delete namespace tensorreaper --ignore-not-found 2>/dev/null || true
        kubectl delete crd fabricaijobs.tensorreaper.ai fabricgpunodes.tensorreaper.ai \
            fabricstorages.tensorreaper.ai fabricnetworks.tensorreaper.ai \
            fabricquotas.tensorreaper.ai --ignore-not-found 2>/dev/null || true
        kubectl delete secret tensorreaper-tls -n tensorreaper --ignore-not-found 2>/dev/null || true
        rm -rf $REMOTE_DIR
        rm -f /etc/tensorreaper/tls.*
        echo 'Done'
    " 2>&1 | grep -v "^Warning" || true

    info "TensorReaper removed from ${HOST}"
    exit 0
fi

TOTAL_STEPS=6
$QUICK_MODE && TOTAL_STEPS=4
$HTTPS_MODE && ((TOTAL_STEPS++))

echo ""
echo "  ╔══════════════════════════════════════════════════╗"
echo "  ║     🚀 TensorReaper Remote Deployment              ║"
echo "  ╚══════════════════════════════════════════════════╝"
echo ""
echo "  Host:     ${USER}@${HOST}"
echo "  Auth:     $([ -n "$PASS" ] && echo "🔑 password" || echo "🔐 SSH key")"
echo "  Local:    $REPO_DIR"
echo "  Remote:   $REMOTE_DIR"
echo "  HTTPS:    $($HTTPS_MODE && echo "🔒 enabled (port $HTTPS_PORT)" || echo "❌ disabled")"
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
        KVER=\$(kubectl version --client -o json 2>/dev/null | python3 -c 'import sys,json; print(json.load(sys.stdin)[\"clientVersion\"][\"gitVersion\"])' 2>/dev/null || echo 'unknown')
        echo \"kubectl: \$KVER\"
    else
        echo 'MISSING: kubectl'
        exit 1
    fi

    # Check helm
    if command -v helm &>/dev/null; then
        echo \"helm: \$(helm version --short 2>/dev/null)\"
    else
        echo 'helm: not found (optional)'
    fi

    # Check cluster
    if kubectl cluster-info &>/dev/null 2>&1; then
        NODES=\$(kubectl get nodes --no-headers 2>/dev/null | wc -l)
        echo \"cluster: \$NODES node(s)\"
    else
        echo 'MISSING: no cluster access'
        exit 1
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
        kubectl create namespace tensorreaper --dry-run=client -o yaml | kubectl apply -f -

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
        kubectl create namespace tensorreaper --dry-run=client -o yaml | kubectl apply -f -
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

# ── HTTPS auto setup ──
if $HTTPS_MODE; then
    if $QUICK_MODE; then
        HTTPS_STEP=4
    else
        HTTPS_STEP=6
    fi
    step "Step ${HTTPS_STEP}/${TOTAL_STEPS}: 🔒 Configuring HTTPS with auto self-signed certificate"

    _ssh "
        set -e
        CERT_DIR=/etc/tensorreaper
        mkdir -p \$CERT_DIR

        # Generate self-signed certificate if not present or expired
        REGEN=false
        if [ ! -f \$CERT_DIR/tls.crt ] || [ ! -f \$CERT_DIR/tls.key ]; then
            REGEN=true
        else
            # Check if cert expires within 30 days
            if ! openssl x509 -checkend 2592000 -noout -in \$CERT_DIR/tls.crt 2>/dev/null; then
                REGEN=true
            fi
        fi

        if \$REGEN; then
            echo '  Generating self-signed TLS certificate...'
            # Validate HOST to prevent injection into -subj (allow IPs and hostnames only)
            if ! echo '${HOST}' | grep -qP '^[a-zA-Z0-9._:-]+\$'; then
                echo '  ERROR: Invalid HOST value for certificate subject'
                exit 1
            fi
            openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
                -keyout \$CERT_DIR/tls.key \
                -out \$CERT_DIR/tls.crt \
                -subj \"/CN=${HOST}/O=TensorReaper\" \
                -addext \"subjectAltName=IP:${HOST},DNS:tensorreaper.local\" \
                2>/dev/null
            chmod 600 \$CERT_DIR/tls.key
            echo '  ✅ Certificate generated (365 days)'
        else
            echo '  ✅ Existing certificate still valid'
        fi

        # Create/update TLS secret in kubernetes
        kubectl -n tensorreaper delete secret tensorreaper-tls --ignore-not-found 2>/dev/null || true
        kubectl -n tensorreaper create secret tls tensorreaper-tls \
            --cert=\$CERT_DIR/tls.crt \
            --key=\$CERT_DIR/tls.key 2>/dev/null
        echo '  ✅ TLS secret created in tensorreaper namespace'

        # Patch UI service to add HTTPS NodePort
        kubectl -n tensorreaper apply -f - <<SVCEOF
apiVersion: v1
kind: Service
metadata:
  name: tensorreaper-ui-https
  namespace: tensorreaper
spec:
  type: NodePort
  ports:
    - port: 443
      targetPort: 443
      nodePort: ${HTTPS_PORT}
      protocol: TCP
      name: https
  selector:
    app: tensorreaper-ui
SVCEOF

        # Deploy nginx TLS termination sidecar as a separate pod
        kubectl -n tensorreaper apply -f - <<TLSEOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: tensorreaper-tls-nginx
  namespace: tensorreaper
data:
  nginx.conf: |
    events { worker_connections 128; }
    http {
      server {
        listen 443 ssl;
        ssl_certificate /etc/tls/tls.crt;
        ssl_certificate_key /etc/tls/tls.key;
        ssl_protocols TLSv1.2 TLSv1.3;
        ssl_ciphers HIGH:!aNULL:!MD5;

        location / {
          proxy_pass http://tensorreaper-ui.tensorreaper.svc:80;
          proxy_set_header Host \\\$host;
          proxy_set_header X-Real-IP \\\$remote_addr;
          proxy_set_header X-Forwarded-For \\\$proxy_add_x_forwarded_for;
          proxy_set_header X-Forwarded-Proto https;
        }

        location /api/ {
          proxy_pass http://tensorreaper-api-gateway.tensorreaper.svc:8080/api/;
          proxy_set_header Host \\\$host;
          proxy_set_header X-Real-IP \\\$remote_addr;
          proxy_set_header X-Forwarded-For \\\$proxy_add_x_forwarded_for;
          proxy_set_header X-Forwarded-Proto https;
        }
      }
    }
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: tensorreaper-tls-proxy
  namespace: tensorreaper
spec:
  replicas: 1
  selector:
    matchLabels:
      app: tensorreaper-tls-proxy
  template:
    metadata:
      labels:
        app: tensorreaper-tls-proxy
    spec:
      containers:
        - name: nginx
          image: nginx:1.27-alpine
          ports:
            - containerPort: 443
          volumeMounts:
            - name: tls
              mountPath: /etc/tls
              readOnly: true
            - name: nginx-conf
              mountPath: /etc/nginx/nginx.conf
              subPath: nginx.conf
              readOnly: true
          resources:
            requests:
              cpu: 50m
              memory: 32Mi
            limits:
              cpu: 200m
              memory: 64Mi
          securityContext:
            readOnlyRootFilesystem: false
            allowPrivilegeEscalation: false
      volumes:
        - name: tls
          secret:
            secretName: tensorreaper-tls
        - name: nginx-conf
          configMap:
            name: tensorreaper-tls-nginx
TLSEOF

        # Patch the HTTPS service selector to point to TLS proxy
        kubectl -n tensorreaper patch svc tensorreaper-ui-https \
            -p '{\"spec\":{\"selector\":{\"app\":\"tensorreaper-tls-proxy\"}}}' 2>/dev/null

        echo '  ✅ TLS proxy deployed'

        # Wait for TLS proxy to be ready
        echo '  Waiting for TLS proxy...'
        kubectl -n tensorreaper rollout status deployment tensorreaper-tls-proxy --timeout=60s 2>/dev/null || true

        echo '  ✅ HTTPS configured on port ${HTTPS_PORT}'
    " 2>&1
    info "HTTPS auto-configured with self-signed certificate"
fi

# ── Verify ──
if $QUICK_MODE; then
    VERIFY_STEP=$((TOTAL_STEPS))
else
    VERIFY_STEP=$((TOTAL_STEPS))
fi
step "Step ${VERIFY_STEP}/${TOTAL_STEPS}: ✅ Verifying deployment"

_ssh "
    echo ''
    echo '📋 CRDs:'
    kubectl get crd | grep tensorreaper || echo '  (none found)'

    echo ''
    echo '📦 Pods:'
    kubectl get pods -n tensorreaper --no-headers 2>/dev/null | head -15 || echo '  (none running)'

    echo ''
    echo '🌐 Services:'
    kubectl get svc -n tensorreaper --no-headers 2>/dev/null || echo '  (none)'
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

# Get service ports
UI_PORT=$(_ssh "kubectl get svc -n tensorreaper tensorreaper-ui -o jsonpath='{.spec.ports[0].nodePort}' 2>/dev/null" || echo "30081")
API_PORT=$(_ssh "kubectl get svc -n tensorreaper tensorreaper-api-gateway -o jsonpath='{.spec.ports[0].nodePort}' 2>/dev/null" || echo "30088")

echo "  🌐 Web Dashboard:"
echo "    http://${HOST}:${UI_PORT}"
if $HTTPS_MODE; then
    echo "    https://${HOST}:${HTTPS_PORT}  (self-signed cert)"
fi
echo ""
echo "  📡 API Gateway:"
echo "    http://${HOST}:${API_PORT}"
if $HTTPS_MODE; then
    echo "    https://${HOST}:${HTTPS_PORT}/api/"
fi
echo ""
echo "  🚀 Submit a job:"
echo "    kubectl apply -f examples/training/simple-pytorch-training.yaml"
echo ""
echo "  📊 Check status:"
echo "    kubectl get fabricgpunodes"
echo "    kubectl get fabricaijobs"
echo "    kubectl get pods -n tensorreaper"
echo ""
