"""Audit API routes."""

# Standard Library
from uuid import UUID

# Third-Party
from fastapi import APIRouter, Depends, Query, Request

# Local
from nebula_api.auth import require_auth
from nebula_api.response import api_error, paginated, success
from nebula_mcp.enums import EnumRegistry
from nebula_mcp.helpers import (
    list_audit_actors,
    list_audit_scopes,
    query_audit_log,
)

router = APIRouter()
ADMIN_SCOPE_NAMES = {"admin"}


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


@router.get("/")
async def list_audit_log(
    request: Request,
    auth: dict = Depends(require_auth),
    table: str | None = None,
    action: str | None = None,
    actor_type: str | None = None,
    actor_id: str | None = None,
    record_id: str | None = None,
    scope_id: str | None = None,
    limit: int = Query(50, le=200),
    offset: int = 0,
) -> dict:
    """List audit log entries with optional filters.

    Args:
        request: FastAPI request.
        auth: Auth context.
        table: Table name filter.
        action: Action filter.
        actor_type: Actor type filter.
        actor_id: Actor id filter.
        record_id: Record id filter.
        scope_id: Privacy scope filter.
        limit: Max rows.
        offset: Offset for pagination.

    Returns:
        Paginated audit log response.
    """

    pool = request.app.state.pool
    enums = request.app.state.enums
    _require_admin_scope(auth, enums)
    if actor_id:
        _require_uuid(actor_id, "actor")
    if record_id:
        _require_uuid(record_id, "record")
    if scope_id:
        _require_uuid(scope_id, "scope")
    rows = await query_audit_log(
        pool,
        table,
        action,
        actor_type,
        actor_id,
        record_id,
        scope_id,
        limit,
        offset,
    )
    return paginated(rows, len(rows), limit, offset)


@router.get("/scopes")
async def list_scopes(
    request: Request,
    auth: dict = Depends(require_auth),
) -> dict:
    """List audit scopes with usage counts.

    Args:
        request: FastAPI request.
        auth: Auth context.

    Returns:
        List of scopes with counts.
    """

    pool = request.app.state.pool
    enums = request.app.state.enums
    _require_admin_scope(auth, enums)
    rows = await list_audit_scopes(pool)
    return success(rows)


@router.get("/actors")
async def list_actors(
    request: Request,
    auth: dict = Depends(require_auth),
    actor_type: str | None = None,
) -> dict:
    """List audit actors with activity counts.

    Args:
        request: FastAPI request.
        auth: Auth context.
        actor_type: Actor type filter.

    Returns:
        List of audit actors.
    """

    pool = request.app.state.pool
    enums = request.app.state.enums
    _require_admin_scope(auth, enums)
    rows = await list_audit_actors(pool, actor_type)
    return success(rows)
