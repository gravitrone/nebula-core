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
from nebula_mcp.enums import require_scopes, require_status
from nebula_mcp.executors import execute_create_job, execute_update_job
from nebula_mcp.helpers import enforce_scope_subset, scope_names_from_ids
from nebula_mcp.models import parse_optional_datetime, validate_metadata_payload
from nebula_mcp.query_loader import QueryLoader

QUERIES = QueryLoader(Path(__file__).resolve().parents[2] / "queries")

router = APIRouter()
ADMIN_SCOPE_NAMES = {"admin"}
JOB_PRIORITY_VALUES = {"low", "medium", "high", "critical"}


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
    assigned_to: str | None = None
    due_at: str | None = None
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
    try:
        data["metadata"] = validate_metadata_payload(data["metadata"]) or {}
    except ValueError as exc:
        api_error("INVALID_INPUT", str(exc), 400)
    if not data.get("scopes"):
        data["scopes"] = ["public"]
    if data.get("priority") and data["priority"] not in JOB_PRIORITY_VALUES:
        api_error("INVALID_INPUT", f"Invalid priority: {data['priority']}", 400)
    try:
        parse_optional_datetime(data.get("due_at"), "due_at")
    except ValueError as exc:
        api_error("INVALID_INPUT", str(exc), 400)
    if auth["caller_type"] == "agent" and not _is_admin(auth, enums):
        allowed = scope_names_from_ids(auth.get("scopes", []), enums)
        try:
            data["scopes"] = enforce_scope_subset(data["scopes"], allowed)
        except ValueError as exc:
            api_error("INVALID_INPUT", str(exc), 400)
        agent_id = auth.get("agent_id")
        data["agent_id"] = str(agent_id) if agent_id else None
    try:
        require_scopes(data["scopes"], enums)
    except ValueError as exc:
        api_error("INVALID_INPUT", str(exc), 400)
    if resp := await maybe_check_agent_approval(pool, auth, "create_job", data):
        return resp
    try:
        result = await execute_create_job(pool, enums, data)
    except ValueError as exc:
        api_error("INVALID_INPUT", str(exc), 400)
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
    try:
        due_before_dt = parse_optional_datetime(due_before, "due_before")
        due_after_dt = parse_optional_datetime(due_after, "due_after")
    except ValueError as exc:
        api_error("INVALID_INPUT", str(exc), 400)
    agent_filter = agent_id
    scope_filter = None if _is_admin(auth, enums) else (auth.get("scopes", []) or [])

    rows = await pool.fetch(
        QUERIES["jobs/query"],
        status_list,
        assigned_to,
        agent_filter,
        priority,
        due_before_dt,
        due_after_dt,
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
    try:
        status_id = require_status(payload.status, enums)
    except ValueError as exc:
        api_error("INVALID_INPUT", str(exc), 400)
    change = {
        "job_id": job_id,
        "status": payload.status,
        "status_reason": payload.status_reason,
        "completed_at": payload.completed_at,
    }
    try:
        completed_at = parse_optional_datetime(payload.completed_at, "completed_at")
    except ValueError as exc:
        api_error("INVALID_INPUT", str(exc), 400)
    if resp := await maybe_check_agent_approval(
        pool, auth, "update_job_status", change
    ):
        return resp

    row = await pool.fetchrow(
        QUERIES["jobs/update_status"],
        job_id,
        status_id,
        payload.status_reason,
        completed_at,
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
    if data.get("status"):
        try:
            require_status(data["status"], enums)
        except ValueError as exc:
            api_error("INVALID_INPUT", str(exc), 400)
    if data.get("priority") and data["priority"] not in JOB_PRIORITY_VALUES:
        api_error("INVALID_INPUT", f"Invalid priority: {data['priority']}", 400)
    if data.get("assigned_to"):
        _require_uuid(data["assigned_to"], "assignee")
    if "due_at" in data:
        try:
            parse_optional_datetime(data.get("due_at"), "due_at")
        except ValueError as exc:
            api_error("INVALID_INPUT", str(exc), 400)
    if data.get("metadata") is None:
        data.pop("metadata", None)
    else:
        try:
            data["metadata"] = validate_metadata_payload(data["metadata"])
        except ValueError as exc:
            api_error("INVALID_INPUT", str(exc), 400)
    change = {"job_id": job_id, **data}
    if resp := await maybe_check_agent_approval(pool, auth, "update_job", change):
        return resp
    try:
        updated = await execute_update_job(pool, enums, change)
    except ValueError as exc:
        api_error("INVALID_INPUT", str(exc), 400)
    return success(updated)


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
    if payload.priority and payload.priority not in JOB_PRIORITY_VALUES:
        api_error("INVALID_INPUT", f"Invalid priority: {payload.priority}", 400)
    data = {
        "title": payload.title,
        "description": payload.description,
        "job_type": None,
        "assigned_to": None,
        "agent_id": (
            str(parent_row.get("agent_id")) if parent_row.get("agent_id") else None
        ),
        "priority": payload.priority,
        "scopes": parent_scope_names or ["public"],
        "parent_job_id": job_id,
        "due_at": payload.due_at,
        "metadata": {},
    }
    try:
        parse_optional_datetime(payload.due_at, "due_at")
    except ValueError as exc:
        api_error("INVALID_INPUT", str(exc), 400)
    if resp := await maybe_check_agent_approval(pool, auth, "create_job", data):
        return resp
    try:
        result = await execute_create_job(pool, enums, data)
    except ValueError as exc:
        api_error("INVALID_INPUT", str(exc), 400)
    return success(result)
