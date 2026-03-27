"""Red team tests for self-referencing relationships."""

# Standard Library
from unittest.mock import MagicMock

# Third-Party
import pytest

# Local
from nebula_mcp.models import CreateRelationshipInput
from nebula_mcp.server import create_relationship


def _make_context(pool, enums, agent):
    """Build a mock MCP context for self-reference tests."""

    ctx = MagicMock()
    ctx.request_context.lifespan_context = {
        "pool": pool,
        "enums": enums,
        "agent": agent,
    }
    return ctx


async def _make_agent(db_pool, enums, name):
    """Insert a test agent for self-reference scenarios."""

    status_id = enums.statuses.name_to_id["active"]
    scope_ids = [enums.scopes.name_to_id["public"]]

    row = await db_pool.fetchrow(
        """
        INSERT INTO agents (name, description, scopes, requires_approval, status_id)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING *
        """,
        name,
        "redteam agent",
        scope_ids,
        False,
        status_id,
    )
    return dict(row)


async def _make_entity(db_pool, enums, name):
    """Insert a test entity for self-reference scenarios."""

    status_id = enums.statuses.name_to_id["active"]
    type_id = enums.entity_types.name_to_id["person"]
    scope_ids = [enums.scopes.name_to_id["public"]]

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
async def test_mcp_create_relationship_self_ref(db_pool, enums):
    """Self-referencing relationships should be rejected."""

    agent = await _make_agent(db_pool, enums, "self-rel-agent")
    entity = await _make_entity(db_pool, enums, "Self Node")
    ctx = _make_context(db_pool, enums, agent)

    payload = CreateRelationshipInput(
        source_type="entity",
        source_id=str(entity["id"]),
        target_type="entity",
        target_id=str(entity["id"]),
        relationship_type="related-to",
    )

    with pytest.raises(ValueError):
        await create_relationship(payload, ctx)
