"""Red team API tests for metadata privacy filtering in queries."""

# Standard Library
import json

# Third-Party
import pytest


async def _make_entity(db_pool, enums, name, scopes, metadata):
    """Insert a test entity for metadata filtering scenarios."""

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
async def test_api_query_entities_filters_context_segments(api, db_pool, enums):
    """API query results should not include context segments outside scopes."""

    metadata = {
        "context_segments": [
            {"text": "public info", "scopes": ["public"]},
            {"text": "private info", "scopes": ["private"]},
        ]
    }
    await _make_entity(db_pool, enums, "Mixed Scope", ["public", "private"], metadata)

    resp = await api.get("/api/entities")
    assert resp.status_code == 200
    data = resp.json()["data"]
    assert data
    segments = data[0]["metadata"].get("context_segments", [])

    assert all("private" not in seg.get("scopes", []) for seg in segments)


@pytest.mark.asyncio
async def test_api_search_entities_filters_context_segments(api, db_pool, enums):
    """API metadata search should not leak context segments outside scopes."""

    metadata = {
        "context_segments": [
            {"text": "public info", "scopes": ["public"]},
            {"text": "private info", "scopes": ["private"]},
        ],
        "signal": "needle",
    }
    await _make_entity(db_pool, enums, "Metadata Leak", ["public", "private"], metadata)

    resp = await api.post(
        "/api/entities/search",
        json={"metadata_query": {"signal": "needle"}},
    )
    assert resp.status_code == 200
    data = resp.json()["data"]
    assert data
    segments = data[0]["metadata"].get("context_segments", [])

    assert all("private" not in seg.get("scopes", []) for seg in segments)


@pytest.mark.asyncio
async def test_api_search_entities_hides_private_entities(api, db_pool, enums):
    """API metadata search should not return private-only entities."""

    metadata = {"signal": "private-only"}
    await _make_entity(db_pool, enums, "Private Node", ["private"], metadata)

    resp = await api.post(
        "/api/entities/search",
        json={"metadata_query": {"signal": "private-only"}},
    )
    assert resp.status_code == 200
    data = resp.json()["data"]

    assert not data
