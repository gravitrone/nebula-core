"""Red team tests for metadata privacy filtering in queries."""

# Standard Library
import json
from unittest.mock import MagicMock
from uuid import UUID, uuid4

# Third-Party
import pytest

# Local
from nebula_mcp.models import QueryEntitiesInput, SearchEntitiesByMetadataInput
from nebula_mcp.server import query_entities, search_entities_by_metadata


async def _make_context(pool, enums, agent):
    """Build a mock MCP context for metadata filtering tests."""

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
        "test metadata filter helper",
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


async def _make_entity(db_pool, enums, name, scopes, metadata):
    """Insert a test entity with scoped metadata."""

    status_id = enums.statuses.name_to_id["active"]
    type_id = enums.entity_types.name_to_id["person"]
    scope_ids = [enums.scopes.name_to_id[s] for s in scopes]

    row = await db_pool.fetchrow(
        """
        INSERT INTO entities (name, type_id, status_id, privacy_scope_ids, tags, metadata)
        VALUES ($1, $2, $3, $4, $5, $6::jsonb)
        RETURNING *
        """,
        name,
        type_id,
        status_id,
        scope_ids,
        ["test"],
        json.dumps(metadata),
    )
    return dict(row)


@pytest.mark.asyncio
async def test_query_entities_filters_context_segments(db_pool, enums):
    """Query results should not include context segments outside agent scopes."""

    metadata = {
        "context_segments": [
            {"text": "public info", "scopes": ["public"]},
            {"text": "private info", "scopes": ["private"]},
        ]
    }
    await _make_entity(db_pool, enums, "Mixed Scope", ["public", "private"], metadata)

    public_agent = {
        "id": "public-agent",
        "scopes": [enums.scopes.name_to_id["public"]],
    }
    ctx = await _make_context(db_pool, enums, public_agent)

    rows = await query_entities(QueryEntitiesInput(), ctx)
    assert rows
    segments = rows[0]["metadata"].get("context_segments", [])

    assert all("private" not in seg.get("scopes", []) for seg in segments)


@pytest.mark.asyncio
async def test_search_entities_by_metadata_filters_context_segments(db_pool, enums):
    """Metadata search should not leak context segments outside agent scopes."""

    metadata = {
        "context_segments": [
            {"text": "public info", "scopes": ["public"]},
            {"text": "private info", "scopes": ["private"]},
        ],
        "signal": "needle",
    }
    await _make_entity(db_pool, enums, "Metadata Leak", ["public", "private"], metadata)

    public_agent = {
        "id": "public-agent",
        "scopes": [enums.scopes.name_to_id["public"]],
    }
    ctx = await _make_context(db_pool, enums, public_agent)

    payload = SearchEntitiesByMetadataInput(metadata_query={"signal": "needle"})
    rows = await search_entities_by_metadata(payload, ctx)
    assert rows
    segments = rows[0]["metadata"].get("context_segments", [])

    assert all("private" not in seg.get("scopes", []) for seg in segments)


@pytest.mark.asyncio
async def test_search_entities_by_metadata_hides_private_entities(db_pool, enums):
    """Metadata search should not return entities outside agent scopes."""

    metadata = {"signal": "private-only"}
    await _make_entity(db_pool, enums, "Private Node", ["private"], metadata)

    public_agent = {
        "id": "public-agent",
        "scopes": [enums.scopes.name_to_id["public"]],
    }
    ctx = await _make_context(db_pool, enums, public_agent)

    payload = SearchEntitiesByMetadataInput(metadata_query={"signal": "private-only"})
    rows = await search_entities_by_metadata(payload, ctx)

    assert not rows
