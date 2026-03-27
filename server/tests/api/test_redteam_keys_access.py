"""Red team API tests for key listing access."""

# Third-Party
import pytest
from httpx import ASGITransport, AsyncClient

# Local
from nebula_api.app import app
from nebula_api.auth import require_auth


async def _make_agent(db_pool, enums, name):
    """Insert a test agent for key access scenarios."""

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


def _auth_override(agent_id, enums):
    """Build an auth override for public agent requests."""

    scope_ids = [enums.scopes.name_to_id["public"]]
    auth_dict = {
        "key_id": None,
        "caller_type": "agent",
        "entity_id": None,
        "entity": None,
        "agent_id": agent_id,
        "agent": {"id": agent_id},
        "scopes": scope_ids,
    }

    async def mock_auth():
        """Mock auth for public agent."""

        return auth_dict

    return mock_auth


@pytest.mark.asyncio
async def test_agent_cannot_list_all_keys(db_pool, enums):
    """Agents should be blocked from listing all API keys."""

    viewer = await _make_agent(db_pool, enums, "keys-viewer")

    app.dependency_overrides[require_auth] = _auth_override(viewer["id"], enums)
    app.state.pool = db_pool
    app.state.enums = enums
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.get("/api/keys/all")
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code in (401, 403)
