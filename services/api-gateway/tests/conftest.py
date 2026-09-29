"""Test helpers: an in-memory fake of the Kubernetes CustomObjectsApi and a way to
mount one router module with fake dependencies (no cluster, no main.py import)."""
import copy
import os
import sys
from typing import Any, Dict, Optional

import pytest
from fastapi import FastAPI
from fastapi.testclient import TestClient
from kubernetes.client.exceptions import ApiException

sys.path.insert(0, os.path.dirname(os.path.dirname(__file__)))

from routers.common import Deps  # noqa: E402


class FakeCustomObjects:
    """Stores objects keyed by (plural, namespace-or-None, name)."""

    def __init__(self) -> None:
        self.store: Dict[tuple, Dict[str, Any]] = {}

    # -- seeding helper -------------------------------------------------
    def add(self, plural: str, obj: Dict[str, Any], namespace: Optional[str] = None) -> None:
        obj = copy.deepcopy(obj)
        obj.setdefault("metadata", {}).setdefault("creationTimestamp", "2026-01-01T00:00:00Z")
        if namespace:
            obj["metadata"]["namespace"] = namespace
        self.store[(plural, namespace, obj["metadata"]["name"])] = obj

    # -- CustomObjectsApi surface used by routers.common ----------------
    def list_cluster_custom_object(self, group, version, plural, **kw):
        return {"items": [copy.deepcopy(o) for (p, _, _), o in self.store.items() if p == plural]}

    def list_namespaced_custom_object(self, group, version, namespace, plural, **kw):
        return {"items": [copy.deepcopy(o) for (p, ns, _), o in self.store.items() if p == plural and ns == namespace]}

    def get_cluster_custom_object(self, group, version, plural, name, **kw):
        return self._get(plural, None, name)

    def get_namespaced_custom_object(self, group, version, namespace, plural, name, **kw):
        return self._get(plural, namespace, name)

    def create_cluster_custom_object(self, group, version, plural, body, **kw):
        return self._create(plural, None, body)

    def create_namespaced_custom_object(self, group, version, namespace, plural, body, **kw):
        return self._create(plural, namespace, body)

    def delete_cluster_custom_object(self, group, version, plural, name, **kw):
        return self._delete(plural, None, name)

    def delete_namespaced_custom_object(self, group, version, namespace, plural, name, **kw):
        return self._delete(plural, namespace, name)

    def patch_cluster_custom_object(self, group, version, plural, name, body, **kw):
        return self._patch(plural, None, name, body)

    def patch_namespaced_custom_object(self, group, version, namespace, plural, name, body, **kw):
        return self._patch(plural, namespace, name, body)

    # -- internals ------------------------------------------------------
    def _get(self, plural, ns, name):
        try:
            return copy.deepcopy(self.store[(plural, ns, name)])
        except KeyError:
            raise ApiException(status=404, reason="Not Found")

    def _create(self, plural, ns, body):
        key = (plural, ns, body["metadata"]["name"])
        if key in self.store:
            raise ApiException(status=409, reason="AlreadyExists")
        obj = copy.deepcopy(body)
        obj["metadata"].setdefault("creationTimestamp", "2026-01-01T00:00:00Z")
        self.store[key] = obj
        return copy.deepcopy(obj)

    def _delete(self, plural, ns, name):
        if (plural, ns, name) not in self.store:
            raise ApiException(status=404, reason="Not Found")
        del self.store[(plural, ns, name)]
        return {"status": "Success"}

    def _patch(self, plural, ns, name, body):
        obj = self.store.get((plural, ns, name))
        if obj is None:
            raise ApiException(status=404, reason="Not Found")

        def merge(dst, src):
            for k, v in src.items():
                if isinstance(v, dict) and isinstance(dst.get(k), dict):
                    merge(dst[k], v)
                else:
                    dst[k] = copy.deepcopy(v)

        merge(obj, body)
        return copy.deepcopy(obj)


class NoopLimiter:
    def limit(self, *_a, **_kw):
        return lambda fn: fn


class FakeCore:
    """Minimal CoreV1Api stand-in; tests can set attributes as needed."""

    def list_node(self, *a, **kw):
        class R:
            items = []
        return R()


async def _allow():  # verify_auth stand-in
    return None


@pytest.fixture
def fake_k8s() -> FakeCustomObjects:
    return FakeCustomObjects()


@pytest.fixture
def make_client(fake_k8s):
    """make_client("network") -> TestClient with routers/network.py mounted."""
    import importlib

    def _make(module: str) -> TestClient:
        deps = Deps(verify_auth=_allow, k8s_custom=fake_k8s, k8s_core=FakeCore(),
                    limiter=NoopLimiter(), job_namespace="default")
        app = FastAPI()
        app.include_router(importlib.import_module(f"routers.{module}").build_router(deps))
        return TestClient(app)

    return _make
