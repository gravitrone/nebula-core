"""Export API routes."""

import csv
import io
import json
from pathlib import Path
from typing import Any

from fastapi import APIRouter, Depends, Query, Request

from nebula_api.auth import require_auth
from nebula_api.response import api_error, success
from nebula_mcp.enums import require_entity_type, require_scopes
from nebula_mcp.helpers import (
    enforce_scope_subset,
    sanitize_relationship_properties,
    scope_names_from_ids,
)
from nebula_mcp.query_loader import QueryLoader
from nebula_mcp.schema import load_export_schema_contract

QUERIES = QueryLoader(Path(__file__).resolve().parents[2] / "queries")

router = APIRouter()
ADMIN_SCOPE_NAMES = {"admin"}


def _is_admin(auth: dict, enums: Any) -> bool:
    """Handle is admin.

    Args:
        auth: Input parameter for _is_admin.
        enums: Input parameter for _is_admin.

    Returns:
        Result value from the operation.
    """

    scope_ids = set(auth.get("scopes", []))
    allowed_ids = {
        enums.scopes.name_to_id.get(name)
        for name in ADMIN_SCOPE_NAMES
        if enums.scopes.name_to_id.get(name)
    }
    return bool(scope_ids.intersection(allowed_ids))


def _resolve_scope_ids(scopes: list[str], auth: dict, enums: Any) -> list:
    """Handle resolve scope ids.

    Args:
        scopes: Input parameter for _resolve_scope_ids.
        auth: Input parameter for _resolve_scope_ids.
        enums: Input parameter for _resolve_scope_ids.

    Returns:
        Result value from the operation.
    """

    caller_scope_ids = auth.get("scopes", []) or []
    if not scopes:
        return caller_scope_ids
    caller_scope_names = scope_names_from_ids(caller_scope_ids, enums)
    try:
        allowed_scope_names = enforce_scope_subset(scopes, caller_scope_names)
        return require_scopes(allowed_scope_names, enums)
    except ValueError as exc:
        api_error("VALIDATION_ERROR", str(exc), 400)


def _visible_scope_names(auth: dict, enums: Any, scope_ids: list | None) -> list[str]:
    """Handle visible scope names.

    Args:
        auth: Input parameter for _visible_scope_names.
        enums: Input parameter for _visible_scope_names.
        scope_ids: Input parameter for _visible_scope_names.

    Returns:
        Result value from the operation.
    """

    if _is_admin(auth, enums):
        return sorted(enums.scopes.name_to_id.keys())
    return scope_names_from_ids(scope_ids or [], enums)


def _normalize_relationship_export_row(row: Any, scope_names: list[str]) -> dict[str, Any]:
    """Handle normalize relationship export row.

    Args:
        row: Input parameter for _normalize_relationship_export_row.
        scope_names: Input parameter for _normalize_relationship_export_row.

    Returns:
        Result value from the operation.
    """

    item = dict(row)
    item["properties"] = sanitize_relationship_properties(item.get("properties"), scope_names)
    return item


async def _job_visible(pool: Any, auth: dict, enums: Any, job_id: str) -> bool:
    """Handle job visible.

    Args:
        pool: Input parameter for _job_visible.
        auth: Input parameter for _job_visible.
        enums: Input parameter for _job_visible.
        job_id: Input parameter for _job_visible.

    Returns:
        Result value from the operation.
    """

    if _is_admin(auth, enums):
        return True
    row = await pool.fetchrow(QUERIES["jobs/get"], job_id)
    if not row:
        return False
    job_scopes = row.get("privacy_scope_ids") or []
    caller_scopes = auth.get("scopes", []) or []
    if not job_scopes:
        return True
    return any(scope in caller_scopes for scope in job_scopes)


def _flatten_value(value: Any) -> str:
    """Flatten complex values for CSV export.

    Args:
        value: Value to serialize.

    Returns:
        String representation for CSV output.
    """

    if value is None:
        return ""
    if isinstance(value, list):
        return ",".join(str(v) for v in value)
    if isinstance(value, dict):
        return json.dumps(value)
    return str(value)


def _to_csv(rows: list[dict[str, Any]], field_order: list[str] | None = None) -> str:
    """Convert rows to CSV string.

    Args:
        rows: List of row dictionaries.
        field_order: Optional list of field names for column order.

    Returns:
        CSV string content.
    """

    if not rows:
        return ""
    if not field_order:
        field_order = list(rows[0].keys())
    buffer = io.StringIO()
    writer = csv.DictWriter(buffer, fieldnames=field_order)
    writer.writeheader()
    for row in rows:
        writer.writerow({k: _flatten_value(row.get(k)) for k in field_order})
    return buffer.getvalue()


def _export_response(rows: list[dict[str, Any]], fmt: str) -> dict[str, Any]:
    """Build a standardized export response.

    Args:
        rows: List of row dictionaries.
        fmt: Export format, json or csv.

    Returns:
        API response payload.
    """

    fmt = (fmt or "json").lower()
    if fmt not in {"json", "csv"}:
        api_error("VALIDATION_ERROR", "Format must be json or csv", 400)
    if fmt == "csv":
        return success({"format": "csv", "content": _to_csv(rows), "count": len(rows)})
    return success({"format": "json", "items": rows, "count": len(rows)})


@router.get("/schema")
async def export_schema(
    request: Request,
    auth: dict = Depends(require_auth),
) -> dict[str, Any]:
    """Return JSON schema contract for export resources."""

    _ = request
    _ = auth
    return success(load_export_schema_contract())


@router.get("/entities")
async def export_entities(
    request: Request,
    auth: dict = Depends(require_auth),
    format: str = "json",
    type: str | None = None,
    tags: list[str] = Query(default_factory=list),
    search_text: str | None = None,
    status_category: str = "active",
    scopes: list[str] = Query(default_factory=list),
    limit: int = Query(500, le=2000),
    offset: int = 0,
) -> dict[str, Any]:
    """Export entities as JSON or CSV.

    Args:
        request: FastAPI request.
        auth: Auth context.
        format: Response format.
        type: Entity type filter.
        tags: Tag filters.
        search_text: Full-text search filter.
        status_category: Status category filter.
        scopes: Privacy scope filters.
        limit: Max rows.
        offset: Offset for pagination.

    Returns:
        Export response payload.
    """

    pool = request.app.state.pool
    enums = request.app.state.enums
    scope_ids = auth.get("scopes", [])
    enums = request.app.state.enums

    try:
        type_id = require_entity_type(type, enums) if type else None
    except ValueError as exc:
        api_error("VALIDATION_ERROR", str(exc), 400)
    scope_ids = _resolve_scope_ids(scopes, auth, enums)

    rows = await pool.fetch(
        QUERIES["entities/query"],
        type_id,
        tags or None,
        search_text,
        status_category,
        scope_ids,
        limit,
        offset,
    )
    results = [dict(row) for row in rows]
    return _export_response(results, format)


@router.get("/context")
async def export_context(
    request: Request,
    auth: dict = Depends(require_auth),
    format: str = "json",
    source_type: str | None = None,
    tags: list[str] = Query(default_factory=list),
    search_text: str | None = None,
    scopes: list[str] = Query(default_factory=list),
    limit: int = Query(500, le=2000),
    offset: int = 0,
) -> dict[str, Any]:
    """Export context items as JSON or CSV.

    Args:
        request: FastAPI request.
        auth: Auth context.
        format: Response format.
        source_type: Context source type filter.
        tags: Tag filters.
        search_text: Full-text search filter.
        scopes: Privacy scope filters.
        limit: Max rows.
        offset: Offset for pagination.

    Returns:
        Export response payload.
    """

    pool = request.app.state.pool
    enums = request.app.state.enums

    scope_ids = _resolve_scope_ids(scopes, auth, enums)

    rows = await pool.fetch(
        QUERIES["context/query"],
        source_type,
        tags or None,
        search_text,
        scope_ids,
        limit,
        offset,
    )
    results = [dict(row) for row in rows]
    return _export_response(results, format)


@router.get("/relationships")
async def export_relationships(
    request: Request,
    auth: dict = Depends(require_auth),
    format: str = "json",
    source_type: str | None = None,
    target_type: str | None = None,
    relationship_types: list[str] = Query(default_factory=list),
    status_category: str = "active",
    limit: int = Query(500, le=2000),
) -> dict[str, Any]:
    """Export relationships as JSON or CSV.

    Args:
        request: FastAPI request.
        auth: Auth context.
        format: Response format.
        source_type: Source node type filter.
        target_type: Target node type filter.
        relationship_types: Relationship type filters.
        status_category: Status category filter.
        limit: Max rows.

    Returns:
        Export response payload.
    """

    pool = request.app.state.pool
    enums = request.app.state.enums
    scope_ids = None if _is_admin(auth, enums) else (auth.get("scopes", []) or [])
    scope_names = _visible_scope_names(auth, enums, scope_ids)

    rows = await pool.fetch(
        QUERIES["relationships/query"],
        source_type,
        target_type,
        relationship_types or None,
        status_category,
        limit,
        scope_ids,
    )
    results = []
    for row in rows:
        if not _is_admin(auth, enums):
            if row["source_type"] == "job":
                if not await _job_visible(pool, auth, enums, row["source_id"]):
                    continue
            if row["target_type"] == "job":
                if not await _job_visible(pool, auth, enums, row["target_id"]):
                    continue
        results.append(_normalize_relationship_export_row(row, scope_names))
    return _export_response(results, format)


@router.get("/jobs")
async def export_jobs(
    request: Request,
    auth: dict = Depends(require_auth),
    format: str = "json",
    status_names: list[str] = Query(default_factory=list),
    assigned_to: str | None = None,
    agent_id: str | None = None,
    priority: str | None = None,
    due_before: str | None = None,
    due_after: str | None = None,
    overdue: bool = False,
    parent_job_id: str | None = None,
    limit: int = Query(500, le=2000),
) -> dict[str, Any]:
    """Export jobs as JSON or CSV.

    Args:
        request: FastAPI request.
        auth: Auth context.
        format: Response format.
        status_names: Status filters.
        assigned_to: Assignee filter.
        agent_id: Agent filter.
        priority: Priority filter.
        due_before: Due date upper bound.
        due_after: Due date lower bound.
        overdue: Overdue filter.
        parent_job_id: Parent job filter.
        limit: Max rows.

    Returns:
        Export response payload.
    """

    pool = request.app.state.pool
    enums = request.app.state.enums
    agent_filter = agent_id
    scope_filter = None if _is_admin(auth, enums) else (auth.get("scopes", []) or [])

    rows = await pool.fetch(
        QUERIES["jobs/query"],
        status_names or None,
        assigned_to,
        agent_filter,
        priority,
        due_before,
        due_after,
        overdue,
        parent_job_id,
        scope_filter,
        limit,
    )
    return _export_response([dict(r) for r in rows], format)


@router.get("/snapshot")
async def export_snapshot(
    request: Request,
    auth: dict = Depends(require_auth),
    format: str = "json",
    limit: int = Query(500, le=2000),
    offset: int = 0,
) -> dict[str, Any]:
    """Export a full workspace snapshot as JSON only.

    Args:
        request: FastAPI request.
        auth: Auth context.
        format: Response format, json only.
        limit: Max rows per table.
        offset: Offset for pagination.

    Returns:
        Export response payload.
    """

    if format.lower() != "json":
        api_error("VALIDATION_ERROR", "Snapshot export supports json only", 400)

    pool = request.app.state.pool
    enums = request.app.state.enums
    scope_ids = auth.get("scopes", []) or []
    scope_filter = None if _is_admin(auth, enums) else scope_ids

    entities_rows = await pool.fetch(
        QUERIES["entities/query"],
        None,
        None,
        None,
        "active",
        scope_ids,
        limit,
        offset,
    )
    context_rows = await pool.fetch(
        QUERIES["context/query"],
        None,
        None,
        None,
        scope_ids,
        limit,
        offset,
    )
    relationships_rows = await pool.fetch(
        QUERIES["relationships/query"],
        None,
        None,
        None,
        "active",
        limit,
        scope_filter,
    )
    job_agent_filter = None
    scope_filter = None if _is_admin(auth, enums) else (scope_ids or [])
    jobs_rows = await pool.fetch(
        QUERIES["jobs/query"],
        None,
        None,
        job_agent_filter,
        None,
        None,
        None,
        False,
        None,
        scope_filter,
        limit,
    )

    scope_names = _visible_scope_names(auth, enums, scope_ids)
    entities = [dict(row) for row in entities_rows]
    context = [dict(row) for row in context_rows]
    relationships = []
    for row in relationships_rows:
        if not _is_admin(auth, enums):
            if row["source_type"] == "job":
                if not await _job_visible(pool, auth, enums, row["source_id"]):
                    continue
            if row["target_type"] == "job":
                if not await _job_visible(pool, auth, enums, row["target_id"]):
                    continue
        relationships.append(_normalize_relationship_export_row(row, scope_names))

    return success(
        {
            "format": "json",
            "entities": entities,
            "context": context,
            "relationships": relationships,
            "jobs": [dict(r) for r in jobs_rows],
        }
    )
