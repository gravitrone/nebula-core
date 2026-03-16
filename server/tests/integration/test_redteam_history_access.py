"""Red team tests for entity history access isolation."""

# Standard Library
import json
from unittest.mock import MagicMock
from uuid import UUID, uuid4

# Third-Party
import pytest

# Local
from nebula_mcp.models import GetEntityHistoryInput
from nebula_mcp.server import get_entity_history


async def _make_context(pool, enums, agent):
    """Build MCP context with a specific agent."""

    normalized_agent = dict(agent)
    try:
        agent_id = str(UUID(str(normalized_agent.get("id", ""))))
    except (TypeError, ValueError):
        agent_id = str(uuid4())
    normalized_agent["id"] = agent_id
    normalized_agent.setdefault("name", f"rt-{agent_id[:8]}")
    normalized_agent.setdefault("requires_approval", False)
    normalized_agent.setdefault("scopes", [enums.scopes.name_to_id["public"]])

    await pool.execute(
        """
        INSERT INTO agents (id, name, description, scopes, requires_approval, status_id)
        VALUES ($1::uuid, $2, $3, $4, $5, $6::uuid)
        ON CONFLICT (id) DO UPDATE
        SET name = EXCLUDED.name,
            scopes = EXCLUDED.scopes,
            requires_approval = EXCLUDED.requires_approval,
            status_id = EXCLUDED.status_id
        """,
        agent_id,
        normalized_agent["name"],
        "test history helper",
        normalized_agent["scopes"],
        normalized_agent["requires_approval"],
        enums.statuses.name_to_id["active"],
    )

    ctx = MagicMock()
    ctx.request_context.lifespan_context = {
        "pool": pool,
        "enums": enums,
        "agent": normalized_agent,
    }
    return ctx


async def _make_entity(db_pool, enums, name, scopes):
    """Insert an entity for history access tests."""

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
async def test_get_entity_history_denies_private_entity(db_pool, enums):
    """Entity history should be denied when entity is outside agent scopes."""

    private_entity = await _make_entity(db_pool, enums, "Private", ["sensitive"])
    await db_pool.execute(
        "UPDATE entities SET name = $1 WHERE id = $2",
        "Private Updated",
        private_entity["id"],
    )

    public_agent = {
        "id": "history-viewer",
        "scopes": [enums.scopes.name_to_id["public"]],
    }
    ctx = await _make_context(db_pool, enums, public_agent)

    payload = GetEntityHistoryInput(entity_id=str(private_entity["id"]))

    with pytest.raises(ValueError):
        await get_entity_history(payload, ctx)
