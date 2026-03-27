"""Red team API tests for entity history access isolation."""

# Standard Library

# Third-Party
import pytest
from httpx import ASGITransport, AsyncClient

# Local
from nebula_api.app import app
from nebula_api.auth import require_auth


async def _make_entity(db_pool, enums, name, scopes):
    """Insert a test entity for history access scenarios."""

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
async def test_api_entity_history_denies_private_entity(db_pool, enums):
    """Entity history should be denied for private entities via API."""

    private_entity = await _make_entity(db_pool, enums, "Private", ["sensitive"])
    await db_pool.execute(
        "UPDATE entities SET name = $1 WHERE id = $2",
        "Private Updated",
        private_entity["id"],
    )

    auth_dict = {
        "key_id": None,
        "caller_type": "agent",
        "entity_id": None,
        "entity": None,
        "agent_id": "history-agent",
        "agent": {"id": "history-agent"},
        "scopes": [enums.scopes.name_to_id["public"]],
    }

    async def mock_auth():
        """Mock auth for public agent."""

        return auth_dict

    app.dependency_overrides[require_auth] = mock_auth
    app.state.pool = db_pool
    app.state.enums = enums
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.get(f"/api/entities/{private_entity['id']}/history")
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 403
