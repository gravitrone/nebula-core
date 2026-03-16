"""Red team tests for context-of privacy filtering via MCP tool."""

# Standard Library
from unittest.mock import MagicMock
from uuid import UUID, uuid4

# Third-Party
import pytest

# Local
from nebula_mcp.models import ListContextByOwnerInput
from nebula_mcp.server import list_context_by_owner


def _build_ctx(pool, enums, agent) -> MagicMock:
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

    ctx = MagicMock()
    ctx.request_context.lifespan_context = {
        "pool": pool,
        "enums": enums,
        "agent": normalized_agent,
    }
    return ctx


async def _ensure_context_of_type(db_pool) -> str:
    """Ensure context-of relationship type exists and return its id."""

    row = await db_pool.fetchrow("SELECT id FROM relationship_types WHERE name = 'context-of'")
    if row:
        return str(row["id"])
    created = await db_pool.fetchrow(
        """
        INSERT INTO relationship_types (name, description, is_symmetric, is_builtin, is_active, metadata)
        VALUES ('context-of', 'Context item used as scoped metadata for an owner', false, true, true, '{}'::jsonb)
        RETURNING id
        """
    )
    return str(created["id"])


async def _make_agent(db_pool, enums, name: str, scopes: list[str]) -> dict:
    """Insert a test agent for context-of filtering."""

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
        False,
        status_id,
    )
    return dict(row)


async def _make_entity(db_pool, enums, name: str, scopes: list[str]) -> dict:
    """Insert a test entity."""

    status_id = enums.statuses.name_to_id["active"]
    type_id = enums.entity_types.name_to_id["person"]
    scope_ids = [enums.scopes.name_to_id[s] for s in scopes]
    row = await db_pool.fetchrow(
        """
        INSERT INTO entities (name, type_id, status_id, privacy_scope_ids, tags)
        VALUES ($1, $2::uuid, $3::uuid, $4::uuid[], $5)
        RETURNING *
        """,
        name,
        type_id,
        status_id,
        scope_ids,
        ["redteam"],
    )
    return dict(row)


async def _make_context_item(db_pool, enums, title: str, scopes: list[str]) -> dict:
    """Insert a context item for context-of filtering tests."""

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


async def _link_context(db_pool, enums, owner_id: str, context_id: str) -> None:
    """Create a context-of relationship between entity and context."""

    type_id = await _ensure_context_of_type(db_pool)
    status_id = enums.statuses.name_to_id["active"]
    await db_pool.execute(
        """
        INSERT INTO relationships (source_type, source_id, target_type, target_id, type_id, status_id, properties)
        VALUES ('entity', $1::uuid, 'context', $2::uuid, $3::uuid, $4::uuid, '{}'::jsonb)
        """,
        owner_id,
        context_id,
        type_id,
        status_id,
    )


@pytest.mark.asyncio
async def test_list_context_by_owner_filters_scopes(db_pool, enums):
    """Public agents should only see public context-of items."""

    entity = await _make_entity(db_pool, enums, "Owner", ["public"])
    ctx_public = await _make_context_item(db_pool, enums, "Public Note", ["public"])
    ctx_private = await _make_context_item(db_pool, enums, "Private Note", ["private"])
    await _link_context(db_pool, enums, entity["id"], ctx_public["id"])
    await _link_context(db_pool, enums, entity["id"], ctx_private["id"])

    agent = await _make_agent(db_pool, enums, "public-agent", ["public"])
    ctx = _build_ctx(db_pool, enums, agent)

    payload = ListContextByOwnerInput(owner_type="entity", owner_id=str(entity["id"]))
    rows = await list_context_by_owner(payload, ctx)
    ids = {str(row["id"]) for row in rows}

    assert str(ctx_public["id"]) in ids
    assert str(ctx_private["id"]) not in ids


@pytest.mark.asyncio
async def test_list_context_by_owner_includes_private_scopes(db_pool, enums):
    """Agents with private scope should see private context-of items."""

    entity = await _make_entity(db_pool, enums, "Owner Two", ["public", "private"])
    ctx_public = await _make_context_item(db_pool, enums, "Public Note", ["public"])
    ctx_private = await _make_context_item(db_pool, enums, "Private Note", ["private"])
    await _link_context(db_pool, enums, entity["id"], ctx_public["id"])
    await _link_context(db_pool, enums, entity["id"], ctx_private["id"])

    agent = await _make_agent(db_pool, enums, "private-agent", ["public", "private"])
    ctx = _build_ctx(db_pool, enums, agent)

    payload = ListContextByOwnerInput(owner_type="entity", owner_id=str(entity["id"]))
    rows = await list_context_by_owner(payload, ctx)
    ids = {str(row["id"]) for row in rows}

    assert str(ctx_public["id"]) in ids
    assert str(ctx_private["id"]) in ids
