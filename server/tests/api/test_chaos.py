"""Chaos tests for API behavior and resilience."""

# Standard Library
import asyncio

# Third-Party
import pytest


@pytest.mark.asyncio
async def test_concurrent_entity_updates(api):
    """Update the same entity concurrently and ensure responses succeed."""

    payload = {
        "name": "chaos-entity",
        "type": "person",
        "status": "active",
        "scopes": ["public"],
        "tags": ["chaos"],
    }
    create = await api.post("/api/entities", json=payload)
    assert create.status_code in (200, 202)
    entity_id = create.json()["data"]["id"]

    async def do_update(i: int):
        """Run a single concurrent update request."""

        return await api.patch(
            f"/api/entities/{entity_id}",
            json={"tags": [f"iter-{i}"]},
        )

    results = await asyncio.gather(*[do_update(i) for i in range(50)])
    assert all(r.status_code in (200, 202) for r in results)


@pytest.mark.asyncio
async def test_state_desync_reflects_db_changes(api, db_pool):
    """Reflect direct DB updates on subsequent API reads."""

    payload = {
        "name": "desync-entity",
        "type": "person",
        "status": "active",
        "scopes": ["public"],
        "tags": [],
    }
    create = await api.post("/api/entities", json=payload)
    assert create.status_code in (200, 202)
    entity_id = create.json()["data"]["id"]

    await db_pool.execute(
        "UPDATE entities SET name = $1 WHERE id = $2",
        "desync-updated",
        entity_id,
    )

    fetched = await api.get(f"/api/entities/{entity_id}")
    assert fetched.status_code == 200
    assert fetched.json()["data"]["name"] == "desync-updated"


@pytest.mark.asyncio
async def test_malformed_json_rejected(api):
    """Reject malformed JSON payloads for entity creation."""

    resp = await api.post(
        "/api/entities",
        content=b"{bad json",
        headers={"content-type": "application/json"},
    )
    assert resp.status_code >= 400


@pytest.mark.asyncio
async def test_large_payload_and_unicode(api):
    """Handle large payloads and unicode data in context creation."""

    big_text = ("x" * 9_999_000) + " 🚀 日本語"
    payload = {
        "title": "big-context",
        "source_type": "note",
        "content": big_text,
        "scopes": ["public"],
        "tags": ["load"],
    }
    resp = await api.post("/api/context", json=payload)
    assert resp.status_code in (200, 202, 413)


@pytest.mark.asyncio
async def test_auth_fuzzing(api_no_auth, api_key_row, db_pool, enums):
    """Reject invalid or revoked API keys during auth checks."""

    raw_key, row = api_key_row

    missing = await api_no_auth.get("/api/entities")
    assert missing.status_code in (401, 403)

    random_key = await api_no_auth.get("/api/entities", headers={"x-api-key": "nope"})
    assert random_key.status_code in (401, 403)

    archived_id = enums.statuses.name_to_id.get("archived")
    if not archived_id:
        archived_id = next(
            (val for key, val in enums.statuses.name_to_id.items() if key != "active"),
            None,
        )
    if not archived_id:
        pytest.skip("No inactive status available to revoke api key")
    await db_pool.execute(
        "UPDATE api_keys SET revoked_at = NOW() WHERE id = $1",
        row["id"],
    )
    revoked = await api_no_auth.get("/api/entities", headers={"x-api-key": raw_key})
    assert revoked.status_code in (401, 403)
