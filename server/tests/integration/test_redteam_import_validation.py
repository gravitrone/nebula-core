"""Red team tests for import validation in MCP bulk import tools."""

# Standard Library
from unittest.mock import MagicMock

# Third-Party
import pytest

# Local
from nebula_mcp.models import BulkImportInput
from nebula_mcp.server import bulk_import_entities


def _make_context(pool, enums, agent):
    """Build MCP context for import validation tests."""

    ctx = MagicMock()
    ctx.request_context.lifespan_context = {
        "pool": pool,
        "enums": enums,
        "agent": agent,
    }
    return ctx


async def _make_agent(db_pool, enums, name, scopes):
    """Insert an agent for import validation tests."""

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


@pytest.mark.asyncio
async def test_bulk_import_entities_rejects_invalid_format(db_pool, enums):
    """Invalid format should not crash MCP bulk import."""

    agent = await _make_agent(db_pool, enums, "import-format-agent", ["public"])
    ctx = _make_context(db_pool, enums, agent)
    payload = BulkImportInput(
        format="xml",
        items=[
            {
                "name": "BadFormat",
                "type": "person",
                "status": "active",
                "scopes": ["public"],
                "tags": [],
            }
        ],
    )

    with pytest.raises(ValueError):
        await bulk_import_entities(payload, ctx)


@pytest.mark.asyncio
async def test_bulk_import_entities_rejects_empty_csv(db_pool, enums):
    """Missing CSV data should not crash MCP bulk import."""

    agent = await _make_agent(db_pool, enums, "import-csv-agent", ["public"])
    ctx = _make_context(db_pool, enums, agent)
    payload = BulkImportInput(format="csv", data="")

    with pytest.raises(ValueError):
        await bulk_import_entities(payload, ctx)
