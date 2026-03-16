"""Context API routes."""

from pathlib import Path
from typing import Any
from uuid import UUID

from fastapi import APIRouter, Depends, HTTPException, Query, Request
from pydantic import BaseModel, field_validator

from nebula_api.auth import maybe_check_agent_approval, require_auth
from nebula_api.response import paginated, success
from nebula_mcp.enums import require_relationship_type, require_scopes, require_status
from nebula_mcp.executors import (
    execute_create_context,
    execute_create_relationship,
    execute_update_context,
)
from nebula_mcp.helpers import enforce_scope_subset, scope_names_from_ids
from nebula_mcp.models import MAX_PAGE_LIMIT, MAX_TAG_LENGTH, MAX_TAGS
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


async def _require_entity_write_access(pool: Any, enums: Any, auth: dict, entity_id: str) -> None:
    """Handle require entity write access.

    Args:
        pool: Input parameter for _require_entity_write_access.
        enums: Input parameter for _require_entity_write_access.
        auth: Input parameter for _require_entity_write_access.
        entity_id: Input parameter for _require_entity_write_access.
    """

    if _is_admin(auth, enums):
        return
    row = await pool.fetchrow(QUERIES["entities/get"], entity_id)
    if not row:
        raise HTTPException(status_code=404, detail="Not Found")
    if not _has_write_scopes(auth.get("scopes", []), row.get("privacy_scope_ids") or []):
        raise HTTPException(status_code=403, detail="Forbidden")


async def _require_entity_read_access(pool: Any, enums: Any, auth: dict, entity_id: str) -> None:
    """Handle require entity read access."""

    if _is_admin(auth, enums):
        return
    row = await pool.fetchrow(QUERIES["entities/get"], entity_id)
    if not row:
        raise HTTPException(status_code=404, detail="Not Found")
    entity_scopes = row.get("privacy_scope_ids") or []
    caller_scopes = auth.get("scopes", []) or []
    if entity_scopes and not any(scope in caller_scopes for scope in entity_scopes):
        raise HTTPException(status_code=403, detail="Forbidden")


async def _require_job_read_access(pool: Any, enums: Any, auth: dict, job_id: str) -> dict:
    """Handle require job read access."""

    if _is_admin(auth, enums):
        row = await pool.fetchrow(QUERIES["jobs/get"], job_id)
        if not row:
            raise HTTPException(status_code=404, detail="Not Found")
        return dict(row)
    row = await pool.fetchrow(QUERIES["jobs/get"], job_id)
    if not row:
        raise HTTPException(status_code=404, detail="Not Found")
    job = dict(row)
    job_scopes = job.get("privacy_scope_ids") or []
    caller_scopes = auth.get("scopes", []) or []
    if job_scopes and not any(scope in caller_scopes for scope in job_scopes):
        raise HTTPException(status_code=403, detail="Forbidden")
    return job


async def _require_job_write_access(pool: Any, enums: Any, auth: dict, job_id: str) -> None:
    """Handle require job write access."""

    job = await _require_job_read_access(pool, enums, auth, job_id)
    if _is_admin(auth, enums):
        return
    if auth["caller_type"] == "user":
        if job.get("agent_id"):
            raise HTTPException(status_code=403, detail="Forbidden")
        assigned_to = str(job.get("assigned_to") or "")
        caller_id = str(auth.get("entity_id") or "")
        if assigned_to and assigned_to != caller_id:
            raise HTTPException(status_code=403, detail="Forbidden")
        return
    if job.get("agent_id") != auth.get("agent_id"):
        raise HTTPException(status_code=403, detail="Forbidden")


async def _require_context_write_access(pool: Any, enums: Any, auth: dict, context_id: str) -> None:
    """Handle require context write access.

    Args:
        pool: Input parameter for _require_context_write_access.
        enums: Input parameter for _require_context_write_access.
        auth: Input parameter for _require_context_write_access.
        context_id: Input parameter for _require_context_write_access.
    """

    if _is_admin(auth, enums):
        return
    row = await pool.fetchrow(QUERIES["context/get"], context_id, None)
    if not row:
        raise HTTPException(status_code=404, detail="Not Found")
    if not _has_write_scopes(auth.get("scopes", []), row.get("privacy_scope_ids") or []):
        raise HTTPException(status_code=403, detail="Forbidden")


def _validate_tag_list(tags: list[str] | None) -> list[str] | None:
    """Handle validate tag list.

    Args:
        tags: Input parameter for _validate_tag_list.

    Returns:
        Result value from the operation.
    """

    if tags is None:
        return None
    if not isinstance(tags, list):
        raise ValueError("Tags must be a list")
    cleaned: list[str] = []
    for tag in tags:
        if not isinstance(tag, str):
            raise ValueError("Tags must contain only strings")
        stripped = tag.strip()
        if stripped:
            cleaned.append(stripped)
    if len(cleaned) > MAX_TAGS:
        raise ValueError("Too many tags")
    for tag in cleaned:
        if len(tag) > MAX_TAG_LENGTH:
            raise ValueError("Tag too long")
    return cleaned


class CreateContextBody(BaseModel):
    """Payload for creating a context item.

    Attributes:
        title: Context title.
        url: Optional URL.
        source_type: Context source type.
        content: Optional content text.
        scopes: Privacy scopes.
        tags: Tag list.
    """

    model_config = {"extra": "forbid"}

    title: str
    url: str | None = None
    source_type: str = "article"
    content: str | None = None
    scopes: list[str] = []
    tags: list[str] = []

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

    @field_validator("url", mode="before")
    @classmethod
    def _validate_url(cls, v: str | None) -> str | None:
        """Handle validate url.

        Args:
            v: Input parameter for _validate_url.

        Returns:
            Result value from the operation.
        """

        if v is None:
            return None
        if not isinstance(v, str):
            raise ValueError("URL must be a string")
        if v == "":
            return v
        v = v.strip()
        if not (v.startswith("http://") or v.startswith("https://")):
            raise ValueError("URL must start with http:// or https://")
        return v


class LinkContextBody(BaseModel):
    """Payload for linking context to an owner."""

    model_config = {"extra": "forbid"}

    owner_type: str
    owner_id: str


class UpdateContextBody(BaseModel):
    """Payload for updating a context item.

    Attributes:
        title: Updated title.
        url: Updated URL.
        source_type: Updated source type.
        content: Updated content.
        status: Updated status name.
        tags: Updated tags.
        scopes: Updated scopes.
    """

    model_config = {"extra": "forbid"}

    title: str | None = None
    url: str | None = None
    source_type: str | None = None
    content: str | None = None
    status: str | None = None
    tags: list[str] | None = None
    scopes: list[str] | None = None

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

    @field_validator("url", mode="before")
    @classmethod
    def _validate_url(cls, v: str | None) -> str | None:
        """Handle validate url.

        Args:
            v: Input parameter for _validate_url.

        Returns:
            Result value from the operation.
        """

        if v is None:
            return None
        if not isinstance(v, str):
            raise ValueError("URL must be a string")
        if v == "":
            return v
        v = v.strip()
        if not (v.startswith("http://") or v.startswith("https://")):
            raise ValueError("URL must start with http:// or https://")
        return v


@router.post("/")
async def create_context(
    payload: CreateContextBody,
    request: Request,
    auth: dict = Depends(require_auth),
) -> dict[str, Any]:
    """Create a context item.

    Args:
        payload: Context creation payload.
        request: FastAPI request.
        auth: Auth context.

    Returns:
        API response with created context or approval requirement.
    """

    pool = request.app.state.pool
    enums = request.app.state.enums
    data = payload.model_dump()
    if auth["caller_type"] == "agent":
        allowed = scope_names_from_ids(auth.get("scopes", []), enums)
        try:
            data["scopes"] = enforce_scope_subset(data["scopes"], allowed)
        except ValueError as exc:
            raise HTTPException(status_code=400, detail=str(exc))

    # Validate scopes before queuing approvals.
    try:
        require_scopes(data["scopes"], enums)
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc))
    if resp := await maybe_check_agent_approval(pool, auth, "create_context", data):
        return resp
    try:
        result = await execute_create_context(pool, enums, data)
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc))
    return success(result)


@router.get("/")
async def query_context(
    request: Request,
    auth: dict = Depends(require_auth),
    source_type: str | None = None,
    tags: str | None = None,
    search_text: str | None = None,
    limit: int = Query(50, le=MAX_PAGE_LIMIT),
    offset: int = 0,
) -> dict[str, Any]:
    """Query context items with filters.

    Args:
        request: FastAPI request.
        auth: Auth context.
        source_type: Source type filter.
        tags: Comma-separated tag filters.
        search_text: Full-text search filter.
        limit: Max rows.
        offset: Offset for pagination.

    Returns:
        Paginated API response with context items.
    """

    pool = request.app.state.pool
    scope_ids = auth.get("scopes", [])
    tag_list = tags.split(",") if tags else None

    rows = await pool.fetch(
        QUERIES["context/query"],
        source_type,
        tag_list,
        search_text,
        scope_ids,
        limit,
        offset,
    )
    results = [dict(row) for row in rows]
    return paginated(results, len(results), limit, offset)


@router.get("/by-owner/{owner_type}/{owner_id}")
async def list_context_by_owner(
    owner_type: str,
    owner_id: str,
    request: Request,
    auth: dict = Depends(require_auth),
    limit: int = Query(100, ge=1, le=MAX_PAGE_LIMIT),
    offset: int = Query(0, ge=0),
) -> dict[str, Any]:
    """List context items linked to an owner via context-of relationships."""

    pool = request.app.state.pool
    enums = request.app.state.enums
    normalized_owner = (owner_type or "").strip().lower()
    if normalized_owner not in {"entity", "job"}:
        raise HTTPException(status_code=400, detail="Invalid owner type")
    try:
        UUID(owner_id)
    except ValueError:
        raise HTTPException(status_code=400, detail="Invalid owner id")

    if normalized_owner == "entity":
        await _require_entity_read_access(pool, enums, auth, owner_id)
    else:
        await _require_job_read_access(pool, enums, auth, owner_id)

    scope_filter = None if _is_admin(auth, enums) else (auth.get("scopes", []) or [])
    type_id = require_relationship_type("context-of", enums)
    rows = await pool.fetch(
        QUERIES["context/by_owner"],
        normalized_owner,
        owner_id,
        type_id,
        scope_filter,
        limit,
        offset,
    )
    results = [dict(row) for row in rows]
    return paginated(results, len(results), limit, offset)


@router.get("/{context_id}")
async def get_context(
    context_id: str,
    request: Request,
    auth: dict = Depends(require_auth),
) -> dict[str, Any]:
    """Fetch a context item by id.

    Args:
        context_id: Context id.
        request: FastAPI request.
        auth: Auth context.

    Returns:
        API response with context data.
    """

    pool = request.app.state.pool
    scope_ids = auth.get("scopes", [])
    try:
        UUID(context_id)
    except ValueError:
        raise HTTPException(status_code=400, detail="Invalid context id")
    row = await pool.fetchrow(
        QUERIES["context/get"],
        context_id,
        scope_ids,
    )
    if not row:
        raise HTTPException(status_code=404, detail="Not Found")
    return success(dict(row))


@router.post("/{context_id}/link")
async def link_to_owner(
    context_id: str,
    payload: LinkContextBody,
    request: Request,
    auth: dict = Depends(require_auth),
) -> dict[str, Any]:
    """Create a context-of relationship from owner to context.

    Args:
        context_id: Context id.
        payload: Link payload with owner info.
        request: FastAPI request.
        auth: Auth context.

    Returns:
        API response with created relationship or approval requirement.
    """

    pool = request.app.state.pool
    enums = request.app.state.enums
    try:
        UUID(context_id)
        UUID(payload.owner_id)
    except ValueError:
        raise HTTPException(status_code=400, detail="Invalid id")
    await _require_context_write_access(pool, enums, auth, context_id)
    if payload.owner_type == "entity":
        await _require_entity_write_access(pool, enums, auth, payload.owner_id)
    elif payload.owner_type == "job":
        await _require_job_write_access(pool, enums, auth, payload.owner_id)
    else:
        raise HTTPException(status_code=400, detail="Invalid owner type")
    relationship_payload = {
        "source_type": payload.owner_type,
        "source_id": payload.owner_id,
        "target_type": "context",
        "target_id": context_id,
        "relationship_type": "context-of",
        "properties": {},
    }
    try:
        require_relationship_type("context-of", enums)
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc))
    if resp := await maybe_check_agent_approval(
        pool, auth, "create_relationship", relationship_payload
    ):
        return resp

    try:
        result = await execute_create_relationship(pool, enums, relationship_payload)
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc))
    return success(result)


@router.patch("/{context_id}")
async def update_context(
    context_id: str,
    payload: UpdateContextBody,
    request: Request,
    auth: dict = Depends(require_auth),
) -> dict[str, Any]:
    """Update a context item.

    Args:
        context_id: Context id.
        payload: Context update payload.
        request: FastAPI request.
        auth: Auth context.

    Returns:
        API response with updated context or approval requirement.
    """

    pool = request.app.state.pool
    enums = request.app.state.enums
    try:
        UUID(context_id)
    except ValueError:
        raise HTTPException(status_code=400, detail="Invalid context id")

    await _require_context_write_access(pool, enums, auth, context_id)

    data = payload.model_dump()
    if data.get("status"):
        try:
            require_status(data["status"], enums)
        except ValueError as exc:
            raise HTTPException(status_code=400, detail=str(exc))
    if auth["caller_type"] == "agent" and data.get("scopes") is not None:
        allowed = scope_names_from_ids(auth.get("scopes", []), enums)
        try:
            data["scopes"] = enforce_scope_subset(data["scopes"], allowed)
        except ValueError as exc:
            raise HTTPException(status_code=400, detail=str(exc))
    if data.get("scopes") is not None:
        try:
            require_scopes(data["scopes"], enums)
        except ValueError as exc:
            raise HTTPException(status_code=400, detail=str(exc))
    change = {"context_id": context_id, **data}
    if resp := await maybe_check_agent_approval(pool, auth, "update_context", change):
        return resp
    try:
        updated = await execute_update_context(pool, enums, change)
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc))
    return success(updated)
