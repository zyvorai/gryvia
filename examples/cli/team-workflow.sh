#!/usr/bin/env bash
# Example: Team manager workflow

TEAM="ml-research"

echo "Team: $TEAM"
echo ""

# 1. Check quota status
echo "=== Quota Status ==="
kubefabric quota --team $TEAM --budget

echo ""

# 2. Check current spending
echo "=== Monthly Costs ==="
kubefabric cost --team $TEAM --period month

echo ""

# 3. List team's running jobs
echo "=== Running Jobs ==="
kubefabric list jobs -n ml-training

echo ""

# 4. Check if approaching budget
BUDGET_STATUS=$(kubefabric quota --team $TEAM --budget | grep "% Used" | awk '{print $NF}')
echo "Budget usage: $BUDGET_STATUS"

if [[ $(echo "$BUDGET_STATUS" | sed 's/%//') -gt 80 ]]; then
    echo "⚠️  WARNING: Team is approaching budget limit!"
else
    echo "✓ Budget status is healthy"
fi
