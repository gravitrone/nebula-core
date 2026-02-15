"""File API routes."""

# Standard Library
from pathlib import Path
from typing import Any
from uuid import UUID

# Third-Party
from fastapi import APIRouter, Depends, Query, Request
from pydantic import BaseModel

# Local
from nebula_api.auth import maybe_check_agent_approval, require_auth
from nebula_api.response import api_error, success
from nebula_mcp.enums import EnumRegistry
from nebula_mcp.executors import execute_create_file, execute_update_file
from nebula_mcp.query_loader import QueryLoader

QUERIES = QueryLoader(Path(__file__).resolve().parents[2] / "queries")

router = APIRouter()
ADMIN_SCOPE_NAMES = {"vault-only", "sensitive"}


def _is_admin(auth: dict, enums: EnumRegistry) -> bool:
    scope_ids = set(auth.get("scopes", []))
    allowed_ids = {
        enums.scopes.name_to_id.get(name)
        for name in ADMIN_SCOPE_NAMES
        if enums.scopes.name_to_id.get(name)
    }
    return bool(scope_ids.intersection(allowed_ids))


async def _file_visible(
    pool: Any, enums: EnumRegistry, auth: dict, file_id: str
) -> bool:
    file_id = str(file_id)
    if auth["caller_type"] != "agent":
        return True
    if _is_admin(auth, enums):
        return True
    scope_ids = auth.get("scopes", []) or []
    all_rows = await pool.fetch(
        QUERIES["relationships/get"], "file", file_id, "both", None, None
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


class CreateFileBody(BaseModel):
    """Payload for creating a file entry."""

    filename: str
    file_path: str
    mime_type: str | None = None
    size_bytes: int | None = None
    checksum: str | None = None
    status: str = "active"
    tags: list[str] = []
    metadata: dict | None = None


class UpdateFileBody(BaseModel):
    """Payload for updating a file entry."""

    filename: str | None = None
    file_path: str | None = None
    mime_type: str | None = None
    size_bytes: int | None = None
    checksum: str | None = None
    status: str | None = None
    tags: list[str] | None = None
    metadata: dict | None = None


@router.get("/")
async def list_files(
    request: Request,
    auth: dict = Depends(require_auth),
    tags: list[str] = Query(default_factory=list),
    mime_type: str | None = None,
    status_category: str = "active",
    limit: int = Query(50, le=500),
    offset: int = 0,
) -> dict[str, Any]:
    """List files with optional filters."""

    pool = request.app.state.pool
    enums = request.app.state.enums
    rows = await pool.fetch(
        QUERIES["files/list"],
        tags or None,
        mime_type,
        status_category,
        limit,
        offset,
    )
    if auth["caller_type"] != "agent" or _is_admin(auth, enums):
        return success([dict(r) for r in rows])
    results = []
    for row in rows:
        if not await _file_visible(pool, enums, auth, row["id"]):
            continue
        results.append(dict(row))
    return success(results)


@router.get("/{file_id}")
async def get_file(
    file_id: str,
    request: Request,
    auth: dict = Depends(require_auth),
) -> dict[str, Any]:
    """Fetch a file by id."""

    pool = request.app.state.pool
    enums = request.app.state.enums

    try:
        UUID(file_id)
    except ValueError:
        api_error("INVALID_INPUT", "Invalid file id", 400)

    row = await pool.fetchrow(QUERIES["files/get"], file_id)
    if not row:
        api_error("NOT_FOUND", f"File '{file_id}' not found", 404)
    if auth["caller_type"] == "agent" and not await _file_visible(
        pool, enums, auth, file_id
    ):
        api_error("FORBIDDEN", "File not in your scopes", 403)
    return success(dict(row))


@router.post("/")
async def create_file(
    payload: CreateFileBody,
    request: Request,
    auth: dict = Depends(require_auth),
) -> dict[str, Any]:
    """Create a file entry."""

    pool = request.app.state.pool
    enums = request.app.state.enums
    data = payload.model_dump()
    if data.get("metadata") is None:
        data["metadata"] = {}

    if resp := await maybe_check_agent_approval(pool, auth, "create_file", data):
        return resp

    result = await execute_create_file(pool, enums, data)
    return success(result)


@router.patch("/{file_id}")
async def update_file(
    file_id: str,
    payload: UpdateFileBody,
    request: Request,
    auth: dict = Depends(require_auth),
) -> dict[str, Any]:
    """Update a file entry."""

    pool = request.app.state.pool
    enums = request.app.state.enums

    try:
        UUID(file_id)
    except ValueError:
        api_error("INVALID_INPUT", "Invalid file id", 400)

    data = payload.model_dump()
    data["file_id"] = file_id

    if auth["caller_type"] == "agent" and not await _file_visible(
        pool, enums, auth, file_id
    ):
        api_error("FORBIDDEN", "Access denied", 403)

    if resp := await maybe_check_agent_approval(pool, auth, "update_file", data):
        return resp

    result = await execute_update_file(pool, enums, data)
    if not result:
        api_error("NOT_FOUND", f"File '{file_id}' not found", 404)
    return success(result)
