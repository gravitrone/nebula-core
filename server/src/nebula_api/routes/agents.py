"""Agent API routes."""

# Standard Library
from pathlib import Path
from typing import Any
from uuid import UUID

# Third-Party
from fastapi import APIRouter, Depends, Request
from pydantic import BaseModel
from starlette.responses import JSONResponse

# Local
from nebula_api.auth import require_auth
from nebula_api.response import api_error, success
from nebula_mcp.enums import EnumRegistry, load_enums, require_scopes
from nebula_mcp.helpers import create_approval_request, create_enrollment_session
from nebula_mcp.query_loader import QueryLoader

QUERIES = QueryLoader(Path(__file__).resolve().parents[2] / "queries")

router = APIRouter()

ADMIN_SCOPE_NAMES = {"admin"}


def _has_admin_scope(auth: dict, enums: EnumRegistry) -> bool:
    """Handle has admin scope.

    Args:
        auth: Input parameter for _has_admin_scope.
        enums: Input parameter for _has_admin_scope.

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


def _require_uuid(value: str, label: str) -> None:
    """Handle require uuid.

    Args:
        value: Input parameter for _require_uuid.
        label: Input parameter for _require_uuid.
    """

    try:
        UUID(str(value))
    except ValueError:
        api_error("INVALID_INPUT", f"Invalid {label} id", 400)


def _require_admin_scope(auth: dict, enums: EnumRegistry) -> None:
    """Handle require admin scope.

    Args:
        auth: Input parameter for _require_admin_scope.
        enums: Input parameter for _require_admin_scope.
    """

    scope_ids = set(auth.get("scopes", []))
    allowed_ids = {
        enums.scopes.name_to_id.get(name)
        for name in ADMIN_SCOPE_NAMES
        if enums.scopes.name_to_id.get(name)
    }
    if not scope_ids.intersection(allowed_ids):
        api_error("FORBIDDEN", "Admin scope required", 403)


class RegisterAgentBody(BaseModel):
    """Payload for registering a new agent.

    Attributes:
        name: Unique agent name.
        description: Optional agent description.
        requested_scopes: Requested privacy scopes.
        capabilities: Agent capability tags.
    """

    name: str
    description: str | None = None
    requested_scopes: list[str] = ["public"]
    requested_requires_approval: bool = False
    capabilities: list[str] = []


class UpdateAgentBody(BaseModel):
    """Payload for updating agent settings.

    Attributes:
        description: Updated description.
        requires_approval: Whether the agent requires approval.
        scopes: New scope list.
    """

    description: str | None = None
    requires_approval: bool | None = None
    scopes: list[str] | None = None


@router.post("/register")
async def register_agent(payload: RegisterAgentBody, request: Request) -> JSONResponse:
    """Register a new agent and create an approval request.

    Args:
        payload: Agent registration payload.
        request: FastAPI request.

    Returns:
        JSON response with approval request info.
    """

    pool = request.app.state.pool
    enums = request.app.state.enums

    # Resolve scope names to UUIDs
    try:
        scope_ids = require_scopes(payload.requested_scopes, enums)
    except ValueError as exc:
        api_error("INVALID_INPUT", str(exc), 400)

    pending_status_id = enums.statuses.name_to_id.get("inactive")
    if not pending_status_id:
        api_error("INTERNAL", "Status 'inactive' not found", 500)

    async with pool.acquire() as conn:
        async with conn.transaction():
            existing = await conn.fetchrow(QUERIES["agents/check_name"], payload.name)
            if existing:
                api_error("CONFLICT", f"Agent '{payload.name}' already exists", 409)

            agent = await conn.fetchrow(
                QUERIES["agents/create"],
                payload.name,
                payload.description,
                scope_ids,
                payload.capabilities,
                pending_status_id,
                payload.requested_requires_approval,
            )

            approval = await create_approval_request(
                pool,
                str(agent["id"]),
                "register_agent",
                {
                    "agent_id": str(agent["id"]),
                    "name": payload.name,
                    "description": payload.description,
                    "requested_scopes": payload.requested_scopes,
                    "requested_requires_approval": payload.requested_requires_approval,
                    "capabilities": payload.capabilities,
                },
                conn=conn,
            )
            enrollment = await create_enrollment_session(
                pool,
                agent_id=str(agent["id"]),
                approval_request_id=str(approval["id"]),
                requested_scope_ids=scope_ids,
                requested_requires_approval=payload.requested_requires_approval,
                conn=conn,
            )

    return JSONResponse(
        status_code=201,
        content={
            "data": {
                "agent_id": str(agent["id"]),
                "approval_request_id": str(approval["id"]),
                "registration_id": str(enrollment["id"]),
                "enrollment_token": enrollment["enrollment_token"],
                "status": "pending_approval",
            }
        },
    )


@router.patch("/{agent_id}")
async def update_agent(
    agent_id: str,
    payload: UpdateAgentBody,
    request: Request,
    auth: dict = Depends(require_auth),
) -> dict[str, Any]:
    """Update agent fields.

    Args:
        agent_id: Agent id.
        payload: Agent update payload.
        request: FastAPI request.
        auth: Auth context.

    Returns:
        API response with updated agent data.
    """

    pool = request.app.state.pool
    enums = request.app.state.enums

    is_self = auth.get("caller_type") == "agent" and str(auth.get("agent_id")) == str(
        agent_id
    )
    if not is_self and not _has_admin_scope(auth, enums):
        api_error("FORBIDDEN", "Admin scope required", 403)

    # Resolve scope names to UUIDs if provided
    scope_ids = None
    if payload.scopes is not None:
        scope_ids = require_scopes(payload.scopes, enums)

    row = await pool.fetchrow(
        QUERIES["agents/update"],
        agent_id,
        payload.description,
        payload.requires_approval,
        scope_ids,
    )

    if not row:
        api_error("NOT_FOUND", "Agent not found", 404)

    return success(dict(row))


@router.get("/{agent_name}")
async def get_agent_info(
    agent_name: str,
    request: Request,
    auth: dict = Depends(require_auth),
) -> dict:
    """Get agent info by name.

    Args:
        agent_name: Agent name.
        request: FastAPI request.
        auth: Auth context.

    Returns:
        API response with agent info.
    """

    pool = request.app.state.pool
    enums = request.app.state.enums

    _require_admin_scope(auth, enums)

    row = await pool.fetchrow(QUERIES["agents/get_info"], agent_name)
    if not row:
        api_error("NOT_FOUND", f"Agent '{agent_name}' not found", 404)

    return success(dict(row))


@router.get("/")
async def list_agents(
    request: Request,
    auth: dict = Depends(require_auth),
    status_category: str = "active",
) -> dict:
    """List agents by status category.

    Args:
        request: FastAPI request.
        auth: Auth context.
        status_category: Status category filter.

    Returns:
        API response with agent list.
    """

    pool = request.app.state.pool
    enums = request.app.state.enums

    _require_admin_scope(auth, enums)

    rows = await pool.fetch(QUERIES["agents/list"], status_category)
    return success([dict(r) for r in rows])


@router.post("/reload-enums")
async def reload_enums_route(
    request: Request, auth: dict = Depends(require_auth)
) -> dict:
    """Reload enum cache.

    Args:
        request: FastAPI request.
        auth: Auth context.

    Returns:
        API response with confirmation message.
    """

    pool = request.app.state.pool
    request.app.state.enums = await load_enums(pool)
    return success({"message": "Enums reloaded"})
