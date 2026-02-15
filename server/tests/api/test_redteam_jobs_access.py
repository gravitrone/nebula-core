"""Red team API tests for job access + write isolation."""

# Standard Library
import json

# Third-Party
from httpx import ASGITransport, AsyncClient
import pytest

# Local
from nebula_api.app import app
from nebula_api.auth import require_auth


async def _make_agent(db_pool, enums, name, requires_approval):
    """Insert a test agent for job access scenarios."""

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
        requires_approval,
        status_id,
    )
    return dict(row)


async def _make_job(db_pool, enums, agent_id):
    """Insert a test job for job access scenarios."""

    status_id = enums.statuses.name_to_id["active"]
    scope_ids = [enums.scopes.name_to_id["public"]]

    row = await db_pool.fetchrow(
        """
        INSERT INTO jobs (title, status_id, agent_id, metadata, privacy_scope_ids)
        VALUES ($1, $2, $3, $4::jsonb, $5)
        RETURNING *
        """,
        "API Private Job",
        status_id,
        agent_id,
        json.dumps({"secret": "job"}),
        scope_ids,
    )
    return dict(row)


@pytest.mark.asyncio
async def test_api_get_job_allows_other_agent_in_scope(db_pool, enums):
    """Agent should be able to fetch scoped jobs via API."""

    owner = await _make_agent(db_pool, enums, "api-owner", False)
    viewer = await _make_agent(db_pool, enums, "api-viewer", False)
    job = await _make_job(db_pool, enums, owner["id"])

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
        resp = await client.get(f"/api/jobs/{job['id']}")
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 200


@pytest.mark.asyncio
async def test_api_query_jobs_includes_other_agents_jobs_in_scope(db_pool, enums):
    """Agent job list should include scoped jobs via API."""

    owner = await _make_agent(db_pool, enums, "api-owner-2", False)
    viewer = await _make_agent(db_pool, enums, "api-viewer-2", False)
    job = await _make_job(db_pool, enums, owner["id"])

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
    async with AsyncClient(
        transport=transport, base_url="http://test", follow_redirects=True
    ) as client:
        resp = await client.get("/api/jobs/")
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 200
    data = resp.json()["data"]
    ids = {row["id"] for row in data}
    assert job["id"] in ids


@pytest.mark.asyncio
async def test_api_update_job_status_denies_other_agent(db_pool, enums):
    """Agent should not update another agent's job status via API."""

    owner = await _make_agent(db_pool, enums, "api-owner-3", False)
    viewer = await _make_agent(db_pool, enums, "api-viewer-3", False)
    job = await _make_job(db_pool, enums, owner["id"])

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
        resp = await client.patch(
            f"/api/jobs/{job['id']}/status",
            json={"status": "completed"},
        )
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 403


@pytest.mark.asyncio
async def test_api_update_job_denies_other_agent(db_pool, enums):
    """Agent should not update another agent's job fields via API."""

    owner = await _make_agent(db_pool, enums, "api-owner-4", False)
    viewer = await _make_agent(db_pool, enums, "api-viewer-4", False)
    job = await _make_job(db_pool, enums, owner["id"])

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
        resp = await client.patch(
            f"/api/jobs/{job['id']}",
            json={"title": "Hijacked"},
        )
    app.dependency_overrides.pop(require_auth, None)

    if resp.status_code == 403:
        return

    assert resp.status_code == 200
    assert resp.json()["data"]["agent_id"] == str(viewer["id"])


@pytest.mark.asyncio
async def test_api_create_subtask_denies_other_agent(db_pool, enums):
    """Agent should not create subtasks on another agent's job."""

    owner = await _make_agent(db_pool, enums, "api-owner-5", False)
    viewer = await _make_agent(db_pool, enums, "api-viewer-5", False)
    job = await _make_job(db_pool, enums, owner["id"])

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
            f"/api/jobs/{job['id']}/subtasks",
            json={"title": "Injected Subtask"},
        )
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 403


@pytest.mark.asyncio
async def test_api_create_job_overrides_agent_id(db_pool, enums):
    """API should prevent agents from creating jobs for other agents."""

    owner = await _make_agent(db_pool, enums, "api-owner-6", False)
    viewer = await _make_agent(db_pool, enums, "api-viewer-6", False)

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
            "/api/jobs/",
            json={
                "title": "Injected Job",
                "agent_id": str(owner["id"]),
            },
        )
    app.dependency_overrides.pop(require_auth, None)

    if resp.status_code == 403:
        return

    assert resp.status_code in (200, 202)
    assert resp.json()["data"]["agent_id"] == str(viewer["id"])


@pytest.mark.asyncio
async def test_api_create_job_handles_uuid_agent_id(db_pool, enums):
    """API job creation should not crash on UUID agent_id."""

    viewer = await _make_agent(db_pool, enums, "api-viewer-7", False)

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
            "/api/jobs/",
            json={
                "title": "UUID Agent Job",
            },
        )
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code in (200, 202)
    assert resp.json()["data"]["agent_id"] == str(viewer["id"])
