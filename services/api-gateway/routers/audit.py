"""Audit trail for the gateway: who changed what, and who was refused.

Every state-changing request (POST, PUT, PATCH, DELETE) is recorded after it completes, including
failed logins and refused requests (401/403), which are the interesting ones. An entry holds the time, a
request id, the caller (auth method, role, tenant, OIDC subject), the method, the route template and path,
the status and outcome, the client address and the duration. It never holds the Authorization header, a
query string or a request body.

Where entries go:
  * the ``gryvia.audit`` logger, one JSON object per line (this is the durable record: ship it with your
    log collector);
  * optionally a JSON-lines file (GRYVIA_AUDIT_FILE), appended with 0600 permissions;
  * a bounded in-memory buffer (GRYVIA_AUDIT_BUFFER entries, default 1000) behind ``GET /api/audit`` for
    the provider administrator. The buffer is per gateway replica and is lost on restart; use the log or
    file for anything that must survive.

The client address is the socket peer, not X-Forwarded-For (which a caller can forge). Behind a proxy it is
the proxy's address.
"""
import json
import logging
import os
import threading
import time
import uuid
from collections import deque
from datetime import datetime, timezone
from typing import Any, Deque, Dict, List, Optional

from fastapi import APIRouter, Depends, FastAPI, Query, Request

from .common import Deps, require_admin
from . import observability

audit_logger = logging.getLogger("gryvia.audit")

MUTATING = frozenset({"POST", "PUT", "PATCH", "DELETE"})
OUTCOMES = ("success", "denied", "error")


def _buffer_size() -> int:
    try:
        n = int(os.environ.get("GRYVIA_AUDIT_BUFFER", "1000"))
    except ValueError:
        return 1000
    return max(10, min(n, 100_000))


class AuditLog:
    """Thread-safe bounded buffer plus the log/file sinks."""

    def __init__(self, size: Optional[int] = None, path: Optional[str] = None) -> None:
        self._lock = threading.Lock()
        self._entries: Deque[Dict[str, Any]] = deque(maxlen=size or _buffer_size())
        self._path = path if path is not None else os.environ.get("GRYVIA_AUDIT_FILE", "") or None

    def record(self, entry: Dict[str, Any]) -> None:
        line = json.dumps(entry, separators=(",", ":"), sort_keys=True)
        with self._lock:
            self._entries.append(entry)
        audit_logger.info("%s", line)
        if self._path:
            try:
                fd = os.open(self._path, os.O_WRONLY | os.O_APPEND | os.O_CREAT, 0o600)
                with os.fdopen(fd, "a", encoding="utf-8") as f:
                    f.write(line + "\n")
            except OSError as exc:  # never fail a request because the audit file is unwritable
                audit_logger.error("audit file %s not writable: %s", self._path, exc)

    def query(self, limit: int = 100, outcome: Optional[str] = None, method: Optional[str] = None) -> List[Dict[str, Any]]:
        with self._lock:
            items = list(self._entries)
        out: List[Dict[str, Any]] = []
        for e in reversed(items):  # newest first
            if outcome and e["outcome"] != outcome:
                continue
            if method and e["method"] != method.upper():
                continue
            out.append(e)
            if len(out) >= limit:
                break
        return out


def outcome_of(status: int) -> str:
    if status in (401, 403):
        return "denied"
    return "success" if status < 400 else "error"


def _actor(request: Request) -> Dict[str, Any]:
    st = request.state
    claims = getattr(st, "user_claims", None) or {}
    subject = ""
    if isinstance(claims, dict):
        subject = str(claims.get("email") or claims.get("sub") or "")[:200]
    return {
        "auth": getattr(st, "auth_method", None) or "none",
        "role": getattr(st, "role", None) or "none",
        "tenant": getattr(st, "tenant", None) or "",
        "subject": subject,
    }


def _route(request: Request) -> str:
    route = request.scope.get("route")
    path = getattr(route, "path", None)
    return path if isinstance(path, str) else "unmatched"


def build_entry(request: Request, status: int, started: float) -> Dict[str, Any]:
    rid = request.headers.get("x-request-id", "")
    if not (0 < len(rid) <= 64 and rid.isprintable()):
        rid = uuid.uuid4().hex
    return {
        "time": datetime.now(timezone.utc).isoformat(timespec="milliseconds").replace("+00:00", "Z"),
        "id": rid,
        "actor": _actor(request),
        "method": request.method,
        "route": _route(request),
        "path": request.url.path[:300],
        "status": status,
        "outcome": outcome_of(status),
        "ip": request.client.host if request.client else "",
        "durationMs": round((time.perf_counter() - started) * 1000, 1),
    }


# One log per process; tests replace it through install(app, log).
LOG = AuditLog()


def install(app: FastAPI, log: Optional[AuditLog] = None) -> AuditLog:
    """Attach the audit middleware to app and return the log it writes to."""
    sink = log or LOG
    app.state.audit_log = sink  # read per request, so tests (or an operator hook) can swap it

    @app.middleware("http")
    async def _audit(request: Request, call_next):
        if request.method not in MUTATING:
            return await call_next(request)
        started = time.perf_counter()
        status = 500
        try:
            response = await call_next(request)
            status = response.status_code
            return response
        finally:
            entry = build_entry(request, status, started)
            request.app.state.audit_log.record(entry)
            observability.record_audit(entry["method"], entry["outcome"])

    return sink


def build_router(deps: Deps) -> APIRouter:
    router = APIRouter()

    @router.get("/api/audit")
    @deps.limiter.limit("30/minute")
    async def list_audit(
        request: Request,
        limit: int = Query(100, ge=1, le=1000),
        outcome: Optional[str] = Query(None, pattern="^(success|denied|error)$"),
        method: Optional[str] = Query(None, pattern="^(?i:POST|PUT|PATCH|DELETE)$"),
        _=Depends(deps.verify_auth),
        __=Depends(require_admin),
    ):
        log: AuditLog = request.app.state.audit_log
        return {"items": log.query(limit, outcome, method), "note": "per-replica buffer, lost on restart; the gryvia.audit log is the durable record"}

    return router
