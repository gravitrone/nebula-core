"""Executor functions for approved actions."""

# Standard Library
import hashlib
import json
from datetime import datetime, timezone
from pathlib import Path
from uuid import UUID

# Third-Party
from asyncpg import Pool

# Local
from .enums import (
    EnumRegistry,
    require_relationship_type,
    require_scopes,
    require_status,
)
from .helpers import normalize_bulk_operation
from .query_loader import QueryLoader

QUERIES = QueryLoader(Path(__file__).resolve().parents[1] / "queries")

CYCLE_SENSITIVE_REL_TYPES = {
    "owns",
    "manages",
    "reports-to",
    "depends-on",
    "blocks",
    "supersedes",
    "applies-to",
    "manages-agent",
}


def _scope_name_from_id(enums: EnumRegistry, scope_id: object) -> str:
    try:
        resolved = UUID(str(scope_id))
    except (TypeError, ValueError):
        return str(scope_id)
    return enums.scopes.id_to_name.get(resolved, str(scope_id))


def _advisory_lock_key(*parts: str) -> int:
    token = "|".join(str(part) for part in parts)
    digest = hashlib.sha256(token.encode("utf-8")).digest()
    return int.from_bytes(digest[:8], "big", signed=True)


async def execute_create_entity(
    pool: Pool, enums: EnumRegistry, change_details: dict
) -> dict:
    """Execute entity creation from approved request.

    Args:
        pool: Database connection pool.
        enums: Enum registry for validation.
        change_details: Payload dict from approval request.

    Returns:
        Created entity row as dict.

    Raises:
        ValueError: If duplicate entity or validation fails.
    """

    from .enums import require_entity_type
    from .models import CreateEntityInput, validate_entity_metadata

    if isinstance(change_details, str):
        change_details = json.loads(change_details)

    payload = CreateEntityInput(**change_details)

    async def _run(conn) -> dict:
        # Validate enums first (fail fast)
        status_id = require_status(payload.status, enums)
        type_id = require_entity_type(payload.type, enums)
        scope_ids = require_scopes(payload.scopes, enums)
        scope_key = ",".join(sorted(str(scope_id) for scope_id in scope_ids))

        lock_parts = [payload.name, type_id, scope_key]
        if payload.vault_file_path:
            lock_parts.append(payload.vault_file_path)

        await conn.execute(
            "SELECT pg_advisory_xact_lock($1)", _advisory_lock_key(*lock_parts)
        )

        # LAYER 1: Vault file path dedup (hard block)
        if payload.vault_file_path:
            existing = await conn.fetchrow(
                QUERIES["entities/check_vault_file"], payload.vault_file_path
            )
            if existing:
                raise ValueError(
                    "Entity already exists for vault file "
                    f"'{payload.vault_file_path}': "
                    f"{existing['name']} (id: {existing['id']})"
                )

        # LAYER 2: Name + Type + Scopes dedup (likely duplicate)
        existing = await conn.fetchrow(
            QUERIES["entities/check_duplicate"], payload.name, type_id, scope_ids
        )
        if existing:
            raise ValueError(
                f"Entity '{payload.name}' with same type and scopes already exists "
                f"(id: {existing['id']}). If intentional, use different scopes or name."
            )

        # Validate metadata structure
        metadata = validate_entity_metadata(payload.type, payload.metadata)

        # Validate context segment privacy rules
        context_segments = metadata.get("context_segments") if metadata else None
        if context_segments:
            allowed_scopes = set(payload.scopes)
            for segment in context_segments:
                segment_scopes = segment.get("scopes", [])
                if not segment_scopes:
                    raise ValueError("Context segment scopes required")
                for scope_name in segment_scopes:
                    if scope_name not in enums.scopes.name_to_id:
                        raise ValueError(f"Unknown scope: {scope_name}")
                    if scope_name not in allowed_scopes:
                        raise ValueError("Context segment scope not in entity scopes")

        # LAYER 3: Insert entity
        row = await conn.fetchrow(
            QUERIES["entities/create"],
            scope_ids,
            payload.name,
            type_id,
            status_id,
            payload.tags,
            json.dumps(metadata) if metadata else None,
            payload.vault_file_path,
        )

        return dict(row) if row else {}

    if isinstance(pool, Pool):
        async with pool.acquire() as conn:
            async with conn.transaction():
                return await _run(conn)

    if pool.is_in_transaction():
        return await _run(pool)

    async with pool.transaction():
        return await _run(pool)


async def execute_create_knowledge(
    pool: Pool, enums: EnumRegistry, change_details: dict
) -> dict:
    """Execute knowledge item creation from approved request.

    Args:
        pool: Database connection pool.
        enums: Enum registry for validation.
        change_details: Payload dict from approval request.

    Returns:
        Created knowledge item row as dict.

    Raises:
        ValueError: If duplicate URL exists.
    """

    from .models import CreateKnowledgeInput

    if isinstance(change_details, str):
        change_details = json.loads(change_details)

    payload = CreateKnowledgeInput(**change_details)

    scope_ids = require_scopes(payload.scopes, enums)
    status_id = require_status("active", enums)

    # URL dedup
    if payload.url:
        existing = await pool.fetchrow(QUERIES["knowledge/check_url"], payload.url)
        if existing:
            raise ValueError(
                f"Knowledge item already exists for URL '{payload.url}': "
                f"id={existing['id']}, title={existing['title']}"
            )

    row = await pool.fetchrow(
        QUERIES["knowledge/create"],
        payload.title,
        payload.url,
        payload.source_type,
        payload.content,
        scope_ids,
        status_id,
        payload.tags,
        json.dumps(payload.metadata) if payload.metadata else "{}",
    )

    return dict(row) if row else {}


async def execute_create_relationship(
    pool: Pool, enums: EnumRegistry, change_details: dict
) -> dict:
    """Execute relationship creation from approved request.

    Args:
        pool: Database connection pool.
        enums: Enum registry for validation.
        change_details: Payload dict from approval request.

    Returns:
        Created relationship row as dict.
    """

    from .models import CreateRelationshipInput

    if isinstance(change_details, str):
        change_details = json.loads(change_details)

    payload = CreateRelationshipInput(**change_details)

    type_id = require_relationship_type(payload.relationship_type, enums)
    status_id = require_status("active", enums)
    if (
        payload.source_type == payload.target_type
        and payload.source_id == payload.target_id
    ):
        raise ValueError("Self-referential relationships are not allowed")
    if payload.relationship_type in CYCLE_SENSITIVE_REL_TYPES:
        from .models import MAX_GRAPH_HOPS

        cycle = await pool.fetchval(
            QUERIES["relationships/check_cycle"],
            payload.source_type,
            payload.target_type,
            type_id,
            payload.target_id,
            MAX_GRAPH_HOPS,
            payload.source_id,
        )
        if cycle:
            raise ValueError("Relationship would create a cycle")

    row = await pool.fetchrow(
        QUERIES["relationships/create"],
        payload.source_type,
        payload.source_id,
        payload.target_type,
        payload.target_id,
        type_id,
        status_id,
        json.dumps(payload.properties) if payload.properties else "{}",
    )

    return dict(row) if row else {}


async def execute_create_job(
    pool: Pool, enums: EnumRegistry, change_details: dict
) -> dict:
    """Execute job creation from approved request.

    Args:
        pool: Database connection pool.
        enums: Enum registry for validation.
        change_details: Payload dict from approval request.

    Returns:
        Created job row as dict.
    """

    from .models import CreateJobInput

    if isinstance(change_details, str):
        change_details = json.loads(change_details)

    payload = CreateJobInput(**change_details)

    status_id = require_status("in-progress", enums)
    scope_ids = require_scopes(payload.scopes, enums)

    row = await pool.fetchrow(
        QUERIES["jobs/create"],
        payload.title,
        payload.description,
        payload.job_type,
        payload.assigned_to,
        payload.agent_id,
        status_id,
        payload.priority,
        payload.parent_job_id,
        payload.due_at,
        json.dumps(payload.metadata) if payload.metadata else "{}",
        scope_ids,
    )

    return dict(row) if row else {}


async def execute_update_relationship(
    pool: Pool, enums: EnumRegistry, change_details: dict
) -> dict:
    """Execute relationship update from approved request.

    Args:
        pool: Database connection pool.
        enums: Enum registry for validation.
        change_details: Payload dict from approval request.

    Returns:
        Updated relationship row as dict.
    """

    from .models import UpdateRelationshipInput

    if isinstance(change_details, str):
        change_details = json.loads(change_details)

    payload = UpdateRelationshipInput(**change_details)
    status_id = require_status(payload.status, enums) if payload.status else None

    row = await pool.fetchrow(
        QUERIES["relationships/update"],
        payload.relationship_id,
        json.dumps(payload.properties) if payload.properties else None,
        status_id,
    )

    if not row:
        raise ValueError("Relationship not found")

    return dict(row)


async def execute_update_job_status(
    pool: Pool, enums: EnumRegistry, change_details: dict
) -> dict:
    """Execute job status update from approved request.

    Args:
        pool: Database connection pool.
        enums: Enum registry for validation.
        change_details: Payload dict from approval request.

    Returns:
        Updated job row as dict.
    """

    from .models import UpdateJobStatusInput

    if isinstance(change_details, str):
        change_details = json.loads(change_details)

    payload = UpdateJobStatusInput(**change_details)
    status_id = require_status(payload.status, enums)

    row = await pool.fetchrow(
        QUERIES["jobs/update_status"],
        payload.job_id,
        status_id,
        payload.status_reason,
        payload.completed_at,
    )

    if not row:
        raise ValueError("Job not found")

    return dict(row)


async def execute_create_file(
    pool: Pool, enums: EnumRegistry, change_details: dict
) -> dict:
    """Execute file metadata creation from approved request.

    Args:
        pool: Database connection pool.
        enums: Enum registry for validation.
        change_details: Payload dict from approval request.

    Returns:
        Created file row as dict.
    """

    from .models import CreateFileInput

    if isinstance(change_details, str):
        change_details = json.loads(change_details)

    payload = CreateFileInput(**change_details)

    status_id = require_status(payload.status, enums)

    row = await pool.fetchrow(
        QUERIES["files/create"],
        payload.filename,
        payload.file_path,
        payload.mime_type,
        payload.size_bytes,
        payload.checksum,
        status_id,
        payload.tags,
        json.dumps(payload.metadata) if payload.metadata else "{}",
    )

    return dict(row) if row else {}


async def execute_update_file(
    pool: Pool, enums: EnumRegistry, change_details: dict
) -> dict:
    """Execute file metadata update from approved request."""

    from .models import UpdateFileInput

    if isinstance(change_details, str):
        change_details = json.loads(change_details)

    payload = UpdateFileInput(**change_details)

    status_id = None
    if payload.status:
        status_id = require_status(payload.status, enums)

    row = await pool.fetchrow(
        QUERIES["files/update"],
        payload.file_id,
        payload.filename,
        payload.file_path,
        payload.mime_type,
        payload.size_bytes,
        payload.checksum,
        status_id,
        payload.tags,
        json.dumps(payload.metadata) if payload.metadata is not None else None,
    )

    return dict(row) if row else {}


async def execute_create_protocol(
    pool: Pool, enums: EnumRegistry, change_details: dict
) -> dict:
    """Execute protocol creation from approved request."""

    from .models import CreateProtocolInput

    if isinstance(change_details, str):
        change_details = json.loads(change_details)

    payload = CreateProtocolInput(**change_details)
    status_id = require_status(payload.status, enums)

    row = await pool.fetchrow(
        QUERIES["protocols/create"],
        payload.name,
        payload.title,
        payload.version,
        payload.content,
        payload.protocol_type,
        payload.applies_to,
        status_id,
        payload.tags,
        payload.trusted,
        json.dumps(payload.metadata) if payload.metadata else "{}",
        payload.vault_file_path,
    )

    return dict(row) if row else {}


async def execute_update_protocol(
    pool: Pool, enums: EnumRegistry, change_details: dict
) -> dict:
    """Execute protocol update from approved request."""

    from .models import UpdateProtocolInput

    if isinstance(change_details, str):
        change_details = json.loads(change_details)

    payload = UpdateProtocolInput(**change_details)
    status_id = None
    if payload.status:
        status_id = require_status(payload.status, enums)

    row = await pool.fetchrow(
        QUERIES["protocols/update"],
        payload.name,
        payload.title,
        payload.version,
        payload.content,
        payload.protocol_type,
        payload.applies_to,
        status_id,
        payload.tags,
        payload.trusted,
        json.dumps(payload.metadata) if payload.metadata else None,
        payload.vault_file_path,
    )

    return dict(row) if row else {}


async def execute_create_log(
    pool: Pool, enums: EnumRegistry, change_details: dict
) -> dict:
    """Execute log creation from approved request."""

    from .enums import require_log_type
    from .models import CreateLogInput

    if isinstance(change_details, str):
        change_details = json.loads(change_details)

    payload = CreateLogInput(**change_details)
    log_type_id = require_log_type(payload.log_type, enums)
    status_id = require_status(payload.status, enums)
    timestamp = payload.timestamp or datetime.now(timezone.utc)

    row = await pool.fetchrow(
        QUERIES["logs/create"],
        log_type_id,
        timestamp,
        json.dumps(payload.value) if payload.value is not None else "{}",
        status_id,
        payload.tags,
        json.dumps(payload.metadata) if payload.metadata is not None else "{}",
    )

    return dict(row) if row else {}


async def execute_update_log(
    pool: Pool, enums: EnumRegistry, change_details: dict
) -> dict:
    """Execute log update from approved request."""

    from .enums import require_log_type
    from .models import UpdateLogInput

    if isinstance(change_details, str):
        change_details = json.loads(change_details)

    payload = UpdateLogInput(**change_details)
    log_type_id = None
    if payload.log_type:
        log_type_id = require_log_type(payload.log_type, enums)
    status_id = None
    if payload.status:
        status_id = require_status(payload.status, enums)

    row = await pool.fetchrow(
        QUERIES["logs/update"],
        payload.id,
        log_type_id,
        payload.timestamp,
        json.dumps(payload.value) if payload.value is not None else None,
        status_id,
        payload.tags,
        json.dumps(payload.metadata) if payload.metadata is not None else None,
    )

    return dict(row) if row else {}


async def execute_update_entity(
    pool: Pool, enums: EnumRegistry, change_details: dict
) -> dict:
    """Execute entity update from approved request.

    Args:
        pool: Database connection pool.
        enums: Enum registry for validation.
        change_details: Payload dict from approval request.

    Returns:
        Updated entity row as dict.

    Raises:
        ValueError: If entity not found.
    """

    from .models import UpdateEntityInput, validate_entity_metadata

    if isinstance(change_details, str):
        change_details = json.loads(change_details)

    payload = UpdateEntityInput(**change_details)

    # Validate status if provided
    status_id = None
    if payload.status:
        status_id = require_status(payload.status, enums)

    # Validate metadata if provided
    metadata = None
    if payload.metadata:
        entity = await pool.fetchrow(QUERIES["entities/get_type_id"], payload.entity_id)
        if not entity:
            raise ValueError("Entity not found")

        type_name = enums.entity_types.id_to_name[entity["type_id"]]
        metadata = validate_entity_metadata(type_name, payload.metadata)

    row = await pool.fetchrow(
        QUERIES["entities/update"],
        payload.entity_id,
        json.dumps(metadata) if metadata else None,
        payload.tags,
        status_id,
        payload.status_reason,
    )

    return dict(row) if row else {}


async def execute_bulk_update_entity_tags(
    pool: Pool, enums: EnumRegistry, change_details: dict
) -> dict:
    """Execute bulk tag updates from approved request."""

    from .models import BulkUpdateEntityTagsInput

    if isinstance(change_details, str):
        change_details = json.loads(change_details)

    payload = BulkUpdateEntityTagsInput(**change_details)
    op = normalize_bulk_operation(payload.op)

    rows = await pool.fetch(
        QUERIES["entities/bulk_update_tags"],
        payload.entity_ids,
        op,
        payload.tags,
    )
    return {"updated": len(rows), "entity_ids": [str(r["id"]) for r in rows]}


async def execute_bulk_update_entity_scopes(
    pool: Pool, enums: EnumRegistry, change_details: dict
) -> dict:
    """Execute bulk scope updates from approved request."""

    from .models import BulkUpdateEntityScopesInput

    if isinstance(change_details, str):
        change_details = json.loads(change_details)

    payload = BulkUpdateEntityScopesInput(**change_details)
    op = normalize_bulk_operation(payload.op)
    scope_ids = require_scopes(payload.scopes, enums)

    rows = await pool.fetch(
        QUERIES["entities/bulk_update_scopes"],
        payload.entity_ids,
        op,
        scope_ids,
    )
    return {"updated": len(rows), "entity_ids": [str(r["id"]) for r in rows]}


async def execute_register_agent(
    pool: Pool,
    enums: EnumRegistry,
    change_details: dict,
    review_details: dict | None = None,
) -> dict:
    """Execute agent registration on approval.

    Activates the agent using reviewer grants. API key is issued at redeem time.

    Args:
        pool: Database connection pool.
        enums: Enum registry for validation.
        change_details: Payload dict from approval request.

    Returns:
        Dict with agent id, name, scopes, and trust mode.

    Raises:
        ValueError: If agent not found.
    """

    if isinstance(change_details, str):
        change_details = json.loads(change_details)

    if isinstance(review_details, str):
        review_details = json.loads(review_details)
    review_details = review_details or {}

    agent_id = change_details["agent_id"]
    requested_scopes = change_details.get("requested_scopes") or ["public"]
    requested_requires_approval = change_details.get(
        "requested_requires_approval", True
    )

    granted_scopes = review_details.get("grant_scopes") or requested_scopes
    granted_scope_ids = review_details.get("grant_scope_ids")
    if not granted_scope_ids:
        granted_scope_ids = require_scopes(granted_scopes, enums)

    if review_details.get("grant_requires_approval") is None:
        granted_requires_approval = requested_requires_approval
    else:
        granted_requires_approval = bool(review_details["grant_requires_approval"])

    # Activate agent
    active_status_id = require_status("active", enums)
    agent = await pool.fetchrow(
        QUERIES["agents/activate"],
        active_status_id,
        granted_scope_ids,
        granted_requires_approval,
        agent_id,
    )
    if not agent:
        raise ValueError(f"Agent '{agent_id}' not found")

    approval_id = review_details.get("_approval_id")
    reviewed_by = review_details.get("_reviewed_by")
    if approval_id and reviewed_by:
        await pool.execute(
            QUERIES["enrollments/mark_approved"],
            approval_id,
            granted_scope_ids,
            granted_requires_approval,
            reviewed_by,
        )

    granted_scope_names = [
        _scope_name_from_id(enums, scope_id) for scope_id in granted_scope_ids
    ]
    return {
        "id": str(agent["id"]),
        "name": agent["name"],
        "scopes": granted_scope_names,
        "requires_approval": granted_requires_approval,
        "status": "approved",
        "approval_id": approval_id,
    }


async def execute_bulk_import_entities(
    pool: Pool, enums: EnumRegistry, change_details: dict
) -> dict:
    """Execute a single entity import row created via bulk import approvals."""

    return await execute_create_entity(pool, enums, change_details)


async def execute_bulk_import_knowledge(
    pool: Pool, enums: EnumRegistry, change_details: dict
) -> dict:
    """Execute a single knowledge import row created via bulk import approvals."""

    return await execute_create_knowledge(pool, enums, change_details)


async def execute_bulk_import_relationships(
    pool: Pool, enums: EnumRegistry, change_details: dict
) -> dict:
    """Execute a single relationship import row created via bulk import approvals."""

    return await execute_create_relationship(pool, enums, change_details)


async def execute_bulk_import_jobs(
    pool: Pool, enums: EnumRegistry, change_details: dict
) -> dict:
    """Execute a single job import row created via bulk import approvals."""

    return await execute_create_job(pool, enums, change_details)


# --- Executor Registry ---
EXECUTORS = {
    "create_entity": execute_create_entity,
    "create_knowledge": execute_create_knowledge,
    "create_relationship": execute_create_relationship,
    "create_job": execute_create_job,
    "update_relationship": execute_update_relationship,
    "update_job_status": execute_update_job_status,
    "create_file": execute_create_file,
    "update_file": execute_update_file,
    "create_protocol": execute_create_protocol,
    "update_protocol": execute_update_protocol,
    "create_log": execute_create_log,
    "update_log": execute_update_log,
    "update_entity": execute_update_entity,
    "bulk_update_entity_tags": execute_bulk_update_entity_tags,
    "bulk_update_entity_scopes": execute_bulk_update_entity_scopes,
    "bulk_import_entities": execute_bulk_import_entities,
    "bulk_import_knowledge": execute_bulk_import_knowledge,
    "bulk_import_relationships": execute_bulk_import_relationships,
    "bulk_import_jobs": execute_bulk_import_jobs,
    "register_agent": execute_register_agent,
}
