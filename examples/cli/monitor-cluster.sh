#!/usr/bin/env bash
# Example: Monitor cluster status

# Show cluster overview
echo "=== Cluster Overview ==="
tensorreaper cluster

echo ""
echo "=== GPU Nodes ==="
tensorreaper list nodes

echo ""
echo "=== Running Jobs ==="
tensorreaper list jobs

echo ""
echo "=== Team Quotas ==="
tensorreaper list quotas

echo ""
echo "=== Cost Analysis ==="
tensorreaper cost

echo ""
echo "=== Health Check ==="
tensorreaper health
