"""Job API routes."""

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
from nebula_mcp.enums import require_status
from nebula_mcp.executors import execute_create_job
from nebula_mcp.helpers import scope_names_from_ids
from nebula_mcp.query_loader import QueryLoader

QUERIES = QueryLoader(Path(__file__).resolve().parents[2] / "queries")

router = APIRouter()
ADMIN_SCOPE_NAMES = {"admin", "vault-only"}


def _is_admin(auth: dict, enums: Any) -> bool:
    scope_ids = set(auth.get("scopes", []))
    allowed_ids = {
        enums.scopes.name_to_id.get(name)
        for name in ADMIN_SCOPE_NAMES
        if enums.scopes.name_to_id.get(name)
    }
    return bool(scope_ids.intersection(allowed_ids))


def _require_job_read(auth: dict, enums: Any, job: dict) -> None:
    if _is_admin(auth, enums):
        return
    job_scopes = job.get("privacy_scope_ids") or []
    caller_scopes = auth.get("scopes", []) or []
    if job_scopes and not any(scope in caller_scopes for scope in job_scopes):
        api_error("FORBIDDEN", "Job not in your scopes", 403)


def _require_job_write(auth: dict, enums: Any, job: dict) -> None:
    _require_job_read(auth, enums, job)
    if auth["caller_type"] != "agent":
        return
    if _is_admin(auth, enums):
        return
    if job.get("agent_id") != auth.get("agent_id"):
        api_error("FORBIDDEN", "Job not in your scope", 403)


def _require_uuid(value: str, label: str) -> None:
    try:
        UUID(str(value))
    except ValueError:
        api_error("INVALID_INPUT", f"Invalid {label} id", 400)


class CreateJobBody(BaseModel):
    """Payload for creating a job.

    Attributes:
        title: Job title.
        description: Optional job description.
        job_type: Optional job type.
        assigned_to: Optional assignee identifier.
        agent_id: Optional agent id.
        priority: Job priority.
        parent_job_id: Parent job id for subtasks.
        due_at: Due date/time string.
        metadata: Arbitrary metadata.
    """

    title: str
    description: str | None = None
    job_type: str | None = None
    assigned_to: str | None = None
    agent_id: str | None = None
    priority: str = "medium"
    scopes: list[str] | None = None
    parent_job_id: str | None = None
    due_at: str | None = None
    metadata: dict | None = None


class UpdateJobStatusBody(BaseModel):
    """Payload for updating job status.

    Attributes:
        status: New status name.
        status_reason: Optional reason for status change.
        completed_at: Completion timestamp if completed.
    """

    status: str
    status_reason: str | None = None
    completed_at: str | None = None


class UpdateJobBody(BaseModel):
    """Payload for updating job fields.

    Attributes:
        title: Updated title.
        description: Updated description.
        status: Updated status name.
        priority: Updated priority.
        metadata: Updated metadata.
    """

    title: str | None = None
    description: str | None = None
    status: str | None = None
    priority: str | None = None
    metadata: dict | None = None


class CreateSubtaskBody(BaseModel):
    """Payload for creating a subtask.

    Attributes:
        title: Subtask title.
        description: Optional subtask description.
        priority: Priority.
        due_at: Due date/time string.
    """

    title: str
    description: str | None = None
    priority: str = "medium"
    due_at: str | None = None


@router.post("/")
async def create_job(
    payload: CreateJobBody,
    request: Request,
    auth: dict = Depends(require_auth),
) -> dict[str, Any]:
    """Create a new job.

    Args:
        payload: Job creation payload.
        request: FastAPI request.
        auth: Auth context.

    Returns:
        API response with created job or approval requirement.
    """
    pool = request.app.state.pool
    enums = request.app.state.enums
    data = payload.model_dump()
    if data.get("metadata") is None:
        data["metadata"] = {}
    if not data.get("scopes"):
        data["scopes"] = ["public"]
    if auth["caller_type"] == "agent" and not _is_admin(auth, enums):
        agent_id = auth.get("agent_id")
        data["agent_id"] = str(agent_id) if agent_id else None
    if resp := await maybe_check_agent_approval(pool, auth, "create_job", data):
        return resp
    result = await execute_create_job(pool, enums, data)
    return success(result)


@router.get("/{job_id}")
async def get_job(
    job_id: str,
    request: Request,
    auth: dict = Depends(require_auth),
) -> dict[str, Any]:
    """Fetch a job by id.

    Args:
        job_id: Job id.
        request: FastAPI request.
        auth: Auth context.

    Returns:
        API response with job data.
    """
    pool = request.app.state.pool

    row = await pool.fetchrow(QUERIES["jobs/get"], job_id)
    if not row:
        api_error("NOT_FOUND", f"Job '{job_id}' not found", 404)

    job = dict(row)
    _require_job_read(auth, request.app.state.enums, job)
    return success(job)


@router.get("/")
async def query_jobs(
    request: Request,
    auth: dict = Depends(require_auth),
    status_names: str | None = None,
    assigned_to: str | None = None,
    agent_id: str | None = None,
    priority: str | None = None,
    due_before: str | None = None,
    due_after: str | None = None,
    overdue_only: bool = False,
    parent_job_id: str | None = None,
    limit: int = Query(50, le=100),
) -> dict[str, Any]:
    """Query jobs with filters.

    Args:
        request: FastAPI request.
        auth: Auth context.
        status_names: Comma-separated status names.
        assigned_to: Assignee filter.
        agent_id: Agent filter.
        priority: Priority filter.
        due_before: Due date upper bound.
        due_after: Due date lower bound.
        overdue_only: Overdue filter.
        parent_job_id: Parent job filter.
        limit: Max rows.

    Returns:
        API response with job list.
    """
    pool = request.app.state.pool
    enums = request.app.state.enums

    status_list = status_names.split(",") if status_names else None
    if assigned_to:
        _require_uuid(assigned_to, "assignee")
    agent_filter = agent_id
    scope_filter = None if _is_admin(auth, enums) else (auth.get("scopes", []) or [])

    rows = await pool.fetch(
        QUERIES["jobs/query"],
        status_list,
        assigned_to,
        agent_filter,
        priority,
        due_before,
        due_after,
        overdue_only,
        parent_job_id,
        scope_filter,
        limit,
    )
    return success([dict(r) for r in rows])


@router.patch("/{job_id}/status")
async def update_job_status(
    job_id: str,
    payload: UpdateJobStatusBody,
    request: Request,
    auth: dict = Depends(require_auth),
) -> dict[str, Any]:
    """Update job status and status fields.

    Args:
        job_id: Job id.
        payload: Status update payload.
        request: FastAPI request.
        auth: Auth context.

    Returns:
        API response with updated job or approval requirement.
    """
    pool = request.app.state.pool
    enums = request.app.state.enums
    row = await pool.fetchrow(QUERIES["jobs/get"], job_id)
    if not row:
        api_error("NOT_FOUND", f"Job '{job_id}' not found", 404)
    _require_job_write(auth, enums, dict(row))
    change = {
        "job_id": job_id,
        "status": payload.status,
        "status_reason": payload.status_reason,
        "completed_at": payload.completed_at,
    }
    if resp := await maybe_check_agent_approval(
        pool, auth, "update_job_status", change
    ):
        return resp

    try:
        status_id = require_status(payload.status, enums)
    except ValueError as exc:
        api_error("INVALID_INPUT", str(exc), 400)

    row = await pool.fetchrow(
        QUERIES["jobs/update_status"],
        job_id,
        status_id,
        payload.status_reason,
        payload.completed_at,
    )
    if not row:
        api_error("NOT_FOUND", f"Job '{job_id}' not found", 404)

    return success(dict(row))


@router.patch("/{job_id}")
async def update_job(
    job_id: str,
    payload: UpdateJobBody,
    request: Request,
    auth: dict = Depends(require_auth),
) -> dict[str, Any]:
    """Update job fields.

    Args:
        job_id: Job id.
        payload: Job update payload.
        request: FastAPI request.
        auth: Auth context.

    Returns:
        API response with updated job or approval requirement.
    """
    pool = request.app.state.pool
    enums = request.app.state.enums
    row = await pool.fetchrow(QUERIES["jobs/get"], job_id)
    if not row:
        api_error("NOT_FOUND", f"Job '{job_id}' not found", 404)
    _require_job_write(auth, enums, dict(row))
    data = payload.model_dump()
    status_id = None
    if data.get("status"):
        status_id = require_status(data["status"], enums)
    if data.get("metadata") is None:
        data.pop("metadata", None)
    if resp := await maybe_check_agent_approval(
        pool, auth, "update_job", {"job_id": job_id, **data}
    ):
        return resp
    row = await pool.fetchrow(
        QUERIES["jobs/update"],
        job_id,
        data.get("title"),
        data.get("description"),
        status_id,
        data.get("priority"),
        data.get("metadata"),
    )
    if not row:
        api_error("NOT_FOUND", f"Job '{job_id}' not found", 404)
    return success(dict(row))


@router.post("/{job_id}/subtasks")
async def create_subtask(
    job_id: str,
    payload: CreateSubtaskBody,
    request: Request,
    auth: dict = Depends(require_auth),
) -> dict[str, Any]:
    """Create a subtask under a parent job.

    Args:
        job_id: Parent job id.
        payload: Subtask payload.
        request: FastAPI request.
        auth: Auth context.

    Returns:
        API response with created subtask or approval requirement.
    """
    pool = request.app.state.pool
    enums = request.app.state.enums
    parent_row = await pool.fetchrow(QUERIES["jobs/get"], job_id)
    if not parent_row:
        api_error("NOT_FOUND", f"Job '{job_id}' not found", 404)
    parent_job = dict(parent_row)
    _require_job_write(auth, enums, parent_job)
    parent_scope_names = scope_names_from_ids(
        parent_job.get("privacy_scope_ids") or [], enums
    )
    data = {
        "title": payload.title,
        "description": payload.description,
        "job_type": None,
        "assigned_to": None,
        "agent_id": parent_row.get("agent_id"),
        "priority": payload.priority,
        "scopes": parent_scope_names or ["public"],
        "parent_job_id": job_id,
        "due_at": payload.due_at,
        "metadata": {},
    }
    if resp := await maybe_check_agent_approval(pool, auth, "create_job", data):
        return resp
    result = await execute_create_job(pool, enums, data)
    return success(result)
