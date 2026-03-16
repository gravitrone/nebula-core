"""Agent authentication and registration API tests."""

# Standard Library
import json

# Third-Party
import pytest

pytestmark = pytest.mark.asyncio


# --- Agent Registration ---


async def test_register_agent_success(api_no_auth, db_pool, enums):
    """Registering a new agent returns 201 with pending approval."""

    r = await api_no_auth.post(
        "/api/agents/register",
        json={
            "name": "new-test-agent",
            "description": "A brand new agent",
            "requested_scopes": ["public"],
        },
    )
    assert r.status_code == 201
    data = r.json()["data"]
    assert data["status"] == "pending_approval"
    assert "agent_id" in data
    assert "approval_request_id" in data


async def test_register_agent_returns_registration_and_enrollment_token(api_no_auth):
    """Register response should include registration id and one-time enrollment token."""

    r = await api_no_auth.post(
        "/api/agents/register",
        json={
            "name": "register-token-test-agent",
            "requested_scopes": ["public"],
        },
    )
    assert r.status_code == 201
    data = r.json()["data"]
    assert data["status"] == "pending_approval"
    assert data["registration_id"]
    assert data["enrollment_token"].startswith("nbe_")


async def test_register_agent_defaults_to_trusted_mode_false(api_no_auth, db_pool):
    """Register defaults should keep requested requires_approval set to False."""

    response = await api_no_auth.post(
        "/api/agents/register",
        json={
            "name": "register-default-trust-false-agent",
            "requested_scopes": ["public"],
        },
    )
    assert response.status_code == 201, response.text
    payload = response.json()["data"]

    created_agent = await db_pool.fetchrow(
        "SELECT requires_approval FROM agents WHERE id = $1::uuid",
        payload["agent_id"],
    )
    assert created_agent is not None
    assert created_agent["requires_approval"] is False

    approval = await db_pool.fetchrow(
        "SELECT change_details FROM approval_requests WHERE id = $1::uuid",
        payload["approval_request_id"],
    )
    assert approval is not None
    change_details = approval["change_details"]
    if isinstance(change_details, str):
        change_details = json.loads(change_details)
    assert change_details["requested_requires_approval"] is False

    enrollment = await db_pool.fetchrow(
        """
        SELECT requested_requires_approval
        FROM agent_enrollment_sessions
        WHERE id = $1::uuid
        """,
        payload["registration_id"],
    )
    assert enrollment is not None
    assert enrollment["requested_requires_approval"] is False


async def test_register_agent_invalid_scope_returns_4xx(api_no_auth):
    """Register should reject unknown scope names."""

    r = await api_no_auth.post(
        "/api/agents/register",
        json={
            "name": "register-invalid-scope-agent",
            "requested_scopes": ["not-a-real-scope"],
        },
    )
    assert 400 <= r.status_code < 500


async def test_register_duplicate_agent_returns_409(api_no_auth, test_agent_row):
    """Registering an agent with an existing name returns 409."""

    r = await api_no_auth.post(
        "/api/agents/register",
        json={
            "name": test_agent_row["name"],
        },
    )
    assert r.status_code == 409


async def test_register_agent_requires_unique_name_under_race(api_no_auth):
    """Sequential duplicate register calls should enforce unique name constraint."""

    payload = {
        "name": "register-race-agent",
        "requested_scopes": ["public"],
    }
    first = await api_no_auth.post("/api/agents/register", json=payload)
    second = await api_no_auth.post("/api/agents/register", json=payload)

    assert first.status_code == 201
    assert second.status_code == 409


async def test_agent_key_authenticates(api_agent_auth, test_agent_row):
    """Agent-authed request should work for read endpoints."""

    r = await api_agent_auth.get("/api/entities/")
    assert r.status_code == 200
    data = r.json()["data"]
    assert isinstance(data, list)


async def test_agent_authed_write_trusted(api_agent_auth, test_agent_row):
    """Trusted agent write request executes directly."""

    r = await api_agent_auth.post(
        "/api/entities/",
        json={
            "name": "Agent Created Entity",
            "type": "project",
            "status": "active",
            "scopes": ["public"],
        },
    )
    assert r.status_code == 200
    assert "id" in r.json()["data"]


async def test_untrusted_agent_write_returns_approval(db_pool, enums, untrusted_agent_row):
    """Untrusted agent write returns 202 approval_required."""

    from httpx import ASGITransport, AsyncClient

    from nebula_api.app import app
    from nebula_api.auth import generate_api_key, require_auth

    # create API key for untrusted agent
    raw_key, prefix, key_hash = generate_api_key()
    await db_pool.execute(
        """
        INSERT INTO api_keys (agent_id, key_hash, key_prefix, name)
        VALUES ($1, $2, $3, $4)
        """,
        untrusted_agent_row["id"],
        key_hash,
        prefix,
        "untrusted-key",
    )

    # clear any auth overrides
    app.dependency_overrides.pop(require_auth, None)
    app.state.pool = db_pool
    app.state.enums = enums

    transport = ASGITransport(app=app)
    async with AsyncClient(
        transport=transport,
        base_url="http://test",
        headers={"Authorization": f"Bearer {raw_key}"},
    ) as client:
        r = await client.post(
            "/api/entities/",
            json={
                "name": "Untrusted Entity",
                "type": "project",
                "status": "active",
                "scopes": ["public"],
            },
        )

    assert r.status_code == 202
    assert r.json()["status"] == "approval_required"


async def test_untrusted_agent_respects_runtime_trust_toggle(db_pool, enums, untrusted_agent_row):
    """Agent writes should switch to direct mode immediately after trust toggle."""

    from httpx import ASGITransport, AsyncClient

    from nebula_api.app import app
    from nebula_api.auth import generate_api_key, require_auth

    raw_key, prefix, key_hash = generate_api_key()
    await db_pool.execute(
        """
        INSERT INTO api_keys (agent_id, key_hash, key_prefix, name)
        VALUES ($1, $2, $3, $4)
        """,
        untrusted_agent_row["id"],
        key_hash,
        prefix,
        "runtime-toggle-key",
    )

    app.dependency_overrides.pop(require_auth, None)
    app.state.pool = db_pool
    app.state.enums = enums

    transport = ASGITransport(app=app)
    headers = {"Authorization": f"Bearer {raw_key}"}
    async with AsyncClient(
        transport=transport,
        base_url="http://test",
        headers=headers,
    ) as client:
        queued = await client.post(
            "/api/entities/",
            json={
                "name": "Toggle Before",
                "type": "project",
                "status": "active",
                "scopes": ["public"],
            },
        )
        assert queued.status_code == 202, queued.text

        await db_pool.execute(
            "UPDATE agents SET requires_approval = FALSE WHERE id = $1::uuid",
            untrusted_agent_row["id"],
        )

        direct = await client.post(
            "/api/entities/",
            json={
                "name": "Toggle After",
                "type": "project",
                "status": "active",
                "scopes": ["public"],
            },
        )

    assert direct.status_code == 200, direct.text
    assert direct.json()["data"]["id"]


async def test_user_request_unchanged(api, auth_override):
    """User-authed request still works normally (regression)."""

    r = await api.get("/api/entities/")
    assert r.status_code == 200


async def test_revoked_agent_key_returns_401(db_pool, enums, test_agent_row):
    """A revoked agent key should return 401."""

    from httpx import ASGITransport, AsyncClient

    from nebula_api.app import app
    from nebula_api.auth import generate_api_key, require_auth

    raw_key, prefix, key_hash = generate_api_key()
    await db_pool.execute(
        """
        INSERT INTO api_keys (agent_id, key_hash, key_prefix, name, revoked_at)
        VALUES ($1, $2, $3, $4, NOW())
        """,
        test_agent_row["id"],
        key_hash,
        prefix,
        "revoked-key",
    )

    app.dependency_overrides.pop(require_auth, None)
    app.state.pool = db_pool
    app.state.enums = enums

    transport = ASGITransport(app=app)
    async with AsyncClient(
        transport=transport,
        base_url="http://test",
        headers={"Authorization": f"Bearer {raw_key}"},
    ) as client:
        r = await client.get("/api/entities/")

    assert r.status_code == 401
