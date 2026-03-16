"""Red team security regression tests for API routes."""

# Standard Library
import json

# Third-Party
import pytest


@pytest.mark.asyncio
async def test_agents_list_requires_admin(api):
    """Non-admin users should not enumerate agents."""

    resp = await api.get("/api/agents/")
    assert resp.status_code == 403


@pytest.mark.asyncio
async def test_agents_info_requires_admin(api, test_agent_row):
    """Non-admin users should not read agent details."""

    resp = await api.get(f"/api/agents/{test_agent_row['name']}")
    assert resp.status_code == 403


@pytest.mark.asyncio
async def test_audit_log_requires_admin(api):
    """Non-admin users should not read audit logs."""

    resp = await api.get("/api/audit")
    assert resp.status_code == 403


@pytest.mark.asyncio
async def test_relationships_hide_private_targets(api, db_pool, enums, test_entity):
    """Relationship list should filter out private nodes for non-admin users."""

    status_id = enums.statuses.name_to_id["active"]
    type_id = enums.entity_types.name_to_id["person"]
    private_scope_id = enums.scopes.name_to_id["sensitive"]

    private_entity = await db_pool.fetchrow(
        """
        INSERT INTO entities (name, type_id, status_id, privacy_scope_ids, tags)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING *
        """,
        "Private Person",
        type_id,
        status_id,
        [private_scope_id],
        ["private"],
    )

    relationship_type_id = enums.relationship_types.name_to_id["related-to"]

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

    resp = await api.get(f"/api/relationships/entity/{test_entity['id']}")
    assert resp.status_code == 200
    data = resp.json()["data"]

    private_id = str(private_entity["id"])
    assert all(
        private_id not in (row.get("source_id"), row.get("target_id")) for row in data
    )
