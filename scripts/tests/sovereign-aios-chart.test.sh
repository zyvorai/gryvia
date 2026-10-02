#!/usr/bin/env bash
# Helm render tests for helm/sovereign-aios: the shared credentials, the LLM gateway key hash, Zyntra's pack, URLs,
# certificate and keys, the agents' Zyntra service token and Zyntra's policy, Netra's certificate and keys, and the
# namespace guard. Needs helm and python3 with PyYAML and
# cryptography; no cluster. The sub-charts come from sibling checkouts (scripts/sovereign-aios-deps.sh, run here when
# charts/ is missing).
set -uo pipefail
cd "$(dirname "$0")/../.." || exit 1
CHART=helm/sovereign-aios

[[ -d "$CHART/charts" ]] || scripts/sovereign-aios-deps.sh >/dev/null || { echo "sovereign-aios-deps failed"; exit 1; }

render() { helm template sa "$CHART" -n "${NS:-gryvia-system}" --kube-version 1.31.0 "$@" 2>&1; }

FAILED=0
echo "namespace guard"
if render_out="$(NS=default render)"; then
  echo "  FAIL rendered outside Gryvia's namespace"; FAILED=1
elif grep -q "install sovereign-aios into Gryvia's namespace" <<<"$render_out"; then
  echo "  ok   refuses another namespace"
else
  echo "  FAIL unexpected error: $render_out"; FAILED=1
fi

check() {
  python3 - "$1" <<'EOF'
import base64, hashlib, sys, yaml
from cryptography import x509

docs = [d for d in yaml.safe_load_all(open(sys.argv[1])) if d]
failed = 0

def ok(cond, msg):
    global failed
    print(("  ok   " if cond else "  FAIL ") + msg)
    failed |= not cond

def find(kind, name=None, contains=None):
    for d in docs:
        n = d["metadata"]["name"]
        if d["kind"] == kind and (n == name or (contains and contains in n)):
            return d
    return None

creds = find("Secret", "sovereign-aios-credentials")["stringData"]
llm = creds["ZYNTRA_AI_API_KEY"]
ok(llm.startswith("gk-") and len(llm) == 67, "LLM key has the gateway's gk-<64 hex> shape")
ok(creds["GRYVIA_API_KEY"] == creds["ZYNTRA_GRAVIA_TOKEN"], "Zyntra reads Gryvia with the Gryvia API key")

h = find("Secret", "gryvia-system.platform-sovereign-aios")
ok(h is not None and h["metadata"]["namespace"] == "gryvia-llm-keys", "key hash Secret in the key namespace")
ok(h["metadata"]["labels"].get("gryvia.io/llm-key") == "true", "key hash Secret is labelled as a gateway key")
ok(h["stringData"]["hash"] == hashlib.sha256(llm.encode()).hexdigest(), "key hash is the SHA-256 of the LLM key")

gw = find("Deployment", "gryvia-api-gateway")["spec"]["template"]["spec"]["containers"][0]
ref = [e for e in gw["env"] if e["name"] == "GRYVIA_API_KEY"][0]["valueFrom"]["secretKeyRef"]["name"]
ok(ref == "sovereign-aios-credentials", "Gryvia's gateway uses the shared credentials")
ok(find("Secret", "gryvia-api-key") is None, "Gryvia's own API key Secret is not rendered")

z = find("Deployment", contains="zyntra")["spec"]["template"]["spec"]["containers"][0]
env = {e["name"]: e.get("value") for e in z["env"]}
ok(z["args"][-1] == "/etc/zyntra-pack", "Zyntra serves the mounted pack")
ok(env.get("ZYNTRA_AI_BASE_URL") == "http://gryvia-llm-gateway:8080/v1", "Zyntra's AI is the Gryvia LLM gateway")
ok(find("Service", "gryvia-llm-gateway") is not None, "the gateway Service has the name Zyntra uses")
ok(env.get("SSL_CERT_FILE") == "/etc/gryvia-tls/tls.crt", "Zyntra trusts Gryvia's certificate")
ok({"name": "sovereign-aios-credentials"} in [e.get("secretRef") for e in z["envFrom"]], "Zyntra gets the shared credentials")

pack = find("ConfigMap", "sovereign-aios-zyntra-pack")["data"]
ok(set(pack) >= {"pack.yaml", "kpis.yaml", "ontology.yaml"}, "pack ConfigMap holds the pack files")
ok("https://gryvia-api-gateway.gryvia-system.svc:8080/api/jobs" in pack["ontology.yaml"], "connectors point at the gateway")

crt = x509.load_pem_x509_certificate(base64.b64decode(find("Secret", "gryvia-tls")["data"]["tls.crt"]))
sans = crt.extensions.get_extension_for_class(x509.SubjectAlternativeName).value.get_values_for_type(x509.DNSName)
ok({"gryvia-api-gateway", "netra"} <= set(sans), "one certificate covers the gateway and Netra")

n = find("Deployment", "netra")["spec"]["template"]["spec"]
ok({"name": "tls-secret", "secret": {"secretName": "gryvia-tls"}} in n["volumes"], "Netra serves Gryvia's certificate")
tok = creds["AGENT_ZYNTRA_TOKEN"]
ok(tok.startswith("zst_") and len(tok) == 68, "agent token has Zyntra's zst_<64 hex> shape")
pol = yaml.safe_load(find("Secret", "sovereign-aios-zyntra-policy")["stringData"]["policy.yaml"])
st = pol["service_tokens"][0]
ok(st["token_sha256"] == hashlib.sha256(tok.encode()).hexdigest() and st["roles"] == ["viewer", "proposer"],
   "Zyntra's policy holds the agent token's SHA-256 with viewer and proposer")
ok(tok not in yaml.safe_dump(pol), "the policy carries the hash, not the token")
zs = find("Deployment", contains="zyntra")["spec"]["template"]["spec"]
ok({"name": "policy", "secret": {"secretName": "sovereign-aios-zyntra-policy",
    "items": [{"key": "policy.yaml", "path": "policy.yaml"}]}} in zs["volumes"], "Zyntra mounts the policy Secret")
ok(env.get("ZYNTRA_POLICY") == "/etc/zyntra/policy.yaml", "Zyntra reads the policy")
ok(find("ConfigMap", contains="zyntra-policy") is None, "no inline policy ConfigMap")

na = find("Secret", "sovereign-aios-netra-auth")["stringData"]
ok(na["api-key"] == creds["ZYNTRA_NETRA_TOKEN"], "Zyntra reads Netra with Netra's API key")
ok(find("Secret", "sovereign-aios-netra-ai")["stringData"]["api-key"] == llm, "Netra's AI uses the LLM key")
sys.exit(failed)
EOF
}

echo "defaults"
OUT="$(mktemp)"
render >"$OUT" || { echo "  FAIL render"; cat "$OUT"; exit 1; }
check "$OUT" || FAILED=1

echo "policy merge and agent namespaces"
VALS="$(mktemp)"
cat >"$VALS" <<'YAML'
agentToken:
  roles: [viewer]
  tenant: alpha
  namespaces: [tenant-alpha, tenant-beta]
zyntra:
  policy: |
    users:
      - {name: ops, roles: [approver]}
    service_tokens:
      - {name: ci, token_sha256: "00", roles: [viewer]}
YAML
render -f "$VALS" >"$OUT" || { echo "  FAIL render"; cat "$OUT"; exit 1; }
python3 - "$OUT" <<'EOF' || FAILED=1
import sys, yaml
docs = [d for d in yaml.safe_load_all(open(sys.argv[1])) if d]
sec = {(d["metadata"].get("namespace"), d["metadata"]["name"]): d for d in docs if d["kind"] == "Secret"}
tok = sec[("gryvia-system", "sovereign-aios-credentials")]["stringData"]["AGENT_ZYNTRA_TOKEN"]
pol = yaml.safe_load(sec[("gryvia-system", "sovereign-aios-zyntra-policy")]["stringData"]["policy.yaml"])
failed = 0
def ok(cond, msg):
    global failed
    print(("  ok   " if cond else "  FAIL ") + msg)
    failed |= not cond
ok(pol["users"] == [{"name": "ops", "roles": ["approver"]}], "zyntra.policy is kept")
names = [t["name"] for t in pol["service_tokens"]]
ok(names == ["ci", "gryvia-agents"], "the agent token is appended to zyntra.policy's service tokens")
ok(pol["service_tokens"][1]["roles"] == ["viewer"] and pol["service_tokens"][1]["tenant"] == "alpha", "roles and tenant")
for ns in ("tenant-alpha", "tenant-beta"):
    ok(sec[(ns, "zyntra-agent-token")]["stringData"]["token"] == tok, f"{ns} gets the token for its agents")
sys.exit(failed)
EOF
rm -f "$VALS"

echo "agent token off, bad token, bad policy"
render --set agentToken.enabled=false >"$OUT" || { echo "  FAIL render"; cat "$OUT"; exit 1; }
if grep -q "service_tokens" "$OUT"; then echo "  FAIL service token in the policy while off"; FAILED=1; else echo "  ok   no service token while off"; fi
if render --set agentToken.token=abc >/dev/null; then echo "  FAIL accepted a token without zst_"; FAILED=1; else echo "  ok   refuses a token without zst_"; fi
if render --set-string 'zyntra.policy=a: [' >/dev/null; then echo "  FAIL accepted invalid policy YAML"; FAILED=1; else echo "  ok   refuses invalid zyntra.policy"; fi

echo "netra and zyntra off"
render --set netra.enabled=false --set zyntra.enabled=false >"$OUT" || { echo "  FAIL render"; cat "$OUT"; exit 1; }
if grep -qE 'name: (netra|sovereign-aios-netra|sovereign-aios-zyntra-pack)$' "$OUT"; then
  echo "  FAIL Netra or Zyntra objects rendered while disabled"; FAILED=1
else
  echo "  ok   only Gryvia and the credentials"
fi
rm -f "$OUT"

[[ $FAILED -eq 0 ]] && echo "PASS" || { echo "FAIL"; exit 1; }
