"""Red team tests for MCP taxonomy admin scope boundaries."""

# Third-Party
from unittest.mock import MagicMock
from uuid import uuid4

import pytest

# Local
from nebula_mcp.models import CreateTaxonomyInput, ListTaxonomyInput
from nebula_mcp.server import create_taxonomy, list_taxonomy


async def _make_context(pool, enums, scope_ids: list) -> MagicMock:
    """Build MCP context with specific scope ids."""

    agent_id = str(uuid4())
    await pool.execute(
        """
        INSERT INTO agents (id, name, description, scopes, requires_approval, status_id)
        VALUES ($1::uuid, $2, $3, $4, $5, $6::uuid)
        """,
        agent_id,
        "taxonomy-sensitive-agent",
        "taxonomy privilege helper",
        scope_ids,
        False,
        enums.statuses.name_to_id["active"],
    )

    ctx = MagicMock()
    ctx.request_context.lifespan_context = {
        "pool": pool,
        "enums": enums,
        "agent": {
            "id": agent_id,
            "name": "taxonomy-sensitive-agent",
            "scopes": scope_ids,
            "requires_approval": False,
        },
    }
    return ctx


@pytest.mark.asyncio
async def test_mcp_taxonomy_sensitive_scope_cannot_create(db_pool, enums):
    """A sensitive-only agent should not create taxonomy rows."""

    ctx = await _make_context(db_pool, enums, [enums.scopes.name_to_id["sensitive"]])
    created: dict | None = None
    try:
        created = await create_taxonomy(
            CreateTaxonomyInput(
                kind="scopes",
                name="rt-mcp-sensitive-bypass-scope",
                description="should be forbidden",
            ),
            ctx,
        )
    except ValueError as exc:
        assert "Admin scope required" in str(exc)
        return
    finally:
        # Taxonomy tables are not truncated by the global test cleanup fixture.
        if created is not None:
            await db_pool.execute(
                "DELETE FROM privacy_scopes WHERE id = $1::uuid",
                str(created["id"]),
            )

    pytest.fail("Expected ValueError for non-admin taxonomy create")


@pytest.mark.asyncio
async def test_mcp_taxonomy_sensitive_scope_cannot_list(db_pool, enums):
    """A sensitive-only agent should not list taxonomy rows."""

    ctx = await _make_context(db_pool, enums, [enums.scopes.name_to_id["sensitive"]])
    with pytest.raises(ValueError, match="Admin scope required"):
        await list_taxonomy(ListTaxonomyInput(kind="scopes"), ctx)
