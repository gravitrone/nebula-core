"""Semantic search API tests."""

# Standard Library
import json

# Third-Party
import pytest


async def _insert_entity(
    db_pool, enums, *, name: str, scopes: list[str], metadata: dict
):
    """Handle insert entity.

    Args:
        db_pool: Input parameter for _insert_entity.
        enums: Input parameter for _insert_entity.
        name: Input parameter for _insert_entity.
        scopes: Input parameter for _insert_entity.
        metadata: Input parameter for _insert_entity.

    Returns:
        Result value from the operation.
    """

    status_id = enums.statuses.name_to_id["active"]
    type_id = enums.entity_types.name_to_id["project"]
    scope_ids = [enums.scopes.name_to_id[s] for s in scopes]
    row = await db_pool.fetchrow(
        """
        INSERT INTO entities (name, type_id, status_id, privacy_scope_ids, tags, metadata)
        VALUES ($1, $2, $3, $4, $5, $6::jsonb)
        RETURNING id, name
        """,
        name,
        type_id,
        status_id,
        scope_ids,
        ["semantic"],
        json.dumps(metadata),
    )
    out = dict(row)
    out["id"] = str(out["id"])
    return out


async def _insert_context(
    db_pool, enums, *, title: str, scopes: list[str], content: str
):
    """Handle insert context.

    Args:
        db_pool: Input parameter for _insert_context.
        enums: Input parameter for _insert_context.
        title: Input parameter for _insert_context.
        scopes: Input parameter for _insert_context.
        content: Input parameter for _insert_context.

    Returns:
        Result value from the operation.
    """

    status_id = enums.statuses.name_to_id["active"]
    scope_ids = [enums.scopes.name_to_id[s] for s in scopes]
    row = await db_pool.fetchrow(
        """
        INSERT INTO context_items (title, source_type, content, status_id, privacy_scope_ids, tags, metadata)
        VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb)
        RETURNING id, title
        """,
        title,
        "note",
        content,
        status_id,
        scope_ids,
        ["semantic"],
        json.dumps({"topic": "memory"}),
    )
    out = dict(row)
    out["id"] = str(out["id"])
    return out


@pytest.mark.asyncio
async def test_semantic_search_happy_path(api, db_pool, enums):
    """Semantic search should return ranked matches for entities and context."""

    entity = await _insert_entity(
        db_pool,
        enums,
        name="Agent Memory Mesh",
        scopes=["public"],
        metadata={"summary": "Context memory mesh for agent collaboration"},
    )
    context = await _insert_context(
        db_pool,
        enums,
        title="Prompt Memory Patterns",
        scopes=["public"],
        content="How retrieval memory improves agent orchestration.",
    )

    resp = await api.post(
        "/api/search/semantic",
        json={"query": "agent retrieval memory", "kinds": ["entity", "context"]},
    )
    assert resp.status_code == 200
    data = resp.json()["data"]
    ids = {item["id"] for item in data}
    assert entity["id"] in ids
    assert context["id"] in ids
    assert all("score" in item for item in data)


@pytest.mark.asyncio
async def test_semantic_search_enforces_scopes(api, db_pool, enums, auth_override):
    """Semantic search should not return private-only items to user callers."""

    public_entity = await _insert_entity(
        db_pool,
        enums,
        name="Public Agent Context",
        scopes=["public"],
        metadata={"summary": "Public context memory"},
    )
    private_entity = await _insert_entity(
        db_pool,
        enums,
        name="Sensitive Agent Context",
        scopes=["sensitive"],
        metadata={"summary": "Private context memory"},
    )

    # User caller search is constrained to public scope for list/search endpoints.
    auth_override["scopes"] = [enums.scopes.name_to_id["public"]]

    resp = await api.post(
        "/api/search/semantic",
        json={"query": "context memory", "kinds": ["entity"]},
    )
    assert resp.status_code == 200
    data = resp.json()["data"]
    ids = {item["id"] for item in data}
    assert public_entity["id"] in ids
    assert private_entity["id"] not in ids


@pytest.mark.asyncio
async def test_semantic_search_rejects_invalid_payload(api):
    """Semantic search should reject empty query payloads."""

    resp = await api.post(
        "/api/search/semantic",
        json={"query": " ", "limit": 0},
    )
    assert resp.status_code == 422
