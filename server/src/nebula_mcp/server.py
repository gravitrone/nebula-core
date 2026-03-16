"""Nebula MCP Server."""

import json
import re
import sys
from collections.abc import AsyncIterator, Callable
from contextlib import asynccontextmanager
from pathlib import Path
from typing import Any
from uuid import UUID

from asyncpg import Pool, UniqueViolationError
from dotenv import load_dotenv
from mcp.server.fastmcp import Context, FastMCP

# Module Path Bootstrap
if __package__ in (None, ""):
    sys.path.append(str(Path(__file__).resolve().parents[1]))

from nebula_api.auth import generate_api_key
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
    execute_create_context,
    execute_create_entity,
    execute_create_file,
    execute_create_job,
    execute_create_log,
    execute_create_protocol,
    execute_create_relationship,
    execute_update_context,
    execute_update_entity,
    execute_update_file,
    execute_update_job,
    execute_update_log,
    execute_update_protocol,
)
from nebula_mcp.helpers import (
    approve_request as do_approve,
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
    normalize_bulk_operation,
    redeem_enrollment_key,
    sanitize_relationship_properties,
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
    get_pending_approvals_all as fetch_pending_approvals_all,
)
from nebula_mcp.helpers import (
    list_audit_actors as fetch_audit_actors,
)
from nebula_mcp.helpers import (
    list_audit_scopes as fetch_audit_scopes,
)
from nebula_mcp.helpers import (
    query_audit_log as fetch_audit_log,
)
from nebula_mcp.helpers import (
    reject_request as do_reject,
)
from nebula_mcp.helpers import (
    revert_entity as do_revert_entity,
)
from nebula_mcp.imports import (
    extract_items,
    normalize_context,
    normalize_entity,
    normalize_job,
    normalize_relationship,
)
from nebula_mcp.models import (
    MAX_GRAPH_HOPS,
    MAX_PAGE_LIMIT,
    AgentAuthAttachInput,
    AgentEnrollRedeemInput,
    AgentEnrollStartInput,
    AgentEnrollWaitInput,
    ApproveRequestInput,
    AttachFileInput,
    BulkImportInput,
    BulkUpdateEntityScopesInput,
    BulkUpdateEntityTagsInput,
    CreateAPIKeyInput,
    CreateContextInput,
    CreateEntityInput,
    CreateFileInput,
    CreateJobInput,
    CreateLogInput,
    CreateProtocolInput,
    CreateRelationshipInput,
    CreateSubtaskInput,
    CreateTaxonomyInput,
    ExportDataInput,
    GetAgentInfoInput,
    GetAgentInput,
    GetApprovalDiffInput,
    GetApprovalInput,
    GetContextInput,
    GetEntityHistoryInput,
    GetEntityInput,
    GetFileInput,
    GetJobInput,
    GetLogInput,
    GetProtocolInput,
    GetRelationshipsInput,
    GraphNeighborsInput,
    GraphShortestPathInput,
    LinkContextInput,
    ListAgentsInput,
    ListAllKeysInput,
    ListAuditActorsInput,
    ListContextByOwnerInput,
    ListTaxonomyInput,
    LoginInput,
    PendingApprovalsInput,
    QueryAuditLogInput,
    QueryContextInput,
    QueryEntitiesInput,
    QueryFilesInput,
    QueryJobsInput,
    QueryLogsInput,
    QueryProtocolsInput,
    QueryRelationshipsInput,
    RejectRequestInput,
    RevertEntityInput,
    RevokeKeyInput,
    SemanticSearchInput,
    ToggleTaxonomyInput,
    UpdateAgentInput,
    UpdateContextInput,
    UpdateEntityInput,
    UpdateFileInput,
    UpdateJobInput,
    UpdateJobStatusInput,
    UpdateLogInput,
    UpdateProtocolInput,
    UpdateRelationshipInput,
    UpdateTaxonomyInput,
    parse_optional_datetime,
)
from nebula_mcp.query_loader import QueryLoader
from nebula_mcp.schema import load_export_schema_contract, load_schema_contract
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
    "scopes": "taxonomy/get_scope_row",
    "entity-types": "taxonomy/get_entity_type_row",
    "relationship-types": "taxonomy/get_relationship_type_row",
    "log-types": "taxonomy/get_log_type_row",
}


def _clamp_limit(value: int) -> int:
    """Handle clamp limit.

    Args:
        value: Input parameter for _clamp_limit.

    Returns:
        Result value from the operation.
    """

    if value < 1:
        return 1
    if value > MAX_PAGE_LIMIT:
        return MAX_PAGE_LIMIT
    return value


def _clamp_hops(value: int) -> int:
    """Handle clamp hops.

    Args:
        value: Input parameter for _clamp_hops.

    Returns:
        Result value from the operation.
    """

    if value < 1:
        return 1
    if value > MAX_GRAPH_HOPS:
        return MAX_GRAPH_HOPS
    return value


def _require_uuid(value: str, label: str) -> None:
    """Handle require uuid.

    Args:
        value: Input parameter for _require_uuid.
        label: Input parameter for _require_uuid.
    """

    try:
        UUID(str(value))
    except (TypeError, ValueError):
        raise ValueError(f"Invalid {label} id")


JOB_ID_PATTERN = re.compile(r"^\d{4}Q[1-4]-[A-Z2-9]{4}$")


def _require_job_id(value: str, label: str) -> None:
    """Handle require job id.

    Args:
        value: Input parameter for _require_job_id.
        label: Input parameter for _require_job_id.
    """

    if not JOB_ID_PATTERN.fullmatch(str(value).strip().upper()):
        raise ValueError(f"Invalid {label} id")


def _require_node_id(node_type: str, value: str, label: str) -> None:
    """Handle require node id.

    Args:
        node_type: Input parameter for _require_node_id.
        value: Input parameter for _require_node_id.
        label: Input parameter for _require_node_id.
    """

    if node_type == "job":
        _require_job_id(value, label)
        return
    _require_uuid(value, label)


def _require_admin(agent: dict, enums: Any) -> None:
    """Handle require admin.

    Args:
        agent: Input parameter for _require_admin.
        enums: Input parameter for _require_admin.
    """

    scope_names = scope_names_from_ids(agent.get("scopes", []), enums)
    if not any(scope in ADMIN_SCOPES for scope in scope_names):
        raise ValueError("Admin scope required")


def _flatten_csv_value(value: Any) -> str:
    """Handle flatten csv value.

    Args:
        value: Input parameter for _flatten_csv_value.

    Returns:
        Result value from the operation.
    """

    if value is None:
        return ""
    if isinstance(value, list):
        return ",".join(str(v) for v in value)
    if isinstance(value, dict):
        return json.dumps(value, sort_keys=True)
    return str(value)


def _rows_to_csv(rows: list[dict[str, Any]]) -> str:
    """Handle rows to csv.

    Args:
        rows: Input parameter for _rows_to_csv.

    Returns:
        Result value from the operation.
    """

    if not rows:
        return ""
    headers = list(rows[0].keys())
    lines = [",".join(headers)]
    for row in rows:
        values = []
        for header in headers:
            cell = _flatten_csv_value(row.get(header))
            escaped = cell.replace('"', '""')
            if any(ch in escaped for ch in {",", "\n", '"'}):
                escaped = f'"{escaped}"'
            values.append(escaped)
        lines.append(",".join(values))
    return "\n".join(lines)


def _export_response_rows(rows: list[dict[str, Any]], fmt: str) -> dict[str, Any]:
    """Handle export response rows.

    Args:
        rows: Input parameter for _export_response_rows.
        fmt: Input parameter for _export_response_rows.

    Returns:
        Result value from the operation.
    """

    if fmt == "csv":
        return {"format": "csv", "content": _rows_to_csv(rows), "count": len(rows)}
    return {"format": "json", "items": rows, "count": len(rows)}


def _resolve_scope_ids_for_export(agent: dict, enums: Any, scope_names: list[str]) -> list[Any]:
    """Handle resolve scope ids for export.

    Args:
        agent: Input parameter for _resolve_scope_ids_for_export.
        enums: Input parameter for _resolve_scope_ids_for_export.
        scope_names: Input parameter for _resolve_scope_ids_for_export.

    Returns:
        Result value from the operation.
    """

    caller_scope_ids = agent.get("scopes", []) or []
    if not scope_names:
        return caller_scope_ids
    allowed_scope_names = scope_names_from_ids(caller_scope_ids, enums)
    requested_scope_names = enforce_scope_subset(scope_names, allowed_scope_names)
    return require_scopes(requested_scope_names, enums)


def _is_admin(agent: dict, enums: Any) -> bool:
    """Handle is admin.

    Args:
        agent: Input parameter for _is_admin.
        enums: Input parameter for _is_admin.

    Returns:
        Result value from the operation.
    """

    scope_names = scope_names_from_ids(agent.get("scopes", []), enums)
    return any(scope in ADMIN_SCOPES for scope in scope_names)


def _scope_filter_ids(agent: dict, enums: Any) -> list | None:
    """Handle scope filter ids.

    Args:
        agent: Input parameter for _scope_filter_ids.
        enums: Input parameter for _scope_filter_ids.

    Returns:
        Result value from the operation.
    """

    if _is_admin(agent, enums):
        return None
    return agent.get("scopes", []) or []


def _visible_scope_names(agent: dict, enums: Any, scope_ids: list | None = None) -> list[str]:
    """Resolve caller-visible scope names, with admin seeing all scopes."""

    if _is_admin(agent, enums):
        return sorted(enums.scopes.name_to_id.keys())
    ids = scope_ids if scope_ids is not None else (agent.get("scopes", []) or [])
    return scope_names_from_ids(ids, enums)


def _normalize_relationship_row(row: Any, visible_scope_names: list[str]) -> dict[str, Any]:
    """Normalize relationship payload shape and scope-filter properties."""

    item = dict(row)
    item["properties"] = sanitize_relationship_properties(
        item.get("properties"), visible_scope_names
    )
    return item


def _entity_semantic_candidate(row: dict[str, Any]) -> dict[str, Any]:
    """Handle entity semantic candidate.

    Args:
        row: Input parameter for _entity_semantic_candidate.

    Returns:
        Result value from the operation.
    """

    tags = row.get("tags") or []
    text = " ".join(
        [
            str(row.get("name", "")),
            str(row.get("type", "")),
            " ".join(str(t) for t in tags),
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


def _context_semantic_candidate(row: dict[str, Any]) -> dict[str, Any]:
    """Handle context semantic candidate.

    Args:
        row: Input parameter for _context_semantic_candidate.

    Returns:
        Result value from the operation.
    """

    tags = row.get("tags") or []
    content = str(row.get("content") or "")
    text = " ".join(
        [
            str(row.get("title", "")),
            str(row.get("source_type", "")),
            content,
            " ".join(str(t) for t in tags),
        ]
    ).strip()
    subtitle = str(row.get("source_type", "") or "context")
    snippet_base = content.strip().replace("\n", " ")
    if len(snippet_base) > 120:
        snippet_base = snippet_base[:120].rstrip() + "..."
    snippet_parts = [subtitle]
    if snippet_base:
        snippet_parts.append(snippet_base)
    return {
        "kind": "context",
        "id": str(row.get("id", "")),
        "title": str(row.get("title", "")),
        "subtitle": subtitle,
        "snippet": " · ".join(part for part in snippet_parts if part),
        "text": text,
    }


def _taxonomy_kind_or_error(kind: str) -> dict[str, Any]:
    """Handle taxonomy kind or error.

    Args:
        kind: Input parameter for _taxonomy_kind_or_error.

    Returns:
        Result value from the operation.
    """

    cfg = TAXONOMY_KIND_MAP.get(kind)
    if cfg is None:
        raise ValueError(f"Unknown taxonomy kind: {kind}")
    return cfg


async def _get_taxonomy_row(pool: Any, kind: str, item_id: str) -> dict | None:
    """Handle get taxonomy row.

    Args:
        pool: Input parameter for _get_taxonomy_row.
        kind: Input parameter for _get_taxonomy_row.
        item_id: Input parameter for _get_taxonomy_row.

    Returns:
        Result value from the operation.
    """

    query = TAXONOMY_ROW_QUERY_MAP[kind]
    row = await pool.fetchrow(
        QUERIES[query],
        item_id,
    )
    return dict(row) if row else None


def _validate_taxonomy_payload(
    kind: str,
    *,
    is_symmetric: bool | None,
    value_schema: dict | None,
) -> None:
    """Handle validate taxonomy payload.

    Args:
        kind: Input parameter for _validate_taxonomy_payload.
        is_symmetric: Input parameter for _validate_taxonomy_payload.
        value_schema: Input parameter for _validate_taxonomy_payload.
    """

    supports = _taxonomy_kind_or_error(kind)["supports"]
    if is_symmetric is not None and "is_symmetric" not in supports:
        raise ValueError("is_symmetric is only valid for relationship-types")
    if value_schema is not None and "value_schema" not in supports:
        raise ValueError("value_schema is only valid for log-types")


async def _refresh_enums_in_context(ctx: Context, pool: Pool) -> None:
    """Handle refresh enums in context.

    Args:
        ctx: Input parameter for _refresh_enums_in_context.
        pool: Input parameter for _refresh_enums_in_context.
    """

    enums = await load_enums(pool)
    ctx.request_context.lifespan_context["enums"] = enums


async def _taxonomy_usage_count(pool: Pool, cfg: dict[str, Any], item_id: str) -> int:
    """Handle taxonomy usage count.

    Args:
        pool: Input parameter for _taxonomy_usage_count.
        cfg: Input parameter for _taxonomy_usage_count.
        item_id: Input parameter for _taxonomy_usage_count.

    Returns:
        Result value from the operation.
    """

    value = await pool.fetchval(QUERIES[cfg["usage"]], item_id)
    if value is None:
        return 0
    return int(value)


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


async def _get_job_row(pool: Pool, job_id: str) -> dict:
    """Handle get job row.

    Args:
        pool: Input parameter for _get_job_row.
        job_id: Input parameter for _get_job_row.

    Returns:
        Result value from the operation.
    """

    row = await pool.fetchrow(QUERIES["jobs/get"], job_id)
    if not row:
        raise ValueError(f"Job '{job_id}' not found")
    return dict(row)


def _require_job_owner(agent: dict, enums: Any, job: dict) -> None:
    """Handle require job owner.

    Args:
        agent: Input parameter for _require_job_owner.
        enums: Input parameter for _require_job_owner.
        job: Input parameter for _require_job_owner.
    """

    _require_job_write(agent, enums, job)


def _require_job_read(agent: dict, enums: Any, job: dict) -> None:
    """Handle require job read.

    Args:
        agent: Input parameter for _require_job_read.
        enums: Input parameter for _require_job_read.
        job: Input parameter for _require_job_read.
    """

    if _is_admin(agent, enums):
        return
    job_scopes = job.get("privacy_scope_ids") or []
    agent_scopes = agent.get("scopes", []) or []
    if job_scopes and not any(scope in agent_scopes for scope in job_scopes):
        raise ValueError("Job not in your scopes")


def _require_job_write(agent: dict, enums: Any, job: dict) -> None:
    """Handle require job write.

    Args:
        agent: Input parameter for _require_job_write.
        enums: Input parameter for _require_job_write.
        job: Input parameter for _require_job_write.
    """

    if _is_admin(agent, enums):
        return
    _require_job_read(agent, enums, job)
    if job.get("agent_id") != agent.get("id"):
        raise ValueError("Access denied")


async def _require_entity_write_access(
    pool: Pool, enums: Any, agent: dict, entity_ids: list[str]
) -> None:
    """Handle require entity write access.

    Args:
        pool: Input parameter for _require_entity_write_access.
        enums: Input parameter for _require_entity_write_access.
        agent: Input parameter for _require_entity_write_access.
        entity_ids: Input parameter for _require_entity_write_access.
    """

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
    """Handle has hidden relationships.

    Args:
        pool: Input parameter for _has_hidden_relationships.
        enums: Input parameter for _has_hidden_relationships.
        agent: Input parameter for _has_hidden_relationships.
        node_type: Input parameter for _has_hidden_relationships.
        node_id: Input parameter for _has_hidden_relationships.

    Returns:
        Result value from the operation.
    """

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
                if rel_type == "context":
                    row = await pool.fetchrow(QUERIES["context/get"], rel_id, None)
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
            job_id = rel["source_id"] if rel["source_type"] == "job" else rel["target_id"]
            job_row = await pool.fetchrow(QUERIES["jobs/get"], job_id)
            if not job_row:
                return True
            try:
                _require_job_read(agent, enums, dict(job_row))
            except ValueError:
                return True
    return False


async def _node_allowed(pool: Pool, enums: Any, agent: dict, node_type: str, node_id: str) -> bool:
    """Handle node allowed.

    Args:
        pool: Input parameter for _node_allowed.
        enums: Input parameter for _node_allowed.
        agent: Input parameter for _node_allowed.
        node_type: Input parameter for _node_allowed.
        node_id: Input parameter for _node_allowed.

    Returns:
        Result value from the operation.
    """

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
    if node_type == "context":
        scope_ids = agent.get("scopes", []) or []
        row = await pool.fetchrow(QUERIES["context/get"], node_id, scope_ids)
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
        return not await _has_hidden_relationships(pool, enums, agent, node_type, node_id)
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
    """Handle validate relationship node.

    Args:
        pool: Input parameter for _validate_relationship_node.
        enums: Input parameter for _validate_relationship_node.
        agent: Input parameter for _validate_relationship_node.
        node_type: Input parameter for _validate_relationship_node.
        node_id: Input parameter for _validate_relationship_node.
        label: Input parameter for _validate_relationship_node.
        require_write: Input parameter for _validate_relationship_node.
    """

    if node_type in {"entity", "context", "job", "file", "log"}:
        _require_node_id(node_type, node_id, label.lower())
    if node_type == "entity":
        row = await pool.fetchrow(QUERIES["entities/get_by_id"], node_id)
        if not row:
            raise ValueError(f"{label} entity not found")
        if require_write:
            if not _has_write_scopes(agent.get("scopes", []), row.get("privacy_scope_ids") or []):
                raise ValueError("Access denied")
        elif not await _node_allowed(pool, enums, agent, node_type, node_id):
            raise ValueError("Access denied")
        return
    if node_type == "context":
        scope_ids = None if require_write else _scope_filter_ids(agent, enums)
        row = await pool.fetchrow(QUERIES["context/get"], node_id, scope_ids)
        if not row:
            raise ValueError(f"{label} context not found")
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
        """Handle validate taxonomy before approval.

        Args:
            normalized: Input parameter for _validate_taxonomy_before_approval.
        """

        if action == "bulk_import_entities":
            require_entity_type(str(normalized.get("type") or ""), enums)
            require_status(str(normalized.get("status") or ""), enums)
            require_scopes(list(normalized.get("scopes") or []), enums)
            return
        if action == "bulk_import_context":
            require_scopes(list(normalized.get("scopes") or []), enums)
            return
        if action == "bulk_import_relationships":
            require_relationship_type(str(normalized.get("relationship_type") or ""), enums)
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

    async with pool.acquire() as conn, conn.transaction():
        await conn.execute(QUERIES["runtime/set_changed_by_type"], "agent")
        await conn.execute(QUERIES["runtime/set_changed_by_id"], str(agent["id"]))

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


@mcp.tool()
async def export_schema(ctx: Context) -> dict:
    """Return the export payload schema contract."""

    _pool, _enums, _agent = await require_context(ctx)
    return load_export_schema_contract()


@mcp.tool()
async def export_data(payload: ExportDataInput, ctx: Context) -> dict:
    """Export entities/context/relationships/jobs/snapshot as JSON or CSV."""

    pool, enums, agent = await require_context(ctx)
    params = payload.params or {}
    limit = min(max(int(params.get("limit", 500)), 1), 2000)
    offset = max(int(params.get("offset", 0)), 0)

    if payload.resource == "entities":
        type_name = params.get("type")
        type_id = require_entity_type(type_name, enums) if type_name else None
        tags = params.get("tags") or None
        search_text = params.get("search_text")
        status_category = str(params.get("status_category", "active"))
        scope_names = params.get("scopes") or []
        scope_ids = _resolve_scope_ids_for_export(agent, enums, scope_names)
        rows = await pool.fetch(
            QUERIES["entities/query"],
            type_id,
            tags,
            search_text,
            status_category,
            scope_ids,
            limit,
            offset,
        )
        out_rows = [dict(row) for row in rows]
        return _export_response_rows(out_rows, payload.format)

    if payload.resource == "context":
        source_type = params.get("source_type")
        tags = params.get("tags") or None
        search_text = params.get("search_text")
        scope_names = params.get("scopes") or []
        scope_ids = _resolve_scope_ids_for_export(agent, enums, scope_names)
        rows = await pool.fetch(
            QUERIES["context/query"],
            source_type,
            tags,
            search_text,
            scope_ids,
            limit,
            offset,
        )
        out_rows = [dict(row) for row in rows]
        return _export_response_rows(out_rows, payload.format)

    if payload.resource == "relationships":
        source_type = params.get("source_type")
        target_type = params.get("target_type")
        rel_types = params.get("relationship_types") or None
        status_category = str(params.get("status_category", "active"))
        scope_filter = _scope_filter_ids(agent, enums)
        visible_scope_names = _visible_scope_names(agent, enums, scope_filter)
        rows = await pool.fetch(
            QUERIES["relationships/query"],
            source_type,
            target_type,
            rel_types,
            status_category,
            limit,
            scope_filter,
        )
        out_rows: list[dict[str, Any]] = []
        for row in rows:
            if row["source_type"] == "job":
                if not await _node_allowed(pool, enums, agent, "job", row["source_id"]):
                    continue
            if row["target_type"] == "job":
                if not await _node_allowed(pool, enums, agent, "job", row["target_id"]):
                    continue
            out_rows.append(_normalize_relationship_row(row, visible_scope_names))
        return _export_response_rows(out_rows, payload.format)

    if payload.resource == "jobs":
        status_names = params.get("status_names") or None
        assigned_to = params.get("assigned_to")
        agent_id = params.get("agent_id")
        priority = params.get("priority")
        due_before = parse_optional_datetime(params.get("due_before"), "due_before")
        due_after = parse_optional_datetime(params.get("due_after"), "due_after")
        overdue_only = bool(params.get("overdue_only", False))
        parent_job_id = params.get("parent_job_id")
        scope_filter = _scope_filter_ids(agent, enums)
        rows = await pool.fetch(
            QUERIES["jobs/query"],
            status_names,
            assigned_to,
            agent_id,
            priority,
            due_before,
            due_after,
            overdue_only,
            parent_job_id,
            scope_filter,
            limit,
        )
        return _export_response_rows([dict(row) for row in rows], payload.format)

    snapshot: dict[str, Any] = {}
    for resource in ("entities", "context", "relationships", "jobs"):
        nested = ExportDataInput(resource=resource, format="json", params=params)
        exported = await export_data(nested, ctx)
        snapshot[resource] = exported.get("items", [])
    return {
        "format": "json",
        "items": snapshot,
        "count": sum(len(v) for v in snapshot.values()),
    }


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

    async with pool.acquire() as conn, conn.transaction():
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
async def login_user(payload: LoginInput, ctx: Context) -> dict:
    """Bootstrap user login and issue an entity API key."""

    pool, enums, agent = await require_context(ctx, allow_bootstrap=True)
    if agent is not None:
        raise ValueError("Agent already authenticated")

    baseline_scope_ids = require_scopes(["public", "private", "sensitive", "admin"], enums)
    entity = await pool.fetchrow(
        QUERIES["entities/find_by_name_type"],
        payload.username,
        require_entity_type("person", enums),
    )
    if not entity:
        status_id = require_status("active", enums)
        entity = await pool.fetchrow(
            QUERIES["entities/create"],
            baseline_scope_ids,
            payload.username,
            require_entity_type("person", enums),
            status_id,
            [],
            None,
        )
    else:
        scope_ids = list(entity.get("privacy_scope_ids") or [])
        for scope_id in baseline_scope_ids:
            if scope_id not in scope_ids:
                scope_ids.append(scope_id)
        if scope_ids != (entity.get("privacy_scope_ids") or []):
            entity = await pool.fetchrow(
                QUERIES["entities/update_scope_ids"], entity["id"], scope_ids
            )

    raw_key, prefix, key_hash = generate_api_key()
    await pool.execute(
        QUERIES["api_keys/create_for_entity"],
        entity["id"],
        key_hash,
        prefix,
        "default",
    )
    return {
        "api_key": raw_key,
        "entity_id": str(entity["id"]),
        "username": payload.username,
    }


@mcp.tool()
async def create_api_key(payload: CreateAPIKeyInput, ctx: Context) -> dict:
    """Create an API key for the active agent or a target entity (admin)."""

    pool, enums, agent = await require_context(ctx)

    raw_key, prefix, key_hash = generate_api_key()
    if payload.entity_id:
        _require_admin(agent, enums)
        _require_uuid(payload.entity_id, "entity")
        owner = await pool.fetchrow(QUERIES["entities/get_by_id"], payload.entity_id)
        if not owner:
            raise ValueError("Entity not found")
        row = await pool.fetchrow(
            QUERIES["api_keys/create_with_returning"],
            payload.entity_id,
            key_hash,
            prefix,
            payload.name,
        )
        if not row:
            raise ValueError("Failed to create API key")
    else:
        row = await pool.fetchrow(
            QUERIES["api_keys/create_for_agent_with_returning"],
            agent["id"],
            key_hash,
            prefix,
            payload.name,
        )
        if not row:
            raise ValueError("Failed to create API key")

    return {
        "api_key": raw_key,
        "key_id": str(row["id"]),
        "prefix": row["key_prefix"],
        "name": row["name"],
    }


@mcp.tool()
async def list_api_keys(ctx: Context) -> list[dict]:
    """List active API keys for the authenticated agent."""

    pool, _enums, agent = await require_context(ctx)
    rows = await pool.fetch(QUERIES["api_keys/list_by_agent"], agent["id"])
    return [dict(r) for r in rows]


@mcp.tool()
async def list_all_api_keys(payload: ListAllKeysInput, ctx: Context) -> list[dict]:
    """List all active API keys (admin)."""

    pool, enums, agent = await require_context(ctx)
    _require_admin(agent, enums)
    rows = await pool.fetch(QUERIES["api_keys/list_all"], payload.limit, payload.offset)
    return [dict(r) for r in rows]


@mcp.tool()
async def revoke_api_key(payload: RevokeKeyInput, ctx: Context) -> dict:
    """Revoke an API key by id."""

    pool, enums, agent = await require_context(ctx)
    _require_uuid(payload.key_id, "key")

    if _is_admin(agent, enums):
        result = await pool.execute(QUERIES["api_keys/revoke_any"], payload.key_id)
    else:
        result = await pool.execute(
            QUERIES["api_keys/revoke_by_agent"], payload.key_id, agent["id"]
        )
    if result == "UPDATE 0":
        raise ValueError("Key not found or already revoked")
    return {"revoked": True}


@mcp.tool()
async def get_pending_approvals(ctx: Context) -> list[dict]:
    """Get pending approval requests for the authenticated agent.

    Args:
        ctx: MCP request context.

    Returns:
        List of pending approval request dicts.
    """

    pool, _enums, agent = await require_context(ctx)

    rows = await pool.fetch(QUERIES["approvals/get_pending_by_agent"], agent["id"])
    return [dict(r) for r in rows]


@mcp.tool()
async def get_pending_approvals_all(payload: PendingApprovalsInput, ctx: Context) -> list[dict]:
    """List all pending approvals for admin review."""

    pool, enums, agent = await require_context(ctx)
    _require_admin(agent, enums)
    return await fetch_pending_approvals_all(pool, limit=payload.limit, offset=payload.offset)


@mcp.tool()
async def get_approval(payload: GetApprovalInput, ctx: Context) -> dict:
    """Fetch one approval request by id."""

    pool, enums, agent = await require_context(ctx)
    _require_admin(agent, enums)
    _require_uuid(payload.approval_id, "approval")
    row = await pool.fetchrow(QUERIES["approvals/get_request"], payload.approval_id)
    if not row:
        raise ValueError("Approval request not found")
    return dict(row)


@mcp.tool()
async def approve_request(payload: ApproveRequestInput, ctx: Context) -> dict:
    """Approve a pending request with optional register-agent grants."""

    pool, enums, agent = await require_context(ctx)
    _require_admin(agent, enums)
    _require_uuid(payload.approval_id, "approval")

    approval_row = await pool.fetchrow(QUERIES["approvals/get_request"], payload.approval_id)
    if not approval_row:
        raise ValueError("Approval request not found")
    approval = dict(approval_row)
    is_register = approval.get("request_type") == "register_agent"

    review_details: dict[str, Any] = {}
    has_grants = False
    if payload.grant_scopes is not None:
        has_grants = True
        review_details["grant_scopes"] = payload.grant_scopes
        review_details["grant_scope_ids"] = [
            str(scope_id) for scope_id in require_scopes(payload.grant_scopes, enums)
        ]
    if payload.grant_requires_approval is not None:
        has_grants = True
        review_details["grant_requires_approval"] = payload.grant_requires_approval

    if has_grants and not is_register:
        raise ValueError(
            "grant_scopes and grant_requires_approval are only valid for register_agent approvals"
        )

    reviewed_by = payload.reviewed_by
    if reviewed_by is not None:
        _require_uuid(reviewed_by, "reviewer")

    return await do_approve(
        pool,
        enums,
        payload.approval_id,
        reviewed_by,
        review_details=review_details if review_details else None,
        review_notes=payload.review_notes,
    )


@mcp.tool()
async def reject_request(payload: RejectRequestInput, ctx: Context) -> dict:
    """Reject a pending approval request."""

    pool, enums, agent = await require_context(ctx)
    _require_admin(agent, enums)
    _require_uuid(payload.approval_id, "approval")
    reviewed_by = payload.reviewed_by
    if reviewed_by is not None:
        _require_uuid(reviewed_by, "reviewer")
    return await do_reject(
        pool,
        payload.approval_id,
        reviewed_by,
        payload.review_notes,
    )


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
async def bulk_import_context(payload: BulkImportInput, ctx: Context) -> dict:
    """Bulk import context items from CSV or JSON."""

    return await _run_bulk_import(
        payload,
        ctx,
        normalize_context,
        execute_create_context,
        "bulk_import_context",
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
        Entity dict.

    Raises:
        ValueError: If entity not found or access denied.
    """

    pool, _enums, agent = await require_context(ctx)

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
    return [dict(row) for row in rows]


@mcp.tool()
async def semantic_search(payload: SemanticSearchInput, ctx: Context) -> list[dict]:
    """Run semantic search across entities and context for the active agent."""

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

    if "context" in payload.kinds:
        rows = await pool.fetch(
            QUERIES["search/context_semantic_candidates"],
            scope_ids,
            payload.candidate_limit,
        )
        candidates.extend(_context_semantic_candidate(dict(row)) for row in rows)

    ranked = rank_semantic_candidates(payload.query, candidates, limit=payload.limit)
    for item in ranked:
        item.pop("text", None)
    return ranked


@mcp.tool()
async def update_entity(payload: UpdateEntityInput, ctx: Context) -> dict:
    """Update entity tags or status."""

    pool, enums, agent = await require_context(ctx)

    _require_uuid(payload.entity_id, "entity")
    await _require_entity_write_access(pool, enums, agent, [payload.entity_id])

    if payload.status is not None:
        require_status(payload.status, enums)

    if resp := await maybe_require_approval(pool, agent, "update_entity", payload.model_dump()):
        return resp

    return await execute_update_entity(pool, enums, payload.model_dump())


@mcp.tool()
async def bulk_update_entity_tags(payload: BulkUpdateEntityTagsInput, ctx: Context) -> dict:
    """Bulk update entity tags."""

    pool, enums, agent = await require_context(ctx)
    await _require_entity_write_access(pool, enums, agent, payload.entity_ids)

    if resp := await maybe_require_approval(
        pool, agent, "bulk_update_entity_tags", payload.model_dump()
    ):
        return resp

    op = normalize_bulk_operation(payload.op)
    updated = await do_bulk_update_entity_tags(pool, payload.entity_ids, payload.tags, op)
    return {"updated": len(updated), "entity_ids": updated}


@mcp.tool()
async def bulk_update_entity_scopes(payload: BulkUpdateEntityScopesInput, ctx: Context) -> dict:
    """Bulk update entity privacy scopes."""

    pool, enums, agent = await require_context(ctx)
    await _require_entity_write_access(pool, enums, agent, payload.entity_ids)

    op = normalize_bulk_operation(payload.op)
    allowed_scopes = scope_names_from_ids(agent.get("scopes", []), enums)
    requested_scopes = enforce_scope_subset(payload.scopes, allowed_scopes)
    require_scopes(requested_scopes, enums)
    data = payload.model_dump()
    data["scopes"] = requested_scopes
    if resp := await maybe_require_approval(pool, agent, "bulk_update_entity_scopes", data):
        return resp
    updated = await do_bulk_update_entity_scopes(
        pool, enums, payload.entity_ids, requested_scopes, op
    )
    return {"updated": len(updated), "entity_ids": updated}


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
async def get_entity_history(payload: GetEntityHistoryInput, ctx: Context) -> list[dict]:
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
async def list_audit_scopes(ctx: Context) -> list[dict]:
    """List audit scopes with usage counts (admin)."""

    pool, enums, agent = await require_context(ctx)
    _require_admin(agent, enums)
    return await fetch_audit_scopes(pool)


@mcp.tool()
async def list_audit_actors(payload: ListAuditActorsInput, ctx: Context) -> list[dict]:
    """List audit actors with activity counts (admin)."""

    pool, enums, agent = await require_context(ctx)
    _require_admin(agent, enums)
    return await fetch_audit_actors(pool, payload.actor_type)


@mcp.tool()
async def revert_entity(payload: RevertEntityInput, ctx: Context) -> dict:
    """Revert an entity to a historical audit entry."""

    pool, _enums, agent = await require_context(ctx)
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

    if resp := await maybe_require_approval(pool, agent, "revert_entity", payload.model_dump()):
        return resp

    async with pool.acquire() as conn:
        await conn.execute(QUERIES["runtime/set_changed_by_type"], "agent")
        await conn.execute(QUERIES["runtime/set_changed_by_id"], str(agent["id"]))
        try:
            return await do_revert_entity(conn, payload.entity_id, payload.audit_id)
        finally:
            await conn.execute(QUERIES["runtime/reset_changed_by_type"])
            await conn.execute(QUERIES["runtime/reset_changed_by_id"])


# --- Context Tools ---


@mcp.tool()
async def get_context(payload: GetContextInput, ctx: Context) -> dict:
    """Retrieve one context item by id."""

    pool, enums, agent = await require_context(ctx)
    _require_uuid(payload.context_id, "context")
    scope_ids = _scope_filter_ids(agent, enums)
    row = await pool.fetchrow(QUERIES["context/get"], payload.context_id, scope_ids)
    if not row:
        raise ValueError("Context not found")
    return dict(row)


@mcp.tool()
async def create_context(payload: CreateContextInput, ctx: Context) -> dict:
    """Create a context item."""

    pool, enums, agent = await require_context(ctx)
    allowed_scopes = scope_names_from_ids(agent.get("scopes", []), enums)
    requested_scopes = enforce_scope_subset(payload.scopes, allowed_scopes)
    data = payload.model_dump()
    data["scopes"] = requested_scopes
    if data.get("url") is not None:
        raw_url = data["url"]
        if not isinstance(raw_url, str):
            raise ValueError("URL must be a string")
        url = raw_url.strip()
        if raw_url != "" and url == "":
            raise ValueError("URL must start with http:// or https://")
        if url and not (url.startswith("http://") or url.startswith("https://")):
            raise ValueError("URL must start with http:// or https://")
        data["url"] = url

    require_scopes(requested_scopes, enums)
    if resp := await maybe_require_approval(pool, agent, "create_context", data):
        return resp

    return await execute_create_context(pool, enums, data)


@mcp.tool()
async def update_context(payload: UpdateContextInput, ctx: Context) -> dict:
    """Update a context item."""

    pool, enums, agent = await require_context(ctx)

    _require_uuid(payload.context_id, "context")
    await _validate_relationship_node(
        pool,
        enums,
        agent,
        "context",
        payload.context_id,
        "Context",
        require_write=True,
    )
    data = payload.model_dump()
    if data.get("status") is not None:
        require_status(str(data["status"]), enums)
    if data.get("scopes") is not None:
        allowed_scopes = scope_names_from_ids(agent.get("scopes", []), enums)
        requested_scopes = enforce_scope_subset(data["scopes"], allowed_scopes)
        require_scopes(requested_scopes, enums)
        data["scopes"] = requested_scopes
    if data.get("url") is not None:
        raw_url = data["url"]
        if not isinstance(raw_url, str):
            raise ValueError("URL must be a string")
        url = raw_url.strip()
        if raw_url != "" and url == "":
            raise ValueError("URL must start with http:// or https://")
        if url and not (url.startswith("http://") or url.startswith("https://")):
            raise ValueError("URL must start with http:// or https://")
        data["url"] = url

    if resp := await maybe_require_approval(pool, agent, "update_context", data):
        return resp
    result = await execute_update_context(pool, enums, data)
    if not result:
        raise ValueError("Context not found")
    return result


@mcp.tool()
async def query_context(payload: QueryContextInput, ctx: Context) -> list[dict]:
    """Search context items with filters."""

    pool, enums, agent = await require_context(ctx)

    allowed_scopes = scope_names_from_ids(agent.get("scopes", []), enums)
    requested_scopes = enforce_scope_subset(payload.scopes, allowed_scopes)
    scope_ids = require_scopes(requested_scopes, enums)
    limit = _clamp_limit(payload.limit)
    offset = max(0, payload.offset)

    rows = await pool.fetch(
        QUERIES["context/query"],
        payload.source_type,
        payload.tags or None,
        payload.search_text,
        scope_ids,
        limit,
        offset,
    )
    return [dict(row) for row in rows]


@mcp.tool()
async def link_context_to_owner(payload: LinkContextInput, ctx: Context) -> dict:
    """Link a context item to an owner via context-of relationship."""

    pool, enums, agent = await require_context(ctx)

    _require_uuid(payload.context_id, "context")
    _require_uuid(payload.owner_id, "owner")

    await _validate_relationship_node(
        pool,
        enums,
        agent,
        "context",
        payload.context_id,
        "Context",
        require_write=True,
    )
    await _validate_relationship_node(
        pool,
        enums,
        agent,
        payload.owner_type,
        payload.owner_id,
        "Owner",
        require_write=True,
    )
    relationship_payload = {
        "source_type": payload.owner_type,
        "source_id": payload.owner_id,
        "target_type": "context",
        "target_id": payload.context_id,
        "relationship_type": "context-of",
        "properties": {},
    }
    require_relationship_type("context-of", enums)
    if resp := await maybe_require_approval(
        pool, agent, "create_relationship", relationship_payload
    ):
        return resp

    return await execute_create_relationship(pool, enums, relationship_payload)


@mcp.tool()
async def list_context_by_owner(payload: ListContextByOwnerInput, ctx: Context) -> list[dict]:
    """List context items linked to an owner via context-of."""

    pool, enums, agent = await require_context(ctx)
    if payload.owner_type == "job":
        _require_job_id(payload.owner_id, "owner")
    else:
        _require_uuid(payload.owner_id, "owner")

    await _validate_relationship_node(
        pool,
        enums,
        agent,
        payload.owner_type,
        payload.owner_id,
        "Owner",
        require_write=False,
    )

    scope_filter = _scope_filter_ids(agent, enums)
    type_id = require_relationship_type("context-of", enums)
    limit = _clamp_limit(payload.limit)
    offset = max(0, payload.offset)
    rows = await pool.fetch(
        QUERIES["context/by_owner"],
        payload.owner_type,
        payload.owner_id,
        type_id,
        scope_filter,
        limit,
        offset,
    )
    return [dict(row) for row in rows]


# --- Log Tools ---


@mcp.tool()
async def create_log(payload: CreateLogInput, ctx: Context) -> dict:
    """Create a log entry."""

    pool, enums, agent = await require_context(ctx)
    require_log_type(payload.log_type, enums)
    require_status(payload.status, enums)
    if resp := await maybe_require_approval(pool, agent, "create_log", payload.model_dump()):
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
    if resp := await maybe_require_approval(pool, agent, "update_log", payload.model_dump()):
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
    visible_scope_names = _visible_scope_names(agent, enums, scope_ids)

    _require_node_id(payload.source_type, payload.source_id, "source")

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
        results.append(_normalize_relationship_row(row, visible_scope_names))
    return results


@mcp.tool()
async def query_relationships(payload: QueryRelationshipsInput, ctx: Context) -> list[dict]:
    """Search relationships with filters."""

    pool, enums, agent = await require_context(ctx)
    limit = _clamp_limit(payload.limit)
    scope_ids = _scope_filter_ids(agent, enums)
    visible_scope_names = _visible_scope_names(agent, enums, scope_ids)

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
        results.append(_normalize_relationship_row(row, visible_scope_names))
    return results


@mcp.tool()
async def update_relationship(payload: UpdateRelationshipInput, ctx: Context) -> dict:
    """Update relationship properties or status."""

    pool, enums, agent = await require_context(ctx)

    _require_uuid(payload.relationship_id, "relationship")

    row = await pool.fetchrow(QUERIES["relationships/get_by_id"], payload.relationship_id)
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
    _require_node_id(payload.source_type, payload.source_id, "source")

    if not await _node_allowed(pool, enums, agent, payload.source_type, payload.source_id):
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
    _require_node_id(payload.source_type, payload.source_id, "source")
    _require_node_id(payload.target_type, payload.target_id, "target")

    if not await _node_allowed(pool, enums, agent, payload.source_type, payload.source_id):
        raise ValueError("Access denied")
    if not await _node_allowed(pool, enums, agent, payload.target_type, payload.target_id):
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
    parse_optional_datetime(data.get("due_at"), "due_at")
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
    due_before = parse_optional_datetime(payload.due_before, "due_before")
    due_after = parse_optional_datetime(payload.due_after, "due_after")
    scope_filter = _scope_filter_ids(agent, enums)

    rows = await pool.fetch(
        QUERIES["jobs/query"],
        payload.status_names or None,
        payload.assigned_to,
        payload.agent_id,
        payload.priority,
        due_before,
        due_after,
        payload.overdue_only,
        payload.parent_job_id,
        scope_filter,
        limit,
    )
    return [dict(r) for r in rows]


@mcp.tool()
async def update_job(payload: UpdateJobInput, ctx: Context) -> dict:
    """Update mutable job fields."""

    pool, enums, agent = await require_context(ctx)
    job = await _get_job_row(pool, payload.job_id)
    _require_job_owner(agent, enums, job)

    data = payload.model_dump(exclude_unset=True)
    if data.get("status"):
        require_status(str(data["status"]), enums)
    if data.get("priority") and str(data["priority"]) not in JOB_PRIORITY_VALUES:
        raise ValueError(f"Invalid priority: {data['priority']}")
    if data.get("assigned_to"):
        _require_uuid(str(data["assigned_to"]), "assignee")
    if "due_at" in data:
        parse_optional_datetime(data.get("due_at"), "due_at")

    if resp := await maybe_require_approval(pool, agent, "update_job", data):
        return resp
    return await execute_update_job(pool, enums, data)


@mcp.tool()
async def update_job_status(payload: UpdateJobStatusInput, ctx: Context) -> dict:
    """Update job status with optional completion timestamp."""

    pool, enums, agent = await require_context(ctx)
    job = await _get_job_row(pool, payload.job_id)
    _require_job_owner(agent, enums, job)
    status_id = require_status(payload.status, enums)
    completed_at = parse_optional_datetime(payload.completed_at, "completed_at")
    if resp := await maybe_require_approval(pool, agent, "update_job_status", payload.model_dump()):
        return resp

    row = await pool.fetchrow(
        QUERIES["jobs/update_status"],
        payload.job_id,
        status_id,
        payload.status_reason,
        completed_at,
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
    parse_optional_datetime(payload.due_at, "due_at")
    parent_scopes = scope_names_from_ids(parent_job.get("privacy_scope_ids") or [], enums)
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
    if resp := await maybe_require_approval(pool, agent, "create_file", payload.model_dump()):
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


@mcp.tool()
async def update_file(payload: UpdateFileInput, ctx: Context) -> dict:
    """Update a file metadata record."""

    pool, enums, agent = await require_context(ctx)
    _require_uuid(payload.file_id, "file")
    if await _has_hidden_relationships(pool, enums, agent, "file", payload.file_id):
        raise ValueError("Access denied")
    data = payload.model_dump()
    if data.get("status"):
        require_status(str(data["status"]), enums)
    if resp := await maybe_require_approval(pool, agent, "update_file", data):
        return resp
    result = await execute_update_file(pool, enums, data)
    if not result:
        raise ValueError("File not found")
    return result


async def _attach_file(ctx: Context, target_type: str, payload: AttachFileInput) -> dict:
    """Attach a file to a target entity or context item.

    Args:
        ctx: MCP request context.
        target_type: Target type for relationship (entity or context).
        payload: File attachment request payload.

    Returns:
        Relationship record or approval response payload.
    """

    pool, enums, agent = await require_context(ctx)
    try:
        UUID(str(payload.file_id))
    except (TypeError, ValueError):
        raise ValueError("Invalid file id format")

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
async def attach_file_to_context(payload: AttachFileInput, ctx: Context) -> dict:
    """Attach a file to a context item."""

    return await _attach_file(ctx, "context", payload)


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


@mcp.tool()
async def query_protocols(payload: QueryProtocolsInput, ctx: Context) -> list[dict]:
    """Search protocols with optional filters."""

    pool, enums, agent = await require_context(ctx)
    rows = await pool.fetch(
        QUERIES["protocols/query"],
        payload.status_category,
        payload.protocol_type,
        payload.search,
        _clamp_limit(payload.limit),
        _is_admin(agent, enums),
    )
    return [dict(r) for r in rows]


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


@mcp.tool()
async def get_agent(payload: GetAgentInput, ctx: Context) -> dict:
    """Get one active agent by id (admin only)."""

    pool, enums, agent = await require_context(ctx)
    _require_admin(agent, enums)
    _require_uuid(payload.agent_id, "agent")
    row = await pool.fetchrow(QUERIES["agents/get_by_id"], payload.agent_id)
    if not row:
        raise ValueError("Agent not found")
    return dict(row)


@mcp.tool()
async def update_agent(payload: UpdateAgentInput, ctx: Context) -> dict:
    """Update agent description/scopes/trust."""

    pool, enums, agent = await require_context(ctx)
    _require_uuid(payload.agent_id, "agent")

    is_self = str(agent.get("id")) == payload.agent_id
    if is_self and (payload.scopes is not None or payload.requires_approval is not None):
        _require_admin(agent, enums)
    if not is_self:
        _require_admin(agent, enums)

    scope_ids = None
    if payload.scopes is not None:
        scope_ids = require_scopes(payload.scopes, enums)

    row = await pool.fetchrow(
        QUERIES["agents/update"],
        payload.agent_id,
        payload.description,
        payload.requires_approval,
        scope_ids,
    )
    if not row:
        raise ValueError("Agent not found")
    return dict(row)


@mcp.tool()
async def register_agent(payload: AgentEnrollStartInput, ctx: Context) -> dict:
    """Register a new agent and create enrollment approval (bootstrap allowed)."""

    pool, enums, agent = await require_context(ctx, allow_bootstrap=True)
    if agent is not None:
        raise ValueError("Agent already authenticated")

    requested_scope_ids = require_scopes(payload.requested_scopes, enums)
    inactive_status_id = require_status("inactive", enums)

    async with pool.acquire() as conn, conn.transaction():
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
                "requested_scopes": payload.requested_scopes,
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
        "agent_id": str(created_agent["id"]),
        "approval_request_id": str(approval["id"]),
        "registration_id": str(session["id"]),
        "enrollment_token": session["enrollment_token"],
        "status": "pending_approval",
    }


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
                (json.dumps(payload.value_schema) if payload.value_schema is not None else None),
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
                json.dumps(payload.metadata) if payload.metadata is not None else None,
            )
        elif payload.kind == "relationship-types":
            row = await pool.fetchrow(
                QUERIES[cfg["update"]],
                payload.item_id,
                name,
                payload.description,
                payload.is_symmetric,
                json.dumps(payload.metadata) if payload.metadata is not None else None,
            )
        else:
            row = await pool.fetchrow(
                QUERIES[cfg["update"]],
                payload.item_id,
                name,
                payload.description,
                json.dumps(payload.value_schema) if payload.value_schema is not None else None,
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
            f"Cannot archive {payload.kind} entry while it is referenced ({usage} records)"
        )

    row = await pool.fetchrow(QUERIES[cfg["set_active"]], payload.item_id, False)
    if not row:
        raise ValueError(f"{payload.kind} entry not found or cannot archive built-in entry")
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
