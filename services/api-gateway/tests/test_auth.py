"""Login endpoint, default-key flag and the proxy-aware rate-limit key. main.py loads a kubeconfig at
import time, so the loaders are stubbed before importing it (same approach as test_main_endpoints)."""
import os
import sys

import pytest
from fastapi.testclient import TestClient

sys.path.insert(0, os.path.dirname(os.path.dirname(__file__)))


@pytest.fixture
def auth(fake_k8s, monkeypatch):
    from kubernetes import config
    monkeypatch.setattr(config, "load_incluster_config", lambda: None)
    monkeypatch.setattr(config, "load_kube_config", lambda: None)
    import main

    async def no_sleep(_):
        return None

    monkeypatch.setattr(main.asyncio, "sleep", no_sleep)
    monkeypatch.setattr(main, "API_KEY", "s3cret-key")
    main.limiter.enabled = True
    main.limiter.reset()
    yield TestClient(main.app), main
    main.limiter.reset()


def login(c, user="admin", pw="s3cret-key"):
    return c.post("/api/auth/login", json={"username": user, "password": pw})


def test_login_returns_the_bearer_for_valid_credentials(auth):
    c, _ = auth
    r = login(c)
    assert r.status_code == 200
    body = r.json()
    assert body["token"] == "s3cret-key" and body["name"] == "admin" and body["usingDefaultKey"] is False


@pytest.mark.parametrize("user,pw", [("admin", "wrong"), ("root", "s3cret-key"), ("", ""), ("admin", "")])
def test_login_rejects_bad_credentials(auth, user, pw):
    c, _ = auth
    r = login(c, user, pw)
    assert r.status_code == 401
    assert "token" not in r.text


def test_login_when_auth_is_not_configured(auth, monkeypatch):
    c, main = auth
    monkeypatch.setattr(main, "API_KEY", "")
    assert login(c, pw="anything").status_code == 401


def test_login_is_rate_limited(auth):
    c, _ = auth
    codes = [login(c, pw="wrong").status_code for _ in range(12)]
    assert codes[:10] == [401] * 10
    assert 429 in codes[10:]


def test_login_rejects_oversized_input(auth):
    c, _ = auth
    assert login(c, pw="x" * 1000).status_code == 422


def test_default_key_is_flagged(auth, monkeypatch):
    c, main = auth
    monkeypatch.setattr(main, "API_KEY", "Admin@321")
    assert login(c, pw="Admin@321").json()["usingDefaultKey"] is True
    me = c.get("/api/auth/me", headers={"Authorization": "Bearer Admin@321"}).json()
    assert me["usingDefaultKey"] is True and me["name"] == "admin"


def test_custom_key_is_not_flagged_in_me(auth):
    c, _ = auth
    me = c.get("/api/auth/me", headers={"Authorization": "Bearer s3cret-key"}).json()
    assert me["usingDefaultKey"] is False


class _Req:
    def __init__(self, peer, xff=None):
        self.client = type("C", (), {"host": peer})()
        self.headers = {"x-forwarded-for": xff} if xff else {}


def test_rate_limit_key_trusts_forwarded_for_only_from_private_peers(auth):
    _, main = auth
    assert main._client_ip(_Req("10.42.0.5", "203.0.113.9, 10.42.0.5")) == "203.0.113.9"
    assert main._client_ip(_Req("127.0.0.1", "198.51.100.7")) == "198.51.100.7"
    # A public peer cannot choose its own bucket by sending the header.
    assert main._client_ip(_Req("8.8.8.8", "1.2.3.4")) == "8.8.8.8"
    assert main._client_ip(_Req("10.0.0.1")) == "10.0.0.1"
