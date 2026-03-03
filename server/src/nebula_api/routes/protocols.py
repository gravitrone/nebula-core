"""Protocol API routes."""

# Standard Library
from pathlib import Path
from typing import Any

# Third-Party
from fastapi import APIRouter, Depends, HTTPException, Query, Request
from pydantic import BaseModel, field_validator

# Local
from nebula_api.auth import maybe_check_agent_approval, require_auth
from nebula_api.response import paginated, success
from nebula_mcp.enums import require_status
from nebula_mcp.executors import execute_create_protocol, execute_update_protocol
from nebula_mcp.models import MAX_TAG_LENGTH, MAX_TAGS
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


class CreateProtocolBody(BaseModel):
    """Payload for creating a protocol."""

    name: str
    title: str
    version: str | None = None
    content: str
    protocol_type: str | None = None
    applies_to: list[str] = []
    status: str = "active"
    tags: list[str] = []
    metadata: dict | None = None
    source_path: str | None = None
    trusted: bool = False

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


class UpdateProtocolBody(BaseModel):
    """Payload for updating a protocol."""

    title: str | None = None
    version: str | None = None
    content: str | None = None
    protocol_type: str | None = None
    applies_to: list[str] | None = None
    status: str | None = None
    tags: list[str] | None = None
    metadata: dict | None = None
    source_path: str | None = None
    trusted: bool | None = None

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


@router.get("/")
async def query_protocols(
    request: Request,
    auth: dict = Depends(require_auth),
    status_category: str | None = None,
    protocol_type: str | None = None,
    search: str | None = None,
    limit: int = Query(50, le=100),
) -> dict[str, Any]:
    """Query protocols with optional filters."""

    pool = request.app.state.pool
    enums = request.app.state.enums
    is_admin = _is_admin(auth, enums)
    rows = await pool.fetch(
        QUERIES["protocols/query"],
        status_category,
        protocol_type,
        search,
        limit,
        is_admin,
    )
    items = [dict(r) for r in rows]
    return paginated(items, len(items), limit, 0)


@router.get("/{protocol_name}")
async def get_protocol(
    protocol_name: str,
    request: Request,
    auth: dict = Depends(require_auth),
) -> dict[str, Any]:
    """Fetch a protocol by name."""

    pool = request.app.state.pool
    enums = request.app.state.enums
    row = await pool.fetchrow(QUERIES["protocols/get"], protocol_name)
    if not row:
        raise HTTPException(status_code=404, detail="Not Found")
    if row.get("trusted") and not _is_admin(auth, enums):
        raise HTTPException(status_code=403, detail="Forbidden")
    return success(dict(row))


@router.post("/")
async def create_protocol(
    payload: CreateProtocolBody,
    request: Request,
    auth: dict = Depends(require_auth),
) -> dict[str, Any]:
    """Create a protocol."""

    pool = request.app.state.pool
    enums = request.app.state.enums
    data = payload.model_dump()
    if data["metadata"] is None:
        data["metadata"] = {}
    if not _is_admin(auth, enums):
        data["trusted"] = False
    try:
        require_status(data["status"], enums)
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc))
    if resp := await maybe_check_agent_approval(pool, auth, "create_protocol", data):
        return resp
    try:
        result = await execute_create_protocol(pool, enums, data)
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc))
    return success(result)


@router.patch("/{protocol_name}")
async def update_protocol(
    protocol_name: str,
    payload: UpdateProtocolBody,
    request: Request,
    auth: dict = Depends(require_auth),
) -> dict[str, Any]:
    """Update a protocol."""

    pool = request.app.state.pool
    enums = request.app.state.enums
    data = payload.model_dump()
    data["name"] = protocol_name
    if data.get("status") is not None:
        try:
            require_status(data["status"], enums)
        except ValueError as exc:
            raise HTTPException(status_code=400, detail=str(exc))
    if not _is_admin(auth, enums) and data.get("trusted") is not None:
        data["trusted"] = False
    if resp := await maybe_check_agent_approval(pool, auth, "update_protocol", data):
        return resp
    try:
        result = await execute_update_protocol(pool, enums, data)
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc))
    return success(result)
