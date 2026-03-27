"""Red team API tests for bulk update isolation."""

# Standard Library

# Third-Party
import pytest
from httpx import ASGITransport, AsyncClient

# Local
from nebula_api.app import app
from nebula_api.auth import require_auth


async def _make_agent(db_pool, enums, name, scopes, requires_approval):
    """Insert an agent for bulk update tests."""

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
    """Insert an entity for bulk update tests."""

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
async def test_api_bulk_update_tags_denies_private_entity(db_pool, enums):
    """API should deny bulk tag updates on private entities by public agents."""

    private_entity = await _make_entity(db_pool, enums, "Private", ["sensitive"])
    viewer = await _make_agent(db_pool, enums, "bulk-tagger", ["public"], False)

    auth_dict = {
        "key_id": None,
        "caller_type": "agent",
        "entity_id": None,
        "entity": None,
        "agent_id": viewer["id"],
        "agent": viewer,
        "scopes": [enums.scopes.name_to_id["public"]],
    }

    async def mock_auth():
        """Mock auth for viewer agent."""

        return auth_dict

    app.dependency_overrides[require_auth] = mock_auth
    app.state.pool = db_pool
    app.state.enums = enums
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.post(
            "/api/entities/bulk/tags",
            json={
                "entity_ids": [str(private_entity["id"])],
                "tags": ["pwn"],
                "op": "add",
            },
        )
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 403


@pytest.mark.asyncio
async def test_api_bulk_update_scopes_denies_private_entity(db_pool, enums):
    """API should deny bulk scope updates on private entities by public agents."""

    private_entity = await _make_entity(db_pool, enums, "Private", ["sensitive"])
    viewer = await _make_agent(db_pool, enums, "bulk-scope", ["public"], False)

    auth_dict = {
        "key_id": None,
        "caller_type": "agent",
        "entity_id": None,
        "entity": None,
        "agent_id": viewer["id"],
        "agent": viewer,
        "scopes": [enums.scopes.name_to_id["public"]],
    }

    async def mock_auth():
        """Mock auth for viewer agent."""

        return auth_dict

    app.dependency_overrides[require_auth] = mock_auth
    app.state.pool = db_pool
    app.state.enums = enums
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.post(
            "/api/entities/bulk/scopes",
            json={
                "entity_ids": [str(private_entity["id"])],
                "scopes": ["public"],
                "op": "add",
            },
        )
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 403


@pytest.mark.asyncio
async def test_api_bulk_update_tags_rejects_invalid_uuid(db_pool, enums):
    """API bulk tag updates should reject malformed UUIDs."""

    viewer = await _make_agent(db_pool, enums, "bulk-tag-invalid", ["public"], False)

    auth_dict = {
        "key_id": None,
        "caller_type": "agent",
        "entity_id": None,
        "entity": None,
        "agent_id": viewer["id"],
        "agent": viewer,
        "scopes": [enums.scopes.name_to_id["public"]],
    }

    async def mock_auth():
        """Mock auth for viewer agent."""

        return auth_dict

    app.dependency_overrides[require_auth] = mock_auth
    app.state.pool = db_pool
    app.state.enums = enums
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.post(
            "/api/entities/bulk/tags",
            json={
                "entity_ids": ["not-a-uuid"],
                "tags": ["pwn"],
                "op": "add",
            },
        )
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code in {400, 404}


@pytest.mark.asyncio
async def test_api_bulk_update_scopes_rejects_invalid_uuid(db_pool, enums):
    """API bulk scope updates should reject malformed UUIDs."""

    viewer = await _make_agent(db_pool, enums, "bulk-scope-invalid", ["public"], False)

    auth_dict = {
        "key_id": None,
        "caller_type": "agent",
        "entity_id": None,
        "entity": None,
        "agent_id": viewer["id"],
        "agent": viewer,
        "scopes": [enums.scopes.name_to_id["public"]],
    }

    async def mock_auth():
        """Mock auth for viewer agent."""

        return auth_dict

    app.dependency_overrides[require_auth] = mock_auth
    app.state.pool = db_pool
    app.state.enums = enums
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.post(
            "/api/entities/bulk/scopes",
            json={
                "entity_ids": ["not-a-uuid"],
                "scopes": ["public"],
                "op": "add",
            },
        )
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code in {400, 404}
