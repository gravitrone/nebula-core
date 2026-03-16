"""Red team tests for write isolation across scopes."""

# Standard Library
import json
from unittest.mock import MagicMock

# Third-Party
import pytest

# Local
from nebula_mcp.models import CreateRelationshipInput, UpdateEntityInput
from nebula_mcp.server import create_relationship, update_entity


def _make_context(pool, enums, agent):
    """Build MCP context with a specific agent."""

    ctx = MagicMock()
    ctx.request_context.lifespan_context = {
        "pool": pool,
        "enums": enums,
        "agent": agent,
    }
    return ctx


async def _make_entity(db_pool, enums, name, scopes):
    """Insert an entity for write isolation tests."""

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


async def _make_trusted_agent(db_pool, enums, scopes):
    """Insert a trusted agent with specified scopes."""

    status_id = enums.statuses.name_to_id["active"]
    scope_ids = [enums.scopes.name_to_id[s] for s in scopes]

    row = await db_pool.fetchrow(
        """
        INSERT INTO agents (name, description, scopes, requires_approval, status_id)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING *
        """,
        "public-writer",
        "Trusted limited agent",
        scope_ids,
        False,
        status_id,
    )
    return dict(row)


@pytest.mark.asyncio
async def test_update_entity_denies_scope_violation(db_pool, enums):
    """Trusted agent should not update entity outside its scopes."""

    sensitive_entity = await _make_entity(db_pool, enums, "Sensitive", ["sensitive"])
    agent = await _make_trusted_agent(db_pool, enums, ["public"])
    ctx = _make_context(db_pool, enums, agent)

    payload = UpdateEntityInput(
        entity_id=str(sensitive_entity["id"]),
        tags=["updated"],
    )

    with pytest.raises(ValueError):
        await update_entity(payload, ctx)


@pytest.mark.asyncio
async def test_create_relationship_denies_private_node(db_pool, enums):
    """Trusted agent should not create relationships to private nodes."""

    public_entity = await _make_entity(db_pool, enums, "Public", ["public"])
    sensitive_entity = await _make_entity(db_pool, enums, "Sensitive", ["sensitive"])
    agent = await _make_trusted_agent(db_pool, enums, ["public"])
    ctx = _make_context(db_pool, enums, agent)

    payload = CreateRelationshipInput(
        source_type="entity",
        source_id=str(public_entity["id"]),
        target_type="entity",
        target_id=str(sensitive_entity["id"]),
        relationship_type="related-to",
        properties={"note": "link"},
    )

    with pytest.raises(ValueError):
        await create_relationship(payload, ctx)
