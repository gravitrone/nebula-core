"""Red team API tests for context-of privacy filtering."""

# Third-Party
from httpx import ASGITransport, AsyncClient
import pytest

# Local
from nebula_api.app import app
from nebula_api.auth import require_auth


def _auth_override(agent_id: str, scope_ids: list[str]) -> callable:
    """Build an auth override for scoped agent requests."""

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
        """Mock auth for agent."""

        return auth_dict

    return mock_auth


async def _ensure_context_of_type(db_pool) -> str:
    """Ensure context-of relationship type exists and return its id."""

    row = await db_pool.fetchrow(
        "SELECT id FROM relationship_types WHERE name = 'context-of'"
    )
    if row:
        return str(row["id"])
    created = await db_pool.fetchrow(
        """
        INSERT INTO relationship_types (name, description, is_symmetric, is_builtin, is_active, metadata)
        VALUES ('context-of', 'Context item used as scoped metadata for an owner', false, true, true, '{}'::jsonb)
        RETURNING id
        """
    )
    return str(created["id"])


async def _make_agent(db_pool, enums, name: str, scopes: list[str]) -> dict:
    """Insert a test agent for context-of scenarios."""

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


async def _make_entity(db_pool, enums, name: str, scopes: list[str]) -> dict:
    """Insert a test entity."""

    status_id = enums.statuses.name_to_id["active"]
    type_id = enums.entity_types.name_to_id["person"]
    scope_ids = [enums.scopes.name_to_id[s] for s in scopes]
    row = await db_pool.fetchrow(
        """
        INSERT INTO entities (name, type_id, status_id, privacy_scope_ids, tags)
        VALUES ($1, $2::uuid, $3::uuid, $4::uuid[], $5)
        RETURNING *
        """,
        name,
        type_id,
        status_id,
        scope_ids,
        ["redteam"],
    )
    return dict(row)


async def _make_context(db_pool, enums, title: str, scopes: list[str]) -> dict:
    """Insert a test context item."""

    status_id = enums.statuses.name_to_id["active"]
    scope_ids = [enums.scopes.name_to_id[s] for s in scopes]
    row = await db_pool.fetchrow(
        """
        INSERT INTO context_items (title, source_type, content, privacy_scope_ids, status_id, tags)
        VALUES ($1, $2, $3, $4, $5, $6)
        RETURNING *
        """,
        title,
        "note",
        "secret",
        scope_ids,
        status_id,
        ["test"],
    )
    return dict(row)


async def _link_context(db_pool, enums, owner_id: str, context_id: str) -> None:
    """Create a context-of relationship between entity and context."""

    type_id = await _ensure_context_of_type(db_pool)
    status_id = enums.statuses.name_to_id["active"]
    await db_pool.execute(
        """
        INSERT INTO relationships (source_type, source_id, target_type, target_id, type_id, status_id, properties)
        VALUES ('entity', $1::uuid, 'context', $2::uuid, $3::uuid, $4::uuid, '{}'::jsonb)
        """,
        owner_id,
        context_id,
        type_id,
        status_id,
    )


@pytest.mark.asyncio
async def test_list_context_by_owner_filters_scopes(db_pool, enums):
    """List by owner should filter context items by agent scopes."""

    entity = await _make_entity(db_pool, enums, "Owner", ["public"])
    ctx_public = await _make_context(db_pool, enums, "Public Note", ["public"])
    ctx_private = await _make_context(db_pool, enums, "Private Note", ["private"])
    await _link_context(db_pool, enums, entity["id"], ctx_public["id"])
    await _link_context(db_pool, enums, entity["id"], ctx_private["id"])

    agent = await _make_agent(db_pool, enums, "context-viewer", ["public"])

    app.dependency_overrides[require_auth] = _auth_override(
        str(agent["id"]), [enums.scopes.name_to_id["public"]]
    )
    app.state.pool = db_pool
    app.state.enums = enums

    transport = ASGITransport(app=app)
    async with AsyncClient(
        transport=transport, base_url="http://test", follow_redirects=True
    ) as client:
        resp = await client.get(f"/api/context/by-owner/entity/{entity['id']}")
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 200
    items = resp.json()["data"]
    ids = {str(row["id"]) for row in items}
    assert str(ctx_public["id"]) in ids
    assert str(ctx_private["id"]) not in ids


@pytest.mark.asyncio
async def test_list_context_by_owner_includes_private_scopes(db_pool, enums):
    """List by owner should include private context when agent has scope."""

    entity = await _make_entity(db_pool, enums, "Owner Two", ["public", "private"])
    ctx_public = await _make_context(db_pool, enums, "Public Note", ["public"])
    ctx_private = await _make_context(db_pool, enums, "Private Note", ["private"])
    await _link_context(db_pool, enums, entity["id"], ctx_public["id"])
    await _link_context(db_pool, enums, entity["id"], ctx_private["id"])

    agent = await _make_agent(db_pool, enums, "context-admin", ["public", "private"])

    app.dependency_overrides[require_auth] = _auth_override(
        str(agent["id"]),
        [
            enums.scopes.name_to_id["public"],
            enums.scopes.name_to_id["private"],
        ],
    )
    app.state.pool = db_pool
    app.state.enums = enums

    transport = ASGITransport(app=app)
    async with AsyncClient(
        transport=transport, base_url="http://test", follow_redirects=True
    ) as client:
        resp = await client.get(f"/api/context/by-owner/entity/{entity['id']}")
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 200
    ids = {str(row["id"]) for row in resp.json()["data"]}
    assert str(ctx_public["id"]) in ids
    assert str(ctx_private["id"]) in ids
