"""Red team API tests for invalid enum handling."""

# Standard Library

# Third-Party
import pytest
from httpx import ASGITransport, AsyncClient

# Local
from nebula_api.app import app
from nebula_api.auth import require_auth


async def _make_agent(db_pool, enums, name):
    """Insert a test agent for invalid enum scenarios."""

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


async def _make_entity(db_pool, enums, name):
    """Insert a test entity for invalid enum scenarios."""

    status_id = enums.statuses.name_to_id["active"]
    type_id = enums.entity_types.name_to_id["person"]
    scope_ids = [enums.scopes.name_to_id["public"]]

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


async def _make_job(db_pool, enums, agent_id):
    """Insert a test job for invalid enum scenarios."""

    status_id = enums.statuses.name_to_id["active"]

    row = await db_pool.fetchrow(
        """
        INSERT INTO jobs (title, status_id, agent_id)
        VALUES ($1, $2, $3)
        RETURNING *
        """,
        "Enum Job",
        status_id,
        agent_id,
    )
    return dict(row)


@pytest.mark.asyncio
async def test_create_entity_invalid_type_returns_400(db_pool, enums):
    """Create entity should reject invalid type with a validation error."""

    agent = await _make_agent(db_pool, enums, "enum-agent")

    auth_dict = {
        "key_id": None,
        "caller_type": "agent",
        "entity_id": None,
        "entity": None,
        "agent_id": agent["id"],
        "agent": agent,
        "scopes": [enums.scopes.name_to_id["public"]],
    }

    async def mock_auth():
        """Mock auth for enum agent."""

        return auth_dict

    app.dependency_overrides[require_auth] = mock_auth
    app.state.pool = db_pool
    app.state.enums = enums
    transport = ASGITransport(app=app)
    async with AsyncClient(
        transport=transport, base_url="http://test", follow_redirects=True
    ) as client:
        resp = await client.post(
            "/api/entities/",
            json={
                "name": "Bad Type",
                "type": "does-not-exist",
                "scopes": ["public"],
            },
        )
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 400


@pytest.mark.asyncio
async def test_create_relationship_invalid_type_returns_400(db_pool, enums):
    """Create relationship should reject invalid relationship type."""

    agent = await _make_agent(db_pool, enums, "enum-rel-agent")
    entity = await _make_entity(db_pool, enums, "Enum Entity")

    auth_dict = {
        "key_id": None,
        "caller_type": "agent",
        "entity_id": None,
        "entity": None,
        "agent_id": agent["id"],
        "agent": agent,
        "scopes": [enums.scopes.name_to_id["public"]],
    }

    async def mock_auth():
        """Mock auth for enum agent."""

        return auth_dict

    app.dependency_overrides[require_auth] = mock_auth
    app.state.pool = db_pool
    app.state.enums = enums
    transport = ASGITransport(app=app)
    async with AsyncClient(
        transport=transport, base_url="http://test", follow_redirects=True
    ) as client:
        resp = await client.post(
            "/api/relationships/",
            json={
                "source_type": "entity",
                "source_id": str(entity["id"]),
                "target_type": "entity",
                "target_id": str(entity["id"]),
                "relationship_type": "made-up",
            },
        )
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 400


@pytest.mark.asyncio
async def test_update_job_status_invalid_returns_400(db_pool, enums):
    """Update job status should reject unknown statuses with validation error."""

    agent = await _make_agent(db_pool, enums, "enum-job-agent")
    job = await _make_job(db_pool, enums, agent["id"])

    auth_dict = {
        "key_id": None,
        "caller_type": "agent",
        "entity_id": None,
        "entity": None,
        "agent_id": agent["id"],
        "agent": agent,
        "scopes": [enums.scopes.name_to_id["public"]],
    }

    async def mock_auth():
        """Mock auth for enum agent."""

        return auth_dict

    app.dependency_overrides[require_auth] = mock_auth
    app.state.pool = db_pool
    app.state.enums = enums
    transport = ASGITransport(app=app)
    async with AsyncClient(
        transport=transport, base_url="http://test", follow_redirects=True
    ) as client:
        resp = await client.patch(
            f"/api/jobs/{job['id']}/status",
            json={
                "status": "not-a-status",
            },
        )
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 400
