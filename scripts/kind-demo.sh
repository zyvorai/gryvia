#!/usr/bin/env bash
# Try Gryvia on a laptop, no GPUs needed: creates a kind cluster, installs Gryvia and loads demo data
# (fictional GPU nodes, a quota and a job).
#
#   ./scripts/kind-demo.sh                 # published release images
#   GRYVIA_LOCAL_IMAGES=1 ./scripts/kind-demo.sh   # build images from this checkout (needs docker)
#   ./scripts/kind-demo.sh --delete        # remove the cluster
#
# Requires: kind, kubectl, helm (and docker when building local images).
set -euo pipefail
cd "$(dirname "$0")/.."

CLUSTER="${GRYVIA_KIND_CLUSTER:-gryvia-demo}"
PORT="${GRYVIA_DEMO_PORT:-8443}"

if [[ "${1:-}" == "--delete" ]]; then kind delete cluster --name "$CLUSTER"; exit 0; fi

for tool in kind kubectl helm; do
  command -v "$tool" >/dev/null 2>&1 || { echo "$tool is required but not installed" >&2; exit 1; }
done

if kind get clusters 2>/dev/null | grep -qx "$CLUSTER"; then
  echo "Reusing kind cluster $CLUSTER"
else
  echo "Creating kind cluster $CLUSTER ..."
  kind create cluster --name "$CLUSTER" --wait 120s
fi
kubectl config use-context "kind-$CLUSTER" >/dev/null

extra=()
if [[ "${GRYVIA_LOCAL_IMAGES:-}" == "1" ]]; then
  command -v docker >/dev/null 2>&1 || { echo "docker is required for GRYVIA_LOCAL_IMAGES=1" >&2; exit 1; }
  TAG="dev"
  for spec in "gpu-operator:operators/gpu-operator:operators/gpu-operator/Dockerfile" \
              "ai-operator:operators/ai-operator:operators/ai-operator/Dockerfile" \
              "quota-operator:operators/quota-operator:operators/quota-operator/Dockerfile" \
              "api-gateway:services/api-gateway:services/api-gateway/Dockerfile" \
              "ui:.:docker/Dockerfile.ui"; do
    IFS=: read -r name ctx df <<<"$spec"
    echo "Building gryvia-$name ..."
    docker build -f "$df" -t "ghcr.io/zyvorai/gryvia-$name:$TAG" "$ctx"
    kind load docker-image --name "$CLUSTER" "ghcr.io/zyvorai/gryvia-$name:$TAG"
  done
  extra=(--set "global.imageTag=$TAG" --set global.imagePullPolicy=Never)
fi

echo "Installing Gryvia ..."
kubectl create namespace gryvia-system --dry-run=client -o yaml | kubectl apply -f - >/dev/null
# The GPU add-ons need real NVIDIA GPUs, so they are off in the demo.
helm upgrade --install gryvia ./helm/gryvia --namespace gryvia-system --set namespace.create=false \
  --set nvidiaDevicePlugin.enabled=false --set dcgmExporter.enabled=false --set monitoring.enabled=false \
  "${extra[@]}" --wait --timeout 300s

echo "Loading demo data ..."
kubectl apply -f examples/demo/demo.yaml

echo
echo "Gryvia demo is ready. Open the dashboard with:"
echo "  kubectl -n gryvia-system port-forward svc/gryvia-ui ${PORT}:443"
echo "then browse to https://localhost:${PORT} (accept the self-signed certificate) and sign in as admin / Admin@321."
echo "The GPU nodes are fictional demo data. Remove everything with: ./scripts/kind-demo.sh --delete"
