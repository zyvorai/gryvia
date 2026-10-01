import hashlib
from datetime import datetime, timezone

from fastapi import FastAPI
from fastapi.testclient import TestClient

from conftest import FakeCore, NoopLimiter, _stub_auth
from routers import llm
from routers.common import Deps

KEY_NS = "gryvia-llm-keys"


def client(fake_k8s, core, role="admin", tenants=(), key_ns=KEY_NS):
    deps = Deps(verify_auth=_stub_auth(role, tenants), k8s_custom=fake_k8s, k8s_core=core, limiter=NoopLimiter(),
                job_namespace="default", llm_key_namespace=key_ns,
                llm_gateway_url="http://gryvia-llm-gateway.gryvia-system.svc.cluster.local:8080")
    app = FastAPI()
    app.include_router(llm.build_router(deps))
    return TestClient(app)


def test_create_returns_the_key_once_and_stores_only_its_hash(fake_k8s):
    core = FakeCore()
    c = client(fake_k8s, core, role="tenant", tenants=["alpha"])
    r = c.post("/api/llm-keys", json={"name": "ci", "description": "CI runs"})
    assert r.status_code == 201, r.text
    body = r.json()
    assert body["key"].startswith("gk-") and len(body["key"]) > 40
    assert body["namespace"] == "tenant-alpha" and body["tenant"] == "alpha" and body["id"] == "tenant-alpha.ci"
    assert body["gatewayURL"].endswith(":8080")
    stored = core.secrets[(KEY_NS, "tenant-alpha.ci")]
    assert stored["stringData"] == {"hash": hashlib.sha256(body["key"].encode()).hexdigest(), "namespace": "tenant-alpha"}
    assert stored["metadata"]["labels"] == {"gryvia.io/llm-key": "true", "gryvia.io/tenant": "alpha",
                                            "gryvia.io/llm-key-name": "ci", "gryvia.io/llm-key-namespace": "tenant-alpha"}
    assert body["key"] not in str(stored)
    listed = c.get("/api/llm-keys").json()["items"]
    assert listed == [{"id": "tenant-alpha.ci", "name": "ci", "namespace": "tenant-alpha", "tenant": "alpha",
                       "prefix": body["key"][:10], "description": "CI runs", "created": "2026-01-01T00:00:00Z"}]
    assert "key" not in listed[0]
    assert c.post("/api/llm-keys", json={"name": "ci"}).status_code == 409


def test_tenants_are_scoped(fake_k8s):
    core = FakeCore()
    alpha = client(fake_k8s, core, role="tenant", tenants=["alpha"])
    beta = client(fake_k8s, core, role="tenant", tenants=["beta"])
    assert alpha.post("/api/llm-keys", json={"name": "ci", "namespace": "tenant-beta"}).status_code == 403
    assert alpha.post("/api/llm-keys", json={"name": "ci"}).status_code == 201
    assert beta.get("/api/llm-keys").json()["items"] == []
    assert beta.delete("/api/llm-keys/ci").status_code == 404
    assert (KEY_NS, "tenant-alpha.ci") in core.secrets
    assert beta.post("/api/llm-keys", json={"name": "ci"}).status_code == 201
    admin = client(fake_k8s, core)
    assert len(admin.get("/api/llm-keys").json()["items"]) == 2
    assert admin.delete("/api/llm-keys/ci").status_code == 409
    assert admin.delete("/api/llm-keys/ci?namespace=tenant-beta").json() == {"status": "deleted", "id": "tenant-beta.ci"}
    assert alpha.delete("/api/llm-keys/tenant-alpha.ci").status_code == 200
    assert core.secrets == {}


def test_admin_key_defaults_to_the_job_namespace_and_validation(fake_k8s):
    core = FakeCore()
    c = client(fake_k8s, core)
    assert c.post("/api/llm-keys", json={"name": "ops"}).json()["namespace"] == "default"
    assert c.post("/api/llm-keys", json={"name": "Bad Name"}).status_code == 422
    assert c.post("/api/llm-keys", json={"name": "x", "extra": 1}).status_code == 422


def test_disabled_gateway_returns_503(fake_k8s):
    c = client(fake_k8s, FakeCore(), key_ns=None)
    assert c.get("/api/llm-keys").status_code == 503
    assert c.post("/api/llm-keys", json={"name": "ci"}).status_code == 503
    assert c.get("/api/llm/models").json()["enabled"] is False


def seed_service(fake_k8s, name, ns, model=None, shared=False, endpoint="http://x", **ann):
    annotations = {**({"gryvia.io/llm-model": model} if model else {}),
                   **({"gryvia.io/llm-shared": "true"} if shared else {}), **ann}
    fake_k8s.add("gryviainferenceservices", {"metadata": {"name": name, "annotations": annotations},
                                             "status": {"endpoint": endpoint, "phase": "Running"}}, namespace=ns)


def test_models_lists_own_and_shared(fake_k8s):
    seed_service(fake_k8s, "chat", "tenant-alpha", "chat", **{"gryvia.io/llm-price-input-per-1m": "0.5",
                                                            "gryvia.io/llm-served-model": "/models/chat"})
    seed_service(fake_k8s, "embed", "platform", "embed", shared=True, endpoint="")
    seed_service(fake_k8s, "private", "tenant-beta", "private")
    seed_service(fake_k8s, "plain", "tenant-alpha")
    seed_service(fake_k8s, "bad", "tenant-alpha", "has space")
    items = client(fake_k8s, FakeCore(), role="tenant", tenants=["alpha"]).get("/api/llm/models").json()["items"]
    assert [(m["model"], m["namespace"]) for m in items] == [("chat", "tenant-alpha"), ("embed", "platform")]
    assert items[0]["servedModel"] == "/models/chat" and items[0]["priceInputPer1M"] == 0.5
    assert items[0]["priceOutputPer1M"] is None and items[0]["ready"] is True
    assert items[1]["shared"] is True and items[1]["ready"] is False
    assert len(client(fake_k8s, FakeCore()).get("/api/llm/models").json()["items"]) == 3


def seed_tokens(fake_k8s, name, ns, model, start, tin, tout, cost):
    fake_k8s.add("gryviausagerecords", {"metadata": {"name": name}, "spec": {
        "kind": "tokens", "tenant": ns.removeprefix("tenant-"), "model": model, "job": f"llm:{model}", "start": start,
        "inputTokens": tin, "outputTokens": tout, "cost": cost, "currency": "USD"}}, namespace=ns)


def test_usage_groups_token_records_and_reports_quotas(fake_k8s):
    today = datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:00:00Z")
    seed_tokens(fake_k8s, "t1", "tenant-alpha", "chat", today, 100, 50, 0.25)
    seed_tokens(fake_k8s, "t2", "tenant-alpha", "embed", today, 10, 0, 0.0)
    seed_tokens(fake_k8s, "t3", "tenant-beta", "chat", today, 1000, 1000, 1.0)
    seed_tokens(fake_k8s, "old", "tenant-alpha", "chat", "2020-01-01T00:00:00Z", 5, 5, 0.0)
    fake_k8s.add("gryviausagerecords", {"metadata": {"name": "gpu"}, "spec": {"job": "train", "start": today, "cost": 9}},
                 namespace="tenant-alpha")
    fake_k8s.add("gryviaquotas", {"metadata": {"name": "alpha"}, "spec": {"namespaces": ["tenant-alpha"], "tokensPerDay": 1000}})
    fake_k8s.add("gryviaquotas", {"metadata": {"name": "beta"}, "spec": {"namespaces": ["tenant-beta"], "tokensPerDay": 10}})
    fake_k8s.add("gryviaquotas", {"metadata": {"name": "gpu-only"}, "spec": {"namespaces": ["tenant-alpha"]}})

    alpha = client(fake_k8s, FakeCore(), role="tenant", tenants=["alpha"]).get("/api/llm/usage").json()
    assert alpha["items"] == [{"model": "chat", "inputTokens": 100, "outputTokens": 50, "cost": 0.25},
                              {"model": "embed", "inputTokens": 10, "outputTokens": 0, "cost": 0.0}]
    assert alpha["total"] == {"inputTokens": 110, "outputTokens": 50, "cost": 0.25}
    assert alpha["quotas"] == [{"name": "alpha", "namespaces": ["tenant-alpha"], "tokensPerDay": 1000, "usedToday": 160}]

    admin = client(fake_k8s, FakeCore()).get("/api/llm/usage?groupBy=tenant").json()
    assert [(g["tenant"], g["inputTokens"]) for g in admin["items"]] == [("alpha", 110), ("beta", 1000)]
    assert len(admin["quotas"]) == 2
    everything = client(fake_k8s, FakeCore()).get("/api/llm/usage?from=2019-01-01&groupBy=day").json()
    assert everything["items"][0]["day"] == "2020-01-01"
    assert client(fake_k8s, FakeCore()).get("/api/llm/usage?from=yesterday").status_code == 400
