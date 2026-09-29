"""Model registry routes (FabricModelRegistry, namespaced) for the dashboard."""
from typing import Any, Dict, List, Literal

from fastapi import APIRouter, Depends, HTTPException, Request
from pydantic import BaseModel, ConfigDict

from .common import Deps, patch_item
from .uiutil import find_one, list_all, meta, prune

PLURAL = "fabricmodelregistries"

# Same promotion ladder the UI offers.
NEXT_STAGE = {"dev": ["staging"], "staging": ["production"], "production": ["archived"]}


class PromoteRequest(BaseModel):
    model_config = ConfigDict(extra="forbid")

    targetStage: Literal["dev", "staging", "production", "archived"]


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
