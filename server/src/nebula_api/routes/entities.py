"""Entity API routes."""

# Standard Library
import json
from pathlib import Path
from typing import Any
from uuid import UUID

# Third-Party
from fastapi import APIRouter, Depends, Query, Request
from pydantic import BaseModel, Field, field_validator

# Local
from nebula_api.auth import maybe_check_agent_approval, require_auth
from nebula_api.response import api_error, paginated, success
from nebula_mcp.enums import require_entity_type, require_scopes, require_status
from nebula_mcp.executors import execute_create_entity, execute_update_entity
from nebula_mcp.helpers import (
    bulk_update_entity_scopes as do_bulk_update_entity_scopes,
)
from nebula_mcp.helpers import (
    bulk_update_entity_tags as do_bulk_update_entity_tags,
)
from nebula_mcp.helpers import (
    enforce_scope_subset,
    filter_context_segments,
    normalize_bulk_operation,
    scope_names_from_ids,
)
from nebula_mcp.helpers import (
    get_entity_history as fetch_entity_history,
)
from nebula_mcp.helpers import (
    revert_entity as do_revert_entity,
)
from nebula_mcp.models import (
    MAX_TAG_LENGTH,
    MAX_TAGS,
    validate_metadata_payload,
)
from nebula_mcp.query_loader import QueryLoader

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


def _list_scope_ids(auth: dict, enums: Any) -> list:
    """Return scopes used for list/search filtering.

    For API user callers, default to public-only to avoid leaking
    private context segments in list/search results.
    """

    scope_ids = auth.get("scopes", []) or []
    if auth.get("caller_type") == "user":
        public_id = enums.scopes.name_to_id.get("public")
        return [public_id] if public_id else []
    return scope_ids


def _has_write_scopes(agent_scopes: list, node_scopes: list) -> bool:
    """Handle has write scopes.

    Args:
        agent_scopes: Input parameter for _has_write_scopes.
        node_scopes: Input parameter for _has_write_scopes.

    Returns:
        Result value from the operation.
    """

    if not node_scopes:
        return True
    if not agent_scopes:
        return False
    return set(node_scopes).issubset(set(agent_scopes))


async def _require_entity_write_access(
    pool: Any, enums: Any, auth: dict, entity_ids: list[str]
) -> None:
    """Handle require entity write access.

    Args:
        pool: Input parameter for _require_entity_write_access.
        enums: Input parameter for _require_entity_write_access.
        auth: Input parameter for _require_entity_write_access.
        entity_ids: Input parameter for _require_entity_write_access.
    """

    for entity_id in entity_ids:
        try:
            UUID(entity_id)
        except ValueError:
            api_error("INVALID_INPUT", "Invalid entity id", 400)
    if _is_admin(auth, enums):
        return
    rows = await pool.fetch(QUERIES["entities/scopes_by_ids"], entity_ids)
    if len(rows) != len(set(entity_ids)):
        api_error("NOT_FOUND", "Entity not found", 404)
    agent_scopes = auth.get("scopes", []) or []
    for row in rows:
        if not _has_write_scopes(agent_scopes, row.get("privacy_scope_ids") or []):
            api_error("FORBIDDEN", "Entity not in your scopes", 403)


def _validate_tag_list(tags: list[str] | None) -> list[str] | None:
    """Handle validate tag list.

    Args:
        tags: Input parameter for _validate_tag_list.

    Returns:
        Result value from the operation.
    """

    if tags is None:
        return None
    cleaned = [t.strip() for t in tags if t and t.strip()]
    if len(cleaned) > MAX_TAGS:
        raise ValueError("Too many tags")
    for tag in cleaned:
        if len(tag) > MAX_TAG_LENGTH:
            raise ValueError("Tag too long")
    return cleaned


class CreateEntityBody(BaseModel):
    """Payload for creating an entity.

    Attributes:
        name: Entity name.
        type: Entity type name.
        status: Status name.
        scopes: Privacy scopes.
        tags: Tag list.
        metadata: Arbitrary metadata.
        source_path: Optional file path reference.
    """

    name: str
    type: str
    status: str = "active"
    scopes: list[str] = []
    tags: list[str] = []
    metadata: dict | None = None
    source_path: str | None = None

    @field_validator("tags", mode="before")
    @classmethod
    def _clean_tags(cls, v: list[str] | None) -> list[str] | None:
        """Handle clean tags.

        Args:
            v: Input parameter for _clean_tags.

        Returns:
            Result value from the operation.
        """

        return _validate_tag_list(v)


class UpdateEntityBody(BaseModel):
    """Payload for updating an entity.

    Attributes:
        metadata: Updated metadata.
        tags: Updated tag list.
        status: Updated status name.
        status_reason: Optional status reason.
    """

    metadata: dict | None = None
    tags: list[str] | None = None
    status: str | None = None
    status_reason: str | None = None

    @field_validator("tags", mode="before")
    @classmethod
    def _clean_tags(cls, v: list[str] | None) -> list[str] | None:
        """Handle clean tags.

        Args:
            v: Input parameter for _clean_tags.

        Returns:
            Result value from the operation.
        """

        return _validate_tag_list(v)


class MetadataSearchBody(BaseModel):
    """Payload for metadata search.

    Attributes:
        metadata_query: JSON query object.
        limit: Max results to return.
    """

    metadata_query: dict
    limit: int = Field(default=50, ge=1, le=1000)


class RevertEntityBody(BaseModel):
    """Payload for reverting an entity to a prior audit entry.

    Attributes:
        audit_id: Audit log entry id.
    """

    audit_id: str


class BulkUpdateTagsBody(BaseModel):
    """Payload for bulk tag updates.

    Attributes:
        entity_ids: Entity ids to update.
        tags: Tags to add, remove, or set.
        op: Operation name (add, remove, set).
    """

    entity_ids: list[str]
    tags: list[str] = []
    op: str = "add"

    @field_validator("tags", mode="before")
    @classmethod
    def _clean_tags(cls, v: list[str] | None) -> list[str] | None:
        """Handle clean tags.

        Args:
            v: Input parameter for _clean_tags.

        Returns:
            Result value from the operation.
        """

        return _validate_tag_list(v)


class BulkUpdateScopesBody(BaseModel):
    """Payload for bulk scope updates.

    Attributes:
        entity_ids: Entity ids to update.
        scopes: Scopes to add, remove, or set.
        op: Operation name (add, remove, set).
    """

    entity_ids: list[str]
    scopes: list[str] = []
    op: str = "add"


@router.post("/")
async def create_entity(
    payload: CreateEntityBody,
    request: Request,
    auth: dict = Depends(require_auth),
) -> dict[str, Any]:
    """Create a new entity.

    Args:
        payload: Entity creation payload.
        request: FastAPI request.
        auth: Auth context.

    Returns:
        API response with created entity or approval requirement.
    """

    pool = request.app.state.pool
    enums = request.app.state.enums
    data = payload.model_dump()
    data.setdefault("metadata", {})
    if data["metadata"] is None:
        data["metadata"] = {}
    try:
        data["metadata"] = validate_metadata_payload(data["metadata"]) or {}
    except ValueError as exc:
        api_error("INVALID_INPUT", str(exc), 400)
    if auth["caller_type"] == "agent":
        allowed = scope_names_from_ids(auth.get("scopes", []), enums)
        try:
            data["scopes"] = enforce_scope_subset(data["scopes"], allowed)
        except ValueError as exc:
            api_error("INVALID_INPUT", str(exc), 400)

    # Validate taxonomy-backed fields before queuing approvals.
    try:
        require_entity_type(data["type"], enums)
        require_status(data["status"], enums)
        require_scopes(data["scopes"], enums)
    except ValueError as exc:
        api_error("INVALID_INPUT", str(exc), 400)

    if resp := await maybe_check_agent_approval(pool, auth, "create_entity", data):
        return resp
    try:
        result = await execute_create_entity(pool, enums, data)
    except ValueError as exc:
        api_error("INVALID_INPUT", str(exc), 400)
    return success(result)


@router.get("/{entity_id}")
async def get_entity(
    entity_id: str,
    request: Request,
    auth: dict = Depends(require_auth),
) -> dict[str, Any]:
    """Fetch an entity by id with privacy filtering.

    Args:
        entity_id: Entity id.
        request: FastAPI request.
        auth: Auth context.

    Returns:
        API response with entity data.
    """

    pool = request.app.state.pool
    enums = request.app.state.enums

    try:
        UUID(entity_id)
    except ValueError:
        api_error("INVALID_INPUT", "Invalid entity id", 400)

    row = await pool.fetchrow(QUERIES["entities/get"], entity_id)
    if not row:
        api_error("NOT_FOUND", "Entity not found", 404)

    entity = dict(row)

    # privacy filtering based on auth entity scopes
    entity_scopes = entity.get("privacy_scope_ids", [])
    auth_scopes = auth.get("scopes", [])

    if entity_scopes and not any(s in auth_scopes for s in entity_scopes):
        api_error("FORBIDDEN", "Entity not in your scopes", 403)

    if entity.get("metadata"):
        scope_names = [enums.scopes.id_to_name.get(s, "") for s in auth_scopes]
        entity["metadata"] = filter_context_segments(entity["metadata"], scope_names)

    return success(entity)


@router.get("/{entity_id}/history")
async def get_entity_history(
    entity_id: str,
    request: Request,
    auth: dict = Depends(require_auth),
    limit: int = Query(50, le=200),
    offset: int = 0,
) -> dict[str, Any]:
    """List audit history entries for an entity.

    Args:
        entity_id: Entity id.
        request: FastAPI request.
        auth: Auth context.
        limit: Max rows.
        offset: Offset for pagination.

    Returns:
        API response with audit history entries.
    """

    pool = request.app.state.pool
    row = await pool.fetchrow(QUERIES["entities/get"], entity_id)
    if not row:
        api_error("NOT_FOUND", "Entity not found", 404)
    entity = dict(row)
    entity_scopes = entity.get("privacy_scope_ids", [])
    auth_scopes = auth.get("scopes", [])
    if entity_scopes and not any(s in auth_scopes for s in entity_scopes):
        api_error("FORBIDDEN", "Entity not in your scopes", 403)
    rows = await fetch_entity_history(pool, entity_id, limit, offset)
    return success(rows)


@router.post("/{entity_id}/revert")
async def revert_entity(
    entity_id: str,
    payload: RevertEntityBody,
    request: Request,
    auth: dict = Depends(require_auth),
) -> dict[str, Any]:
    """Revert an entity to a prior audit entry.

    Args:
        entity_id: Entity id.
        payload: Revert payload containing audit id.
        request: FastAPI request.
        auth: Auth context.

    Returns:
        API response with revert result.
    """

    if auth["caller_type"] != "user":
        api_error("FORBIDDEN", "Only users can revert entities", 403)

    pool = request.app.state.pool
    async with pool.acquire() as conn:
        await conn.execute("SET app.changed_by_type = 'entity'")
        await conn.execute("SET app.changed_by_id = $1", auth["entity_id"])
        try:
            result = await do_revert_entity(conn, entity_id, payload.audit_id)
        finally:
            await conn.execute("RESET app.changed_by_type")
            await conn.execute("RESET app.changed_by_id")

    return success(result)


@router.post("/bulk/tags")
async def bulk_update_entity_tags(
    payload: BulkUpdateTagsBody,
    request: Request,
    auth: dict = Depends(require_auth),
) -> dict[str, Any]:
    """Bulk update entity tags.

    Args:
        payload: Bulk tag update payload.
        request: FastAPI request.
        auth: Auth context.

    Returns:
        API response with updated entity ids.
    """

    if not payload.entity_ids:
        api_error("VALIDATION_ERROR", "No entity ids provided", 400)

    op = normalize_bulk_operation(payload.op)
    if op != "set" and not payload.tags:
        api_error("VALIDATION_ERROR", "No tags provided", 400)

    pool = request.app.state.pool
    enums = request.app.state.enums
    await _require_entity_write_access(pool, enums, auth, payload.entity_ids)
    if resp := await maybe_check_agent_approval(
        pool, auth, "bulk_update_entity_tags", payload.model_dump()
    ):
        return resp

    updated = await do_bulk_update_entity_tags(
        pool, payload.entity_ids, payload.tags, op
    )
    return success({"updated": len(updated), "entity_ids": updated})


@router.post("/bulk/scopes")
async def bulk_update_entity_scopes(
    payload: BulkUpdateScopesBody,
    request: Request,
    auth: dict = Depends(require_auth),
) -> dict[str, Any]:
    """Bulk update entity scopes.

    Args:
        payload: Bulk scope update payload.
        request: FastAPI request.
        auth: Auth context.

    Returns:
        API response with updated entity ids.
    """

    if not payload.entity_ids:
        api_error("VALIDATION_ERROR", "No entity ids provided", 400)

    op = normalize_bulk_operation(payload.op)
    if op != "set" and not payload.scopes:
        api_error("VALIDATION_ERROR", "No scopes provided", 400)

    pool = request.app.state.pool
    enums = request.app.state.enums

    await _require_entity_write_access(pool, enums, auth, payload.entity_ids)

    scopes = payload.scopes
    if auth["caller_type"] == "agent":
        allowed = scope_names_from_ids(auth.get("scopes", []), enums)
        try:
            scopes = enforce_scope_subset(scopes, allowed)
        except ValueError as exc:
            api_error("INVALID_INPUT", str(exc), 400)

    # Validate scope names before queuing approvals.
    try:
        require_scopes(scopes, enums)
    except ValueError as exc:
        api_error("INVALID_INPUT", str(exc), 400)
    data = payload.model_dump()
    data["scopes"] = scopes
    if resp := await maybe_check_agent_approval(
        pool, auth, "bulk_update_entity_scopes", data
    ):
        return resp

    updated = await do_bulk_update_entity_scopes(
        pool, enums, payload.entity_ids, scopes, op
    )
    return success({"updated": len(updated), "entity_ids": updated})


@router.get("/")
async def query_entities(
    request: Request,
    auth: dict = Depends(require_auth),
    type: str | None = None,
    tags: str | None = None,
    search_text: str | None = None,
    status_category: str = "active",
    limit: int = Query(50, le=100),
    offset: int = 0,
) -> dict[str, Any]:
    """Query entities with filters.

    Args:
        request: FastAPI request.
        auth: Auth context.
        type: Entity type filter.
        tags: Comma-separated tag filters.
        search_text: Full-text search filter.
        status_category: Status category filter.
        limit: Max rows.
        offset: Offset for pagination.

    Returns:
        Paginated API response with entities.
    """

    pool = request.app.state.pool
    enums = request.app.state.enums

    type_id = require_entity_type(type, enums) if type else None
    tag_list = tags.split(",") if tags else None
    scope_ids = _list_scope_ids(auth, enums)

    rows = await pool.fetch(
        QUERIES["entities/query"],
        type_id,
        tag_list,
        search_text,
        status_category,
        scope_ids,
        limit,
        offset,
    )
    scope_names = [enums.scopes.id_to_name.get(s, "") for s in scope_ids]
    results = []
    for row in rows:
        entity = dict(row)
        if entity.get("metadata"):
            entity["metadata"] = filter_context_segments(
                entity["metadata"], scope_names
            )
        results.append(entity)
    return paginated(results, len(results), limit, offset)


@router.patch("/{entity_id}")
async def update_entity(
    entity_id: str,
    payload: UpdateEntityBody,
    request: Request,
    auth: dict = Depends(require_auth),
) -> dict[str, Any]:
    """Update an entity.

    Args:
        entity_id: Entity id.
        payload: Entity update payload.
        request: FastAPI request.
        auth: Auth context.

    Returns:
        API response with updated entity or approval requirement.
    """

    pool = request.app.state.pool
    enums = request.app.state.enums

    try:
        UUID(entity_id)
    except ValueError:
        api_error("INVALID_INPUT", "Invalid entity id", 400)

    await _require_entity_write_access(pool, enums, auth, [entity_id])

    change = payload.model_dump()
    change["entity_id"] = entity_id
    if change.get("metadata") is None:
        change.pop("metadata", None)
    else:
        try:
            change["metadata"] = validate_metadata_payload(change["metadata"])
        except ValueError as exc:
            api_error("INVALID_INPUT", str(exc), 400)
    if auth["caller_type"] == "agent" and change.get("scopes") is not None:
        allowed = scope_names_from_ids(auth.get("scopes", []), enums)
        try:
            change["scopes"] = enforce_scope_subset(change["scopes"], allowed)
        except ValueError as exc:
            api_error("INVALID_INPUT", str(exc), 400)

    # Validate taxonomy-backed fields before queuing approvals.
    if change.get("status") is not None:
        try:
            require_status(change["status"], enums)
        except ValueError as exc:
            api_error("INVALID_INPUT", str(exc), 400)
    if resp := await maybe_check_agent_approval(pool, auth, "update_entity", change):
        return resp
    try:
        result = await execute_update_entity(pool, enums, change)
    except ValueError as exc:
        api_error("INVALID_INPUT", str(exc), 400)
    return success(result)


@router.post("/search")
async def search_by_metadata(
    payload: MetadataSearchBody,
    request: Request,
    auth: dict = Depends(require_auth),
) -> dict[str, Any]:
    """Search entities by metadata fields.

    Args:
        payload: Metadata search payload.
        request: FastAPI request.
        auth: Auth context.

    Returns:
        API response with matching entities.
    """

    pool = request.app.state.pool
    enums = request.app.state.enums
    scope_ids = _list_scope_ids(auth, enums)

    rows = await pool.fetch(
        QUERIES["entities/search_by_metadata"],
        json.dumps(payload.metadata_query),
        payload.limit,
        scope_ids,
    )
    scope_names = [enums.scopes.id_to_name.get(s, "") for s in scope_ids]
    results = []
    for row in rows:
        entity = dict(row)
        if entity.get("metadata"):
            entity["metadata"] = filter_context_segments(
                entity["metadata"], scope_names
            )
        results.append(entity)
    return success(results)
