"""Red team API tests for file access isolation."""

# Standard Library
import json

import pytest

# Third-Party
from httpx import ASGITransport, AsyncClient

# Local
from nebula_api.app import app
from nebula_api.auth import require_auth


async def _make_agent(db_pool, enums, name, scopes, requires_approval):
    """Insert an agent for file access tests."""

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
    """Insert an entity for file access tests."""

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


async def _make_file(db_pool, enums):
    """Insert a file record for file access tests."""

    status_id = enums.statuses.name_to_id["active"]

    row = await db_pool.fetchrow(
        """
        INSERT INTO files (filename, file_path, status_id, notes)
        VALUES ($1, $2, $3, $4)
        RETURNING *
        """,
        "secret.pdf",
        "/vault/secret.pdf",
        status_id,
        json.dumps({"class": "private"}),
    )
    return dict(row)


async def _attach_file(db_pool, enums, file_id, entity_id):
    """Attach a file to an entity."""

    status_id = enums.statuses.name_to_id["active"]
    rel_type_id = enums.relationship_types.name_to_id["has-file"]

    await db_pool.execute(
        """
        INSERT INTO relationships (source_type, source_id, target_type, target_id, type_id, status_id, notes)
        VALUES ('entity', $1, 'file', $2, $3, $4, $5)
        """,
        str(entity_id),
        str(file_id),
        rel_type_id,
        status_id,
        json.dumps({"note": "private file"}),
    )


@pytest.mark.asyncio
async def test_api_get_file_denies_private_entity(db_pool, enums):
    """Agent should not fetch file attached to private entity via API."""

    private_entity = await _make_entity(db_pool, enums, "Private", ["sensitive"])
    file_row = await _make_file(db_pool, enums)
    await _attach_file(db_pool, enums, file_row["id"], private_entity["id"])

    viewer = await _make_agent(db_pool, enums, "api-file-viewer", ["public"], False)

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
        resp = await client.get(f"/api/files/{file_row['id']}")
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 403


@pytest.mark.asyncio
async def test_api_list_files_hides_private_entity_files(db_pool, enums):
    """Agent should not list files attached to private entities via API."""

    private_entity = await _make_entity(db_pool, enums, "Private", ["sensitive"])
    file_row = await _make_file(db_pool, enums)
    await _attach_file(db_pool, enums, file_row["id"], private_entity["id"])

    viewer = await _make_agent(db_pool, enums, "api-file-viewer-2", ["public"], False)

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
        resp = await client.get("/api/files/")
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 200
    data = resp.json()["data"]
    ids = {row["id"] for row in data}
    assert str(file_row["id"]) not in ids
