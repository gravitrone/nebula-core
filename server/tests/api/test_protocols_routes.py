"""Protocol route tests."""

# Standard Library
import json

# Third-Party
import pytest


@pytest.mark.asyncio
async def test_query_protocols_filters_trusted_for_non_admin(api, db_pool, enums):
    """Protocol list should hide trusted rows for non-admin callers."""

    status_id = enums.statuses.name_to_id["active"]
    await db_pool.execute(
        """
        INSERT INTO protocols (
            name, title, version, content, protocol_type, applies_to,
            status_id, tags, trusted, metadata, source_path
        )
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,TRUE,$9::jsonb,$10)
        """,
        "trusted-hidden",
        "Trusted Hidden",
        "1.0.0",
        "internal-only",
        "system",
        ["agents"],
        status_id,
        ["internal"],
        json.dumps({"kind": "trusted"}),
        None,
    )
    await db_pool.execute(
        """
        INSERT INTO protocols (
            name, title, version, content, protocol_type, applies_to,
            status_id, tags, trusted, metadata, source_path
        )
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,FALSE,$9::jsonb,$10)
        """,
        "public-visible",
        "Public Visible",
        "1.0.0",
        "public",
        "system",
        ["agents"],
        status_id,
        ["public"],
        json.dumps({"kind": "public"}),
        None,
    )

    resp = await api.get("/api/protocols/")
    assert resp.status_code == 200
    names = {item["name"] for item in resp.json()["data"]}
    assert "public-visible" in names
    assert "trusted-hidden" not in names


@pytest.mark.asyncio
async def test_query_protocols_limit_does_not_starve_public_rows(api, db_pool, enums):
    """Non-admin protocol list should still return public rows under tight limits."""

    status_id = enums.statuses.name_to_id["active"]
    await db_pool.execute(
        """
        INSERT INTO protocols (
            name, title, version, content, protocol_type, applies_to,
            status_id, tags, trusted, metadata, source_path
        )
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,TRUE,$9::jsonb,$10)
        """,
        "a-trusted-protocol",
        "Trusted First",
        "1.0.0",
        "internal-only",
        "system",
        ["agents"],
        status_id,
        ["internal"],
        json.dumps({"kind": "trusted"}),
        None,
    )
    await db_pool.execute(
        """
        INSERT INTO protocols (
            name, title, version, content, protocol_type, applies_to,
            status_id, tags, trusted, metadata, source_path
        )
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,FALSE,$9::jsonb,$10)
        """,
        "z-public-protocol",
        "Public Later",
        "1.0.0",
        "public",
        "system",
        ["agents"],
        status_id,
        ["public"],
        json.dumps({"kind": "public"}),
        None,
    )

    resp = await api.get("/api/protocols/", params={"status_category": "active", "limit": 1})
    assert resp.status_code == 200
    names = [item["name"] for item in resp.json()["data"]]
    assert names
    assert "z-public-protocol" in names


@pytest.mark.asyncio
async def test_get_protocol_not_found(api):
    """Protocol get should return 404 when name does not exist."""

    resp = await api.get("/api/protocols/missing-protocol")
    assert resp.status_code == 404


@pytest.mark.asyncio
async def test_create_protocol_invalid_status_returns_400(api):
    """Protocol create should reject unknown status names."""

    resp = await api.post(
        "/api/protocols/",
        json={
            "name": "bad-status-create",
            "title": "Bad Status",
            "version": "1.0.0",
            "content": "x",
            "status": "todo",
            "tags": ["rt"],
        },
    )
    assert resp.status_code == 400


@pytest.mark.asyncio
async def test_update_protocol_invalid_status_returns_400(api):
    """Protocol update should reject unknown status names."""

    create = await api.post(
        "/api/protocols/",
        json={
            "name": "bad-status-update",
            "title": "Bad Status Update",
            "version": "1.0.0",
            "content": "x",
            "status": "active",
            "tags": ["rt"],
        },
    )
    assert create.status_code == 200

    resp = await api.patch(
        "/api/protocols/bad-status-update",
        json={"status": "todo"},
    )
    assert resp.status_code == 400


@pytest.mark.asyncio
async def test_update_protocol_missing_row_returns_empty_payload(api):
    """Protocol update currently returns empty data for unknown names."""

    resp = await api.patch(
        "/api/protocols/missing-for-update",
        json={"title": "won't apply"},
    )
    assert resp.status_code == 200
    assert resp.json()["data"] == {}
