"""Model registry routes (GryviaModelRegistry, namespaced) for the dashboard."""
from typing import Any, Dict, List, Literal, Optional

from fastapi import APIRouter, Depends, HTTPException, Request
from pydantic import BaseModel, ConfigDict, Field, model_validator

from .common import Deps, create_item, patch_item
from .uiutil import NAME_MAX, NAME_PATTERN, find_one, list_all, meta, namespaces, prune

PLURAL = "gryviamodelregistries"
KIND = "GryviaModelRegistry"

# Same promotion ladder the UI offers.
NEXT_STAGE = {"dev": ["staging"], "staging": ["production"], "production": ["archived"]}


class PromoteRequest(BaseModel):
    model_config = ConfigDict(extra="forbid")

    targetStage: Literal["dev", "staging", "production", "archived"]


class Artifacts(BaseModel):
    model_config = ConfigDict(extra="forbid")

    s3Path: str = Field(default="", max_length=1024, pattern=r"^([a-z0-9]+://[^\s]+)?$")
    pvcName: str = Field(default="", max_length=NAME_MAX, pattern=r"^(" + NAME_PATTERN[1:-1] + r")?$")
    subPath: str = Field(default="", max_length=512, pattern=r"^([A-Za-z0-9._/-]*)?$")
    format: str = Field(default="", max_length=64, pattern=r"^([A-Za-z0-9._-]*)?$")

    @model_validator(mode="after")
    def _check(self):
        if not (self.s3Path or self.pvcName):
            raise ValueError("artifacts needs an s3Path or a pvcName")
        if ".." in self.subPath.split("/"):
            raise ValueError("artifacts.subPath must not contain '..'")
        return self


class ServingConfig(BaseModel):
    model_config = ConfigDict(extra="forbid")

    backend: Literal["triton", "vllm", "tensorrt-llm", "torchserve"] = "vllm"
    replicas: int = Field(default=1, ge=1, le=100)
    gpuCount: int = Field(default=0, ge=0, le=64)
    gpuType: str = Field(default="", max_length=63, pattern=r"^([A-Za-z0-9][A-Za-z0-9._-]*)?$")


class CreateModel(BaseModel):
    """Register a model version. ``modelName`` defaults to ``name``.

    The registry entry is metadata; only ``autoServe`` on a ``production`` entry makes the operator create a
    serving GryviaInferenceService for it.
    """
    model_config = ConfigDict(extra="forbid")

    name: str = Field(min_length=1, max_length=NAME_MAX, pattern=NAME_PATTERN)
    modelName: Optional[str] = Field(default=None, min_length=1, max_length=128,
                                     pattern=r"^[A-Za-z0-9][A-Za-z0-9._-]*$")
    version: str = Field(min_length=1, max_length=63, pattern=r"^[A-Za-z0-9][A-Za-z0-9._+-]*$")
    stage: Literal["dev", "staging", "production"] = "dev"
    description: str = Field(default="", max_length=1024)
    sourceJob: str = Field(default="", max_length=NAME_MAX, pattern=r"^(" + NAME_PATTERN[1:-1] + r")?$")
    artifacts: Artifacts
    autoServe: bool = False
    servingConfig: Optional[ServingConfig] = None


def _artifacts(art: Dict[str, Any]) -> List[str]:
    out: List[str] = []
    if art.get("s3Path"):
        out.append(art["s3Path"])
    if art.get("pvcName"):
        out.append(f"pvc://{art['pvcName']}/{art.get('subPath', '').lstrip('/')}".rstrip("/"))
    return out


def to_ui(obj: Dict[str, Any]) -> Dict[str, Any]:
    spec, st = obj.get("spec") or {}, obj.get("status") or {}
    src = spec.get("source") or {}
    return {
        "metadata": meta(obj),
        "spec": prune({
            "version": spec.get("version"),
            "stage": spec.get("stage"),
            "sourceJob": src.get("jobRef") or None,
            "artifacts": _artifacts(spec.get("artifacts") or {}),
        }),
        "status": prune({"servingEndpoint": st.get("servingEndpoint") or None}),
    }


def build_router(deps: Deps) -> APIRouter:
    router = APIRouter()

    @router.get("/api/models")
    @deps.limiter.limit("30/minute")
    async def list_models(request: Request, _=Depends(deps.verify_auth)):
        return {"items": [to_ui(o) for o in await list_all(request, deps, PLURAL)]}

    @router.get("/api/models/{name}")
    @deps.limiter.limit("30/minute")
    async def get_model(request: Request, name: str, _=Depends(deps.verify_auth)):
        obj, _ns = await find_one(request, deps, PLURAL, name)
        return to_ui(obj)

    @router.post("/api/models", status_code=201)
    @deps.limiter.limit("10/minute")
    async def create_model(request: Request, body: CreateModel, _=Depends(deps.verify_auth)):
        arts = {k: v for k, v in body.artifacts.model_dump().items() if v}
        spec: Dict[str, Any] = {
            "modelName": body.modelName or body.name,
            "version": body.version,
            "stage": body.stage,
            "artifacts": arts,
        }
        if body.description:
            spec["description"] = body.description
        if body.sourceJob:
            spec["source"] = {"jobRef": body.sourceJob}
        if body.autoServe:
            spec["autoServe"] = True
        if body.servingConfig:
            sc = {k: v for k, v in body.servingConfig.model_dump().items() if v not in ("", 0)}
            sc["backend"] = body.servingConfig.backend
            spec["servingConfig"] = sc
        ns = namespaces(request, deps)[0]
        return to_ui(await create_item(deps, PLURAL, KIND, body.name, spec, namespace=ns))

    @router.post("/api/models/{name}/promote")
    @deps.limiter.limit("10/minute")
    async def promote_model(request: Request, name: str, body: PromoteRequest, _=Depends(deps.verify_auth)):
        obj, ns = await find_one(request, deps, PLURAL, name)
        current = (obj.get("spec") or {}).get("stage") or "dev"
        if body.targetStage not in NEXT_STAGE.get(current, []):
            raise HTTPException(status_code=409,
                                detail=f"Cannot promote model from '{current}' to '{body.targetStage}'")
        updated = await patch_item(deps, PLURAL, name, {"spec": {"stage": body.targetStage}}, namespace=ns)
        return to_ui(updated)

    return router
