"""Red team tests for approval diff access control."""

# Standard Library
import json
from unittest.mock import MagicMock

# Third-Party
import pytest

# Local
from nebula_mcp.models import GetApprovalDiffInput
from nebula_mcp.server import get_approval_diff


def _make_context(pool, enums, agent):
    """Build a mock MCP context for approval diff tests."""

    ctx = MagicMock()
    ctx.request_context.lifespan_context = {
        "pool": pool,
        "enums": enums,
        "agent": agent,
    }
    return ctx


async def _make_agent(db_pool, enums, name):
    """Insert a test agent for approval diff scenarios."""

    status_id = enums.statuses.name_to_id["active"]
    scope_ids = [enums.scopes.name_to_id["public"]]

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


async def _make_approval_request(db_pool, agent_id):
    """Insert a pending approval request owned by an agent."""

    row = await db_pool.fetchrow(
        """
        INSERT INTO approval_requests (request_type, requested_by, change_details, status)
        VALUES ($1, $2, $3::jsonb, $4)
        RETURNING *
        """,
        "create_entity",
        agent_id,
        json.dumps(
            {
                "name": "ApprovalLeak",
                "type": "person",
                "scopes": ["public"],
                "tags": [],
                "source_path": None,
                "status": "active",
            }
        ),
        "pending",
    )
    return dict(row)


@pytest.mark.asyncio
async def test_get_approval_diff_leaks_other_agent(db_pool, enums):
    """Approval diff should be restricted to requester or admin."""

    requester = await _make_agent(db_pool, enums, "approval-owner")
    viewer = await _make_agent(db_pool, enums, "approval-viewer")
    approval = await _make_approval_request(db_pool, requester["id"])

    ctx = _make_context(db_pool, enums, viewer)
    payload = GetApprovalDiffInput(approval_id=str(approval["id"]))

    with pytest.raises(ValueError):
        await get_approval_diff(payload, ctx)
