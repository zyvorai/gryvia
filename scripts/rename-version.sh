#!/usr/bin/env bash
# Moves the gryvia.io API from v1 to v1alpha1 (Kinds, apiVersion strings, Go GroupVersion, client constants).
# Idempotent. Only Gryvia's own group is touched: other groups (core v1, k8s.cni.cncf.io/v1, ...) are left alone.
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

files() {
  git ls-files -z | grep -zvE '(^scripts/rename-(kinds|version)\.sh$|^CHANGELOG\.md$|package-lock\.json|Cargo\.lock|go\.sum|^crds/|^helm/gryvia/crds/|\.(png|jpg|jpeg|gif|ico|svg|woff2?|ttf|pdf|tar|gz)$)'
}

# 1. apiVersion strings, everywhere (docs, examples, manifests, code).
files | xargs -0 grep -lIE 'gryvia\.io/v1([^a-z0-9]|$)' 2>/dev/null \
  | xargs perl -pi -e 's{gryvia\.io/v1(?![a-z0-9])}{gryvia.io/v1alpha1}g' || true

# 2. Go: the group version, plus the marker controller-gen reads (the Go package stays named v1).
for f in operators/*/api/v1/groupversion_info.go; do
  perl -pi -e 's/(Group: "gryvia\.io", Version: )"v1"/$1"v1alpha1"/' "$f"
  grep -q '+versionName=v1alpha1' "$f" || perl -pi -e 's{^(// \+groupName=gryvia\.io)$}{$1\n// +versionName=v1alpha1}' "$f"
done

# 3. Clients that pass the version separately (multi-line aware; the group string anchors each match).
files | xargs -0 grep -lIE '"gryvia\.io",\s*"v1"|group="gryvia\.io", *version="v1"|group = "gryvia\.io", version = "v1"' 2>/dev/null \
  | xargs perl -0pi -e 's/("gryvia\.io",\s*)"v1"/$1"v1alpha1"/g; s/(group="gryvia\.io", *version=)"v1"/$1"v1alpha1"/g; s/(group = "gryvia\.io", version = )"v1"/$1"v1alpha1"/g' || true
perl -pi -e 's/^VERSION = "v1"$/VERSION = "v1alpha1"/' services/api-gateway/routers/common.py
# main.py keeps the version on the line after group= in several calls.
perl -0pi -e 's/(group="gryvia\.io",\s*\n?\s*)version="v1"/$1version="v1alpha1"/g; s/(\n\s+)version="v1",(\n\s+(?:namespace=[^\n]*\n\s+)?plural="gryvia)/$1version="v1alpha1",$2/g' services/api-gateway/main.py
echo "Done. Regenerate CRDs: ./scripts/gen-crds.sh"
