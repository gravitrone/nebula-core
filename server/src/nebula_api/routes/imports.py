"""Bulk import API routes."""

# Standard Library
from pathlib import Path
from typing import Any, Callable

# Third-Party
from fastapi import APIRouter, Depends, Request
from pydantic import BaseModel
from starlette.responses import JSONResponse

# Local
from nebula_api.auth import maybe_check_agent_approval, require_auth
from nebula_api.response import api_error, success
from nebula_mcp.enums import (
    require_entity_type,
    require_relationship_type,
    require_scopes,
    require_status,
)
from nebula_mcp.executors import (
    execute_create_entity,
    execute_create_job,
    execute_create_knowledge,
    execute_create_relationship,
)
from nebula_mcp.helpers import (
    create_approval_request,
    enforce_scope_subset,
    ensure_approval_capacity,
    scope_names_from_ids,
)
from nebula_mcp.imports import (
    extract_items,
    normalize_entity,
    normalize_job,
    normalize_knowledge,
    normalize_relationship,
)
from nebula_mcp.query_loader import QueryLoader

router = APIRouter()
ADMIN_SCOPE_NAMES = {"admin"}
QUERIES = QueryLoader(Path(__file__).resolve().parents[2] / "queries")
JOB_PRIORITY_VALUES = {"low", "medium", "high", "critical"}


def _validate_taxonomy_before_approval(
    approval_action: str, enums: Any, normalized: dict[str, Any]
) -> None:
    """Validate taxonomy-backed fields before queuing approvals.

    This prevents "queued poison" approvals that later explode during execution.
    """

    if approval_action == "bulk_import_entities":
        require_entity_type(str(normalized.get("type") or ""), enums)
        require_status(str(normalized.get("status") or ""), enums)
        require_scopes(list(normalized.get("scopes") or []), enums)
        return
    if approval_action == "bulk_import_knowledge":
        require_scopes(list(normalized.get("scopes") or []), enums)
        return
    if approval_action == "bulk_import_relationships":
        require_relationship_type(str(normalized.get("relationship_type") or ""), enums)
        return
    if approval_action == "bulk_import_jobs":
        priority = normalized.get("priority")
        if priority and str(priority) not in JOB_PRIORITY_VALUES:
            raise ValueError(f"Invalid priority: {priority}")
        return


def _is_admin(auth: dict, enums: Any) -> bool:
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


async def _require_entity_write_access(
    pool: Any, enums: Any, auth: dict, entity_id: str
) -> None:
    if auth["caller_type"] != "agent":
        return
    if _is_admin(auth, enums):
        return
    row = await pool.fetchrow(QUERIES["entities/get"], entity_id)
    if not row:
        raise ValueError("Entity not found")
    if not _has_write_scopes(
        auth.get("scopes", []), row.get("privacy_scope_ids") or []
    ):
        raise ValueError("Access denied")


async def _require_knowledge_write_access(
    pool: Any, enums: Any, auth: dict, knowledge_id: str
) -> None:
    if auth["caller_type"] != "agent":
        return
    if _is_admin(auth, enums):
        return
    row = await pool.fetchrow(QUERIES["knowledge/get"], knowledge_id, None)
    if not row:
        raise ValueError("Knowledge not found")
    if not _has_write_scopes(
        auth.get("scopes", []), row.get("privacy_scope_ids") or []
    ):
        raise ValueError("Access denied")


async def _require_job_owner(pool: Any, auth: dict, job_id: str) -> None:
    if auth["caller_type"] != "agent":
        return
    row = await pool.fetchrow(QUERIES["jobs/get"], job_id)
    if not row:
        raise ValueError("Job not found")
    if row.get("agent_id") != auth.get("agent_id"):
        raise ValueError("Access denied")


async def _validate_relationship_node(
    pool: Any, enums: Any, auth: dict, node_type: str, node_id: str
) -> None:
    if node_type == "entity":
        await _require_entity_write_access(pool, enums, auth, node_id)
        return
    if node_type == "knowledge":
        await _require_knowledge_write_access(pool, enums, auth, node_id)
        return
    if node_type == "job":
        await _require_job_owner(pool, auth, node_id)
        return


class BulkImportBody(BaseModel):
    """Payload for bulk imports across resource types.

    Attributes:
        format: Input format, json or csv.
        data: CSV string data when format is csv.
        items: JSON items when format is json.
        defaults: Default values applied to each item.
    """

    format: str = "json"
    data: str | None = None
    items: list[dict[str, Any]] | None = None
    defaults: dict[str, Any] | None = None


async def _run_import(
    request: Request,
    auth: dict[str, Any],
    payload: BulkImportBody,
    normalizer: Callable[[dict[str, Any], dict[str, Any] | None], dict[str, Any]],
    executor: Callable[..., Any],
    approval_action: str,
) -> dict[str, Any]:
    """Run a bulk import with normalization and approval gating.

    Args:
        request: FastAPI request.
        auth: Auth context.
        payload: Bulk import payload.
        normalizer: Normalizer function for items.
        executor: Executor to persist normalized items.
        approval_action: Approval action name for audit/approval workflow.

    Returns:
        API response with created items and errors.
    """
    pool = request.app.state.pool
    enums = request.app.state.enums

    try:
        items = extract_items(payload.format, payload.data, payload.items)
    except ValueError as exc:
        api_error("VALIDATION_ERROR", str(exc), 400)
    allowed_scopes = scope_names_from_ids(auth.get("scopes", []), enums)
    if auth["caller_type"] == "agent" and auth["agent"].get("requires_approval", True):
        agent = auth["agent"]
        try:
            await ensure_approval_capacity(pool, agent["id"], len(items))
        except ValueError as exc:
            return JSONResponse(
                status_code=429,
                content={"status": "rate_limited", "message": str(exc)},
            )
        approvals: list[dict[str, Any]] = []
        errors: list[dict[str, Any]] = []
        for idx, item in enumerate(items, start=1):
            try:
                normalized = normalizer(item, payload.defaults)
                if "scopes" in normalized:
                    normalized["scopes"] = enforce_scope_subset(
                        normalized["scopes"], allowed_scopes
                    )
                if approval_action == "bulk_import_jobs" and not _is_admin(auth, enums):
                    normalized["agent_id"] = auth.get("agent_id")
                if approval_action == "bulk_import_relationships":
                    await _validate_relationship_node(
                        pool,
                        enums,
                        auth,
                        normalized.get("source_type", ""),
                        normalized.get("source_id", ""),
                    )
                    await _validate_relationship_node(
                        pool,
                        enums,
                        auth,
                        normalized.get("target_type", ""),
                        normalized.get("target_id", ""),
                    )
                _validate_taxonomy_before_approval(approval_action, enums, normalized)
                approval = await create_approval_request(
                    pool,
                    agent["id"],
                    approval_action,
                    normalized,
                )
                approvals.append({"row": idx, "approval_id": str(approval["id"])})
            except Exception as exc:
                errors.append({"row": idx, "error": str(exc)})
        return JSONResponse(
            status_code=202,
            content={
                "status": "approval_required",
                "created": 0,
                "failed": len(errors),
                "errors": errors,
                "approvals": approvals,
            },
        )

    if resp := await maybe_check_agent_approval(
        pool, auth, approval_action, {"format": payload.format, "items": items}
    ):
        return resp

    created: list[dict[str, Any]] = []
    errors: list[dict[str, Any]] = []

    async with pool.acquire() as conn:
        async with conn.transaction():
            if auth["caller_type"] == "user":
                await conn.execute(
                    "SELECT set_config('app.changed_by_type', $1, true)", "entity"
                )
                await conn.execute(
                    "SELECT set_config('app.changed_by_id', $1, true)",
                    str(auth["entity_id"]),
                )
            else:
                await conn.execute(
                    "SELECT set_config('app.changed_by_type', $1, true)", "agent"
                )
                await conn.execute(
                    "SELECT set_config('app.changed_by_id', $1, true)",
                    str(auth["agent_id"]),
                )

            for idx, item in enumerate(items, start=1):
                try:
                    normalized = normalizer(item, payload.defaults)
                    if "scopes" in normalized:
                        normalized["scopes"] = enforce_scope_subset(
                            normalized["scopes"], allowed_scopes
                        )
                    if approval_action == "bulk_import_jobs" and not _is_admin(
                        auth, enums
                    ):
                        normalized["agent_id"] = auth.get("agent_id")
                    if approval_action == "bulk_import_relationships":
                        await _validate_relationship_node(
                            conn,
                            enums,
                            auth,
                            normalized.get("source_type", ""),
                            normalized.get("source_id", ""),
                        )
                        await _validate_relationship_node(
                            conn,
                            enums,
                            auth,
                            normalized.get("target_type", ""),
                            normalized.get("target_id", ""),
                        )
                    result = await executor(conn, enums, normalized)
                    created.append(result)
                except Exception as exc:
                    errors.append({"row": idx, "error": str(exc)})

    return success(
        {
            "created": len(created),
            "failed": len(errors),
            "errors": errors,
            "items": created,
        }
    )


@router.post("/entities")
async def import_entities(
    payload: BulkImportBody,
    request: Request,
    auth: dict = Depends(require_auth),
) -> dict[str, Any]:
    """Bulk import entities.

    Args:
        payload: Bulk import payload.
        request: FastAPI request.
        auth: Auth context.

    Returns:
        API response with created entities or approval requirement.
    """
    return await _run_import(
        request,
        auth,
        payload,
        normalize_entity,
        execute_create_entity,
        "bulk_import_entities",
    )


@router.post("/knowledge")
async def import_knowledge(
    payload: BulkImportBody,
    request: Request,
    auth: dict = Depends(require_auth),
) -> dict[str, Any]:
    """Bulk import knowledge items.

    Args:
        payload: Bulk import payload.
        request: FastAPI request.
        auth: Auth context.

    Returns:
        API response with created knowledge or approval requirement.
    """
    return await _run_import(
        request,
        auth,
        payload,
        normalize_knowledge,
        execute_create_knowledge,
        "bulk_import_knowledge",
    )


@router.post("/relationships")
async def import_relationships(
    payload: BulkImportBody,
    request: Request,
    auth: dict = Depends(require_auth),
) -> dict[str, Any]:
    """Bulk import relationships.

    Args:
        payload: Bulk import payload.
        request: FastAPI request.
        auth: Auth context.

    Returns:
        API response with created relationships or approval requirement.
    """
    return await _run_import(
        request,
        auth,
        payload,
        normalize_relationship,
        execute_create_relationship,
        "bulk_import_relationships",
    )


@router.post("/jobs")
async def import_jobs(
    payload: BulkImportBody,
    request: Request,
    auth: dict = Depends(require_auth),
) -> dict[str, Any]:
    """Bulk import jobs.

    Args:
        payload: Bulk import payload.
        request: FastAPI request.
        auth: Auth context.

    Returns:
        API response with created jobs or approval requirement.
    """
    return await _run_import(
        request,
        auth,
        payload,
        normalize_job,
        execute_create_job,
        "bulk_import_jobs",
    )
