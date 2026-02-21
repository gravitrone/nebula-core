"""Approval API routes."""

# Standard Library
from pathlib import Path
from typing import Any
from uuid import UUID

# Third-Party
from fastapi import APIRouter, Depends, Query, Request
from pydantic import BaseModel

# Local
from nebula_api.auth import require_auth
from nebula_api.response import api_error, success
from nebula_mcp.enums import require_scopes
from nebula_mcp.helpers import (
    approve_request as do_approve,
)
from nebula_mcp.helpers import (
    get_approval_diff as compute_approval_diff,
)
from nebula_mcp.helpers import (
    get_approval_request,
    get_pending_approvals_all,
)
from nebula_mcp.helpers import (
    reject_request as do_reject,
)
from nebula_mcp.query_loader import QueryLoader

QUERIES = QueryLoader(Path(__file__).resolve().parents[2] / "queries")

router = APIRouter()
ADMIN_SCOPE_NAMES = {"admin"}


def _require_admin_scope(auth: dict, enums: Any) -> None:
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


class RejectBody(BaseModel):
    """Payload for rejecting an approval request.

    Attributes:
        review_notes: Optional reviewer notes explaining the rejection.
    """

    review_notes: str = ""


class ApproveBody(BaseModel):
    """Optional reviewer input for approval grants.

    Attributes:
        review_notes: Optional reviewer notes to persist.
        grant_scopes: Optional final scope names (register_agent only).
        grant_requires_approval: Optional final trust mode (register_agent only).
    """

    review_notes: str | None = None
    grant_scopes: list[str] | None = None
    grant_requires_approval: bool | None = None


@router.get("/pending")
async def get_pending(
    request: Request,
    auth: dict = Depends(require_auth),
    limit: int = Query(200, ge=1, le=5000),
    offset: int = Query(0, ge=0),
) -> dict[str, Any]:
    """List pending approval requests.

    Args:
        request: FastAPI request.
        auth: Auth context.

    Returns:
        API response with pending approvals.
    """

    pool = request.app.state.pool
    enums = request.app.state.enums
    _require_admin_scope(auth, enums)
    results = await get_pending_approvals_all(pool, limit=limit, offset=offset)
    return success(results)


@router.get("/{approval_id}")
async def get_approval(
    approval_id: str,
    request: Request,
    auth: dict = Depends(require_auth),
) -> dict[str, Any]:
    """Fetch a single approval request by id.

    Args:
        approval_id: Approval request UUID.
        request: FastAPI request.
        auth: Auth context.

    Returns:
        API response with approval request data.
    """

    pool = request.app.state.pool
    enums = request.app.state.enums
    _require_admin_scope(auth, enums)
    _require_uuid(approval_id, "approval")

    row = await get_approval_request(pool, approval_id)
    if not row:
        api_error("NOT_FOUND", "Approval request not found", 404)

    return success(row)


@router.post("/{approval_id}/approve")
async def approve(
    approval_id: str,
    request: Request,
    payload: ApproveBody | None = None,
    auth: dict = Depends(require_auth),
) -> dict[str, Any]:
    """Approve an approval request.

    Args:
        approval_id: Approval request UUID.
        request: FastAPI request.
        auth: Auth context.

    Returns:
        API response with approval result.
    """

    pool = request.app.state.pool
    enums = request.app.state.enums
    _require_admin_scope(auth, enums)
    _require_uuid(approval_id, "approval")

    approval_row = await get_approval_request(pool, approval_id)
    if not approval_row:
        api_error("NOT_FOUND", "Approval request not found", 404)
    approval = dict(approval_row)
    is_register = approval.get("request_type") == "register_agent"

    review_notes = payload.review_notes if payload else None
    review_details: dict[str, Any] = {}
    has_grants = False
    if payload and payload.grant_scopes is not None:
        has_grants = True
        review_details["grant_scopes"] = payload.grant_scopes
        try:
            review_details["grant_scope_ids"] = [
                str(scope_id)
                for scope_id in require_scopes(payload.grant_scopes, enums)
            ]
        except ValueError as exc:
            api_error("INVALID_INPUT", str(exc), 400)
    if payload and payload.grant_requires_approval is not None:
        has_grants = True
        review_details["grant_requires_approval"] = payload.grant_requires_approval

    if has_grants and not is_register:
        api_error(
            "INVALID_INPUT",
            (
                "grant_scopes and grant_requires_approval are only valid "
                "for register_agent approvals"
            ),
            400,
        )

    try:
        result = await do_approve(
            pool,
            enums,
            approval_id,
            str(auth["entity_id"]),
            review_details=review_details if review_details else None,
            review_notes=review_notes,
        )
    except ValueError as exc:
        api_error("EXECUTION_FAILED", str(exc), 400)
    except Exception as exc:
        # Execution errors should not bubble as raw 500s. The helper marks the
        # request as approved-failed, so we can surface a controlled payload.
        failed_row = await pool.fetchrow(QUERIES["approvals/get_request"], approval_id)
        failed = dict(failed_row) if failed_row else {}
        msg = failed.get("execution_error") or str(exc) or "Approval execution failed"
        api_error("EXECUTION_FAILED", msg, 409)
    return success(result)


@router.post("/{approval_id}/reject")
async def reject(
    approval_id: str,
    payload: RejectBody,
    request: Request,
    auth: dict = Depends(require_auth),
) -> dict[str, Any]:
    """Reject an approval request.

    Args:
        approval_id: Approval request UUID.
        payload: Rejection payload with optional notes.
        request: FastAPI request.
        auth: Auth context.

    Returns:
        API response with rejection result.
    """

    pool = request.app.state.pool
    enums = request.app.state.enums
    _require_admin_scope(auth, enums)
    _require_uuid(approval_id, "approval")

    result = await do_reject(
        pool, approval_id, str(auth["entity_id"]), payload.review_notes
    )
    return success(result)


@router.get("/{approval_id}/diff")
async def get_diff(
    approval_id: str,
    request: Request,
    auth: dict = Depends(require_auth),
) -> dict[str, Any]:
    """Compute the diff for an approval request.

    Args:
        approval_id: Approval request UUID.
        request: FastAPI request.
        auth: Auth context.

    Returns:
        API response with approval diff data.
    """

    pool = request.app.state.pool
    enums = request.app.state.enums
    _require_admin_scope(auth, enums)
    _require_uuid(approval_id, "approval")
    try:
        result = await compute_approval_diff(pool, approval_id)
    except ValueError as exc:
        message = str(exc)
        if "not found" in message.lower():
            api_error("NOT_FOUND", message, 404)
        api_error("INVALID_INPUT", message, 400)
    return success(result)
