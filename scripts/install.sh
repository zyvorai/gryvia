#!/usr/bin/env bash
# Install Gryvia from the published Helm chart into the current kubectl context.
#
#   ./scripts/install.sh                         # release chart from ghcr.io
#   GRYVIA_VERSION=0.1.0 ./scripts/install.sh    # a specific version
#   GRYVIA_CHART=./helm/gryvia ./scripts/install.sh   # a local chart (development)
#
# Environment:
#   GRYVIA_VERSION    chart version (default: latest)
#   GRYVIA_CHART      chart reference (default: oci://ghcr.io/zyvorai/charts/gryvia)
#   GRYVIA_NAMESPACE  namespace (default: gryvia-system)
#   GRYVIA_API_KEY    sign-in key; the dashboard login is admin / <key>
#                     (default: the well-known lab key Admin@321 - change it on shared clusters)
#   GRYVIA_NODEPORT   expose the dashboard on this NodePort (default: ClusterIP; use port-forward)
#   HELM_EXTRA_ARGS   extra arguments passed to `helm upgrade --install`
set -euo pipefail

NS="${GRYVIA_NAMESPACE:-gryvia-system}"
CHART="${GRYVIA_CHART:-oci://ghcr.io/zyvorai/charts/gryvia}"
KEY="${GRYVIA_API_KEY:-Admin@321}"

for tool in kubectl helm; do
  command -v "$tool" >/dev/null 2>&1 || { echo "$tool is required but not installed" >&2; exit 1; }
done
kubectl version --request-timeout=10s >/dev/null 2>&1 || { echo "cannot reach a cluster with kubectl (current context: $(kubectl config current-context 2>&1))" >&2; exit 1; }

args=(upgrade --install gryvia "$CHART" --namespace "$NS" --create-namespace --set namespace.create=false --set "auth.apiKey=$KEY" --wait --timeout 300s)
[[ -n "${GRYVIA_VERSION:-}" ]] && args+=(--version "$GRYVIA_VERSION")
if [[ -n "${GRYVIA_NODEPORT:-}" ]]; then args+=(--set ui.service.type=NodePort --set "ui.service.nodePort=$GRYVIA_NODEPORT"); fi
# shellcheck disable=SC2206
[[ -n "${HELM_EXTRA_ARGS:-}" ]] && args+=(${HELM_EXTRA_ARGS})

echo "Installing Gryvia into namespace $NS ..."
kubectl get namespace "$NS" >/dev/null 2>&1 || kubectl create namespace "$NS"
helm "${args[@]}"

echo
echo "Gryvia is installed."
if [[ -n "${GRYVIA_NODEPORT:-}" ]]; then
  echo "Dashboard: https://<node-ip>:${GRYVIA_NODEPORT}"
else
  echo "Dashboard: kubectl -n $NS port-forward svc/gryvia-ui 8443:443   then open https://localhost:8443"
fi
echo "Sign in as: admin / $KEY   (self-signed certificate: accept the browser warning once)"
if [[ "$KEY" == "Admin@321" ]]; then
  echo "WARNING: this is the default lab credential. Set GRYVIA_API_KEY for anything reachable from an untrusted network."
fi
