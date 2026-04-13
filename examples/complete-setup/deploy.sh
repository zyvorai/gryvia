#!/bin/bash
# Automated deployment script for complete TensorReaper setup

set -e

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${GREEN}TensorReaper Complete Setup Deployment${NC}"
echo "========================================"

# Check prerequisites
echo -e "\n${YELLOW}Checking prerequisites...${NC}"
command -v kubectl >/dev/null 2>&1 || { echo -e "${RED}kubectl required${NC}"; exit 1; }
command -v helm >/dev/null 2>&1 || { echo -e "${RED}helm required${NC}"; exit 1; }

# Create namespace
echo -e "\n${YELLOW}Creating namespace...${NC}"
kubectl create namespace tensorreaper --dry-run=client -o yaml | kubectl apply -f -

# Deploy CRDs
echo -e "\n${YELLOW}Deploying CRDs...${NC}"
kubectl apply -f ../../crds/

# Deploy operators
echo -e "\n${YELLOW}Deploying operators...${NC}"
for op in gpu-operator ai-operator storage-operator network-operator quota-operator; do
    echo "  - $op"
    kubectl apply -f ../../operators/$op/config/deployment.yaml
done

# Wait for operators
echo -e "\n${YELLOW}Waiting for operators to be ready...${NC}"
sleep 10
kubectl wait --for=condition=available --timeout=300s \
    -n tensorreaper \
    deployment/tensorreaper-gpu-operator \
    deployment/tensorreaper-ai-operator \
    deployment/tensorreaper-storage-operator \
    deployment/tensorreaper-network-operator \
    deployment/tensorreaper-quota-operator

# Deploy GPU nodes
echo -e "\n${YELLOW}Configuring GPU nodes...${NC}"
kubectl apply -f gpu-nodes/

# Deploy storage
echo -e "\n${YELLOW}Configuring storage...${NC}"
kubectl apply -f storage-config.yaml

# Deploy network
echo -e "\n${YELLOW}Configuring network...${NC}"
kubectl apply -f network-config.yaml

# Create quotas
echo -e "\n${YELLOW}Creating team quotas...${NC}"
kubectl apply -f quotas/

# Deploy monitoring
echo -e "\n${YELLOW}Deploying monitoring stack...${NC}"
kubectl apply -f ../../monitoring/

# Deploy API Gateway and Web UI
echo -e "\n${YELLOW}Deploying Web UI...${NC}"
kubectl apply -f ../../manifests/deploy/api-gateway-deployment.yaml
kubectl apply -f ../../manifests/deploy/ui-deployment.yaml

echo -e "\n${GREEN}========================================"
echo "Deployment complete!"
echo "========================================${NC}"
echo ""
echo "Next steps:"
echo "  1. Access Web UI: kubectl port-forward -n tensorreaper svc/tensorreaper-ui 8080:80"
echo "  2. Submit example jobs: kubectl apply -f jobs/"
echo "  3. Monitor: kubectl port-forward -n tensorreaper svc/prometheus-grafana 3000:80"
echo ""
