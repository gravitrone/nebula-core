"""Red team tests for bulk update isolation."""

# Standard Library
import json
from unittest.mock import MagicMock

# Third-Party
import pytest

# Local
from nebula_mcp.models import BulkUpdateEntityScopesInput, BulkUpdateEntityTagsInput
from nebula_mcp.server import bulk_update_entity_scopes, bulk_update_entity_tags


def _make_context(pool, enums, agent):
    """Build a mock MCP context for bulk update tests."""

    ctx = MagicMock()
    ctx.request_context.lifespan_context = {
        "pool": pool,
        "enums": enums,
        "agent": agent,
    }
    return ctx


async def _make_agent(db_pool, enums, name, scopes, requires_approval):
    """Insert a test agent for bulk update scenarios."""

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
    """Insert a test entity for bulk update scenarios."""

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


@pytest.mark.asyncio
async def test_bulk_update_tags_denies_private_entity(db_pool, enums):
    """Public agents should not bulk update tags on private entities."""

    private_entity = await _make_entity(db_pool, enums, "Private", ["sensitive"])
    agent = await _make_agent(db_pool, enums, "bulk-tagger", ["public"], False)
    ctx = _make_context(db_pool, enums, agent)

    payload = BulkUpdateEntityTagsInput(
        entity_ids=[str(private_entity["id"])],
        tags=["pwn"],
        op="add",
    )

    with pytest.raises(ValueError):
        await bulk_update_entity_tags(payload, ctx)


@pytest.mark.asyncio
async def test_bulk_update_scopes_denies_private_entity(db_pool, enums):
    """Public agents should not bulk update scopes on private entities."""

    private_entity = await _make_entity(db_pool, enums, "Private", ["sensitive"])
    agent = await _make_agent(db_pool, enums, "bulk-scope", ["public"], False)
    ctx = _make_context(db_pool, enums, agent)

    payload = BulkUpdateEntityScopesInput(
        entity_ids=[str(private_entity["id"])],
        scopes=["public"],
        op="add",
    )

    with pytest.raises(ValueError):
        await bulk_update_entity_scopes(payload, ctx)


@pytest.mark.asyncio
async def test_bulk_update_tags_rejects_invalid_uuid(db_pool, enums):
    """Bulk tag updates should reject malformed UUIDs."""

    agent = await _make_agent(db_pool, enums, "bulk-tag-invalid", ["public"], False)
    ctx = _make_context(db_pool, enums, agent)

    payload = BulkUpdateEntityTagsInput(
        entity_ids=["not-a-uuid"],
        tags=["pwn"],
        op="add",
    )

    with pytest.raises(ValueError):
        await bulk_update_entity_tags(payload, ctx)


@pytest.mark.asyncio
async def test_bulk_update_scopes_rejects_invalid_uuid(db_pool, enums):
    """Bulk scope updates should reject malformed UUIDs."""

    agent = await _make_agent(db_pool, enums, "bulk-scope-invalid", ["public"], False)
    ctx = _make_context(db_pool, enums, agent)

    payload = BulkUpdateEntityScopesInput(
        entity_ids=["not-a-uuid"],
        scopes=["public"],
        op="add",
    )

    with pytest.raises(ValueError):
        await bulk_update_entity_scopes(payload, ctx)
