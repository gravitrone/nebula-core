"""Red team API tests for agent read isolation on entities."""

# Standard Library

# Third-Party
import pytest
from httpx import ASGITransport, AsyncClient

# Local
from nebula_api.app import app
from nebula_api.auth import require_auth


async def _make_agent(db_pool, enums, name, scopes):
    """Insert a test agent for read isolation scenarios."""

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


async def _make_entity(db_pool, enums, name, scopes):
    """Insert an entity for read isolation scenarios."""

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


def _auth_override(agent_id, enums, scopes):
    """Build a scoped agent auth override for API requests."""

    auth_dict = {
        "key_id": None,
        "caller_type": "agent",
        "entity_id": None,
        "entity": None,
        "agent_id": agent_id,
        "agent": {"id": agent_id},
        "scopes": [enums.scopes.name_to_id[s] for s in scopes],
    }

    async def mock_auth():
        """Mock auth for agent scoped requests."""

        return auth_dict

    return mock_auth


@pytest.mark.asyncio
async def test_api_agent_query_entities_hides_private(db_pool, enums):
    """Public-only agents should not list private entities."""

    public_entity = await _make_entity(db_pool, enums, "Public", ["public"])
    private_entity = await _make_entity(db_pool, enums, "Private", ["private"])
    agent = await _make_agent(db_pool, enums, "public-agent", ["public"])

    app.dependency_overrides[require_auth] = _auth_override(agent["id"], enums, ["public"])
    app.state.pool = db_pool
    app.state.enums = enums
    transport = ASGITransport(app=app)
    async with AsyncClient(
        transport=transport, base_url="http://test", follow_redirects=True
    ) as client:
        resp = await client.get("/api/entities/")
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 200
    ids = {row["id"] for row in resp.json()["data"]}
    assert str(private_entity["id"]) not in ids
    assert str(public_entity["id"]) in ids


@pytest.mark.asyncio
async def test_api_agent_get_entity_denies_private(db_pool, enums):
    """Public-only agents should be blocked from private entities."""

    private_entity = await _make_entity(db_pool, enums, "Private 2", ["private"])
    agent = await _make_agent(db_pool, enums, "public-agent-2", ["public"])

    app.dependency_overrides[require_auth] = _auth_override(agent["id"], enums, ["public"])
    app.state.pool = db_pool
    app.state.enums = enums
    transport = ASGITransport(app=app)
    async with AsyncClient(
        transport=transport, base_url="http://test", follow_redirects=True
    ) as client:
        resp = await client.get(f"/api/entities/{private_entity['id']}")
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 403
