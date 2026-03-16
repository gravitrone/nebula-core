"""E2E test: entity create -> query -> update -> audit trail."""

# Standard Library

import pytest

from nebula_mcp.executors import execute_create_entity, execute_update_entity

pytestmark = pytest.mark.e2e


# --- Entity Lifecycle ---


@pytest.mark.asyncio
async def test_entity_full_lifecycle(db_pool, enums):
    """Create an entity, query it back, update it, and verify the audit trail."""

    # --- Create ---
    create_payload = {
        "name": "Lifecycle Entity",
        "type": "project",
        "status": "active",
        "scopes": ["public"],
        "tags": ["lifecycle", "test"],
    }

    created = await execute_create_entity(db_pool, enums, create_payload)
    entity_id = str(created["id"])

    # --- Query back directly ---
    row = await db_pool.fetchrow("SELECT * FROM entities WHERE id = $1::uuid", entity_id)
    assert row is not None
    assert row["name"] == "Lifecycle Entity"
    assert "lifecycle" in row["tags"]

    # --- Update ---
    update_payload = {
        "entity_id": entity_id,
        "tags": ["lifecycle", "test", "updated"],
    }

    updated = await execute_update_entity(db_pool, enums, update_payload)
    assert "updated" in updated["tags"]

    # --- Verify audit trail ---
    audit_rows = await db_pool.fetch(
        """
        SELECT action FROM audit_log
        WHERE table_name = 'entities' AND record_id = $1
        ORDER BY changed_at ASC
        """,
        entity_id,
    )

    actions = [r["action"] for r in audit_rows]
    assert "insert" in actions
    assert "update" in actions
