"""Red team tests for relationship privacy across entity scopes."""

# Standard Library
import json
from unittest.mock import MagicMock

# Third-Party
import pytest

# Local
from nebula_mcp.models import GetRelationshipsInput, QueryRelationshipsInput
from nebula_mcp.server import get_relationships, query_relationships


def _make_context(pool, enums, agent):
    """Build a mock MCP context for relationship privacy tests."""

    ctx = MagicMock()
    ctx.request_context.lifespan_context = {
        "pool": pool,
        "enums": enums,
        "agent": agent,
    }
    return ctx


def _properties_dict(value):
    """Normalize relationship properties payload into a dict."""

    if isinstance(value, dict):
        return value
    if isinstance(value, str):
        try:
            parsed = json.loads(value)
        except json.JSONDecodeError:
            return {}
        return parsed if isinstance(parsed, dict) else {}
    return {}


async def _make_agent(db_pool, enums, name, scopes):
    """Insert a test agent for relationship privacy scenarios."""

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


async def _make_entity(db_pool, enums, name, scopes):
    """Insert a test entity for relationship privacy scenarios."""

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


async def _make_relationship(db_pool, enums, source_id, target_id):
    """Insert a relationship linking two entities."""

    status_id = enums.statuses.name_to_id["active"]
    type_id = enums.relationship_types.name_to_id["related-to"]

    row = await db_pool.fetchrow(
        """
        INSERT INTO relationships (source_type, source_id, target_type, target_id, type_id, status_id, properties)
        VALUES ('entity', $1, 'entity', $2, $3, $4, $5::jsonb)
        RETURNING *
        """,
        str(source_id),
        str(target_id),
        type_id,
        status_id,
        json.dumps({"note": "private-link"}),
    )
    return dict(row)


async def _make_scoped_properties_relationship(db_pool, enums, source_id, target_id):
    """Insert a relationship with mixed-scope context segments in properties."""

    status_id = enums.statuses.name_to_id["active"]
    type_id = enums.relationship_types.name_to_id["related-to"]
    properties = {
        "context_segments": [
            {"text": "public edge context", "scopes": ["public"]},
            {"text": "sensitive edge context", "scopes": ["sensitive"]},
        ],
        "note": "mixed-scope",
    }
    row = await db_pool.fetchrow(
        """
        INSERT INTO relationships (source_type, source_id, target_type, target_id, type_id, status_id, properties)
        VALUES ('entity', $1, 'entity', $2, $3, $4, $5::jsonb)
        RETURNING *
        """,
        str(source_id),
        str(target_id),
        type_id,
        status_id,
        json.dumps(properties),
    )
    return dict(row)


@pytest.mark.asyncio
async def test_get_relationships_hides_private_entities(db_pool, enums):
    """Get relationships should hide links to private entities."""

    public_entity = await _make_entity(db_pool, enums, "Public", ["public"])
    private_entity = await _make_entity(db_pool, enums, "Private", ["sensitive"])
    rel = await _make_relationship(
        db_pool, enums, public_entity["id"], private_entity["id"]
    )

    viewer = await _make_agent(db_pool, enums, "rel-viewer", ["public"])
    ctx = _make_context(db_pool, enums, viewer)

    payload = GetRelationshipsInput(
        source_type="entity",
        source_id=str(public_entity["id"]),
        direction="both",
        relationship_type=None,
    )
    results = await get_relationships(payload, ctx)
    ids = {row["id"] for row in results}

    assert rel["id"] not in ids


@pytest.mark.asyncio
async def test_query_relationships_hides_private_entities(db_pool, enums):
    """Query relationships should not expose private entity links."""

    public_entity = await _make_entity(db_pool, enums, "Public 2", ["public"])
    private_entity = await _make_entity(db_pool, enums, "Private 2", ["sensitive"])
    rel = await _make_relationship(
        db_pool, enums, public_entity["id"], private_entity["id"]
    )

    viewer = await _make_agent(db_pool, enums, "rel-viewer-2", ["public"])
    ctx = _make_context(db_pool, enums, viewer)

    payload = QueryRelationshipsInput(
        source_type=None,
        target_type=None,
        relationship_types=[],
        status_category="active",
        limit=50,
    )
    results = await query_relationships(payload, ctx)
    ids = {row["id"] for row in results}

    assert rel["id"] not in ids


@pytest.mark.asyncio
async def test_get_relationships_properties_payload_is_object(db_pool, enums):
    """MCP relationship payload should return properties as object, not JSON string."""

    source_entity = await _make_entity(db_pool, enums, "Public Type Src", ["public"])
    target_entity = await _make_entity(db_pool, enums, "Public Type Dst", ["public"])
    rel = await _make_scoped_properties_relationship(
        db_pool, enums, source_entity["id"], target_entity["id"]
    )

    viewer = await _make_agent(db_pool, enums, "rel-type-viewer", ["public"])
    ctx = _make_context(db_pool, enums, viewer)

    results = await get_relationships(
        GetRelationshipsInput(
            source_type="entity",
            source_id=str(source_entity["id"]),
            direction="both",
            relationship_type=None,
        ),
        ctx,
    )
    row = next((item for item in results if item["id"] == rel["id"]), None)
    assert row is not None
    assert isinstance(row.get("properties"), dict)


@pytest.mark.asyncio
async def test_get_relationships_filters_properties_context_segments(db_pool, enums):
    """MCP get_relationships should scope-filter relationship properties segments."""

    source_entity = await _make_entity(db_pool, enums, "Public Src", ["public"])
    target_entity = await _make_entity(db_pool, enums, "Public Dst", ["public"])
    rel = await _make_scoped_properties_relationship(
        db_pool, enums, source_entity["id"], target_entity["id"]
    )

    viewer = await _make_agent(db_pool, enums, "rel-viewer-props", ["public"])
    ctx = _make_context(db_pool, enums, viewer)

    results = await get_relationships(
        GetRelationshipsInput(
            source_type="entity",
            source_id=str(source_entity["id"]),
            direction="both",
            relationship_type=None,
        ),
        ctx,
    )
    row = next((item for item in results if item["id"] == rel["id"]), None)
    assert row is not None
    segments = _properties_dict(row.get("properties")).get("context_segments", [])
    texts = {seg.get("text") for seg in segments if isinstance(seg, dict)}
    assert "public edge context" in texts
    assert "sensitive edge context" not in texts


@pytest.mark.asyncio
async def test_query_relationships_filters_properties_context_segments(db_pool, enums):
    """MCP query_relationships should scope-filter relationship properties segments."""

    source_entity = await _make_entity(db_pool, enums, "Public Src Q", ["public"])
    target_entity = await _make_entity(db_pool, enums, "Public Dst Q", ["public"])
    rel = await _make_scoped_properties_relationship(
        db_pool, enums, source_entity["id"], target_entity["id"]
    )

    viewer = await _make_agent(db_pool, enums, "rel-viewer-props-q", ["public"])
    ctx = _make_context(db_pool, enums, viewer)

    results = await query_relationships(
        QueryRelationshipsInput(
            source_type=None,
            target_type=None,
            relationship_types=[],
            status_category="active",
            limit=50,
        ),
        ctx,
    )
    row = next((item for item in results if item["id"] == rel["id"]), None)
    assert row is not None
    segments = _properties_dict(row.get("properties")).get("context_segments", [])
    texts = {seg.get("text") for seg in segments if isinstance(seg, dict)}
    assert "public edge context" in texts
    assert "sensitive edge context" not in texts
