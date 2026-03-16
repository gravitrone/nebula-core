"""API test fixtures: async test client with mocked auth."""

# Standard Library
# Third-Party
import sys
from pathlib import Path

import pytest
from httpx import ASGITransport, AsyncClient

SRC_DIR = Path(__file__).resolve().parents[2] / "src"
sys.path.insert(0, str(SRC_DIR))

from nebula_api.app import app
from nebula_api.auth import generate_api_key, require_auth


@pytest.fixture
async def test_entity(db_pool, enums):
    """Create a person entity for API tests."""

    status_id = enums.statuses.name_to_id["active"]
    type_id = enums.entity_types.name_to_id["person"]
    scope_ids = [
        enums.scopes.name_to_id["public"],
        enums.scopes.name_to_id["private"],
    ]

    row = await db_pool.fetchrow(
        """
        INSERT INTO entities (name, type_id, status_id, privacy_scope_ids, tags)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING *
        """,
        "api-test-user",
        type_id,
        status_id,
        scope_ids,
        ["test"],
    )
    return dict(row)


@pytest.fixture
async def auth_override(test_entity, enums):
    """Override require_auth dependency to return test entity context."""

    scope_ids = [
        enums.scopes.name_to_id["public"],
        enums.scopes.name_to_id["private"],
    ]

    auth_dict = {
        "key_id": None,
        "caller_type": "user",
        "entity_id": test_entity["id"],
        "entity": test_entity,
        "agent_id": None,
        "agent": None,
        "scopes": scope_ids,
    }

    async def mock_auth():
        """Mock auth."""

        return auth_dict

    app.dependency_overrides[require_auth] = mock_auth
    yield auth_dict
    app.dependency_overrides.pop(require_auth, None)


@pytest.fixture
async def api(db_pool, enums, auth_override):
    """Async test client with real DB and mocked auth."""

    app.state.pool = db_pool
    app.state.enums = enums
    transport = ASGITransport(app=app)
    async with AsyncClient(
        transport=transport, base_url="http://test", follow_redirects=True
    ) as client:
        yield client


@pytest.fixture
async def api_no_auth(db_pool, enums):
    """Async test client WITHOUT auth override (for testing auth itself)."""

    app.state.pool = db_pool
    app.state.enums = enums
    app.dependency_overrides.pop(require_auth, None)
    transport = ASGITransport(app=app)
    async with AsyncClient(
        transport=transport, base_url="http://test", follow_redirects=True
    ) as client:
        yield client
    app.dependency_overrides.pop(require_auth, None)


@pytest.fixture
async def api_key_row(db_pool, test_entity):
    """Create a real API key in the DB and return (raw_key, row)."""

    raw_key, prefix, key_hash = generate_api_key()
    row = await db_pool.fetchrow(
        """
        INSERT INTO api_keys (entity_id, key_hash, key_prefix, name)
        VALUES ($1, $2, $3, $4)
        RETURNING *
        """,
        test_entity["id"],
        key_hash,
        prefix,
        "test-key",
    )
    return raw_key, dict(row)


@pytest.fixture
async def test_agent_row(db_pool, enums):
    """Create an active agent for API agent auth tests."""

    status_id = enums.statuses.name_to_id["active"]
    scope_ids = [
        enums.scopes.name_to_id["public"],
        enums.scopes.name_to_id["private"],
    ]

    row = await db_pool.fetchrow(
        """
        INSERT INTO agents (name, description, scopes, requires_approval, status_id)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING *
        """,
        "api-test-agent",
        "Test agent for API auth",
        scope_ids,
        False,
        status_id,
    )
    return dict(row)


@pytest.fixture
async def untrusted_agent_row(db_pool, enums):
    """Create an untrusted agent (requires_approval=True)."""

    status_id = enums.statuses.name_to_id["active"]
    scope_ids = [enums.scopes.name_to_id["public"]]

    row = await db_pool.fetchrow(
        """
        INSERT INTO agents (name, description, scopes, requires_approval, status_id)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING *
        """,
        "untrusted-api-agent",
        "Untrusted test agent",
        scope_ids,
        True,
        status_id,
    )
    return dict(row)


@pytest.fixture
async def agent_api_key_row(db_pool, test_agent_row):
    """Create a real agent API key in the DB and return (raw_key, row)."""

    raw_key, prefix, key_hash = generate_api_key()
    row = await db_pool.fetchrow(
        """
        INSERT INTO api_keys (agent_id, key_hash, key_prefix, name)
        VALUES ($1, $2, $3, $4)
        RETURNING *
        """,
        test_agent_row["id"],
        key_hash,
        prefix,
        "agent-test-key",
    )
    return raw_key, dict(row)


@pytest.fixture
async def agent_auth_override(test_agent_row, enums):
    """Override require_auth to simulate an agent caller."""

    scope_ids = [
        enums.scopes.name_to_id["public"],
        enums.scopes.name_to_id["private"],
    ]

    auth_dict = {
        "key_id": None,
        "caller_type": "agent",
        "entity_id": None,
        "entity": None,
        "agent_id": test_agent_row["id"],
        "agent": test_agent_row,
        "scopes": scope_ids,
    }

    async def mock_auth():
        """Mock auth."""

        return auth_dict

    app.dependency_overrides[require_auth] = mock_auth
    yield auth_dict
    app.dependency_overrides.pop(require_auth, None)


@pytest.fixture
async def api_agent_auth(db_pool, enums, agent_auth_override):
    """Async test client with agent auth mock."""

    app.state.pool = db_pool
    app.state.enums = enums
    transport = ASGITransport(app=app)
    async with AsyncClient(
        transport=transport, base_url="http://test", follow_redirects=True
    ) as client:
        yield client
