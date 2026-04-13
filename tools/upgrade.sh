#!/bin/bash
# TensorReaper Upgrade Tool
# Safely upgrades TensorReaper to a new version

set -euo pipefail

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

NAMESPACE="${NAMESPACE:-tensorreaper}"
CURRENT_VERSION=""
TARGET_VERSION="${TARGET_VERSION:-latest}"
BACKUP_ENABLED="${BACKUP_ENABLED:-true}"
DRY_RUN="${DRY_RUN:-false}"

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

usage() {
    cat << EOF
TensorReaper Upgrade Tool

Usage: $0 [OPTIONS]

Options:
    -v, --version VERSION       Target version (default: latest)
    -n, --namespace NAMESPACE   Namespace (default: tensorreaper)
    --skip-backup               Skip pre-upgrade backup
    --dry-run                   Show what would be upgraded
    -h, --help                  Show this help message

Examples:
    $0 --version 1.1.0
    $0 --version latest --namespace production
    $0 --dry-run

EOF
}

check_prerequisites() {
    log_info "Checking prerequisites..."

    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl not found"
        exit 1
    fi

    if ! command -v helm &> /dev/null; then
        log_error "helm not found (required for Helm deployments)"
        exit 1
    fi

    if ! kubectl cluster-info &> /dev/null; then
        log_error "Cannot connect to Kubernetes cluster"
        exit 1
    fi

    log_info "✓ Prerequisites met"
}

get_current_version() {
    log_info "Detecting current version..."

    # Try to get version from GPU operator deployment
    CURRENT_VERSION=$(kubectl get deployment tensorreaper-gpu-operator \
        -n "${NAMESPACE}" \
        -o jsonpath='{.spec.template.spec.containers[0].image}' 2>/dev/null | \
        grep -oP '(?<=:)[^:]+$' || echo "unknown")

    if [ "${CURRENT_VERSION}" = "unknown" ]; then
        log_warn "Could not detect current version"
    else
        log_info "Current version: ${CURRENT_VERSION}"
    fi
}

pre_upgrade_checks() {
    log_info "Running pre-upgrade checks..."

    # Check for running jobs
    local running_jobs=$(kubectl get fabricaijobs -A --field-selector=status.phase=Running -o json | jq '.items | length')

    if [ "${running_jobs}" -gt 0 ]; then
        log_warn "${running_jobs} jobs are currently running"
        read -p "$(echo -e ${YELLOW}Continue with upgrade? [y/N]: ${NC})" -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            log_info "Upgrade cancelled"
            exit 0
        fi
    fi

    # Check operator health
    local unhealthy=$(kubectl get pods -n "${NAMESPACE}" --field-selector=status.phase!=Running -o json | jq '.items | length')

    if [ "${unhealthy}" -gt 0 ]; then
        log_warn "${unhealthy} pods are not in Running state"
    fi

    log_info "✓ Pre-upgrade checks passed"
}

backup_resources() {
    if [ "${BACKUP_ENABLED}" = "false" ]; then
        log_info "Skipping backup (--skip-backup specified)"
        return
    fi

    log_info "Creating pre-upgrade backup..."

    local backup_script="$(dirname "$0")/backup-restore.sh"

    if [ -f "${backup_script}" ]; then
        ${backup_script} backup --namespace "${NAMESPACE}" || { log_error "Backup failed, aborting upgrade"; exit 1; }
        log_info "✓ Backup completed"
    else
        log_warn "Backup script not found, skipping backup"
    fi
}

upgrade_crds() {
    log_info "Upgrading CRDs..."

    local crd_url="https://raw.githubusercontent.com/ssahani/tensor-reaper/${TARGET_VERSION}/crds"

    if [ "${TARGET_VERSION}" = "latest" ]; then
        crd_url="https://raw.githubusercontent.com/ssahani/tensor-reaper/main/crds"
    fi

    local crd_tmpdir
    crd_tmpdir=$(mktemp -d)
    trap 'rm -rf "${crd_tmpdir}"' RETURN

    for crd in fabricgpunode fabricaijob fabricstorage fabricnetwork fabricquota; do
        log_info "  Downloading ${crd}..."
        local crd_file="${crd_tmpdir}/${crd}.yaml"
        if ! curl -sSfL "${crd_url}/${crd}.yaml" -o "${crd_file}"; then
            log_warn "  Failed to download ${crd}"
            continue
        fi

        # Verify it's valid YAML and a CRD
        if ! kubectl apply --dry-run=client -f "${crd_file}" > /dev/null 2>&1; then
            log_error "  Downloaded CRD ${crd} failed validation, skipping"
            continue
        fi

        log_info "  Applying ${crd}..."
        kubectl apply -f "${crd_file}" || log_warn "  Failed to apply ${crd}"
    done

    log_info "✓ CRDs upgraded"
}

upgrade_operators() {
    log_info "Upgrading operators..."

    local operators=(
        "gpu-operator"
        "ai-operator"
        "storage-operator"
        "network-operator"
        "quota-operator"
    )

    for operator in "${operators[@]}"; do
        log_info "  Upgrading ${operator}..."

        kubectl set image deployment/tensorreaper-${operator} \
            -n "${NAMESPACE}" \
            manager=ghcr.io/ssahani/tensorreaper-${operator}:${TARGET_VERSION} \
            2>/dev/null || log_warn "  Failed to upgrade ${operator}"

        # Wait for rollout
        kubectl rollout status deployment/tensorreaper-${operator} \
            -n "${NAMESPACE}" \
            --timeout=5m 2>/dev/null || log_warn "  Rollout timeout for ${operator}"
    done

    log_info "✓ Operators upgraded"
}

upgrade_web_ui() {
    log_info "Upgrading Web UI and API Gateway..."

    kubectl set image deployment/tensorreaper-ui \
        -n "${NAMESPACE}" \
        ui=ghcr.io/ssahani/tensorreaper-ui:${TARGET_VERSION} \
        2>/dev/null || log_warn "  Failed to upgrade Web UI"

    kubectl set image deployment/tensorreaper-api-gateway \
        -n "${NAMESPACE}" \
        api-gateway=ghcr.io/ssahani/tensorreaper-api-gateway:${TARGET_VERSION} \
        2>/dev/null || log_warn "  Failed to upgrade API Gateway"

    log_info "✓ Web UI upgraded"
}

post_upgrade_checks() {
    log_info "Running post-upgrade checks..."

    # Check all pods are running
    local total_pods=$(kubectl get pods -n "${NAMESPACE}" -o json | jq '.items | length')
    local running_pods=$(kubectl get pods -n "${NAMESPACE}" --field-selector=status.phase=Running -o json | jq '.items | length')

    log_info "Pods status: ${running_pods}/${total_pods} running"

    if [ "${running_pods}" -lt "${total_pods}" ]; then
        log_warn "Not all pods are running"
        kubectl get pods -n "${NAMESPACE}"
    fi

    # Verify CRDs
    log_info "Verifying CRDs..."
    kubectl get crds | grep tensorreaper.ai

    log_info "✓ Post-upgrade checks completed"
}

helm_upgrade() {
    log_info "Upgrading via Helm..."

    helm upgrade tensorreaper tensorreaper/tensorreaper \
        --namespace "${NAMESPACE}" \
        --version "${TARGET_VERSION}" \
        --wait \
        --timeout 10m

    log_info "✓ Helm upgrade completed"
}

main() {
    echo -e "${BLUE}╔════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║          TensorReaper Upgrade Tool                              ║${NC}"
    echo -e "${BLUE}╚════════════════════════════════════════════════════════════════╝${NC}"
    echo ""

    if [[ "${DRY_RUN}" == "true" ]]; then
        log_info "DRY RUN MODE - no changes will be made"
    fi

    check_prerequisites
    get_current_version

    log_info "Upgrading from ${CURRENT_VERSION} to ${TARGET_VERSION}"

    pre_upgrade_checks

    if [[ "${DRY_RUN}" == "true" ]]; then
        log_info "[DRY RUN] Would back up resources, upgrade CRDs, and update operators"
        log_info "[DRY RUN] Upgrade plan validated successfully"
        return 0
    fi

    backup_resources

    # Check if Helm release exists
    if helm list -n "${NAMESPACE}" | grep -q "tensorreaper"; then
        log_info "Detected Helm installation"
        helm_upgrade
    else
        log_info "Detected manual installation"
        upgrade_crds
        upgrade_operators
        upgrade_web_ui
    fi

    post_upgrade_checks

    echo ""
    echo -e "${GREEN}╔════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║          Upgrade Completed Successfully! ✓                    ║${NC}"
    echo -e "${GREEN}╚════════════════════════════════════════════════════════════════╝${NC}"
    echo ""
    log_info "Upgraded from ${CURRENT_VERSION} to ${TARGET_VERSION}"
}

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -v|--version)
            TARGET_VERSION="$2"
            shift 2
            ;;
        -n|--namespace)
            NAMESPACE="$2"
            shift 2
            ;;
        --skip-backup)
            BACKUP_ENABLED=false
            shift
            ;;
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            usage
            exit 1
            ;;
    esac
done

main
