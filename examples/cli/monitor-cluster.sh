#!/usr/bin/env bash
# Example: Monitor cluster status

# Show cluster overview
echo "=== Cluster Overview ==="
kubefabric cluster

echo ""
echo "=== GPU Nodes ==="
kubefabric list nodes

echo ""
echo "=== Running Jobs ==="
kubefabric list jobs

echo ""
echo "=== Team Quotas ==="
kubefabric list quotas

echo ""
echo "=== Cost Analysis ==="
kubefabric cost

echo ""
echo "=== Health Check ==="
kubefabric health
