"""On-demand monthly invoice estimates computed from GryviaUsageRecord objects.

Nothing is persisted and no payment is processed. Records are bucketed by the UTC month of ``spec.start``.
Admins see every tenant; a tenant user is forced to its own tenant (asking for another tenant is a 404).
"""
import calendar
import csv
import io
import re
from collections import OrderedDict
from datetime import datetime, timezone
from typing import Any, Dict, List, Literal, Optional

from fastapi import APIRouter, Depends, HTTPException, Query, Request
from fastapi.responses import Response

from .tenancy import is_admin
from .uiutil import namespaces, parse_ts
from .usage import _csv_cell, _currency, _num, _start_of, _tenant_of, fetch_records
from .common import Deps

CSV_COLUMNS = ["invoice", "tenant", "period_from", "period_to", "sku", "gpuType", "jobs", "gpuHours", "rate",
               "amount", "currency"]
NOTE = "Estimate from job run time; not a tax invoice."
_MONTH = re.compile(r"^(\d{4})-(0[1-9]|1[0-2])$")


def parse_month(value: Optional[str]) -> "tuple[int, int]":
    if not value:
        now = datetime.now(timezone.utc)
        return now.year, now.month
    m = _MONTH.match(value)
    if not m or int(m.group(1)) < 1970:
        raise HTTPException(status_code=400, detail="month must be YYYY-MM")
    return int(m.group(1)), int(m.group(2))


def in_month(rec: Dict[str, Any], year: int, month: int) -> bool:
    s = (rec.get("spec") or {}).get("start")
    start = parse_ts(s) if s else _start_of(rec)
    if start is None:
        return False
    u = start.astimezone(timezone.utc)
    return u.year == year and u.month == month


def build_invoice(tenant: str, year: int, month: int, records: List[Dict[str, Any]]) -> Dict[str, Any]:
    groups: "OrderedDict[str, Dict[str, Any]]" = OrderedDict()
    all_cur: set = set()
    jobs_all: set = set()
    open_ = False
    for r in records:
        spec, meta = r.get("spec") or {}, r.get("metadata") or {}
        key = spec.get("sku") or spec.get("gpuType") or "unknown"
        g = groups.setdefault(key, {"gpuType": spec.get("gpuType") or "", "hours": 0.0, "amount": 0.0,
                                    "rates": set(), "cur": set(), "jobs": set()})
        job = (meta.get("namespace"), spec.get("job") or meta.get("name"))
        cur = spec.get("currency") or "USD"
        g["hours"] += _num(spec.get("gpuHours"))
        g["amount"] += _num(spec.get("cost"))
        g["rates"].add(_num(spec.get("rate")))
        g["cur"].add(cur)
        g["jobs"].add(job)
        if not g["gpuType"]:
            g["gpuType"] = spec.get("gpuType") or ""
        all_cur.add(cur)
        jobs_all.add(job)
        if not spec.get("final", False):
            open_ = True
    lines = []
    for key in sorted(groups, key=lambda k: (-groups[k]["amount"], k)):
        g = groups[key]
        if len(g["rates"]) == 1:
            rate = next(iter(g["rates"]))
        else:
            rate = g["amount"] / g["hours"] if g["hours"] else 0.0
        lines.append({"sku": key, "gpuType": g["gpuType"], "gpuHours": round(g["hours"], 4),
                      "rate": round(rate, 4), "amount": round(g["amount"], 2), "jobs": len(g["jobs"]),
                      "currency": _currency(g["cur"])})
    last = calendar.monthrange(year, month)[1]
    currency = _currency(all_cur)
    inv: Dict[str, Any] = {
        "number": f"INV-{tenant}-{year:04d}{month:02d}", "tenant": tenant,
        "period": {"from": f"{year:04d}-{month:02d}-01", "to": f"{year:04d}-{month:02d}-{last:02d}"},
        "currency": currency, "status": "estimate",
        "generatedAt": datetime.now(timezone.utc).isoformat().replace("+00:00", "Z"),
        "lines": lines, "subtotal": round(sum(g["amount"] for g in groups.values()), 2),
        "jobs": len(jobs_all), "note": NOTE}
    if currency == "MIXED":
        inv["mixedCurrency"] = True
    if open_:
        inv["open"] = True
    return inv


def invoice_csv(inv: Dict[str, Any]) -> str:
    buf = io.StringIO()
    w = csv.writer(buf, lineterminator="\n")
    w.writerow(CSV_COLUMNS)
    p = inv["period"]

    def row(sku, gpu, jobs, hours, rate, amount, cur):
        w.writerow([_csv_cell(v) for v in (inv["number"], inv["tenant"], p["from"], p["to"], sku, gpu, jobs,
                                            hours, rate, amount, cur)])
    for ln in inv["lines"]:
        row(ln["sku"], ln["gpuType"], ln["jobs"], ln["gpuHours"], ln["rate"], ln["amount"], ln["currency"])
    row("TOTAL", "", inv["jobs"], round(sum(ln["gpuHours"] for ln in inv["lines"]), 4), "", inv["subtotal"],
        inv["currency"])
    return buf.getvalue()


def build_router(deps: Deps) -> APIRouter:
    router = APIRouter()

    async def _month_invoices(request: Request, year: int, month: int,
                              tenant: Optional[str]) -> List[Dict[str, Any]]:
        if is_admin(request):
            records = await fetch_records(deps.k8s_custom, None)
        else:
            tenant = None  # forced: only the caller's own namespaces are ever loaded
            records = await fetch_records(deps.k8s_custom, namespaces(request, deps))
        by_tenant: "OrderedDict[str, List[Dict[str, Any]]]" = OrderedDict()
        for r in records:
            if in_month(r, year, month):
                t = _tenant_of(r)
                if tenant and t != tenant:
                    continue
                by_tenant.setdefault(t, []).append(r)
        return [build_invoice(t, year, month, by_tenant[t]) for t in sorted(by_tenant)]

    @router.get("/api/invoices")
    @deps.limiter.limit("30/minute")
    async def list_invoices(request: Request, month: Optional[str] = Query(default=None, max_length=10),
                            tenant: Optional[str] = Query(default=None, max_length=63),
                            _=Depends(deps.verify_auth)):
        y, m = parse_month(month)
        return {"month": f"{y:04d}-{m:02d}", "items": await _month_invoices(request, y, m, tenant)}

    @router.get("/api/invoices/{tenant}/{month}")
    @deps.limiter.limit("30/minute")
    async def get_invoice(request: Request, tenant: str, month: str,
                          format: Literal["json", "csv"] = "json", _=Depends(deps.verify_auth)):
        y, m = parse_month(month)
        found = await _month_invoices(request, y, m, tenant)
        inv = next((i for i in found if i["tenant"] == tenant), None)
        if inv is None:
            raise HTTPException(status_code=404, detail="no usage for that tenant in that month")
        if format == "csv":
            return Response(invoice_csv(inv), media_type="text/csv", headers={
                "Content-Disposition": f'attachment; filename="{inv["number"]}.csv"'})
        return inv

    return router
