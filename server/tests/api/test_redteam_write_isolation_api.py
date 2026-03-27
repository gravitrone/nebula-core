"""Red team API tests for write isolation on private records."""

# Standard Library
import json

# Third-Party
import pytest
from httpx import ASGITransport, AsyncClient

# Local
from nebula_api.app import app
from nebula_api.auth import require_auth


async def _make_agent(db_pool, enums, name, scopes, requires_approval):
    """Insert a test agent for write isolation scenarios."""

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
    """Insert a test entity for write isolation scenarios."""

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


async def _make_context(db_pool, enums, title, scopes):
    """Insert a test context item for write isolation scenarios."""

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


async def _make_log(db_pool, enums):
    """Insert a test log for write isolation scenarios."""

    status_id = enums.statuses.name_to_id["active"]
    log_type_id = enums.log_types.name_to_id["note"]

    row = await db_pool.fetchrow(
        """
        INSERT INTO logs (log_type_id, timestamp, status_id, content, notes)
        VALUES ($1, NOW(), $2, $3, $4)
        RETURNING *
        """,
        log_type_id,
        status_id,
        json.dumps({"note": "secret"}),
        json.dumps({"meta": "secret"}),
    )
    return dict(row)


async def _make_file(db_pool, enums):
    """Insert a test file for write isolation scenarios."""

    status_id = enums.statuses.name_to_id["active"]

    row = await db_pool.fetchrow(
        """
        INSERT INTO files (filename, file_path, status_id, notes)
        VALUES ($1, $2, $3, $4)
        RETURNING *
        """,
        "secret.txt",
        "/vault/secret.txt",
        status_id,
        json.dumps({"meta": "secret"}),
    )
    return dict(row)


async def _attach_relationship(
    db_pool, enums, source_type, source_id, target_type, target_id, rel_name
):
    """Attach a relationship between two nodes for isolation tests."""

    status_id = enums.statuses.name_to_id["active"]
    rel_type_id = enums.relationship_types.name_to_id[rel_name]

    await db_pool.execute(
        """
        INSERT INTO relationships (source_type, source_id, target_type, target_id, type_id, status_id, notes)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
        """,
        source_type,
        str(source_id),
        target_type,
        str(target_id),
        rel_type_id,
        status_id,
        json.dumps({"note": "link"}),
    )


def _auth_override(agent_id, enums):
    """Build an auth override for public agent requests."""

    auth_dict = {
        "key_id": None,
        "caller_type": "agent",
        "entity_id": None,
        "entity": None,
        "agent_id": agent_id,
        "agent": {"id": agent_id},
        "scopes": [enums.scopes.name_to_id["public"]],
    }

    async def mock_auth():
        """Mock auth for public agent."""

        return auth_dict

    return mock_auth


def _user_auth_override(entity_row, enums, scopes=None):
    """Build an auth override for public-scoped user requests."""

    scope_ids = [
        enums.scopes.name_to_id[s] for s in (scopes or ["public"]) if s in enums.scopes.name_to_id
    ]
    auth_dict = {
        "key_id": None,
        "caller_type": "user",
        "entity_id": entity_row["id"],
        "entity": entity_row,
        "agent_id": None,
        "agent": None,
        "scopes": scope_ids,
    }

    async def mock_auth():
        """Mock auth for public-scoped user."""

        return auth_dict

    return mock_auth


@pytest.mark.asyncio
async def test_api_update_context_denies_private_scope(db_pool, enums):
    """Public agents should not update private context items."""

    private_context = await _make_context(db_pool, enums, "Private", ["sensitive"])
    viewer = await _make_agent(db_pool, enums, "context-viewer", ["public"], False)

    app.dependency_overrides[require_auth] = _auth_override(viewer["id"], enums)
    app.state.pool = db_pool
    app.state.enums = enums
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.patch(
            f"/api/context/{private_context['id']}",
            json={"title": "Hijacked"},
        )
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 403


@pytest.mark.asyncio
async def test_api_update_log_denies_private_attachment(db_pool, enums):
    """Public agents should not update logs attached to private entities."""

    private_entity = await _make_entity(db_pool, enums, "Private", ["sensitive"])
    log_row = await _make_log(db_pool, enums)
    await _attach_relationship(
        db_pool,
        enums,
        "entity",
        private_entity["id"],
        "log",
        log_row["id"],
        "related-to",
    )
    viewer = await _make_agent(db_pool, enums, "log-viewer", ["public"], False)

    app.dependency_overrides[require_auth] = _auth_override(viewer["id"], enums)
    app.state.pool = db_pool
    app.state.enums = enums
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.patch(
            f"/api/logs/{log_row['id']}",
            json={"notes": "note: hijack"},
        )
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 403


@pytest.mark.asyncio
async def test_api_update_file_denies_private_attachment(db_pool, enums):
    """Public agents should not update files attached to private entities."""

    private_entity = await _make_entity(db_pool, enums, "Private", ["sensitive"])
    file_row = await _make_file(db_pool, enums)
    await _attach_relationship(
        db_pool,
        enums,
        "entity",
        private_entity["id"],
        "file",
        file_row["id"],
        "has-file",
    )
    viewer = await _make_agent(db_pool, enums, "file-viewer", ["public"], False)

    app.dependency_overrides[require_auth] = _auth_override(viewer["id"], enums)
    app.state.pool = db_pool
    app.state.enums = enums
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.patch(
            f"/api/files/{file_row['id']}",
            json={"notes": "note: hijack"},
        )
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 403


@pytest.mark.asyncio
async def test_api_update_context_denies_private_scope_for_user(db_pool, enums):
    """Public-scoped user should not update private context items."""

    private_context = await _make_context(db_pool, enums, "Private User Context", ["sensitive"])
    user_entity = await _make_entity(db_pool, enums, "User Entity", ["public"])

    app.dependency_overrides[require_auth] = _user_auth_override(user_entity, enums)
    app.state.pool = db_pool
    app.state.enums = enums
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.patch(
            f"/api/context/{private_context['id']}",
            json={"title": "user-hijack"},
        )
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 403


@pytest.mark.asyncio
async def test_api_link_context_denies_private_scope_for_user(db_pool, enums):
    """Public-scoped user should not link private context to entities."""

    private_context = await _make_context(db_pool, enums, "Private Link Context", ["sensitive"])
    public_target = await _make_entity(db_pool, enums, "Public Target", ["public"])
    user_entity = await _make_entity(db_pool, enums, "User Link Entity", ["public"])

    app.dependency_overrides[require_auth] = _user_auth_override(user_entity, enums)
    app.state.pool = db_pool
    app.state.enums = enums
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.post(
            f"/api/context/{private_context['id']}/link",
            json={
                "owner_type": "entity",
                "owner_id": str(public_target["id"]),
            },
        )
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 403


@pytest.mark.asyncio
async def test_api_update_entity_denies_private_scope_for_user(db_pool, enums):
    """Public-scoped user should not update private entities."""

    private_entity = await _make_entity(db_pool, enums, "Private Target Entity", ["sensitive"])
    user_entity = await _make_entity(db_pool, enums, "User Entity Editor", ["public"])

    app.dependency_overrides[require_auth] = _user_auth_override(user_entity, enums)
    app.state.pool = db_pool
    app.state.enums = enums
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.patch(
            f"/api/entities/{private_entity['id']}",
            json={"status_reason": "user-write"},
        )
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 403


@pytest.mark.asyncio
async def test_api_bulk_update_entity_tags_denies_private_scope_for_user(db_pool, enums):
    """Public-scoped user should not bulk-update tags on private entities."""

    private_entity = await _make_entity(db_pool, enums, "Private Bulk Entity", ["sensitive"])
    user_entity = await _make_entity(db_pool, enums, "User Bulk Entity", ["public"])

    app.dependency_overrides[require_auth] = _user_auth_override(user_entity, enums)
    app.state.pool = db_pool
    app.state.enums = enums
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.post(
            "/api/entities/bulk/tags",
            json={
                "entity_ids": [str(private_entity["id"])],
                "tags": ["owned-by-user"],
                "op": "add",
            },
        )
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 403


@pytest.mark.asyncio
async def test_api_bulk_update_entity_scopes_denies_private_scope_for_user(db_pool, enums):
    """Public-scoped user should not bulk-update scopes on private entities."""

    private_entity = await _make_entity(db_pool, enums, "Private Scope Entity", ["sensitive"])
    user_entity = await _make_entity(db_pool, enums, "User Scope Entity", ["public"])

    app.dependency_overrides[require_auth] = _user_auth_override(user_entity, enums)
    app.state.pool = db_pool
    app.state.enums = enums
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.post(
            "/api/entities/bulk/scopes",
            json={
                "entity_ids": [str(private_entity["id"])],
                "scopes": ["public"],
                "op": "set",
            },
        )
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 403
