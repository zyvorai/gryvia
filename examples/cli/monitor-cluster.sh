#!/usr/bin/env bash
# Example: Monitor cluster status

# Show cluster overview
echo "=== Cluster Overview ==="
gryvia cluster

echo ""
echo "=== GPU Nodes ==="
gryvia list nodes

echo ""
echo "=== Running Jobs ==="
gryvia list jobs

echo ""
echo "=== Team Quotas ==="
gryvia list quotas

echo ""
echo "=== Cost Analysis ==="
gryvia cost

echo ""
echo "=== Health Check ==="
gryvia health
