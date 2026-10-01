#!/usr/bin/env bash
# `helm upgrade --reuse-values` (what OPERATIONS.md documents) renders the new chart with the previous
# release's values only, so a value map added after that release is missing and `.Values.a.b.c` fails with
# "nil pointer evaluating interface {}.c". Render the head chart with the oldest tagged release's values.
# Needs the chart dependencies (`helm dependency build helm/gryvia`) and the git tags.
set -euo pipefail
cd "$(dirname "$0")/../.."
base_ref="${BASE_REF:-$(git tag --list 'v*' --sort=creatordate | head -1)}"
[ -n "$base_ref" ] || { echo "no release tag to take values from" >&2; exit 1; }
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
cp -R helm/gryvia "$work/gryvia"
git show "$base_ref:helm/gryvia/values.yaml" > "$work/gryvia/values.yaml"
helm template gryvia "$work/gryvia" --kube-version 1.34.0 > /dev/null
# The same with every opt-in gate that reads a newer value map turned on through --set, as a user would
# when enabling a feature after the upgrade.
helm template gryvia "$work/gryvia" --kube-version 1.34.0 \
  --set webhook.enabled=true --set quotaOperator.enabled=true \
  --set quotaOperator.usageRecordWebhook.enabled=true --set quotaOperator.budgetWebhook.enabled=true \
  --set aiOperator.modelWatch.enabled=true > /dev/null
echo "head chart renders with the $base_ref values (helm upgrade --reuse-values)"
