"""Red team tests for file attachment isolation."""

# Standard Library
import json
from unittest.mock import MagicMock

# Third-Party
import pytest

# Local
from nebula_mcp.models import AttachFileInput
from nebula_mcp.server import (
    attach_file_to_context,
    attach_file_to_entity,
    attach_file_to_job,
)


def _make_context(pool, enums, agent):
    """Build a mock MCP context for file attachment tests."""

    ctx = MagicMock()
    ctx.request_context.lifespan_context = {
        "pool": pool,
        "enums": enums,
        "agent": agent,
    }
    return ctx


async def _make_agent(db_pool, enums, name, scopes, requires_approval):
    """Insert a test agent for file attachment scenarios."""

    status_id = enums.statuses.name_to_id["active"]
    scope_ids = [enums.scopes.name_to_id[s] for s in scopes]

    row = await db_pool.fetchrow(
        """
        INSERT INTO agents (name, description, scopes, requires_approval, status_id)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING *
        """,
        name,
        "redteam agent",
        scope_ids,
        requires_approval,
        status_id,
    )
    return dict(row)


async def _make_entity(db_pool, enums, name, scopes):
    """Insert a test entity for file attachment scenarios."""

    status_id = enums.statuses.name_to_id["active"]
    type_id = enums.entity_types.name_to_id["person"]
    scope_ids = [enums.scopes.name_to_id[s] for s in scopes]

    row = await db_pool.fetchrow(
        """
        INSERT INTO entities (name, type_id, status_id, privacy_scope_ids, tags)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING *
        """,
        name,
        type_id,
        status_id,
        scope_ids,
        ["test"],
    )
    return dict(row)


async def _make_file(db_pool, enums):
    """Insert a test file for attachment scenarios."""

    status_id = enums.statuses.name_to_id["active"]

    row = await db_pool.fetchrow(
        """
        INSERT INTO files (filename, file_path, status_id, metadata)
        VALUES ($1, $2, $3, $4::jsonb)
        RETURNING *
        """,
        "attached.txt",
        "/vault/attached.txt",
        status_id,
        json.dumps({"note": "attachment"}),
    )
    return dict(row)


async def _make_job(db_pool, enums, agent_id):
    """Insert a test job for attachment scenarios."""

    status_id = enums.statuses.name_to_id["active"]

    row = await db_pool.fetchrow(
        """
        INSERT INTO jobs (title, status_id, agent_id)
        VALUES ($1, $2, $3)
        RETURNING *
        """,
        "Private Job",
        status_id,
        agent_id,
    )
    return dict(row)


async def _make_context_item(db_pool, enums, title, scopes):
    """Insert a test context item for attachment scenarios."""

    status_id = enums.statuses.name_to_id["active"]
    scope_ids = [enums.scopes.name_to_id[s] for s in scopes]

    row = await db_pool.fetchrow(
        """
        INSERT INTO context_items (title, source_type, content, privacy_scope_ids, status_id, tags)
        VALUES ($1, $2, $3, $4, $5, $6)
        RETURNING *
        """,
        title,
        "note",
        "secret",
        scope_ids,
        status_id,
        ["test"],
    )
    return dict(row)


@pytest.mark.asyncio
async def test_attach_file_to_private_entity_denied(db_pool, enums):
    """Public agents should not attach files to private entities."""

    private_entity = await _make_entity(db_pool, enums, "Private", ["sensitive"])
    file_row = await _make_file(db_pool, enums)

    viewer = await _make_agent(db_pool, enums, "file-attacher", ["public"], False)
    ctx = _make_context(db_pool, enums, viewer)

    payload = AttachFileInput(file_id=str(file_row["id"]), target_id=str(private_entity["id"]))

    with pytest.raises(ValueError):
        await attach_file_to_entity(payload, ctx)


@pytest.mark.asyncio
async def test_attach_file_to_foreign_job_denied(db_pool, enums):
    """Agents should not attach files to jobs owned by other agents."""

    owner = await _make_agent(db_pool, enums, "job-owner", ["public"], False)
    viewer = await _make_agent(db_pool, enums, "job-viewer", ["public"], False)
    job = await _make_job(db_pool, enums, owner["id"])
    file_row = await _make_file(db_pool, enums)

    ctx = _make_context(db_pool, enums, viewer)
    payload = AttachFileInput(file_id=str(file_row["id"]), target_id=str(job["id"]))

    with pytest.raises(ValueError):
        await attach_file_to_job(payload, ctx)


@pytest.mark.asyncio
async def test_attach_file_to_owned_job_accepts_job_id_format(db_pool, enums):
    """File attachment should accept canonical job IDs for job targets."""

    owner = await _make_agent(db_pool, enums, "job-owner-attach", ["public"], False)
    job = await _make_job(db_pool, enums, owner["id"])
    file_row = await _make_file(db_pool, enums)

    ctx = _make_context(db_pool, enums, owner)
    payload = AttachFileInput(file_id=str(file_row["id"]), target_id=str(job["id"]))

    result = await attach_file_to_job(payload, ctx)
    assert result["target_type"] == "job"
    assert result["target_id"] == str(job["id"])


@pytest.mark.asyncio
async def test_attach_file_to_private_context_denied(db_pool, enums):
    """Public agents should not attach files to private context."""

    context = await _make_context_item(db_pool, enums, "Private Context", ["sensitive"])
    file_row = await _make_file(db_pool, enums)
    viewer = await _make_agent(db_pool, enums, "context-attacher", ["public"], False)
    ctx = _make_context(db_pool, enums, viewer)

    payload = AttachFileInput(file_id=str(file_row["id"]), target_id=str(context["id"]))

    with pytest.raises(ValueError):
        await attach_file_to_context(payload, ctx)


@pytest.mark.asyncio
async def test_attach_file_to_entity_rejects_invalid_file_id_format(db_pool, enums):
    """Entity attachment should reject malformed file IDs with a clean ValueError."""

    owner = await _make_agent(db_pool, enums, "file-id-entity-owner", ["public"], False)
    target = await _make_entity(db_pool, enums, "Public Entity", ["public"])
    ctx = _make_context(db_pool, enums, owner)
    payload = AttachFileInput(file_id="not-a-uuid", target_id=str(target["id"]))

    with pytest.raises(ValueError, match="Invalid file id format"):
        await attach_file_to_entity(payload, ctx)


@pytest.mark.asyncio
async def test_attach_file_to_context_rejects_invalid_file_id_format(db_pool, enums):
    """Context attachment should reject malformed file IDs with a clean ValueError."""

    owner = await _make_agent(db_pool, enums, "file-id-context-owner", ["public"], False)
    target = await _make_context_item(db_pool, enums, "Public Context", ["public"])
    ctx = _make_context(db_pool, enums, owner)
    payload = AttachFileInput(file_id="not-a-uuid", target_id=str(target["id"]))

    with pytest.raises(ValueError, match="Invalid file id format"):
        await attach_file_to_context(payload, ctx)


@pytest.mark.asyncio
async def test_attach_file_to_job_rejects_invalid_file_id_format(db_pool, enums):
    """Job attachment should reject malformed file IDs with a clean ValueError."""

    owner = await _make_agent(db_pool, enums, "file-id-job-owner", ["public"], False)
    target = await _make_job(db_pool, enums, owner["id"])
    ctx = _make_context(db_pool, enums, owner)
    payload = AttachFileInput(file_id="not-a-uuid", target_id=str(target["id"]))

    with pytest.raises(ValueError, match="Invalid file id format"):
        await attach_file_to_job(payload, ctx)
