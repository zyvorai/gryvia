#!/usr/bin/env bash
# Helpers for .github/workflows/e2e-ml.yml (source this file). Not meant to be run on its own.
#
# The ML controllers (workspace, inference, model registry, workflow, auto tuner) are exercised on a kind cluster
# with tiny CPU images, so no GPU is involved. What is NOT proven by this e2e: real Jupyter/code-server/vLLM/Triton
# images, GPUs, and an HPA reacting to real metrics (kind has no metrics-server here).

NS="${E2E_NS:-gryvia-system}"
API="${E2E_API:-https://localhost:8443}"
KEY="${E2E_KEY:-Admin@321}"

api() { # api PATH [curl args...]: authenticated gateway call, fails on HTTP errors
  local path="$1"; shift
  curl -sfk -H "Authorization: Bearer $KEY" -H 'Content-Type: application/json' "$@" "$API$path"
}

api_status() { # api_status PATH [curl args...]: HTTP status code only
  local path="$1"; shift
  curl -sk -o /dev/null -w '%{http_code}' -H "Authorization: Bearer $KEY" -H 'Content-Type: application/json' "$@" "$API$path"
}

wait_for() { # wait_for "what" SECONDS command...: poll until the command succeeds
  local what="$1" timeout="$2"; shift 2
  local end=$((SECONDS + timeout))
  until "$@" >/dev/null 2>&1; do
    if [ "$SECONDS" -ge "$end" ]; then echo "TIMEOUT after ${timeout}s waiting for: $what" >&2; return 1; fi
    sleep 3
  done
  echo "ok: $what"
}

jp() { # jp KIND/NAME JSONPATH: a field of an object in the namespace
  kubectl -n "$NS" get "$1" -o "jsonpath=$2" 2>/dev/null
}

field_is() { [ "$(jp "$1" "$2")" = "$3" ]; }        # field_is KIND/NAME JSONPATH VALUE
field_set() { [ -n "$(jp "$1" "$2")" ]; }            # field_set KIND/NAME JSONPATH
field_empty() { [ -z "$(jp "$1" "$2")" ]; }          # field_empty KIND/NAME JSONPATH
gone() { ! kubectl -n "$NS" get "$@" >/dev/null 2>&1; } # gone KIND/NAME

assert_eq() { # assert_eq "what" ACTUAL EXPECTED
  if [ "$2" != "$3" ]; then echo "ASSERT FAILED: $1: got '$2', want '$3'" >&2; return 1; fi
  echo "ok: $1 = $3"
}

assert_nonempty() { # assert_nonempty "what" VALUE
  if [ -z "$2" ] || [ "$2" = "null" ]; then echo "ASSERT FAILED: $1 is empty" >&2; return 1; fi
  echo "ok: $1 = $2"
}

gw() { api "$1" | jq -r "$2"; } # gw PATH JQ_FILTER: a field of a gateway answer
