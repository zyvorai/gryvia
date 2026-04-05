#!/bin/bash
# Build all KubeFabric components
# Compiles operators, CLI, web UI, and creates Docker images

set -euo pipefail

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

REGISTRY="${REGISTRY:-ghcr.io/ssahani}"
VERSION="${VERSION:-$(git describe --tags --always --dirty)}"

echo -e "${GREEN}Building KubeFabric ${VERSION}${NC}"
echo "=================================="

# Build operators
echo -e "\n${YELLOW}Building operators...${NC}"
for operator in operators/*-operator; do
    op_name=$(basename "$operator")
    echo "  Building $op_name..."
    (cd "$operator" && go build -o bin/manager main.go)

    echo "  Building Docker image for $op_name..."
    docker build -t "${REGISTRY}/kubefabric-${op_name}:${VERSION}" "$operator"
    docker tag "${REGISTRY}/kubefabric-${op_name}:${VERSION}" "${REGISTRY}/kubefabric-${op_name}:latest"
done
echo -e "${GREEN}✓ Operators built${NC}"

# Build CLI
echo -e "\n${YELLOW}Building CLI...${NC}"
(cd cli && cargo build --release)
echo -e "${GREEN}✓ CLI built: cli/target/release/kubefabric${NC}"

# Build Web UI
echo -e "\n${YELLOW}Building Web UI...${NC}"
(cd web-ui && npm ci && npm run build)
docker build -t ${REGISTRY}/kubefabric-ui:${VERSION} -f docker/Dockerfile.ui .
docker tag ${REGISTRY}/kubefabric-ui:${VERSION} ${REGISTRY}/kubefabric-ui:latest
echo -e "${GREEN}✓ Web UI built${NC}"

# Build API Gateway
echo -e "\n${YELLOW}Building API Gateway...${NC}"
docker build -t ${REGISTRY}/kubefabric-api-gateway:${VERSION} services/api-gateway
docker tag ${REGISTRY}/kubefabric-api-gateway:${VERSION} ${REGISTRY}/kubefabric-api-gateway:latest
echo -e "${GREEN}✓ API Gateway built${NC}"

echo -e "\n${GREEN}=================================="
echo "Build complete!"
echo "==================================${NC}"
echo ""
echo "Built images:"
docker images | grep kubefabric | head -10
echo ""
echo "To push images:"
echo "  export REGISTRY=${REGISTRY}"
echo "  make docker-push"
echo ""
