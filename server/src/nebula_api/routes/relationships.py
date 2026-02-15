"""Relationship API routes."""

# Standard Library
import json
from pathlib import Path
from typing import Any
from uuid import UUID

# Third-Party
from fastapi import APIRouter, Depends, Query, Request
from pydantic import BaseModel

# Local
from nebula_api.auth import maybe_check_agent_approval, require_auth
from nebula_api.response import api_error, success
from nebula_mcp.enums import EnumRegistry, require_relationship_type, require_status
from nebula_mcp.executors import execute_create_relationship
from nebula_mcp.query_loader import QueryLoader

QUERIES = QueryLoader(Path(__file__).resolve().parents[2] / "queries")

router = APIRouter()
ADMIN_SCOPE_NAMES = {"admin"}


def _is_admin(auth: dict, enums: EnumRegistry) -> bool:
    scope_ids = set(auth.get("scopes", []))
    allowed_ids = {
        enums.scopes.name_to_id.get(name)
        for name in ADMIN_SCOPE_NAMES
        if enums.scopes.name_to_id.get(name)
    }
    return bool(scope_ids.intersection(allowed_ids))


def _has_write_scopes(agent_scopes: list, node_scopes: list) -> bool:
    if not node_scopes:
        return True
    if not agent_scopes:
        return False
    return set(node_scopes).issubset(set(agent_scopes))


async def _job_visible(pool: Any, auth: dict, enums: EnumRegistry, job_id: str) -> bool:
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


async def _validate_relationship_node(
    pool: Any, enums: EnumRegistry, auth: dict, node_type: str, node_id: str
) -> None:
    if auth["caller_type"] != "agent":
        return
    if _is_admin(auth, enums):
        return
    if node_type == "entity":
        row = await pool.fetchrow(QUERIES["entities/get"], node_id)
        if not row:
            api_error("NOT_FOUND", "Entity not found", 404)
        if not _has_write_scopes(
            auth.get("scopes", []), row.get("privacy_scope_ids") or []
        ):
            api_error("FORBIDDEN", "Access denied", 403)
        return
    if node_type == "knowledge":
        row = await pool.fetchrow(QUERIES["knowledge/get"], node_id, None)
        if not row:
            api_error("NOT_FOUND", "Knowledge not found", 404)
        if not _has_write_scopes(
            auth.get("scopes", []), row.get("privacy_scope_ids") or []
        ):
            api_error("FORBIDDEN", "Access denied", 403)
        return
    if node_type == "job":
        if not await _job_visible(pool, auth, enums, node_id):
            api_error("FORBIDDEN", "Access denied", 403)
        return
    return


class CreateRelationshipBody(BaseModel):
    """Payload for creating a relationship.

    Attributes:
        source_type: Source node type.
        source_id: Source node id.
        target_type: Target node type.
        target_id: Target node id.
        relationship_type: Relationship type name.
        properties: Optional relationship properties.
    """

    source_type: str
    source_id: str
    target_type: str
    target_id: str
    relationship_type: str
    properties: dict | None = None


class UpdateRelationshipBody(BaseModel):
    """Payload for updating a relationship.

    Attributes:
        properties: Updated properties.
        status: Updated status name.
    """

    properties: dict | None = None
    status: str | None = None


@router.post("/")
async def create_relationship(
    payload: CreateRelationshipBody,
    request: Request,
    auth: dict = Depends(require_auth),
) -> dict[str, Any]:
    """Create a relationship.

    Args:
        payload: Relationship creation payload.
        request: FastAPI request.
        auth: Auth context.

    Returns:
        API response with created relationship or approval requirement.
    """
    pool = request.app.state.pool
    enums = request.app.state.enums
    data = payload.model_dump()
    if data.get("properties") is None:
        data["properties"] = {}
    await _validate_relationship_node(
        pool, enums, auth, data["source_type"], data["source_id"]
    )
    await _validate_relationship_node(
        pool, enums, auth, data["target_type"], data["target_id"]
    )
    # Validate taxonomy-backed fields before queuing approvals.
    try:
        require_relationship_type(data["relationship_type"], enums)
    except ValueError as exc:
        api_error("INVALID_INPUT", str(exc), 400)
    if resp := await maybe_check_agent_approval(
        pool, auth, "create_relationship", data
    ):
        return resp
    try:
        result = await execute_create_relationship(pool, enums, data)
    except ValueError as exc:
        api_error("INVALID_INPUT", str(exc), 400)
    return success(result)


@router.get("/{source_type}/{source_id}")
async def get_relationships(
    source_type: str,
    source_id: str,
    request: Request,
    auth: dict = Depends(require_auth),
    direction: str = "both",
    relationship_type: str | None = None,
) -> dict[str, Any]:
    """Get relationships for a source node.

    Args:
        source_type: Source node type.
        source_id: Source node id.
        request: FastAPI request.
        auth: Auth context.
        direction: Direction filter.
        relationship_type: Relationship type filter.

    Returns:
        API response with relationships.
    """
    pool = request.app.state.pool
    enums = request.app.state.enums

    try:
        UUID(source_id)
    except ValueError:
        api_error("INVALID_INPUT", "Invalid source id", 400)

    scope_ids = None if _is_admin(auth, enums) else auth.get("scopes", [])

    rows = await pool.fetch(
        QUERIES["relationships/get"],
        source_type,
        source_id,
        direction,
        relationship_type,
        scope_ids,
    )
    if auth["caller_type"] != "agent" or _is_admin(auth, enums):
        return success([dict(r) for r in rows])
    results = []
    for row in rows:
        if row["source_type"] == "job":
            if not await _job_visible(pool, auth, enums, row["source_id"]):
                continue
        if row["target_type"] == "job":
            if not await _job_visible(pool, auth, enums, row["target_id"]):
                continue
        results.append(dict(row))
    return success(results)


@router.get("/")
async def query_relationships(
    request: Request,
    auth: dict = Depends(require_auth),
    source_type: str | None = None,
    target_type: str | None = None,
    relationship_types: str | None = None,
    status_category: str = "active",
    limit: int = Query(50, le=100),
) -> dict[str, Any]:
    """Query relationships with filters.

    Args:
        request: FastAPI request.
        auth: Auth context.
        source_type: Source type filter.
        target_type: Target type filter.
        relationship_types: Comma-separated relationship types.
        status_category: Status category filter.
        limit: Max rows.

    Returns:
        API response with relationship list.
    """
    pool = request.app.state.pool
    enums = request.app.state.enums
    scope_ids = None if _is_admin(auth, enums) else auth.get("scopes", [])

    type_list = relationship_types.split(",") if relationship_types else None

    rows = await pool.fetch(
        QUERIES["relationships/query"],
        source_type,
        target_type,
        type_list,
        status_category,
        limit,
        scope_ids,
    )
    if auth["caller_type"] != "agent" or _is_admin(auth, enums):
        return success([dict(r) for r in rows])
    results = []
    for row in rows:
        if row["source_type"] == "job":
            if not await _job_visible(pool, auth, enums, row["source_id"]):
                continue
        if row["target_type"] == "job":
            if not await _job_visible(pool, auth, enums, row["target_id"]):
                continue
        results.append(dict(row))
    return success(results)


@router.patch("/{relationship_id}")
async def update_relationship(
    relationship_id: str,
    payload: UpdateRelationshipBody,
    request: Request,
    auth: dict = Depends(require_auth),
) -> dict[str, Any]:
    """Update a relationship.

    Args:
        relationship_id: Relationship id.
        payload: Relationship update payload.
        request: FastAPI request.
        auth: Auth context.

    Returns:
        API response with updated relationship or approval requirement.
    """
    pool = request.app.state.pool
    enums = request.app.state.enums

    try:
        UUID(relationship_id)
    except ValueError:
        api_error("INVALID_INPUT", "Invalid relationship id", 400)

    change = {
        "relationship_id": relationship_id,
        "properties": payload.properties,
        "status": payload.status,
    }
    row = await pool.fetchrow(QUERIES["relationships/get_by_id"], relationship_id)
    if not row:
        api_error("NOT_FOUND", "Relationship not found", 404)
    await _validate_relationship_node(
        pool, enums, auth, row["source_type"], row["source_id"]
    )
    await _validate_relationship_node(
        pool, enums, auth, row["target_type"], row["target_id"]
    )
    # Validate taxonomy-backed fields before queuing approvals.
    try:
        status_id = require_status(payload.status, enums) if payload.status else None
    except ValueError as exc:
        api_error("INVALID_INPUT", str(exc), 400)
    if resp := await maybe_check_agent_approval(
        pool, auth, "update_relationship", change
    ):
        return resp

    row = await pool.fetchrow(
        QUERIES["relationships/update"],
        relationship_id,
        json.dumps(payload.properties) if payload.properties else None,
        status_id,
    )
    if not row:
        api_error("NOT_FOUND", "Relationship not found", 404)

    return success(dict(row))
