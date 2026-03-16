"""Red team tests for bulk import ownership and scope isolation."""

# Standard Library
import json
from unittest.mock import MagicMock

# Third-Party
import pytest

# Local
from nebula_mcp.models import BulkImportInput
from nebula_mcp.server import bulk_import_jobs, bulk_import_relationships


def _make_context(pool, enums, agent):
    """Build a mock MCP context for bulk import tests."""

    ctx = MagicMock()
    ctx.request_context.lifespan_context = {
        "pool": pool,
        "enums": enums,
        "agent": agent,
    }
    return ctx


async def _make_agent(db_pool, enums, name, scopes, requires_approval):
    """Insert a test agent for bulk import scenarios."""

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
        requires_approval,
        status_id,
    )
    return dict(row)


async def _make_entity(db_pool, enums, name, scopes):
    """Insert a test entity for bulk import scenarios."""

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
async def test_bulk_import_jobs_spoofing_denied(db_pool, enums):
    """Agents should not bulk import jobs owned by other agents."""

    owner = await _make_agent(db_pool, enums, "bulk-owner", ["public"], False)
    viewer = await _make_agent(db_pool, enums, "bulk-viewer", ["public"], False)
    ctx = _make_context(db_pool, enums, viewer)

    payload = BulkImportInput(
        format="json",
        items=[
            {
                "title": "Spoofed Job",
                "agent_id": str(owner["id"]),
                "priority": "high",
            }
        ],
    )

    result = await bulk_import_jobs(payload, ctx)
    items = result.get("items", [])
    if not items:
        assert result.get("failed", 0) >= 1 or result.get("created", 0) == 0
        return
    assert items[0]["agent_id"] == str(viewer["id"])


@pytest.mark.asyncio
async def test_bulk_import_relationships_private_target_denied(db_pool, enums):
    """Agents should not bulk import relationships to private entities."""

    private_entity = await _make_entity(db_pool, enums, "Private", ["sensitive"])
    public_entity = await _make_entity(db_pool, enums, "Public", ["public"])
    viewer = await _make_agent(db_pool, enums, "bulk-linker", ["public"], False)
    ctx = _make_context(db_pool, enums, viewer)

    payload = BulkImportInput(
        format="json",
        items=[
            {
                "source_type": "entity",
                "source_id": str(public_entity["id"]),
                "target_type": "entity",
                "target_id": str(private_entity["id"]),
                "relationship_type": "related-to",
            }
        ],
    )

    result = await bulk_import_relationships(payload, ctx)
    items = result.get("items", [])
    if not items:
        assert result.get("failed", 0) >= 1 or result.get("created", 0) == 0
        return
    assert items[0]["target_id"] != str(private_entity["id"])
