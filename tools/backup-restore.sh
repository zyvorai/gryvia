#!/bin/bash
# TensorReaper Backup and Restore Tool
# Backs up CRDs, configurations, and state

set -euo pipefail

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

BACKUP_DIR="${BACKUP_DIR:-${HOME}/.tensorreaper/backups}"
NAMESPACE="${NAMESPACE:-default}"
TIMESTAMP=$(date +"%Y%m%d-%H%M%S")

usage() {
    cat << EOF
TensorReaper Backup and Restore Tool

Usage: $0 [COMMAND] [OPTIONS]

Commands:
    backup      Create a backup of TensorReaper resources
    restore     Restore from a backup
    list        List available backups
    verify      Verify backup integrity

Options:
    -n, --namespace NAMESPACE    Namespace (default: default)
    -d, --dir DIRECTORY         Backup directory (default: $BACKUP_DIR)
    -f, --file FILE             Specific backup file for restore
    -h, --help                  Show this help message

Examples:
    $0 backup
    $0 backup --namespace production
    $0 restore --file /backups/tensorreaper-20240101-120000.tar.gz
    $0 list
    $0 verify --file /backups/tensorreaper-20240101-120000.tar.gz

EOF
}

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

check_prerequisites() {
    log_info "Checking prerequisites..."

    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl not found"
        exit 1
    fi

    if ! kubectl cluster-info &> /dev/null; then
        log_error "Cannot connect to Kubernetes cluster"
        exit 1
    fi

    log_info "✓ Prerequisites met"
}

backup() {
    local backup_name="tensorreaper-${TIMESTAMP}"
    local backup_path="${BACKUP_DIR}/${backup_name}"

    log_info "Starting backup: ${backup_name}"

    # Create backup directory with restrictive permissions
    mkdir -p "${backup_path}"
    chmod 700 "${BACKUP_DIR}"
    chmod 700 "${backup_path}"

    # Backup CRDs
    log_info "Backing up CRDs..."
    local crd_names
    crd_names=$(kubectl get crds -o name | grep tensorreaper || true)
    if [ -n "$crd_names" ]; then
        echo "$crd_names" | xargs kubectl get -o yaml > "${backup_path}/crds.yaml"
        if [ ! -s "${backup_path}/crds.yaml" ]; then
            log_warn "CRD backup file is empty, backup may be incomplete"
        fi
    else
        log_warn "No TensorReaper CRDs found, skipping CRD backup"
        echo "# No TensorReaper CRDs found during backup" > "${backup_path}/crds.yaml"
    fi

    # Backup FabricGpuNodes
    log_info "Backing up GPU Nodes..."
    kubectl get fabricgpunodes -o yaml > "${backup_path}/gpu-nodes.yaml" 2>/dev/null || true

    # Backup FabricAIJobs
    log_info "Backing up AI Jobs..."
    kubectl get fabricaijobs -n "${NAMESPACE}" -o yaml > "${backup_path}/ai-jobs.yaml" 2>/dev/null || true

    # Backup FabricQuotas
    log_info "Backing up Quotas..."
    kubectl get fabricquotas -o yaml > "${backup_path}/quotas.yaml" 2>/dev/null || true

    # Backup FabricStorages
    log_info "Backing up Storage configs..."
    kubectl get fabricstorages -o yaml > "${backup_path}/storages.yaml" 2>/dev/null || true

    # Backup FabricNetworks
    log_info "Backing up Network configs..."
    kubectl get fabricnetworks -o yaml > "${backup_path}/networks.yaml" 2>/dev/null || true

    # Backup ConfigMaps
    log_info "Backing up ConfigMaps..."
    kubectl get configmaps -n tensorreaper -o yaml > "${backup_path}/configmaps.yaml" 2>/dev/null || true

    # Skip secrets by default to avoid storing sensitive data in plaintext backups
    # To include secrets, create them separately using: kubectl get secrets -n tensorreaper -o yaml | kubeseal > sealed-secrets.yaml
    log_info "Skipping secrets backup (use sealed-secrets for secret backup)"
    echo "Secrets excluded from backup for security. Use 'kubeseal' for encrypted secret backups." > "${backup_path}/secrets-skipped.txt"

    # Backup RBAC
    log_info "Backing up RBAC..."
    kubectl get clusterroles,clusterrolebindings,roles,rolebindings -o yaml > "${backup_path}/rbac.yaml" 2>/dev/null || true

    # Create metadata
    cat > "${backup_path}/metadata.json" << EOF
{
  "timestamp": "${TIMESTAMP}",
  "namespace": "${NAMESPACE}",
  "kubernetes_version": "$(kubectl version -o json 2>/dev/null | python3 -c 'import sys,json; print(json.load(sys.stdin)["serverVersion"]["gitVersion"])' 2>/dev/null || echo 'unknown')",
  "tensorreaper_version": "$(kubectl get deployment tensorreaper-gpu-operator -n tensorreaper -o jsonpath='{.spec.template.spec.containers[0].image}' 2>/dev/null || echo 'unknown')"
}
EOF

    # Create tarball
    log_info "Creating tarball..."
    tar -czf "${BACKUP_DIR}/${backup_name}.tar.gz" -C "${BACKUP_DIR}" "${backup_name}"

    # Calculate checksum
    sha256sum "${BACKUP_DIR}/${backup_name}.tar.gz" > "${BACKUP_DIR}/${backup_name}.tar.gz.sha256"

    # Cleanup temporary directory
    rm -rf "${backup_path}"

    local backup_size=$(du -h "${BACKUP_DIR}/${backup_name}.tar.gz" | cut -f1)
    log_info "✓ Backup completed successfully"
    log_info "  Location: ${BACKUP_DIR}/${backup_name}.tar.gz"
    log_info "  Size: ${backup_size}"
    log_info "  Checksum: ${BACKUP_DIR}/${backup_name}.tar.gz.sha256"
}

restore() {
    local backup_file="$1"

    if [ ! -f "${backup_file}" ]; then
        log_error "Backup file not found: ${backup_file}"
        exit 1
    fi

    log_info "Restoring from: ${backup_file}"

    # Verify checksum if exists
    if [ -f "${backup_file}.sha256" ]; then
        log_info "Verifying checksum..."
        if sha256sum -c "${backup_file}.sha256"; then
            log_info "✓ Checksum verified"
        else
            log_error "Checksum verification failed"
            exit 1
        fi
    else
        log_warn "No checksum file found, skipping verification"
    fi

    # Extract backup
    local restore_path="${BACKUP_DIR}/restore-${TIMESTAMP}"
    mkdir -p "${restore_path}"

    log_info "Extracting backup..."
    tar -xzf "${backup_file}" -C "${restore_path}"

    local backup_name=$(basename "${backup_file}" .tar.gz)
    local backup_path="${restore_path}/${backup_name}"

    # Show metadata
    if [ -f "${backup_path}/metadata.json" ]; then
        log_info "Backup metadata:"
        cat "${backup_path}/metadata.json"
    fi

    # Confirm restore
    read -p "$(echo -e ${YELLOW}Continue with restore? [y/N]: ${NC})" -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        log_info "Restore cancelled"
        rm -rf "${restore_path}"
        exit 0
    fi

    # Restore CRDs first
    if [ -f "${backup_path}/crds.yaml" ]; then
        log_info "Restoring CRDs..."
        kubectl apply -f "${backup_path}/crds.yaml"
        sleep 5  # Wait for CRDs to be registered
    fi

    # Restore resources
    for resource in gpu-nodes quotas storages networks ai-jobs; do
        local file="${backup_path}/${resource}.yaml"
        if [ -f "${file}" ]; then
            log_info "Restoring ${resource}..."
            kubectl apply -f "${file}"
        fi
    done

    # Restore ConfigMaps
    if [ -f "${backup_path}/configmaps.yaml" ]; then
        log_info "Restoring ConfigMaps..."
        kubectl apply -f "${backup_path}/configmaps.yaml"
    fi

    # Restore Secrets
    if [ -f "${backup_path}/secrets.yaml" ]; then
        log_warn "Restoring Secrets..."
        kubectl apply -f "${backup_path}/secrets.yaml"
    fi

    # Restore RBAC
    if [ -f "${backup_path}/rbac.yaml" ]; then
        log_info "Restoring RBAC..."
        kubectl apply -f "${backup_path}/rbac.yaml"
    fi

    # Cleanup
    rm -rf "${restore_path}"

    log_info "✓ Restore completed successfully"
}

list_backups() {
    log_info "Available backups in ${BACKUP_DIR}:"

    if [ ! -d "${BACKUP_DIR}" ] || [ -z "$(ls -A ${BACKUP_DIR}/*.tar.gz 2>/dev/null)" ]; then
        log_warn "No backups found"
        return
    fi

    echo ""
    printf "%-35s %-10s %-20s\n" "Backup Name" "Size" "Date"
    echo "--------------------------------------------------------------------------------"

    for backup in ${BACKUP_DIR}/*.tar.gz; do
        if [ -f "$backup" ]; then
            local name=$(basename "$backup")
            local size=$(du -h "$backup" | cut -f1)
            local date=$(stat -c %y "$backup" 2>/dev/null || stat -f %Sm -t "%Y-%m-%d %H:%M:%S" "$backup")
            printf "%-35s %-10s %-20s\n" "$name" "$size" "$date"
        fi
    done
}

verify_backup() {
    local backup_file="$1"

    if [ ! -f "${backup_file}" ]; then
        log_error "Backup file not found: ${backup_file}"
        exit 1
    fi

    log_info "Verifying backup: ${backup_file}"

    # Check checksum
    if [ -f "${backup_file}.sha256" ]; then
        log_info "Checking checksum..."
        if sha256sum -c "${backup_file}.sha256"; then
            log_info "✓ Checksum valid"
        else
            log_error "✗ Checksum invalid"
            exit 1
        fi
    else
        log_warn "No checksum file found"
    fi

    # Test extraction
    log_info "Testing extraction..."
    local test_dir="${BACKUP_DIR}/verify-${TIMESTAMP}"
    mkdir -p "${test_dir}"

    if tar -tzf "${backup_file}" > /dev/null 2>&1; then
        log_info "✓ Archive is valid"
    else
        log_error "✗ Archive is corrupted"
        rm -rf "${test_dir}"
        exit 1
    fi

    # Extract and check contents
    tar -xzf "${backup_file}" -C "${test_dir}"

    local backup_name=$(basename "${backup_file}" .tar.gz)
    local backup_path="${test_dir}/${backup_name}"

    log_info "Checking backup contents..."

    local files=("metadata.json" "crds.yaml")
    local found=0

    for file in "${files[@]}"; do
        if [ -f "${backup_path}/${file}" ]; then
            found=$((found + 1))
            log_info "✓ Found ${file}"
        fi
    done

    rm -rf "${test_dir}"

    if [ $found -gt 0 ]; then
        log_info "✓ Backup verification passed"
    else
        log_error "✗ Backup appears to be incomplete"
        exit 1
    fi
}

# Parse arguments
COMMAND="${1:-}"
shift || true

while [[ $# -gt 0 ]]; do
    case $1 in
        -n|--namespace)
            NAMESPACE="$2"
            shift 2
            ;;
        -d|--dir)
            BACKUP_DIR="$2"
            shift 2
            ;;
        -f|--file)
            BACKUP_FILE="$2"
            shift 2
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

# Main execution
case "${COMMAND}" in
    backup)
        check_prerequisites
        backup
        ;;
    restore)
        check_prerequisites
        if [ -z "${BACKUP_FILE:-}" ]; then
            log_error "Backup file not specified. Use -f option."
            exit 1
        fi
        restore "${BACKUP_FILE}"
        ;;
    list)
        list_backups
        ;;
    verify)
        if [ -z "${BACKUP_FILE:-}" ]; then
            log_error "Backup file not specified. Use -f option."
            exit 1
        fi
        verify_backup "${BACKUP_FILE}"
        ;;
    *)
        usage
        exit 1
        ;;
esac
