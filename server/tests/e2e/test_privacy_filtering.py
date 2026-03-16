"""E2E test: privacy scope filtering."""

# Standard Library
from unittest.mock import MagicMock

import pytest

from nebula_mcp.models import GetEntityInput, ListContextByOwnerInput
from nebula_mcp.server import get_entity, list_context_by_owner

pytestmark = pytest.mark.e2e


# --- Helpers ---


def _mock_ctx(pool, enums, agent):
    """Build a MagicMock MCP context with pool, enums, and agent in lifespan_context."""

    ctx = MagicMock()
    ctx.request_context.lifespan_context = {
        "pool": pool,
        "enums": enums,
        "agent": agent,
    }
    return ctx


async def _make_agent_with_scopes(pool, enums, name, scope_names):
    """Insert an agent with specific scopes and return the row."""

    status_id = enums.statuses.name_to_id["active"]
    scope_ids = [enums.scopes.name_to_id[s] for s in scope_names]

    row = await pool.fetchrow(
        """
        INSERT INTO agents (name, status_id, scopes, requires_approval)
        VALUES ($1, $2, $3, false)
        RETURNING *
        """,
        name,
        status_id,
        scope_ids,
    )
    return row


async def _make_entity(pool, enums, name, scope_names):
    """Insert an entity with scopes and return the row."""

    status_id = enums.statuses.name_to_id["active"]
    type_id = enums.entity_types.name_to_id["person"]
    scope_ids = [enums.scopes.name_to_id[s] for s in scope_names]

    row = await pool.fetchrow(
        """
        INSERT INTO entities (privacy_scope_ids, name, type_id, status_id)
        VALUES ($1, $2, $3, $4)
        RETURNING *
        """,
        scope_ids,
        name,
        type_id,
        status_id,
    )
    return row


async def _ensure_context_of_type(pool):
    """Ensure context-of relationship type exists and return id."""

    row = await pool.fetchrow(
        "SELECT id FROM relationship_types WHERE lower(name) = lower('context-of')"
    )
    if row:
        return row["id"]
    row = await pool.fetchrow(
        """
        INSERT INTO relationship_types (name, description, is_symmetric, is_builtin, is_active, metadata)
        VALUES ('context-of', 'Context item used as scoped metadata for an owner', false, true, true, '{}'::jsonb)
        RETURNING id
        """
    )
    return row["id"]


async def _make_context_item(pool, enums, title, scope_names):
    """Insert a context item with scopes and return the row."""

    status_id = enums.statuses.name_to_id["active"]
    scope_ids = [enums.scopes.name_to_id[s] for s in scope_names]
    row = await pool.fetchrow(
        """
        INSERT INTO context_items (privacy_scope_ids, title, source_type, content, status_id, tags)
        VALUES ($1, $2, 'note', 'body', $3, '{}'::text[])
        RETURNING *
        """,
        scope_ids,
        title,
        status_id,
    )
    return row


async def _link_context_to_entity(pool, entity_id, context_id):
    """Link a context item to an entity via context-of."""

    type_id = await _ensure_context_of_type(pool)
    await pool.execute(
        """
        INSERT INTO relationships (source_type, source_id, target_type, target_id, type_id, properties)
        VALUES ('entity', $1, 'context', $2, $3, '{}'::jsonb)
        """,
        str(entity_id),
        str(context_id),
        type_id,
    )


# --- Privacy Filtering ---


@pytest.mark.asyncio
async def test_agent_denied_when_scope_mismatch(db_pool, enums):
    """Agent with only public scope cannot access a private entity."""

    agent = await _make_agent_with_scopes(db_pool, enums, "public-only-agent", ["public"])

    private_scope_id = enums.scopes.name_to_id["private"]
    status_id = enums.statuses.name_to_id["active"]
    type_id = enums.entity_types.name_to_id["project"]

    entity = await db_pool.fetchrow(
        """
        INSERT INTO entities (privacy_scope_ids, name, type_id, status_id)
        VALUES ($1, $2, $3, $4)
        RETURNING *
        """,
        [private_scope_id],
        "private-only-project",
        type_id,
        status_id,
    )

    ctx = _mock_ctx(db_pool, enums, dict(agent))
    payload = GetEntityInput(
        entity_id=str(entity["id"]),
    )

    with pytest.raises(ValueError, match="Access denied"):
        await get_entity(payload, ctx)


@pytest.mark.asyncio
async def test_context_by_owner_filters_scopes(db_pool, enums):
    """Agent with public scope only sees public context items."""

    agent = await _make_agent_with_scopes(db_pool, enums, "public-agent", ["public"])

    entity = await _make_entity(db_pool, enums, "multi-scope-person", ["public", "private"])
    public_ctx = await _make_context_item(db_pool, enums, "public-note", ["public"])
    private_ctx = await _make_context_item(db_pool, enums, "private-note", ["private"])
    await _link_context_to_entity(db_pool, entity["id"], public_ctx["id"])
    await _link_context_to_entity(db_pool, entity["id"], private_ctx["id"])

    ctx = _mock_ctx(db_pool, enums, dict(agent))
    payload = ListContextByOwnerInput(
        owner_type="entity",
        owner_id=str(entity["id"]),
    )

    result = await list_context_by_owner(payload, ctx)
    ids = {str(row["id"]) for row in result}
    assert str(public_ctx["id"]) in ids
    assert str(private_ctx["id"]) not in ids


@pytest.mark.asyncio
async def test_context_by_owner_with_all_scopes_sees_all(db_pool, enums):
    """Agent with both public and private scopes sees all context items."""

    agent = await _make_agent_with_scopes(db_pool, enums, "all-scope-agent", ["public", "private"])

    entity = await _make_entity(db_pool, enums, "full-scope-person", ["public", "private"])
    public_ctx = await _make_context_item(db_pool, enums, "public-note", ["public"])
    private_ctx = await _make_context_item(db_pool, enums, "private-note", ["private"])
    await _link_context_to_entity(db_pool, entity["id"], public_ctx["id"])
    await _link_context_to_entity(db_pool, entity["id"], private_ctx["id"])

    ctx = _mock_ctx(db_pool, enums, dict(agent))
    payload = ListContextByOwnerInput(
        owner_type="entity",
        owner_id=str(entity["id"]),
    )

    result = await list_context_by_owner(payload, ctx)
    ids = {str(row["id"]) for row in result}
    assert str(public_ctx["id"]) in ids
    assert str(private_ctx["id"]) in ids
