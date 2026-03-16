"""Red team MCP security tests focused on agent isolation."""

# Standard Library
import json
from unittest.mock import MagicMock
from uuid import UUID, uuid4

# Third-Party
import pytest

# Local
from nebula_mcp.models import (
    GetEntityInput,
    GetRelationshipsInput,
    QueryAuditLogInput,
    QueryEntitiesInput,
    QueryRelationshipsInput,
)
from nebula_mcp.server import (
    get_entity,
    get_relationships,
    query_audit_log,
    query_entities,
    query_relationships,
)


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
        "test mcp security helper",
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
    """Insert an entity for MCP isolation tests."""

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
async def test_query_entities_respects_agent_scopes(db_pool, enums, test_entity):
    """Agents should not see entities outside their scopes."""

    private_entity = await _make_entity(db_pool, enums, "Private Node", ["private"])

    public_agent = {
        "id": "test-agent",
        "scopes": [enums.scopes.name_to_id["public"]],
    }
    ctx = await _make_context(db_pool, enums, public_agent)

    payload = QueryEntitiesInput()
    rows = await query_entities(payload, ctx)
    row_ids = {row["id"] for row in rows}

    assert str(private_entity["id"]) not in row_ids


@pytest.mark.asyncio
async def test_get_entity_denies_private_scope(db_pool, enums):
    """Agents should be denied when fetching private entities."""

    private_entity = await _make_entity(db_pool, enums, "Private Node", ["private"])

    public_agent = {
        "id": "test-agent",
        "scopes": [enums.scopes.name_to_id["public"]],
    }
    ctx = await _make_context(db_pool, enums, public_agent)

    payload = GetEntityInput(entity_id=str(private_entity["id"]))
    with pytest.raises(ValueError, match="Access denied"):
        await get_entity(payload, ctx)


@pytest.mark.asyncio
async def test_get_entity_not_found_uses_generic_error(db_pool, enums):
    """Missing entities should not leak existence details."""

    public_agent = {
        "id": "test-agent",
        "scopes": [enums.scopes.name_to_id["public"]],
    }
    ctx = await _make_context(db_pool, enums, public_agent)

    missing_id = "00000000-0000-0000-0000-000000000001"
    payload = GetEntityInput(entity_id=missing_id)
    with pytest.raises(ValueError) as excinfo:
        await get_entity(payload, ctx)

    msg = str(excinfo.value)
    assert "Not found" in msg
    assert missing_id not in msg


@pytest.mark.asyncio
async def test_mcp_relationships_hide_private_nodes(
    db_pool, enums, test_entity, untrusted_mcp_context
):
    """Relationship list should not leak private node ids."""

    private_entity = await _make_entity(db_pool, enums, "Private Node", ["sensitive"])
    relationship_type_id = enums.relationship_types.name_to_id["related-to"]
    status_id = enums.statuses.name_to_id["active"]

    await db_pool.execute(
        """
        INSERT INTO relationships (source_type, source_id, target_type, target_id, type_id, status_id, properties)
        VALUES ('entity', $1, 'entity', $2, $3, $4, $5::jsonb)
        """,
        str(test_entity["id"]),
        str(private_entity["id"]),
        relationship_type_id,
        status_id,
        json.dumps({"note": "secret link"}),
    )

    payload = GetRelationshipsInput(
        source_type="entity",
        source_id=str(test_entity["id"]),
        direction="both",
        relationship_type=None,
    )
    rows = await get_relationships(payload, untrusted_mcp_context)
    private_id = str(private_entity["id"])

    assert all(private_id not in (row.get("source_id"), row.get("target_id")) for row in rows)


@pytest.mark.asyncio
async def test_mcp_query_relationships_hides_private_nodes(
    db_pool, enums, test_entity, untrusted_mcp_context
):
    """Relationship query should not leak private node ids."""

    private_entity = await _make_entity(db_pool, enums, "Private Node", ["sensitive"])
    relationship_type_id = enums.relationship_types.name_to_id["related-to"]
    status_id = enums.statuses.name_to_id["active"]

    await db_pool.execute(
        """
        INSERT INTO relationships (source_type, source_id, target_type, target_id, type_id, status_id, properties)
        VALUES ('entity', $1, 'entity', $2, $3, $4, $5::jsonb)
        """,
        str(test_entity["id"]),
        str(private_entity["id"]),
        relationship_type_id,
        status_id,
        json.dumps({"note": "secret link"}),
    )

    payload = QueryRelationshipsInput(
        source_type="entity",
        target_type="entity",
        relationship_types=["related-to"],
        status_category="active",
        limit=50,
    )
    rows = await query_relationships(payload, untrusted_mcp_context)
    private_id = str(private_entity["id"])

    assert all(private_id not in (row.get("source_id"), row.get("target_id")) for row in rows)


@pytest.mark.asyncio
async def test_mcp_audit_log_requires_admin(db_pool, enums, untrusted_mcp_context):
    """Audit log should be restricted to admin agents."""

    await _make_entity(db_pool, enums, "Audit Target", ["public"])

    payload = QueryAuditLogInput(limit=50)
    with pytest.raises(ValueError, match="Admin scope required"):
        await query_audit_log(payload, untrusted_mcp_context)
