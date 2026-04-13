#!/bin/bash
# Set up a local development environment using kind
# Creates a kind cluster with GPU simulation for testing

set -euo pipefail

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

CLUSTER_NAME="${CLUSTER_NAME:-tensorreaper-dev}"

echo -e "${GREEN}Setting up TensorReaper development environment${NC}"

# Check prerequisites
echo -e "\n${YELLOW}Checking prerequisites...${NC}"
command -v kind >/dev/null 2>&1 || { echo -e "${RED}kind is required. Install: https://kind.sigs.k8s.io/${NC}"; exit 1; }
command -v kubectl >/dev/null 2>&1 || { echo -e "${RED}kubectl is required${NC}"; exit 1; }
command -v helm >/dev/null 2>&1 || { echo -e "${RED}helm is required${NC}"; exit 1; }
command -v docker >/dev/null 2>&1 || { echo -e "${RED}docker is required${NC}"; exit 1; }

# Create kind cluster
echo -e "\n${YELLOW}Creating kind cluster: $CLUSTER_NAME...${NC}"
cat <<EOF | kind create cluster --name $CLUSTER_NAME --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  extraPortMappings:
  - containerPort: 30080
    hostPort: 8080
    protocol: TCP
  - containerPort: 30090
    hostPort: 9090
    protocol: TCP
- role: worker
  labels:
    tensorreaper.ai/gpu: "true"
    tensorreaper.ai/gpu-type: "H100"
    tensorreaper.ai/gpu-count: "8"
- role: worker
  labels:
    tensorreaper.ai/gpu: "true"
    tensorreaper.ai/gpu-type: "A100-80G"
    tensorreaper.ai/gpu-count: "8"
- role: worker
  labels:
    tensorreaper.ai/gpu: "true"
    tensorreaper.ai/gpu-type: "L40"
    tensorreaper.ai/gpu-count: "4"
EOF

echo -e "${GREEN}✓ Kind cluster created${NC}"

# Load local images (if built)
echo -e "\n${YELLOW}Loading local Docker images...${NC}"
for operator in gpu-operator ai-operator storage-operator network-operator quota-operator; do
    if docker images | grep -q "tensorreaper-$operator"; then
        echo "  - Loading tensorreaper-$operator..."
        kind load docker-image tensorreaper-$operator:latest --name $CLUSTER_NAME
    fi
done
echo -e "${GREEN}✓ Images loaded${NC}"

# Install cert-manager (required for webhooks)
echo -e "\n${YELLOW}Installing cert-manager...${NC}"
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.14.0/cert-manager.yaml
kubectl wait --for=condition=available --timeout=300s deployment/cert-manager -n cert-manager
echo -e "${GREEN}✓ cert-manager installed${NC}"

# Install Prometheus (for metrics)
echo -e "\n${YELLOW}Installing Prometheus...${NC}"
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update
helm install prometheus prometheus-community/kube-prometheus-stack \
    --namespace tensorreaper \
    --create-namespace \
    --set prometheus.service.type=NodePort \
    --set prometheus.service.nodePort=30090
echo -e "${GREEN}✓ Prometheus installed${NC}"

# Run quick start
echo -e "\n${YELLOW}Running quick start...${NC}"
bash scripts/quick-start.sh

echo -e "\n${GREEN}=================================="
echo "Development environment ready!"
echo "==================================${NC}"
echo ""
echo "Cluster name: $CLUSTER_NAME"
echo "Kubeconfig: kind get kubeconfig --name $CLUSTER_NAME"
echo ""
echo "Access services:"
echo "  Web UI:     http://localhost:8080 (after port-forward)"
echo "  Prometheus: http://localhost:9090"
echo ""
echo "Useful commands:"
echo "  kubectl get pods -n tensorreaper"
echo "  kubectl logs -n tensorreaper -l app=tensorreaper-gpu-operator"
echo "  kind delete cluster --name $CLUSTER_NAME"
echo ""
