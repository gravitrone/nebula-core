"""Nebula MCP Server."""

# Standard Library
import json
import sys
from collections.abc import AsyncIterator
from contextlib import asynccontextmanager
from pathlib import Path
from typing import Any, Callable
from uuid import UUID

# Third-Party
from asyncpg import Pool, UniqueViolationError
from dotenv import load_dotenv
from mcp.server.fastmcp import Context, FastMCP

# Module Path Bootstrap
if __package__ in (None, ""):
    sys.path.append(str(Path(__file__).resolve().parents[1]))

# Local
from nebula_mcp.context import (
    authenticate_agent_optional,
    authenticate_agent_with_key,
    maybe_require_approval,
    require_context,
)
from nebula_mcp.db import get_pool
from nebula_mcp.enums import (
    load_enums,
    require_entity_type,
    require_log_type,
    require_relationship_type,
    require_scopes,
    require_status,
)
from nebula_mcp.executors import (
    execute_create_entity,
    execute_create_file,
    execute_create_job,
    execute_create_knowledge,
    execute_create_log,
    execute_create_protocol,
    execute_create_relationship,
    execute_update_entity,
    execute_update_log,
    execute_update_protocol,
)
from nebula_mcp.helpers import (
    bulk_update_entity_scopes as do_bulk_update_entity_scopes,
)
from nebula_mcp.helpers import (
    bulk_update_entity_tags as do_bulk_update_entity_tags,
)
from nebula_mcp.helpers import (
    create_approval_request,
    create_enrollment_session,
    enforce_scope_subset,
    ensure_approval_capacity,
    filter_context_segments,
    normalize_bulk_operation,
    redeem_enrollment_key,
    scope_names_from_ids,
    wait_for_enrollment_status,
)
from nebula_mcp.helpers import (
    get_approval_diff as compute_approval_diff,
)
from nebula_mcp.helpers import (
    get_entity_history as fetch_entity_history,
)
from nebula_mcp.helpers import (
    query_audit_log as fetch_audit_log,
)
from nebula_mcp.helpers import (
    revert_entity as do_revert_entity,
)
from nebula_mcp.imports import (
    extract_items,
    normalize_entity,
    normalize_job,
    normalize_knowledge,
    normalize_relationship,
)
from nebula_mcp.models import (
    MAX_GRAPH_HOPS,
    MAX_PAGE_LIMIT,
    AgentAuthAttachInput,
    AgentEnrollRedeemInput,
    AgentEnrollStartInput,
    AgentEnrollWaitInput,
    AttachFileInput,
    BulkImportInput,
    BulkUpdateEntityScopesInput,
    BulkUpdateEntityTagsInput,
    CreateEntityInput,
    CreateFileInput,
    CreateJobInput,
    CreateKnowledgeInput,
    CreateLogInput,
    CreateProtocolInput,
    CreateRelationshipInput,
    CreateSubtaskInput,
    CreateTaxonomyInput,
    GetAgentInfoInput,
    GetApprovalDiffInput,
    GetEntityHistoryInput,
    GetEntityInput,
    GetFileInput,
    GetJobInput,
    GetLogInput,
    GetProtocolInput,
    GetRelationshipsInput,
    GraphNeighborsInput,
    GraphShortestPathInput,
    LinkKnowledgeInput,
    ListAgentsInput,
    ListTaxonomyInput,
    QueryAuditLogInput,
    QueryEntitiesInput,
    QueryFilesInput,
    QueryJobsInput,
    QueryKnowledgeInput,
    QueryLogsInput,
    QueryRelationshipsInput,
    RevertEntityInput,
    SearchEntitiesByMetadataInput,
    SemanticSearchInput,
    ToggleTaxonomyInput,
    UpdateEntityInput,
    UpdateJobStatusInput,
    UpdateLogInput,
    UpdateProtocolInput,
    UpdateRelationshipInput,
    UpdateTaxonomyInput,
)
from nebula_mcp.query_loader import QueryLoader
from nebula_mcp.schema import load_schema_contract
from nebula_mcp.semantic import rank_semantic_candidates

QUERIES = QueryLoader(Path(__file__).resolve().parents[1] / "queries")

load_dotenv()

ADMIN_SCOPES = {"admin"}
JOB_PRIORITY_VALUES = {"low", "medium", "high", "critical"}

TAXONOMY_KIND_MAP = {
    "scopes": {
        "list": "taxonomy/list_scopes",
        "create": "taxonomy/create_scope",
        "update": "taxonomy/update_scope",
        "set_active": "taxonomy/set_scope_active",
        "usage": "taxonomy/count_scope_usage",
        "supports": set(),
    },
    "entity-types": {
        "list": "taxonomy/list_entity_types",
        "create": "taxonomy/create_entity_type",
        "update": "taxonomy/update_entity_type",
        "set_active": "taxonomy/set_entity_type_active",
        "usage": "taxonomy/count_entity_type_usage",
        "supports": set(),
    },
    "relationship-types": {
        "list": "taxonomy/list_relationship_types",
        "create": "taxonomy/create_relationship_type",
        "update": "taxonomy/update_relationship_type",
        "set_active": "taxonomy/set_relationship_type_active",
        "usage": "taxonomy/count_relationship_type_usage",
        "supports": {"is_symmetric"},
    },
    "log-types": {
        "list": "taxonomy/list_log_types",
        "create": "taxonomy/create_log_type",
        "update": "taxonomy/update_log_type",
        "set_active": "taxonomy/set_log_type_active",
        "usage": "taxonomy/count_log_type_usage",
        "supports": {"value_schema"},
    },
}

TAXONOMY_ROW_QUERY_MAP = {
    "scopes": "SELECT id, name, is_builtin FROM privacy_scopes WHERE id = $1::uuid",
    "entity-types": "SELECT id, name, is_builtin FROM entity_types WHERE id = $1::uuid",
    "relationship-types": (
        "SELECT id, name, is_builtin FROM relationship_types WHERE id = $1::uuid"
    ),
    "log-types": "SELECT id, name, is_builtin FROM log_types WHERE id = $1::uuid",
}


def _clamp_limit(value: int) -> int:
    if value < 1:
        return 1
    if value > MAX_PAGE_LIMIT:
        return MAX_PAGE_LIMIT
    return value


def _clamp_hops(value: int) -> int:
    if value < 1:
        return 1
    if value > MAX_GRAPH_HOPS:
        return MAX_GRAPH_HOPS
    return value


def _require_uuid(value: str, label: str) -> None:
    try:
        UUID(str(value))
    except (TypeError, ValueError):
        raise ValueError(f"Invalid {label} id")


def _require_admin(agent: dict, enums: Any) -> None:
    scope_names = scope_names_from_ids(agent.get("scopes", []), enums)
    if not any(scope in ADMIN_SCOPES for scope in scope_names):
        raise ValueError("Admin scope required")


def _is_admin(agent: dict, enums: Any) -> bool:
    scope_names = scope_names_from_ids(agent.get("scopes", []), enums)
    return any(scope in ADMIN_SCOPES for scope in scope_names)


def _scope_filter_ids(agent: dict, enums: Any) -> list | None:
    if _is_admin(agent, enums):
        return None
    return agent.get("scopes", []) or []


def _entity_semantic_candidate(row: dict[str, Any]) -> dict[str, Any]:
    metadata = row.get("metadata") or {}
    tags = row.get("tags") or []
    text = " ".join(
        [
            str(row.get("name", "")),
            str(row.get("type", "")),
            " ".join(str(t) for t in tags),
            json.dumps(metadata, sort_keys=True),
        ]
    ).strip()
    subtitle = str(row.get("type", "") or "entity")
    snippet_parts = [subtitle]
    if tags:
        snippet_parts.append(", ".join(str(t) for t in tags[:3]))
    return {
        "kind": "entity",
        "id": str(row.get("id", "")),
        "title": str(row.get("name", "")),
        "subtitle": subtitle,
        "snippet": " · ".join(part for part in snippet_parts if part),
        "text": text,
    }


def _knowledge_semantic_candidate(row: dict[str, Any]) -> dict[str, Any]:
    metadata = row.get("metadata") or {}
    tags = row.get("tags") or []
    content = str(row.get("content") or "")
    text = " ".join(
        [
            str(row.get("title", "")),
            str(row.get("source_type", "")),
            content,
            " ".join(str(t) for t in tags),
            json.dumps(metadata, sort_keys=True),
        ]
    ).strip()
    subtitle = str(row.get("source_type", "") or "knowledge")
    snippet_base = content.strip().replace("\n", " ")
    if len(snippet_base) > 120:
        snippet_base = snippet_base[:120].rstrip() + "..."
    snippet_parts = [subtitle]
    if snippet_base:
        snippet_parts.append(snippet_base)
    return {
        "kind": "knowledge",
        "id": str(row.get("id", "")),
        "title": str(row.get("title", "")),
        "subtitle": subtitle,
        "snippet": " · ".join(part for part in snippet_parts if part),
        "text": text,
    }


def _taxonomy_kind_or_error(kind: str) -> dict[str, Any]:
    cfg = TAXONOMY_KIND_MAP.get(kind)
    if cfg is None:
        raise ValueError(f"Unknown taxonomy kind: {kind}")
    return cfg


async def _get_taxonomy_row(pool: Any, kind: str, item_id: str) -> dict | None:
    query = TAXONOMY_ROW_QUERY_MAP[kind]
    row = await pool.fetchrow(
        query,
        item_id,
    )
    return dict(row) if row else None


def _validate_taxonomy_payload(
    kind: str,
    *,
    is_symmetric: bool | None,
    value_schema: dict | None,
) -> None:
    supports = _taxonomy_kind_or_error(kind)["supports"]
    if is_symmetric is not None and "is_symmetric" not in supports:
        raise ValueError("is_symmetric is only valid for relationship-types")
    if value_schema is not None and "value_schema" not in supports:
        raise ValueError("value_schema is only valid for log-types")


async def _refresh_enums_in_context(ctx: Context, pool: Pool) -> None:
    enums = await load_enums(pool)
    ctx.request_context.lifespan_context["enums"] = enums


async def _taxonomy_usage_count(pool: Pool, cfg: dict[str, Any], item_id: str) -> int:
    value = await pool.fetchval(QUERIES[cfg["usage"]], item_id)
    if value is None:
        return 0
    return int(value)


def _has_write_scopes(agent_scopes: list, node_scopes: list) -> bool:
    if not node_scopes:
        return True
    if not agent_scopes:
        return False
    return set(node_scopes).issubset(set(agent_scopes))


async def _get_job_row(pool: Pool, job_id: str) -> dict:
    row = await pool.fetchrow(QUERIES["jobs/get"], job_id)
    if not row:
        raise ValueError(f"Job '{job_id}' not found")
    return dict(row)


def _require_job_owner(agent: dict, enums: Any, job: dict) -> None:
    _require_job_write(agent, enums, job)


def _require_job_read(agent: dict, enums: Any, job: dict) -> None:
    if _is_admin(agent, enums):
        return
    job_scopes = job.get("privacy_scope_ids") or []
    agent_scopes = agent.get("scopes", []) or []
    if job_scopes and not any(scope in agent_scopes for scope in job_scopes):
        raise ValueError("Job not in your scopes")


def _require_job_write(agent: dict, enums: Any, job: dict) -> None:
    if _is_admin(agent, enums):
        return
    _require_job_read(agent, enums, job)
    if job.get("agent_id") != agent.get("id"):
        raise ValueError("Access denied")


async def _require_entity_write_access(
    pool: Pool, enums: Any, agent: dict, entity_ids: list[str]
) -> None:
    for entity_id in entity_ids:
        _require_uuid(entity_id, "entity")
    if _is_admin(agent, enums):
        return
    if not entity_ids:
        return
    rows = await pool.fetch(QUERIES["entities/scopes_by_ids"], entity_ids)
    if len(rows) != len(set(entity_ids)):
        raise ValueError("Entity not found")
    agent_scopes = agent.get("scopes", []) or []
    for row in rows:
        if not _has_write_scopes(agent_scopes, row.get("privacy_scope_ids") or []):
            raise ValueError("Access denied")


async def _has_hidden_relationships(
    pool: Pool, enums: Any, agent: dict, node_type: str, node_id: str
) -> bool:
    node_id = str(node_id)
    if _is_admin(agent, enums):
        return False
    scope_ids = agent.get("scopes", []) or []
    all_rows = await pool.fetch(
        QUERIES["relationships/get"], node_type, node_id, "both", None, None
    )
    if not all_rows:
        return False
    scoped_rows = await pool.fetch(
        QUERIES["relationships/get"], node_type, node_id, "both", None, scope_ids
    )
    if node_type in {"file", "log"}:
        for rel in all_rows:
            for side in ("source", "target"):
                rel_type = rel[f"{side}_type"]
                rel_id = rel[f"{side}_id"]
                if rel_type == "entity":
                    row = await pool.fetchrow(QUERIES["entities/get_by_id"], rel_id)
                    if not row:
                        return True
                    scopes = row.get("privacy_scope_ids") or []
                    if scopes and not any(s in scope_ids for s in scopes):
                        return True
                if rel_type == "knowledge":
                    row = await pool.fetchrow(QUERIES["knowledge/get"], rel_id, None)
                    if not row:
                        return True
                    scopes = row.get("privacy_scope_ids") or []
                    if scopes and not any(s in scope_ids for s in scopes):
                        return True
                if rel_type == "job":
                    job_row = await pool.fetchrow(QUERIES["jobs/get"], rel_id)
                    if not job_row:
                        return True
                    try:
                        _require_job_read(agent, enums, dict(job_row))
                    except ValueError:
                        return True
        return False

    if len(scoped_rows) < len(all_rows):
        return True
    for rel in all_rows:
        if rel["source_type"] == "job" or rel["target_type"] == "job":
            job_id = (
                rel["source_id"] if rel["source_type"] == "job" else rel["target_id"]
            )
            job_row = await pool.fetchrow(QUERIES["jobs/get"], job_id)
            if not job_row:
                return True
            try:
                _require_job_read(agent, enums, dict(job_row))
            except ValueError:
                return True
    return False


async def _node_allowed(
    pool: Pool, enums: Any, agent: dict, node_type: str, node_id: str
) -> bool:
    if _is_admin(agent, enums):
        return True
    if node_type == "entity":
        row = await pool.fetchrow(QUERIES["entities/get_by_id"], node_id)
        if not row:
            return False
        scopes = row.get("privacy_scope_ids") or []
        if not scopes:
            return True
        return any(s in agent.get("scopes", []) for s in scopes)
    if node_type == "knowledge":
        scope_ids = agent.get("scopes", []) or []
        row = await pool.fetchrow(QUERIES["knowledge/get"], node_id, scope_ids)
        return row is not None
    if node_type == "job":
        row = await pool.fetchrow(QUERIES["jobs/get"], node_id)
        if not row:
            return False
        try:
            _require_job_read(agent, enums, dict(row))
        except ValueError:
            return False
        return True
    if node_type in {"file", "log"}:
        return not await _has_hidden_relationships(
            pool, enums, agent, node_type, node_id
        )
    return True


async def _validate_relationship_node(
    pool: Pool,
    enums: Any,
    agent: dict,
    node_type: str,
    node_id: str,
    label: str,
    require_write: bool = False,
) -> None:
    if node_type in {"entity", "knowledge", "job", "file", "log"}:
        _require_uuid(node_id, label.lower())
    if node_type == "entity":
        row = await pool.fetchrow(QUERIES["entities/get_by_id"], node_id)
        if not row:
            raise ValueError(f"{label} entity not found")
        if require_write:
            if not _has_write_scopes(
                agent.get("scopes", []), row.get("privacy_scope_ids") or []
            ):
                raise ValueError("Access denied")
        elif not await _node_allowed(pool, enums, agent, node_type, node_id):
            raise ValueError("Access denied")
        return
    if node_type == "knowledge":
        scope_ids = None if require_write else _scope_filter_ids(agent, enums)
        row = await pool.fetchrow(QUERIES["knowledge/get"], node_id, scope_ids)
        if not row:
            raise ValueError(f"{label} knowledge not found")
        if require_write and not _has_write_scopes(
            agent.get("scopes", []), row.get("privacy_scope_ids") or []
        ):
            raise ValueError("Access denied")
        return
    if node_type == "job":
        row = await pool.fetchrow(QUERIES["jobs/get"], node_id)
        if not row:
            raise ValueError(f"{label} job not found")
        job = dict(row)
        if require_write:
            _require_job_write(agent, enums, job)
        else:
            _require_job_read(agent, enums, job)
        return
    raise ValueError(f"Unsupported {label.lower()} type")


@asynccontextmanager
async def lifespan(app: FastMCP) -> AsyncIterator[dict[str, Any]]:
    """Initialize and teardown shared application resources.

    Args:
        app: FastMCP application instance.

    Yields:
        Dict with pool, enums, and agent/bootstrap context.
    """

    pool = await get_pool()
    try:
        enums = await load_enums(pool)
        agent, bootstrap_mode = await authenticate_agent_optional(pool, enums)
        yield {
            "pool": pool,
            "enums": enums,
            "agent": agent,
            "bootstrap_mode": bootstrap_mode,
        }
    finally:
        await pool.close()


mcp = FastMCP("NebulaMCP", json_response=True, lifespan=lifespan)


async def _run_bulk_import(
    payload: BulkImportInput,
    ctx: Context,
    normalizer: Callable[[dict[str, Any], dict[str, Any] | None], dict[str, Any]],
    executor: Callable[..., Any],
    action: str,
) -> dict:
    """Run a bulk import with normalization and approval gating.

    Args:
        payload: Bulk import request payload.
        ctx: MCP request context.
        normalizer: Callable that validates and normalizes item rows.
        executor: Callable that persists a normalized item.
        action: Approval action name for audit/approval workflow.

    Returns:
        Dict with created count, error list, and created items.
    """
    pool, enums, agent = await require_context(ctx)
    items = extract_items(payload.format, payload.data, payload.items)
    allowed_scopes = scope_names_from_ids(agent.get("scopes", []), enums)
    created: list[dict] = []
    errors: list[dict] = []

    def _validate_taxonomy_before_approval(normalized: dict[str, Any]) -> None:
        if action == "bulk_import_entities":
            require_entity_type(str(normalized.get("type") or ""), enums)
            require_status(str(normalized.get("status") or ""), enums)
            require_scopes(list(normalized.get("scopes") or []), enums)
            return
        if action == "bulk_import_knowledge":
            require_scopes(list(normalized.get("scopes") or []), enums)
            return
        if action == "bulk_import_relationships":
            require_relationship_type(
                str(normalized.get("relationship_type") or ""), enums
            )
            return
        if action == "bulk_import_jobs":
            priority = normalized.get("priority")
            if priority and str(priority) not in JOB_PRIORITY_VALUES:
                raise ValueError(f"Invalid priority: {priority}")
            return

    if agent.get("requires_approval", True):
        await ensure_approval_capacity(pool, agent["id"], len(items))
        approvals: list[dict] = []
        for idx, item in enumerate(items, start=1):
            try:
                normalized = normalizer(item, payload.defaults)
                if "scopes" in normalized:
                    normalized["scopes"] = enforce_scope_subset(
                        normalized["scopes"], allowed_scopes
                    )
                if action == "bulk_import_jobs" and not _is_admin(agent, enums):
                    normalized["agent_id"] = agent.get("id")
                if action == "bulk_import_relationships":
                    await _validate_relationship_node(
                        pool,
                        enums,
                        agent,
                        normalized.get("source_type", ""),
                        normalized.get("source_id", ""),
                        "Source",
                        require_write=True,
                    )
                    await _validate_relationship_node(
                        pool,
                        enums,
                        agent,
                        normalized.get("target_type", ""),
                        normalized.get("target_id", ""),
                        "Target",
                        require_write=True,
                    )
                _validate_taxonomy_before_approval(normalized)
                approval = await create_approval_request(
                    pool,
                    agent["id"],
                    action,
                    normalized,
                )
                approvals.append({"row": idx, "approval_id": str(approval["id"])})
            except Exception as exc:
                errors.append({"row": idx, "error": str(exc)})
        return {
            "created": 0,
            "failed": len(errors),
            "errors": errors,
            "approvals": approvals,
            "status": "approval_required",
        }

    async with pool.acquire() as conn:
        async with conn.transaction():
            await conn.execute(
                "SELECT set_config('app.changed_by_type', $1, true)", "agent"
            )
            await conn.execute(
                "SELECT set_config('app.changed_by_id', $1, true)", str(agent["id"])
            )

            for idx, item in enumerate(items, start=1):
                try:
                    normalized = normalizer(item, payload.defaults)
                    if "scopes" in normalized:
                        normalized["scopes"] = enforce_scope_subset(
                            normalized["scopes"], allowed_scopes
                        )
                    if action == "bulk_import_jobs" and not _is_admin(agent, enums):
                        normalized["agent_id"] = agent.get("id")
                    if action == "bulk_import_relationships":
                        await _validate_relationship_node(
                            conn,
                            enums,
                            agent,
                            normalized.get("source_type", ""),
                            normalized.get("source_id", ""),
                            "Source",
                            require_write=True,
                        )
                        await _validate_relationship_node(
                            conn,
                            enums,
                            agent,
                            normalized.get("target_type", ""),
                            normalized.get("target_id", ""),
                            "Target",
                            require_write=True,
                        )
                    result = await executor(conn, enums, normalized)
                    created.append(result)
                except Exception as exc:
                    errors.append({"row": idx, "error": str(exc)})

    return {
        "created": len(created),
        "failed": len(errors),
        "errors": errors,
        "items": created,
    }


# --- Schema Tools ---


@mcp.tool()
async def get_schema(ctx: Context) -> dict:
    """Return the canonical schema contract (active taxonomy + constraints)."""

    pool, _enums, _agent = await require_context(ctx)
    return await load_schema_contract(pool)


# --- Admin Tools ---


@mcp.tool()
async def reload_enums(ctx: Context) -> str:
    """Reload enum cache from the database.

    Args:
        ctx: MCP request context.

    Returns:
        Confirmation message.
    """

    pool, enums, agent = await require_context(ctx)
    _require_admin(agent, enums)
    await _refresh_enums_in_context(ctx, pool)
    return "Enums reloaded."


# --- Enrollment Tools ---


@mcp.tool()
async def agent_enroll_start(payload: AgentEnrollStartInput, ctx: Context) -> dict:
    """Start MCP-native agent enrollment in bootstrap mode."""

    pool, enums, agent = await require_context(ctx, allow_bootstrap=True)
    if agent is not None:
        raise ValueError("Agent already authenticated")

    requested_scopes = payload.requested_scopes or ["public"]
    requested_scope_ids = require_scopes(requested_scopes, enums)
    inactive_status_id = require_status("inactive", enums)

    async with pool.acquire() as conn:
        async with conn.transaction():
            existing = await conn.fetchrow(QUERIES["agents/check_name"], payload.name)
            if existing:
                raise ValueError(f"Agent '{payload.name}' already exists")

            created_agent = await conn.fetchrow(
                QUERIES["agents/create"],
                payload.name,
                payload.description,
                requested_scope_ids,
                payload.capabilities,
                inactive_status_id,
                payload.requested_requires_approval,
            )
            if not created_agent:
                raise ValueError("Failed to create enrollment agent")

            approval = await create_approval_request(
                pool,
                str(created_agent["id"]),
                "register_agent",
                {
                    "agent_id": str(created_agent["id"]),
                    "name": payload.name,
                    "description": payload.description,
                    "requested_scopes": requested_scopes,
                    "requested_requires_approval": payload.requested_requires_approval,
                    "capabilities": payload.capabilities,
                },
                conn=conn,
            )
            session = await create_enrollment_session(
                pool,
                agent_id=str(created_agent["id"]),
                approval_request_id=str(approval["id"]),
                requested_scope_ids=requested_scope_ids,
                requested_requires_approval=payload.requested_requires_approval,
                conn=conn,
            )

    return {
        "registration_id": str(session["id"]),
        "enrollment_token": session["enrollment_token"],
        "status": "pending_approval",
    }


@mcp.tool()
async def agent_enroll_wait(payload: AgentEnrollWaitInput, ctx: Context) -> dict:
    """Wait for enrollment approval status in bootstrap mode."""

    pool, _, agent = await require_context(ctx, allow_bootstrap=True)
    if agent is not None:
        raise ValueError("Agent already authenticated")

    _require_uuid(payload.registration_id, "registration")
    session = await wait_for_enrollment_status(
        pool,
        registration_id=payload.registration_id,
        enrollment_token=payload.enrollment_token,
        timeout_seconds=payload.timeout_seconds,
    )
    status = str(session.get("status") or "pending_approval")
    response: dict[str, Any] = {
        "status": status,
        "can_redeem": status == "approved",
    }
    if status == "pending_approval":
        response["retry_after_ms"] = int(session.get("retry_after_ms") or 1000)
    elif status == "rejected":
        reason = session.get("rejected_reason") or session.get("review_notes")
        if reason:
            response["reason"] = str(reason)
    elif status == "expired":
        response["reason"] = "Enrollment expired"
    return response


@mcp.tool()
async def agent_enroll_redeem(payload: AgentEnrollRedeemInput, ctx: Context) -> dict:
    """Redeem an approved enrollment session for an API key once."""

    pool, enums, agent = await require_context(ctx, allow_bootstrap=True)
    if agent is not None:
        raise ValueError("Agent already authenticated")

    _require_uuid(payload.registration_id, "registration")
    result = await redeem_enrollment_key(
        pool,
        registration_id=payload.registration_id,
        enrollment_token=payload.enrollment_token,
    )
    return {
        "api_key": result["api_key"],
        "agent_id": result["agent_id"],
        "agent_name": result["agent_name"],
        "scopes": scope_names_from_ids(result.get("scope_ids", []), enums),
        "requires_approval": bool(result.get("requires_approval", True)),
    }


@mcp.tool()
async def agent_auth_attach(payload: AgentAuthAttachInput, ctx: Context) -> dict:
    """Attach an API key and authenticate the current MCP session without restart."""

    pool, enums, agent = await require_context(ctx, allow_bootstrap=True)
    if agent is not None:
        raise ValueError("Agent already authenticated")

    authed_agent = await authenticate_agent_with_key(pool, payload.api_key)
    lifespan_ctx = ctx.request_context.lifespan_context
    if not lifespan_ctx:
        raise ValueError("Lifespan context not initialized")

    lifespan_ctx["agent"] = authed_agent
    lifespan_ctx["bootstrap_mode"] = False

    return {
        "status": "authenticated",
        "agent_id": str(authed_agent["id"]),
        "agent_name": authed_agent["name"],
        "scopes": scope_names_from_ids(authed_agent.get("scopes", []), enums),
        "requires_approval": bool(authed_agent.get("requires_approval", True)),
    }


@mcp.tool()
async def get_pending_approvals(ctx: Context) -> list[dict]:
    """Get pending approval requests for the authenticated agent.

    Args:
        ctx: MCP request context.

    Returns:
        List of pending approval request dicts.
    """

    pool, enums, agent = await require_context(ctx)

    rows = await pool.fetch(QUERIES["approvals/get_pending_by_agent"], agent["id"])
    return [dict(r) for r in rows]


@mcp.tool()
async def get_approval_diff(payload: GetApprovalDiffInput, ctx: Context) -> dict:
    """Compute diff for an approval request."""

    pool, enums, agent = await require_context(ctx)
    _require_uuid(payload.approval_id, "approval")
    row = await pool.fetchrow(QUERIES["approvals/get_request"], payload.approval_id)
    if not row:
        raise ValueError("Approval request not found")
    if not _is_admin(agent, enums) and row.get("requested_by") != agent.get("id"):
        raise ValueError("Access denied")
    return await compute_approval_diff(pool, payload.approval_id)


# --- Bulk Import Tools ---


@mcp.tool()
async def bulk_import_entities(payload: BulkImportInput, ctx: Context) -> dict:
    """Bulk import entities from CSV or JSON."""

    return await _run_bulk_import(
        payload,
        ctx,
        normalize_entity,
        execute_create_entity,
        "bulk_import_entities",
    )


@mcp.tool()
async def bulk_import_knowledge(payload: BulkImportInput, ctx: Context) -> dict:
    """Bulk import knowledge items from CSV or JSON."""

    return await _run_bulk_import(
        payload,
        ctx,
        normalize_knowledge,
        execute_create_knowledge,
        "bulk_import_knowledge",
    )


@mcp.tool()
async def bulk_import_relationships(payload: BulkImportInput, ctx: Context) -> dict:
    """Bulk import relationships from CSV or JSON."""

    return await _run_bulk_import(
        payload,
        ctx,
        normalize_relationship,
        execute_create_relationship,
        "bulk_import_relationships",
    )


@mcp.tool()
async def bulk_import_jobs(payload: BulkImportInput, ctx: Context) -> dict:
    """Bulk import jobs from CSV or JSON."""

    return await _run_bulk_import(
        payload,
        ctx,
        normalize_job,
        execute_create_job,
        "bulk_import_jobs",
    )


# --- Entity Tools ---


@mcp.tool()
async def get_entity(payload: GetEntityInput, ctx: Context) -> dict:
    """Retrieve entity by ID with privacy filtering.

    Args:
        payload: Input with entity_id.
        ctx: MCP request context.

    Returns:
        Entity dict with filtered metadata.

    Raises:
        ValueError: If entity not found or access denied.
    """

    pool, enums, agent = await require_context(ctx)

    _require_uuid(payload.entity_id, "entity")

    row = await pool.fetchrow(QUERIES["entities/get"], payload.entity_id)
    if not row:
        raise ValueError("Not found: not found")

    entity = dict(row)

    # Privacy check
    entity_scopes = entity.get("privacy_scope_ids", [])
    agent_scopes = agent.get("scopes", [])

    if entity_scopes and not any(s in agent_scopes for s in entity_scopes):
        raise ValueError("Access denied: entity not in agent scopes")

    # Filter context segments
    if entity.get("metadata"):
        scope_names = [enums.scopes.id_to_name.get(s, "") for s in agent_scopes]
        entity["metadata"] = filter_context_segments(entity["metadata"], scope_names)

    return entity


@mcp.tool()
async def query_entities(payload: QueryEntitiesInput, ctx: Context) -> list[dict]:
    """Search entities with filters and full-text search."""

    pool, enums, agent = await require_context(ctx)

    type_id = require_entity_type(payload.type, enums) if payload.type else None
    allowed_scopes = scope_names_from_ids(agent.get("scopes", []), enums)
    requested_scopes = enforce_scope_subset(payload.scopes, allowed_scopes)
    scope_ids = require_scopes(requested_scopes, enums)
    limit = _clamp_limit(payload.limit)
    offset = max(0, payload.offset)

    rows = await pool.fetch(
        QUERIES["entities/query"],
        type_id,
        payload.tags or None,
        payload.search_text,
        payload.status_category,
        scope_ids,
        limit,
        offset,
    )
    results = []
    for row in rows:
        entity = dict(row)
        if entity.get("metadata"):
            entity["metadata"] = filter_context_segments(
                entity["metadata"], allowed_scopes
            )
        results.append(entity)
    return results


@mcp.tool()
async def semantic_search(payload: SemanticSearchInput, ctx: Context) -> list[dict]:
    """Run semantic search across entities and knowledge for the active agent."""

    pool, enums, agent = await require_context(ctx)
    scope_ids = _scope_filter_ids(agent, enums)
    candidates: list[dict[str, Any]] = []

    if "entity" in payload.kinds:
        rows = await pool.fetch(
            QUERIES["search/entities_semantic_candidates"],
            scope_ids,
            payload.candidate_limit,
        )
        candidates.extend(_entity_semantic_candidate(dict(row)) for row in rows)

    if "knowledge" in payload.kinds:
        rows = await pool.fetch(
            QUERIES["search/knowledge_semantic_candidates"],
            scope_ids,
            payload.candidate_limit,
        )
        candidates.extend(_knowledge_semantic_candidate(dict(row)) for row in rows)

    ranked = rank_semantic_candidates(payload.query, candidates, limit=payload.limit)
    for item in ranked:
        item.pop("text", None)
    return ranked


@mcp.tool()
async def update_entity(payload: UpdateEntityInput, ctx: Context) -> dict:
    """Update entity metadata, tags, or status."""

    pool, enums, agent = await require_context(ctx)

    _require_uuid(payload.entity_id, "entity")

    row = await pool.fetchrow(QUERIES["entities/get_by_id"], payload.entity_id)
    if not row:
        raise ValueError("Entity not found")
    entity = dict(row)
    if not _has_write_scopes(
        agent.get("scopes", []), entity.get("privacy_scope_ids") or []
    ):
        raise ValueError("Access denied")

    if payload.status is not None:
        require_status(payload.status, enums)

    if resp := await maybe_require_approval(
        pool, agent, "update_entity", payload.model_dump()
    ):
        return resp

    return await execute_update_entity(pool, enums, payload.model_dump())


@mcp.tool()
async def bulk_update_entity_tags(
    payload: BulkUpdateEntityTagsInput, ctx: Context
) -> dict:
    """Bulk update entity tags."""

    pool, enums, agent = await require_context(ctx)
    await _require_entity_write_access(pool, enums, agent, payload.entity_ids)

    if resp := await maybe_require_approval(
        pool, agent, "bulk_update_entity_tags", payload.model_dump()
    ):
        return resp

    op = normalize_bulk_operation(payload.op)
    updated = await do_bulk_update_entity_tags(
        pool, payload.entity_ids, payload.tags, op
    )
    return {"updated": len(updated), "entity_ids": updated}


@mcp.tool()
async def bulk_update_entity_scopes(
    payload: BulkUpdateEntityScopesInput, ctx: Context
) -> dict:
    """Bulk update entity privacy scopes."""

    pool, enums, agent = await require_context(ctx)
    await _require_entity_write_access(pool, enums, agent, payload.entity_ids)

    op = normalize_bulk_operation(payload.op)
    allowed_scopes = scope_names_from_ids(agent.get("scopes", []), enums)
    requested_scopes = enforce_scope_subset(payload.scopes, allowed_scopes)
    require_scopes(requested_scopes, enums)
    data = payload.model_dump()
    data["scopes"] = requested_scopes
    if resp := await maybe_require_approval(
        pool, agent, "bulk_update_entity_scopes", data
    ):
        return resp
    updated = await do_bulk_update_entity_scopes(
        pool, enums, payload.entity_ids, requested_scopes, op
    )
    return {"updated": len(updated), "entity_ids": updated}


@mcp.tool()
async def search_entities_by_metadata(
    payload: SearchEntitiesByMetadataInput, ctx: Context
) -> list[dict]:
    """Search entities by JSONB metadata containment."""

    pool, enums, agent = await require_context(ctx)
    scope_ids = agent.get("scopes", [])
    limit = _clamp_limit(payload.limit)
    scope_names = scope_names_from_ids(scope_ids, enums)

    rows = await pool.fetch(
        QUERIES["entities/search_by_metadata"],
        json.dumps(payload.metadata_query),
        limit,
        scope_ids,
    )
    results = []
    for row in rows:
        entity = dict(row)
        if entity.get("metadata"):
            entity["metadata"] = filter_context_segments(
                entity["metadata"], scope_names
            )
        results.append(entity)
    return results


@mcp.tool()
async def create_entity(payload: CreateEntityInput, ctx: Context) -> dict:
    """Create an entity in the Nebula database."""

    pool, enums, agent = await require_context(ctx)
    allowed_scopes = scope_names_from_ids(agent.get("scopes", []), enums)
    requested_scopes = enforce_scope_subset(payload.scopes, allowed_scopes)
    data = payload.model_dump()
    data["scopes"] = requested_scopes

    require_entity_type(payload.type, enums)
    require_status(payload.status, enums)
    require_scopes(requested_scopes, enums)

    if resp := await maybe_require_approval(pool, agent, "create_entity", data):
        return resp

    return await execute_create_entity(pool, enums, data)


@mcp.tool()
async def get_entity_history(
    payload: GetEntityHistoryInput, ctx: Context
) -> list[dict]:
    """List audit history entries for an entity."""

    pool, enums, agent = await require_context(ctx)
    limit = _clamp_limit(payload.limit)
    offset = max(0, payload.offset)
    if not await _node_allowed(pool, enums, agent, "entity", payload.entity_id):
        raise ValueError("Access denied")
    return await fetch_entity_history(pool, payload.entity_id, limit, offset)


@mcp.tool()
async def query_audit_log(payload: QueryAuditLogInput, ctx: Context) -> list[dict]:
    """Query audit log entries with filters."""

    pool, enums, agent = await require_context(ctx)
    _require_admin(agent, enums)
    limit = _clamp_limit(payload.limit)
    offset = max(0, payload.offset)
    if payload.actor_id:
        _require_uuid(payload.actor_id, "actor")
    if payload.record_id:
        _require_uuid(payload.record_id, "record")
    if payload.scope_id:
        _require_uuid(payload.scope_id, "scope")
    return await fetch_audit_log(
        pool,
        payload.table_name,
        payload.action,
        payload.actor_type,
        payload.actor_id,
        payload.record_id,
        payload.scope_id,
        limit,
        offset,
    )


@mcp.tool()
async def revert_entity(payload: RevertEntityInput, ctx: Context) -> dict:
    """Revert an entity to a historical audit entry."""

    pool, enums, agent = await require_context(ctx)
    try:
        UUID(str(payload.audit_id))
    except ValueError as exc:
        raise ValueError("Invalid audit id") from exc
    audit_row = await pool.fetchrow(QUERIES["audit/get"], payload.audit_id)
    if not audit_row:
        raise ValueError("Audit entry not found")
    if audit_row.get("table_name") != "entities":
        raise ValueError("Audit entry is not for entities")
    if audit_row.get("record_id") != payload.entity_id:
        raise ValueError("Audit entry does not match entity")

    if resp := await maybe_require_approval(
        pool, agent, "revert_entity", payload.model_dump()
    ):
        return resp

    async with pool.acquire() as conn:
        await conn.execute("SET app.changed_by_type = 'agent'")
        await conn.execute("SET app.changed_by_id = $1", agent["id"])
        try:
            return await do_revert_entity(conn, payload.entity_id, payload.audit_id)
        finally:
            await conn.execute("RESET app.changed_by_type")
            await conn.execute("RESET app.changed_by_id")


# --- Knowledge Tools ---


@mcp.tool()
async def create_knowledge(payload: CreateKnowledgeInput, ctx: Context) -> dict:
    """Create a knowledge item."""

    pool, enums, agent = await require_context(ctx)
    allowed_scopes = scope_names_from_ids(agent.get("scopes", []), enums)
    requested_scopes = enforce_scope_subset(payload.scopes, allowed_scopes)
    data = payload.model_dump()
    data["scopes"] = requested_scopes
    if data.get("url"):
        url = data["url"].strip()
        if not (url.startswith("http://") or url.startswith("https://")):
            raise ValueError("URL must start with http:// or https://")
        data["url"] = url

    require_scopes(requested_scopes, enums)
    if resp := await maybe_require_approval(pool, agent, "create_knowledge", data):
        return resp

    return await execute_create_knowledge(pool, enums, data)


@mcp.tool()
async def query_knowledge(payload: QueryKnowledgeInput, ctx: Context) -> list[dict]:
    """Search knowledge items with filters."""

    pool, enums, agent = await require_context(ctx)

    allowed_scopes = scope_names_from_ids(agent.get("scopes", []), enums)
    requested_scopes = enforce_scope_subset(payload.scopes, allowed_scopes)
    scope_ids = require_scopes(requested_scopes, enums)
    limit = _clamp_limit(payload.limit)
    offset = max(0, payload.offset)

    rows = await pool.fetch(
        QUERIES["knowledge/query"],
        payload.source_type,
        payload.tags or None,
        payload.search_text,
        scope_ids,
        limit,
        offset,
    )
    scope_names = scope_names_from_ids(scope_ids, enums)
    results = []
    for row in rows:
        item = dict(row)
        if item.get("metadata"):
            item["metadata"] = filter_context_segments(item["metadata"], scope_names)
        results.append(item)
    return results


@mcp.tool()
async def link_knowledge_to_entity(payload: LinkKnowledgeInput, ctx: Context) -> dict:
    """Link knowledge item to entity via relationship."""

    pool, enums, agent = await require_context(ctx)

    _require_uuid(payload.knowledge_id, "knowledge")
    _require_uuid(payload.entity_id, "entity")

    await _validate_relationship_node(
        pool,
        enums,
        agent,
        "knowledge",
        payload.knowledge_id,
        "Source",
        require_write=True,
    )
    await _validate_relationship_node(
        pool,
        enums,
        agent,
        "entity",
        payload.entity_id,
        "Target",
        require_write=True,
    )
    relationship_payload = {
        "source_type": "knowledge",
        "source_id": payload.knowledge_id,
        "target_type": "entity",
        "target_id": payload.entity_id,
        "relationship_type": payload.relationship_type,
        "properties": {},
    }
    require_relationship_type(payload.relationship_type, enums)
    if resp := await maybe_require_approval(
        pool, agent, "create_relationship", relationship_payload
    ):
        return resp

    return await execute_create_relationship(pool, enums, relationship_payload)


# --- Log Tools ---


@mcp.tool()
async def create_log(payload: CreateLogInput, ctx: Context) -> dict:
    """Create a log entry."""

    pool, enums, agent = await require_context(ctx)
    require_log_type(payload.log_type, enums)
    require_status(payload.status, enums)
    if resp := await maybe_require_approval(
        pool, agent, "create_log", payload.model_dump()
    ):
        return resp
    return await execute_create_log(pool, enums, payload.model_dump())


@mcp.tool()
async def get_log(payload: GetLogInput, ctx: Context) -> dict:
    """Retrieve a log entry by id."""

    pool, enums, agent = await require_context(ctx)
    _require_uuid(payload.log_id, "log")
    row = await pool.fetchrow(QUERIES["logs/get"], payload.log_id)
    if not row:
        raise ValueError("Log not found")
    if await _has_hidden_relationships(pool, enums, agent, "log", payload.log_id):
        raise ValueError("Access denied")
    return dict(row)


@mcp.tool()
async def query_logs(payload: QueryLogsInput, ctx: Context) -> list[dict]:
    """Query log entries."""

    pool, enums, agent = await require_context(ctx)
    log_type_id = None
    if payload.log_type:
        log_type_id = require_log_type(payload.log_type, enums)
    tags = payload.tags if payload.tags else None
    limit = _clamp_limit(payload.limit)
    offset = max(0, payload.offset)
    rows = await pool.fetch(
        QUERIES["logs/query"],
        log_type_id,
        tags,
        payload.status_category,
        limit,
        offset,
    )
    results = []
    for row in rows:
        if await _has_hidden_relationships(pool, enums, agent, "log", str(row["id"])):
            continue
        results.append(dict(row))
    return results


@mcp.tool()
async def update_log(payload: UpdateLogInput, ctx: Context) -> dict:
    """Update a log entry."""

    pool, enums, agent = await require_context(ctx)
    _require_uuid(payload.id, "log")
    if await _has_hidden_relationships(pool, enums, agent, "log", payload.id):
        raise ValueError("Access denied")
    if payload.log_type is not None:
        require_log_type(payload.log_type, enums)
    if payload.status is not None:
        require_status(payload.status, enums)
    if resp := await maybe_require_approval(
        pool, agent, "update_log", payload.model_dump()
    ):
        return resp
    return await execute_update_log(pool, enums, payload.model_dump())


# --- Relationship Tools ---


@mcp.tool()
async def create_relationship(payload: CreateRelationshipInput, ctx: Context) -> dict:
    """Create a polymorphic relationship between items."""

    pool, enums, agent = await require_context(ctx)
    await _validate_relationship_node(
        pool,
        enums,
        agent,
        payload.source_type,
        payload.source_id,
        "Source",
        require_write=True,
    )
    await _validate_relationship_node(
        pool,
        enums,
        agent,
        payload.target_type,
        payload.target_id,
        "Target",
        require_write=True,
    )
    require_relationship_type(payload.relationship_type, enums)

    if resp := await maybe_require_approval(
        pool, agent, "create_relationship", payload.model_dump()
    ):
        return resp

    return await execute_create_relationship(pool, enums, payload.model_dump())


@mcp.tool()
async def get_relationships(payload: GetRelationshipsInput, ctx: Context) -> list[dict]:
    """Get relationships for an item with direction filter."""

    pool, enums, agent = await require_context(ctx)
    scope_ids = _scope_filter_ids(agent, enums)

    _require_uuid(payload.source_id, "source")

    rows = await pool.fetch(
        QUERIES["relationships/get"],
        payload.source_type,
        payload.source_id,
        payload.direction,
        payload.relationship_type,
        scope_ids,
    )
    results = []
    for row in rows:
        if row["source_type"] == "job":
            if not await _node_allowed(pool, enums, agent, "job", row["source_id"]):
                continue
        if row["target_type"] == "job":
            if not await _node_allowed(pool, enums, agent, "job", row["target_id"]):
                continue
        results.append(dict(row))
    return results


@mcp.tool()
async def query_relationships(
    payload: QueryRelationshipsInput, ctx: Context
) -> list[dict]:
    """Search relationships with filters."""

    pool, enums, agent = await require_context(ctx)
    limit = _clamp_limit(payload.limit)
    scope_ids = _scope_filter_ids(agent, enums)

    rows = await pool.fetch(
        QUERIES["relationships/query"],
        payload.source_type,
        payload.target_type,
        payload.relationship_types or None,
        payload.status_category,
        limit,
        scope_ids,
    )
    results = []
    for row in rows:
        if row["source_type"] == "job":
            if not await _node_allowed(pool, enums, agent, "job", row["source_id"]):
                continue
        if row["target_type"] == "job":
            if not await _node_allowed(pool, enums, agent, "job", row["target_id"]):
                continue
        results.append(dict(row))
    return results


@mcp.tool()
async def update_relationship(payload: UpdateRelationshipInput, ctx: Context) -> dict:
    """Update relationship properties or status."""

    pool, enums, agent = await require_context(ctx)

    _require_uuid(payload.relationship_id, "relationship")

    row = await pool.fetchrow(
        QUERIES["relationships/get_by_id"], payload.relationship_id
    )
    if not row:
        raise ValueError("Relationship not found")

    relationship = dict(row)
    await _validate_relationship_node(
        pool,
        enums,
        agent,
        relationship["source_type"],
        relationship["source_id"],
        "Source",
        require_write=True,
    )
    await _validate_relationship_node(
        pool,
        enums,
        agent,
        relationship["target_type"],
        relationship["target_id"],
        "Target",
        require_write=True,
    )

    status_id = require_status(payload.status, enums) if payload.status else None
    if resp := await maybe_require_approval(
        pool, agent, "update_relationship", payload.model_dump()
    ):
        return resp

    row = await pool.fetchrow(
        QUERIES["relationships/update"],
        payload.relationship_id,
        json.dumps(payload.properties) if payload.properties else None,
        status_id,
    )
    return dict(row) if row else {}


# --- Graph Tools ---


def _decode_graph_path(path: list[str] | None) -> list[dict]:
    """Decode a graph path list into typed node dictionaries.

    Args:
        path: List of encoded path entries in the form "type:id".

    Returns:
        A list of node dictionaries with "type" and "id".
    """
    if not path:
        return []

    nodes: list[dict] = []
    for item in path:
        if ":" not in item:
            continue
        node_type, node_id = item.split(":", 1)
        nodes.append({"type": node_type, "id": node_id})
    return nodes


@mcp.tool()
async def graph_neighbors(payload: GraphNeighborsInput, ctx: Context) -> list[dict]:
    """Return neighbors within max hops."""

    pool, enums, agent = await require_context(ctx)
    max_hops = _clamp_hops(payload.max_hops)
    limit = _clamp_limit(payload.limit)
    _require_uuid(payload.source_id, "source")

    if not await _node_allowed(
        pool, enums, agent, payload.source_type, payload.source_id
    ):
        raise ValueError("Access denied")

    rows = await pool.fetch(
        QUERIES["graph/neighbors"],
        payload.source_type,
        payload.source_id,
        max_hops,
        limit,
    )
    results = []
    for row in rows:
        path_nodes = _decode_graph_path(row["path"])
        allowed = True
        for node in path_nodes:
            if not await _node_allowed(pool, enums, agent, node["type"], node["id"]):
                allowed = False
                break
        if not allowed:
            continue
        results.append(
            {
                "node_type": row["node_type"],
                "node_id": row["node_id"],
                "depth": row["depth"],
                "path": path_nodes,
            }
        )
    return results


@mcp.tool()
async def graph_shortest_path(payload: GraphShortestPathInput, ctx: Context) -> dict:
    """Return shortest path between two nodes."""

    pool, enums, agent = await require_context(ctx)
    max_hops = _clamp_hops(payload.max_hops)
    _require_uuid(payload.source_id, "source")
    _require_uuid(payload.target_id, "target")

    if not await _node_allowed(
        pool, enums, agent, payload.source_type, payload.source_id
    ):
        raise ValueError("Access denied")
    if not await _node_allowed(
        pool, enums, agent, payload.target_type, payload.target_id
    ):
        raise ValueError("Access denied")

    row = await pool.fetchrow(
        QUERIES["graph/shortest_path"],
        payload.source_type,
        payload.source_id,
        payload.target_type,
        payload.target_id,
        max_hops,
    )
    if not row:
        raise ValueError("No path found")

    path_nodes = _decode_graph_path(row["path"])
    for node in path_nodes:
        if not await _node_allowed(pool, enums, agent, node["type"], node["id"]):
            raise ValueError("No path found")

    return {"depth": row["depth"], "path": path_nodes}


# --- Job Tools ---


@mcp.tool()
async def create_job(payload: CreateJobInput, ctx: Context) -> dict:
    """Create a new job with auto-generated ID."""

    pool, enums, agent = await require_context(ctx)
    data = payload.model_dump()
    if not data.get("scopes"):
        data["scopes"] = ["public"]
    if data.get("priority") and data["priority"] not in JOB_PRIORITY_VALUES:
        raise ValueError(f"Invalid priority: {data['priority']}")
    if not _is_admin(agent, enums):
        allowed_scopes = scope_names_from_ids(agent.get("scopes", []), enums)
        data["scopes"] = enforce_scope_subset(data["scopes"], allowed_scopes)
    if not _is_admin(agent, enums):
        agent_id = agent.get("id")
        data["agent_id"] = str(agent_id) if agent_id else None
    require_scopes(data["scopes"], enums)

    if resp := await maybe_require_approval(pool, agent, "create_job", data):
        return resp

    return await execute_create_job(pool, enums, data)


@mcp.tool()
async def get_job(payload: GetJobInput, ctx: Context) -> dict:
    """Retrieve job by ID."""

    pool, enums, agent = await require_context(ctx)

    job = await _get_job_row(pool, payload.job_id)
    _require_job_read(agent, enums, job)
    return job


@mcp.tool()
async def query_jobs(payload: QueryJobsInput, ctx: Context) -> list[dict]:
    """Search jobs with multiple filters."""

    pool, enums, agent = await require_context(ctx)
    limit = _clamp_limit(payload.limit)
    if payload.assigned_to:
        _require_uuid(payload.assigned_to, "assignee")
    scope_filter = _scope_filter_ids(agent, enums)

    rows = await pool.fetch(
        QUERIES["jobs/query"],
        payload.status_names or None,
        payload.assigned_to,
        payload.agent_id,
        payload.priority,
        payload.due_before,
        payload.due_after,
        payload.overdue_only,
        payload.parent_job_id,
        scope_filter,
        limit,
    )
    return [dict(r) for r in rows]


@mcp.tool()
async def update_job_status(payload: UpdateJobStatusInput, ctx: Context) -> dict:
    """Update job status with optional completion timestamp."""

    pool, enums, agent = await require_context(ctx)
    job = await _get_job_row(pool, payload.job_id)
    _require_job_owner(agent, enums, job)
    status_id = require_status(payload.status, enums)
    if resp := await maybe_require_approval(
        pool, agent, "update_job_status", payload.model_dump()
    ):
        return resp

    row = await pool.fetchrow(
        QUERIES["jobs/update_status"],
        payload.job_id,
        status_id,
        payload.status_reason,
        payload.completed_at,
    )
    if not row:
        raise ValueError(f"Job '{payload.job_id}' not found")

    return dict(row)


@mcp.tool()
async def create_subtask(payload: CreateSubtaskInput, ctx: Context) -> dict:
    """Create a subtask under a parent job."""

    pool, enums, agent = await require_context(ctx)
    parent_job = await _get_job_row(pool, payload.parent_job_id)
    _require_job_owner(agent, enums, parent_job)
    parent_scopes = scope_names_from_ids(
        parent_job.get("privacy_scope_ids") or [], enums
    )
    subtask_payload = {
        "title": payload.title,
        "description": payload.description,
        "job_type": None,
        "assigned_to": None,
        "agent_id": parent_job.get("agent_id"),
        "priority": payload.priority,
        "scopes": parent_scopes or ["public"],
        "parent_job_id": payload.parent_job_id,
        "due_at": payload.due_at,
        "metadata": {},
    }
    if subtask_payload.get("priority") not in JOB_PRIORITY_VALUES:
        raise ValueError(f"Invalid priority: {subtask_payload['priority']}")
    require_scopes(subtask_payload["scopes"], enums)
    if resp := await maybe_require_approval(pool, agent, "create_job", subtask_payload):
        return resp
    return await execute_create_job(pool, enums, subtask_payload)


# --- File Tools ---


@mcp.tool()
async def create_file(payload: CreateFileInput, ctx: Context) -> dict:
    """Create a file metadata record."""

    pool, enums, agent = await require_context(ctx)

    require_status(payload.status, enums)
    if resp := await maybe_require_approval(
        pool, agent, "create_file", payload.model_dump()
    ):
        return resp

    return await execute_create_file(pool, enums, payload.model_dump())


@mcp.tool()
async def get_file(payload: GetFileInput, ctx: Context) -> dict:
    """Retrieve a file by ID."""

    pool, enums, agent = await require_context(ctx)

    _require_uuid(payload.file_id, "file")

    row = await pool.fetchrow(QUERIES["files/get"], payload.file_id)
    if not row:
        raise ValueError("File not found")
    if await _has_hidden_relationships(pool, enums, agent, "file", payload.file_id):
        raise ValueError("Access denied")

    return dict(row)


@mcp.tool()
async def list_files(payload: QueryFilesInput, ctx: Context) -> list[dict]:
    """List files with filters."""

    pool, enums, agent = await require_context(ctx)
    limit = _clamp_limit(payload.limit)
    offset = max(0, payload.offset)

    rows = await pool.fetch(
        QUERIES["files/list"],
        payload.tags or None,
        payload.mime_type,
        payload.status_category,
        limit,
        offset,
    )
    results = []
    for row in rows:
        if await _has_hidden_relationships(pool, enums, agent, "file", row["id"]):
            continue
        results.append(dict(row))
    return results


async def _attach_file(
    ctx: Context, target_type: str, payload: AttachFileInput
) -> dict:
    """Attach a file to a target entity or knowledge item.

    Args:
        ctx: MCP request context.
        target_type: Target type for relationship (entity or knowledge).
        payload: File attachment request payload.

    Returns:
        Relationship record or approval response payload.
    """
    pool, enums, agent = await require_context(ctx)

    file_row = await pool.fetchrow(QUERIES["files/get"], payload.file_id)
    if not file_row:
        raise ValueError("File not found")
    if await _has_hidden_relationships(pool, enums, agent, "file", payload.file_id):
        raise ValueError("Access denied")
    await _validate_relationship_node(
        pool,
        enums,
        agent,
        target_type,
        payload.target_id,
        "Target",
        require_write=True,
    )
    require_relationship_type(payload.relationship_type, enums)

    relationship = {
        "source_type": "file",
        "source_id": payload.file_id,
        "target_type": target_type,
        "target_id": payload.target_id,
        "relationship_type": payload.relationship_type,
        "properties": {},
    }

    if resp := await maybe_require_approval(
        pool,
        agent,
        "create_relationship",
        relationship,
    ):
        return resp

    return await execute_create_relationship(pool, enums, relationship)


@mcp.tool()
async def attach_file_to_entity(payload: AttachFileInput, ctx: Context) -> dict:
    """Attach a file to an entity."""

    return await _attach_file(ctx, "entity", payload)


@mcp.tool()
async def attach_file_to_knowledge(payload: AttachFileInput, ctx: Context) -> dict:
    """Attach a file to a knowledge item."""

    return await _attach_file(ctx, "knowledge", payload)


@mcp.tool()
async def attach_file_to_job(payload: AttachFileInput, ctx: Context) -> dict:
    """Attach a file to a job."""

    return await _attach_file(ctx, "job", payload)


# --- Protocol Tools ---


@mcp.tool()
async def get_protocol(payload: GetProtocolInput, ctx: Context) -> dict:
    """Retrieve protocol by name."""

    pool, enums, agent = await require_context(ctx)

    row = await pool.fetchrow(QUERIES["protocols/get"], payload.protocol_name)
    if not row:
        raise ValueError(f"Protocol '{payload.protocol_name}' not found")
    if row.get("trusted") and not _is_admin(agent, enums):
        raise ValueError("Access denied")

    return dict(row)


@mcp.tool()
async def create_protocol(payload: CreateProtocolInput, ctx: Context) -> dict:
    """Create a protocol."""

    pool, enums, agent = await require_context(ctx)
    data = payload.model_dump()
    if not _is_admin(agent, enums):
        data["trusted"] = False
    require_status(data["status"], enums)
    if resp := await maybe_require_approval(pool, agent, "create_protocol", data):
        return resp
    return await execute_create_protocol(pool, enums, data)


@mcp.tool()
async def update_protocol(payload: UpdateProtocolInput, ctx: Context) -> dict:
    """Update a protocol."""

    pool, enums, agent = await require_context(ctx)
    data = payload.model_dump()
    if data.get("status") is not None:
        require_status(str(data["status"]), enums)
    if not _is_admin(agent, enums) and data.get("trusted") is not None:
        data["trusted"] = False
    if resp := await maybe_require_approval(pool, agent, "update_protocol", data):
        return resp
    return await execute_update_protocol(pool, enums, data)


@mcp.tool()
async def list_active_protocols(ctx: Context) -> list[dict]:
    """List all active protocols."""

    pool, enums, agent = await require_context(ctx)
    rows = await pool.fetch(QUERIES["protocols/list_active"])
    items = [dict(r) for r in rows]
    if not _is_admin(agent, enums):
        items = [item for item in items if not item.get("trusted")]
    return items


# --- Agent Tools ---


@mcp.tool()
async def get_agent_info(payload: GetAgentInfoInput, ctx: Context) -> dict:
    """Retrieve agent configuration including system_prompt."""

    pool, enums, agent = await require_context(ctx)
    _require_admin(agent, enums)

    row = await pool.fetchrow(QUERIES["agents/get_info"], payload.name)
    if not row:
        raise ValueError(f"Agent '{payload.name}' not found")

    return dict(row)


@mcp.tool()
async def list_agents(payload: ListAgentsInput, ctx: Context) -> list[dict]:
    """List agents by status category."""

    pool, enums, agent = await require_context(ctx)
    _require_admin(agent, enums)
    rows = await pool.fetch(QUERIES["agents/list"], payload.status_category)
    return [dict(r) for r in rows]


# --- Taxonomy Tools ---


@mcp.tool()
async def list_taxonomy(payload: ListTaxonomyInput, ctx: Context) -> list[dict]:
    """List taxonomy rows by kind."""

    pool, enums, agent = await require_context(ctx)
    _require_admin(agent, enums)
    cfg = _taxonomy_kind_or_error(payload.kind)
    rows = await pool.fetch(
        QUERIES[cfg["list"]],
        payload.include_inactive,
        payload.search,
        _clamp_limit(payload.limit),
        max(0, payload.offset),
    )
    return [dict(r) for r in rows]


@mcp.tool()
async def create_taxonomy(payload: CreateTaxonomyInput, ctx: Context) -> dict:
    """Create a taxonomy row by kind."""

    pool, enums, agent = await require_context(ctx)
    _require_admin(agent, enums)
    cfg = _taxonomy_kind_or_error(payload.kind)
    _validate_taxonomy_payload(
        payload.kind,
        is_symmetric=payload.is_symmetric,
        value_schema=payload.value_schema,
    )

    name = payload.name.strip()
    if not name:
        raise ValueError("Taxonomy name required")

    try:
        if payload.kind in {"scopes", "entity-types"}:
            row = await pool.fetchrow(
                QUERIES[cfg["create"]],
                name,
                payload.description,
                json.dumps(payload.metadata or {}),
            )
        elif payload.kind == "relationship-types":
            row = await pool.fetchrow(
                QUERIES[cfg["create"]],
                name,
                payload.description,
                payload.is_symmetric,
                json.dumps(payload.metadata or {}),
            )
        else:
            row = await pool.fetchrow(
                QUERIES[cfg["create"]],
                name,
                payload.description,
                (
                    json.dumps(payload.value_schema)
                    if payload.value_schema is not None
                    else None
                ),
            )
    except UniqueViolationError as exc:
        raise ValueError(f"{payload.kind} entry already exists") from exc

    await _refresh_enums_in_context(ctx, pool)
    return dict(row)


@mcp.tool()
async def update_taxonomy(payload: UpdateTaxonomyInput, ctx: Context) -> dict:
    """Update a taxonomy row by kind."""

    pool, enums, agent = await require_context(ctx)
    _require_admin(agent, enums)
    cfg = _taxonomy_kind_or_error(payload.kind)
    _validate_taxonomy_payload(
        payload.kind,
        is_symmetric=payload.is_symmetric,
        value_schema=payload.value_schema,
    )
    _require_uuid(payload.item_id, "taxonomy")
    current = await _get_taxonomy_row(pool, payload.kind, payload.item_id)
    if current is None:
        raise ValueError(f"{payload.kind} entry not found")

    name = payload.name.strip() if payload.name is not None else None
    if payload.name is not None and not name:
        raise ValueError("Taxonomy name cannot be empty")
    if current["is_builtin"] and name is not None and name != current["name"]:
        raise ValueError("Built-in taxonomy names are immutable")

    try:
        if payload.kind in {"scopes", "entity-types"}:
            row = await pool.fetchrow(
                QUERIES[cfg["update"]],
                payload.item_id,
                name,
                payload.description,
                json.dumps(payload.metadata)
                if payload.metadata is not None
                else None,
            )
        elif payload.kind == "relationship-types":
            row = await pool.fetchrow(
                QUERIES[cfg["update"]],
                payload.item_id,
                name,
                payload.description,
                payload.is_symmetric,
                json.dumps(payload.metadata)
                if payload.metadata is not None
                else None,
            )
        else:
            row = await pool.fetchrow(
                QUERIES[cfg["update"]],
                payload.item_id,
                name,
                payload.description,
                json.dumps(payload.value_schema)
                if payload.value_schema is not None
                else None,
            )
    except UniqueViolationError as exc:
        raise ValueError(f"{payload.kind} entry already exists") from exc

    if not row:
        raise ValueError(f"{payload.kind} entry not found")
    await _refresh_enums_in_context(ctx, pool)
    return dict(row)


@mcp.tool()
async def archive_taxonomy(payload: ToggleTaxonomyInput, ctx: Context) -> dict:
    """Archive a taxonomy row by kind."""

    pool, enums, agent = await require_context(ctx)
    _require_admin(agent, enums)
    cfg = _taxonomy_kind_or_error(payload.kind)
    _require_uuid(payload.item_id, "taxonomy")

    usage = await _taxonomy_usage_count(pool, cfg, payload.item_id)
    if usage > 0:
        raise ValueError(
            "Cannot archive "
            f"{payload.kind} entry while it is referenced ({usage} records)"
        )

    row = await pool.fetchrow(QUERIES[cfg["set_active"]], payload.item_id, False)
    if not row:
        raise ValueError(
            f"{payload.kind} entry not found or cannot archive built-in entry"
        )
    await _refresh_enums_in_context(ctx, pool)
    return dict(row)


@mcp.tool()
async def activate_taxonomy(payload: ToggleTaxonomyInput, ctx: Context) -> dict:
    """Activate a taxonomy row by kind."""

    pool, enums, agent = await require_context(ctx)
    _require_admin(agent, enums)
    cfg = _taxonomy_kind_or_error(payload.kind)
    _require_uuid(payload.item_id, "taxonomy")

    row = await pool.fetchrow(QUERIES[cfg["set_active"]], payload.item_id, True)
    if not row:
        raise ValueError(f"{payload.kind} entry not found")
    await _refresh_enums_in_context(ctx, pool)
    return dict(row)


# --- Main ---


def main() -> None:
    """Run the Nebula MCP server."""

    load_dotenv()
    mcp.run()


if __name__ == "__main__":
    main()
