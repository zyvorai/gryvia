"""Agent routes (GryviaAgent, namespaced) for the dashboard and CLI.

POST /api/agents/{name}/chat proxies an OpenAI-style chat request to the agent's in-cluster runtime (the agent's
NetworkPolicy admits the api-gateway pods). The agent calls its model through the LLM gateway with its own key.
"""
import re
from typing import Any, Dict, List, Literal, Optional

import httpx
from fastapi import APIRouter, Depends, HTTPException, Request
from pydantic import BaseModel, ConfigDict, Field, model_validator

from .common import Deps, create_item, delete_item, patch_item
from .uiutil import NAME_MAX, NAME_PATTERN, find_one, list_all, meta, namespaces, prune

PLURAL = "gryviaagents"
KIND = "GryviaAgent"
AGENT_PORT = 8080
CHAT_TIMEOUT_SECONDS = 180.0

_DNS = r"^[a-z0-9]([-a-z0-9.]*[a-z0-9])?$"
_QUANTITY = r"^([0-9]+(\.[0-9]+)?(m|Ki|Mi|Gi|Ti|K|M|G|T)?)?$"


class Tool(BaseModel):
    model_config = ConfigDict(extra="forbid")
    name: str = Field(min_length=1, max_length=64, pattern=r"^[a-zA-Z0-9_-]+$")
    description: str = Field(default="", max_length=1024)
    type: Literal["retrieval", "http"]
    vectorIndexRef: str = Field(default="", max_length=253, pattern=r"^(" + _DNS[1:-1] + r")?$")
    topK: int = Field(default=0, ge=0, le=20)
    urls: List[str] = Field(default_factory=list, max_length=16)
    method: Literal["GET", "POST"] = "GET"

    @model_validator(mode="after")
    def _by_type(self) -> "Tool":
        if self.type == "retrieval":
            if not self.vectorIndexRef:
                raise ValueError(f"tool {self.name}: a retrieval tool needs vectorIndexRef")
            if self.urls:
                raise ValueError(f"tool {self.name}: urls apply to http tools")
        else:
            if not self.urls:
                raise ValueError(f"tool {self.name}: an http tool needs urls")
            if self.vectorIndexRef or self.topK:
                raise ValueError(f"tool {self.name}: vectorIndexRef and topK apply to retrieval tools")
            for url in self.urls:
                if not re.match(r"^https?://[^\s/?#]+", url) or len(url) > 2048:
                    raise ValueError(f"tool {self.name}: {url!r} is not an absolute http(s) URL")
        return self


class CreateAgent(BaseModel):
    model_config = ConfigDict(extra="forbid")

    name: str = Field(min_length=1, max_length=NAME_MAX, pattern=NAME_PATTERN)
    model: str = Field(min_length=1, max_length=63, pattern=r"^[A-Za-z0-9._-]+$")
    systemPrompt: str = Field(default="", max_length=16000)
    tools: List[Tool] = Field(default_factory=list, max_length=16)
    maxSteps: int = Field(default=0, ge=0, le=20)
    replicas: Optional[int] = Field(default=None, ge=0, le=20)
    image: str = Field(default="", max_length=512, pattern=r"^\S*$")
    cpu: str = Field(default="", max_length=16, pattern=_QUANTITY)
    memory: str = Field(default="", max_length=16, pattern=_QUANTITY)

    @model_validator(mode="after")
    def _unique_tools(self) -> "CreateAgent":
        names = [t.name for t in self.tools]
        if len(names) != len(set(names)):
            raise ValueError("tool names must be unique")
        return self


class ChatMessage(BaseModel):
    model_config = ConfigDict(extra="forbid")
    role: Literal["system", "user", "assistant"]
    content: str = Field(max_length=32000)


class ChatRequest(BaseModel):
    model_config = ConfigDict(extra="forbid")
    messages: List[ChatMessage] = Field(min_length=1, max_length=100)


class Scale(BaseModel):
    model_config = ConfigDict(extra="forbid")
    replicas: int = Field(ge=0, le=20)


def build_tool(t: Tool) -> Dict[str, Any]:
    out: Dict[str, Any] = {"name": t.name, "type": t.type}
    if t.description:
        out["description"] = t.description
    if t.type == "retrieval":
        out["retrieval"] = prune({"vectorIndexRef": t.vectorIndexRef, "topK": t.topK or None})
    else:
        out["http"] = {"urls": t.urls, "method": t.method}
    return out


def build_spec(body: CreateAgent) -> Dict[str, Any]:
    spec: Dict[str, Any] = {"model": body.model}
    if body.systemPrompt:
        spec["systemPrompt"] = body.systemPrompt
    if body.tools:
        spec["tools"] = [build_tool(t) for t in body.tools]
    if body.maxSteps:
        spec["maxSteps"] = body.maxSteps
    if body.replicas is not None:
        spec["replicas"] = body.replicas
    if body.image:
        spec["image"] = body.image
    requests = prune({"cpu": body.cpu or None, "memory": body.memory or None})
    if requests:
        spec["resources"] = {"requests": requests}
    return spec


def tool_to_ui(t: Dict[str, Any]) -> Dict[str, Any]:
    retrieval, http = t.get("retrieval") or {}, t.get("http") or {}
    return prune({
        "name": t.get("name"),
        "description": t.get("description"),
        "type": t.get("type"),
        "vectorIndexRef": retrieval.get("vectorIndexRef"),
        "topK": retrieval.get("topK"),
        "urls": http.get("urls"),
        "method": http.get("method"),
    })


def to_ui(obj: Dict[str, Any]) -> Dict[str, Any]:
    spec, st = obj.get("spec") or {}, obj.get("status") or {}
    return {
        "metadata": meta(obj),
        "spec": prune({
            "model": spec.get("model"),
            "systemPrompt": spec.get("systemPrompt"),
            "tools": [tool_to_ui(t) for t in spec.get("tools") or []],
            "maxSteps": spec.get("maxSteps"),
            "replicas": spec.get("replicas"),
            "image": spec.get("image"),
        }),
        "status": prune({
            "phase": st.get("phase"),
            "message": st.get("message"),
            "endpoint": st.get("endpoint"),
            "readyReplicas": st.get("readyReplicas"),
            "keySecret": st.get("keySecret"),
        }),
    }


_LABEL = re.compile(r"[a-z0-9]([-a-z0-9]{0,61}[a-z0-9])?")


def chat_url(obj: Dict[str, Any]) -> str:
    """The runtime's chat URL: the status endpoint when it is the agent namespace's in-cluster service, else derived.

    Only the stored object is used, never the request path, and both host labels must be DNS labels.
    """
    md = obj.get("metadata") or {}
    name, namespace = md.get("name") or "", md.get("namespace") or ""
    service = f"{name}-agent"
    if not (_LABEL.fullmatch(service) and _LABEL.fullmatch(namespace)):
        raise HTTPException(status_code=409, detail="agent has no valid in-cluster service name")
    endpoint = ((obj.get("status") or {}).get("endpoint") or "").rstrip("/")
    m = re.fullmatch(r"http://([^./:]+)\.([^./:]+)\.svc\.cluster\.local:(\d+)", endpoint)
    if m and _LABEL.fullmatch(m.group(1)) and m.group(2) == namespace and m.group(3) == str(AGENT_PORT):
        service = m.group(1)
    return f"http://{service}.{namespace}.svc.cluster.local:{AGENT_PORT}/v1/chat/completions"


async def _post(url: str, body: Dict[str, Any]) -> Any:
    async with httpx.AsyncClient(timeout=CHAT_TIMEOUT_SECONDS, follow_redirects=False, trust_env=False) as client:
        resp = await client.post(url, json=body)
    try:
        data = resp.json()
    except ValueError:
        data = {"error": {"message": resp.text[:300]}}
    return resp.status_code, data


def build_router(deps: Deps) -> APIRouter:
    router = APIRouter()

    @router.get("/api/agents")
    @deps.limiter.limit("30/minute")
    async def list_agents(request: Request, _=Depends(deps.verify_auth)):
        return {"items": [to_ui(o) for o in await list_all(request, deps, PLURAL)]}

    @router.get("/api/agents/{name}")
    @deps.limiter.limit("30/minute")
    async def get_agent(request: Request, name: str, _=Depends(deps.verify_auth)):
        obj, _ns = await find_one(request, deps, PLURAL, name)
        return to_ui(obj)

    @router.post("/api/agents", status_code=201)
    @deps.limiter.limit("10/minute")
    async def create_agent(request: Request, body: CreateAgent, _=Depends(deps.verify_auth)):
        ns = namespaces(request, deps)[0]
        return to_ui(await create_item(deps, PLURAL, KIND, body.name, build_spec(body), namespace=ns))

    @router.post("/api/agents/{name}/scale")
    @deps.limiter.limit("10/minute")
    async def scale_agent(request: Request, name: str, body: Scale, _=Depends(deps.verify_auth)):
        _obj, ns = await find_one(request, deps, PLURAL, name)
        await patch_item(deps, PLURAL, name, {"spec": {"replicas": body.replicas}}, namespace=ns)
        return {"status": "scaled", "name": name, "replicas": body.replicas}

    @router.post("/api/agents/{name}/chat")
    @deps.limiter.limit("30/minute")
    async def chat(request: Request, name: str, body: ChatRequest, _=Depends(deps.verify_auth)):
        obj, _ns = await find_one(request, deps, PLURAL, name)
        phase = (obj.get("status") or {}).get("phase")
        if phase != "Ready":
            raise HTTPException(status_code=409, detail=f"agent {name} is not ready (phase {phase or 'unknown'})")
        url = chat_url(obj)
        payload = {"messages": [m.model_dump() for m in body.messages]}
        try:
            status, data = await (deps.agent_chat or _post)(url, payload)
        except httpx.HTTPError as exc:
            raise HTTPException(status_code=502, detail=f"agent {name} is unreachable: {type(exc).__name__}")
        if status != 200:
            err = (data or {}).get("error") if isinstance(data, dict) else None
            msg = err.get("message") if isinstance(err, dict) else str(err or "")
            code = status if status in (400, 401, 404, 429, 503) else 502
            raise HTTPException(status_code=code, detail=f"agent {name} answered {status}: {msg}"[:500])
        return data

    @router.delete("/api/agents/{name}")
    @deps.limiter.limit("10/minute")
    async def delete_agent(request: Request, name: str, _=Depends(deps.verify_auth)):
        _obj, ns = await find_one(request, deps, PLURAL, name)
        await delete_item(deps, PLURAL, name, namespace=ns)
        return {"status": "deleted", "name": name}

    return router
