"""Log API routes."""

# Standard Library
from datetime import datetime
from pathlib import Path
from typing import Any
from uuid import UUID

# Third-Party
from fastapi import APIRouter, Depends, Query, Request
from pydantic import BaseModel

# Local
from nebula_api.auth import maybe_check_agent_approval, require_auth
from nebula_api.response import api_error, success
from nebula_mcp.enums import require_log_type, require_status
from nebula_mcp.executors import execute_create_log, execute_update_log
from nebula_mcp.query_loader import QueryLoader

QUERIES = QueryLoader(Path(__file__).resolve().parents[2] / "queries")

router = APIRouter()
ADMIN_SCOPE_NAMES = {"admin"}


def _is_admin(auth: dict, enums: Any) -> bool:
    scope_ids = set(auth.get("scopes", []))
    allowed_ids = {
        enums.scopes.name_to_id.get(name)
        for name in ADMIN_SCOPE_NAMES
        if enums.scopes.name_to_id.get(name)
    }
    return bool(scope_ids.intersection(allowed_ids))


async def _log_visible(pool: Any, enums: Any, auth: dict, log_id: str) -> bool:
    log_id = str(log_id)
    if auth["caller_type"] != "agent":
        return True
    if _is_admin(auth, enums):
        return True
    scope_ids = auth.get("scopes", []) or []
    all_rows = await pool.fetch(
        QUERIES["relationships/get"], "log", log_id, "both", None, None
    )
    if not all_rows:
        return True
    for rel in all_rows:
        for side in ("source", "target"):
            rel_type = rel[f"{side}_type"]
            rel_id = rel[f"{side}_id"]
            if rel_type == "entity":
                row = await pool.fetchrow(QUERIES["entities/get_by_id"], rel_id)
                if not row:
                    return False
                scopes = row.get("privacy_scope_ids") or []
                if scopes and not any(s in scope_ids for s in scopes):
                    return False
            if rel_type == "knowledge":
                row = await pool.fetchrow(QUERIES["knowledge/get"], rel_id, None)
                if not row:
                    return False
                scopes = row.get("privacy_scope_ids") or []
                if scopes and not any(s in scope_ids for s in scopes):
                    return False
            if rel_type == "job":
                job_row = await pool.fetchrow(QUERIES["jobs/get"], rel_id)
                if not job_row:
                    return False
                scopes = job_row.get("privacy_scope_ids") or []
                if scopes and not any(s in scope_ids for s in scopes):
                    return False
    return True


class CreateLogBody(BaseModel):
    """Payload for creating a log entry.

    Attributes:
        log_type: Log type name.
        timestamp: Timestamp for the log entry.
        value: Log value payload.
        status: Status name.
        tags: Optional tag list.
        metadata: Optional metadata payload.
    """

    log_type: str
    timestamp: datetime | None = None
    value: dict | None = None
    status: str = "active"
    tags: list[str] = []
    metadata: dict | None = None


class UpdateLogBody(BaseModel):
    """Payload for updating a log entry.

    Attributes:
        log_type: Updated log type name.
        timestamp: Updated timestamp.
        value: Updated value payload.
        status: Updated status name.
        tags: Updated tags.
        metadata: Updated metadata.
    """

    log_type: str | None = None
    timestamp: datetime | None = None
    value: dict | None = None
    status: str | None = None
    tags: list[str] | None = None
    metadata: dict | None = None


@router.post("/")
async def create_log(
    payload: CreateLogBody,
    request: Request,
    auth: dict = Depends(require_auth),
) -> dict[str, Any]:
    """Create a log entry."""

    pool = request.app.state.pool
    enums = request.app.state.enums
    data = payload.model_dump()
    if data.get("value") is None:
        data["value"] = {}
    if data.get("metadata") is None:
        data["metadata"] = {}

    # Validate taxonomy-backed fields before queuing approvals.
    try:
        require_log_type(data["log_type"], enums)
        require_status(data["status"], enums)
    except ValueError as exc:
        api_error("INVALID_INPUT", str(exc), 400)
    if resp := await maybe_check_agent_approval(pool, auth, "create_log", data):
        return resp

    try:
        result = await execute_create_log(pool, enums, data)
    except ValueError as exc:
        api_error("INVALID_INPUT", str(exc), 400)
    return success(result)


@router.get("/{log_id}")
async def get_log(
    log_id: str,
    request: Request,
    auth: dict = Depends(require_auth),
) -> dict[str, Any]:
    """Fetch a log entry by id."""

    pool = request.app.state.pool
    enums = request.app.state.enums

    try:
        UUID(log_id)
    except ValueError:
        api_error("INVALID_INPUT", "Invalid log id", 400)

    row = await pool.fetchrow(QUERIES["logs/get"], log_id)
    if not row:
        api_error("NOT_FOUND", f"Log '{log_id}' not found", 404)
    if auth["caller_type"] == "agent" and not await _log_visible(
        pool, enums, auth, log_id
    ):
        api_error("FORBIDDEN", "Log not in your scopes", 403)
    return success(dict(row))


@router.get("/")
async def query_logs(
    request: Request,
    auth: dict = Depends(require_auth),
    log_type: str | None = None,
    tags: list[str] = Query(default_factory=list),
    status_category: str = "active",
    limit: int = Query(50, le=500),
    offset: int = 0,
) -> dict[str, Any]:
    """Query log entries with filters."""

    pool = request.app.state.pool
    enums = request.app.state.enums

    try:
        log_type_id = require_log_type(log_type, enums) if log_type else None
    except ValueError as exc:
        api_error("INVALID_INPUT", str(exc), 400)

    rows = await pool.fetch(
        QUERIES["logs/query"],
        log_type_id,
        tags or None,
        status_category,
        limit,
        offset,
    )
    if auth["caller_type"] != "agent" or _is_admin(auth, enums):
        return success([dict(r) for r in rows])
    results = []
    for row in rows:
        if not await _log_visible(pool, enums, auth, row["id"]):
            continue
        results.append(dict(row))
    return success(results)


@router.patch("/{log_id}")
async def update_log(
    log_id: str,
    payload: UpdateLogBody,
    request: Request,
    auth: dict = Depends(require_auth),
) -> dict[str, Any]:
    """Update a log entry."""

    pool = request.app.state.pool
    enums = request.app.state.enums

    try:
        UUID(log_id)
    except ValueError:
        api_error("INVALID_INPUT", "Invalid log id", 400)

    data = payload.model_dump()
    data["id"] = log_id

    if auth["caller_type"] == "agent" and not await _log_visible(
        pool, enums, auth, log_id
    ):
        api_error("FORBIDDEN", "Access denied", 403)

    # Validate taxonomy-backed fields before queuing approvals.
    try:
        if data.get("log_type"):
            require_log_type(str(data["log_type"]), enums)
        if data.get("status"):
            require_status(str(data["status"]), enums)
    except ValueError as exc:
        api_error("INVALID_INPUT", str(exc), 400)

    if resp := await maybe_check_agent_approval(pool, auth, "update_log", data):
        return resp

    if payload.log_type:
        data["log_type"] = payload.log_type

    try:
        result = await execute_update_log(pool, enums, data)
    except ValueError as exc:
        api_error("INVALID_INPUT", str(exc), 400)
    if not result:
        api_error("NOT_FOUND", f"Log '{log_id}' not found", 404)
    return success(result)
