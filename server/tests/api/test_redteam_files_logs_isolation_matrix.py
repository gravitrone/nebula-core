"""Red team API tests for files/logs isolation across knowledge and job attachments."""

# Standard Library
import json
from datetime import UTC, datetime

# Third-Party
from httpx import ASGITransport, AsyncClient
import pytest

# Local
from nebula_api.app import app
from nebula_api.auth import require_auth


async def _make_agent(db_pool, enums, name, scopes):
    """Insert a test agent with explicit scopes."""

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


async def _make_job(db_pool, enums, title, agent_id, scopes):
    """Insert a job owned by a specific agent with privacy scopes."""

    status_id = enums.statuses.name_to_id["active"]
    scope_ids = [enums.scopes.name_to_id[s] for s in scopes]
    row = await db_pool.fetchrow(
        """
        INSERT INTO jobs (title, status_id, agent_id, privacy_scope_ids, metadata)
        VALUES ($1, $2, $3, $4, $5::jsonb)
        RETURNING *
        """,
        title,
        status_id,
        agent_id,
        scope_ids,
        json.dumps({"note": "owned"}),
    )
    return dict(row)


async def _make_knowledge(db_pool, enums, title, scopes):
    """Insert a knowledge item with specific privacy scopes."""

    status_id = enums.statuses.name_to_id["active"]
    scope_ids = [enums.scopes.name_to_id[s] for s in scopes]
    row = await db_pool.fetchrow(
        """
        INSERT INTO knowledge_items (title, source_type, content, privacy_scope_ids, status_id, tags, metadata)
        VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb)
        RETURNING *
        """,
        title,
        "note",
        "secret",
        scope_ids,
        status_id,
        ["test"],
        json.dumps({"class": "private"}),
    )
    return dict(row)


async def _make_file(db_pool, enums):
    """Insert a file record for attachment isolation tests."""

    status_id = enums.statuses.name_to_id["active"]
    row = await db_pool.fetchrow(
        """
        INSERT INTO files (filename, file_path, status_id, metadata)
        VALUES ($1, $2, $3, $4::jsonb)
        RETURNING *
        """,
        "secret.pdf",
        "/vault/secret.pdf",
        status_id,
        json.dumps({"class": "private"}),
    )
    return dict(row)


async def _make_log(db_pool, enums):
    """Insert a log record for attachment isolation tests."""

    status_id = enums.statuses.name_to_id["active"]
    log_type_id = enums.log_types.name_to_id["workout"]
    row = await db_pool.fetchrow(
        """
        INSERT INTO logs (log_type_id, timestamp, value, status_id, metadata)
        VALUES ($1, $2, $3::jsonb, $4, $5::jsonb)
        RETURNING *
        """,
        log_type_id,
        datetime.now(UTC),
        json.dumps({"note": "secret"}),
        status_id,
        json.dumps({"class": "private"}),
    )
    return dict(row)


async def _attach_relationship(db_pool, enums, source_type, source_id, target_type, target_id, rel_name):
    """Attach two nodes via relationships row insertion."""

    status_id = enums.statuses.name_to_id["active"]
    type_id = enums.relationship_types.name_to_id[rel_name]
    await db_pool.execute(
        """
        INSERT INTO relationships (source_type, source_id, target_type, target_id, type_id, status_id, properties)
        VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb)
        """,
        source_type,
        str(source_id),
        target_type,
        str(target_id),
        type_id,
        status_id,
        json.dumps({"note": "attach"}),
    )


def _auth_override(agent_id, enums):
    """Build an auth override for a public-scope agent."""

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
        """Mock auth for the viewer agent."""

        return auth_dict

    return mock_auth


@pytest.mark.asyncio
async def test_api_file_hidden_when_attached_to_private_knowledge(db_pool, enums):
    """Public agent should not see files attached to private knowledge items."""

    knowledge = await _make_knowledge(db_pool, enums, "Private Know", ["sensitive"])
    file_row = await _make_file(db_pool, enums)
    await _attach_relationship(
        db_pool, enums, "knowledge", knowledge["id"], "file", file_row["id"], "has-file"
    )
    viewer = await _make_agent(db_pool, enums, "file-knowledge-viewer", ["public"])

    app.dependency_overrides[require_auth] = _auth_override(viewer["id"], enums)
    app.state.pool = db_pool
    app.state.enums = enums

    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        get_resp = await client.get(f"/api/files/{file_row['id']}")
        list_resp = await client.get("/api/files/")
    app.dependency_overrides.pop(require_auth, None)

    assert get_resp.status_code == 403
    assert list_resp.status_code == 200
    ids = {row["id"] for row in list_resp.json()["data"]}
    assert str(file_row["id"]) not in ids


@pytest.mark.asyncio
async def test_api_file_hidden_when_attached_to_out_of_scope_job(db_pool, enums):
    """Public agent should not see or update files attached to out-of-scope jobs."""

    owner = await _make_agent(db_pool, enums, "file-job-owner", ["public"])
    viewer = await _make_agent(db_pool, enums, "file-job-viewer", ["public"])
    job = await _make_job(db_pool, enums, "Owner Job", owner["id"], ["personal"])
    file_row = await _make_file(db_pool, enums)
    await _attach_relationship(
        db_pool, enums, "job", job["id"], "file", file_row["id"], "has-file"
    )

    app.dependency_overrides[require_auth] = _auth_override(viewer["id"], enums)
    app.state.pool = db_pool
    app.state.enums = enums

    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        get_resp = await client.get(f"/api/files/{file_row['id']}")
        list_resp = await client.get("/api/files/")
        patch_resp = await client.patch(
            f"/api/files/{file_row['id']}",
            json={"metadata": {"note": "hijack"}},
        )
    app.dependency_overrides.pop(require_auth, None)

    assert get_resp.status_code == 403
    assert list_resp.status_code == 200
    ids = {row["id"] for row in list_resp.json()["data"]}
    assert str(file_row["id"]) not in ids
    assert patch_resp.status_code == 403


@pytest.mark.asyncio
async def test_api_log_hidden_when_attached_to_private_knowledge(db_pool, enums):
    """Public agent should not see logs attached to private knowledge items."""

    knowledge = await _make_knowledge(db_pool, enums, "Private Know", ["sensitive"])
    log_row = await _make_log(db_pool, enums)
    await _attach_relationship(
        db_pool, enums, "log", log_row["id"], "knowledge", knowledge["id"], "related-to"
    )
    viewer = await _make_agent(db_pool, enums, "log-knowledge-viewer", ["public"])

    app.dependency_overrides[require_auth] = _auth_override(viewer["id"], enums)
    app.state.pool = db_pool
    app.state.enums = enums

    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        get_resp = await client.get(f"/api/logs/{log_row['id']}")
        list_resp = await client.get("/api/logs/")
    app.dependency_overrides.pop(require_auth, None)

    assert get_resp.status_code == 403
    assert list_resp.status_code == 200
    ids = {row["id"] for row in list_resp.json()["data"]}
    assert str(log_row["id"]) not in ids


@pytest.mark.asyncio
async def test_api_log_hidden_when_attached_to_out_of_scope_job(db_pool, enums):
    """Public agent should not see or update logs attached to out-of-scope jobs."""

    owner = await _make_agent(db_pool, enums, "log-job-owner", ["public"])
    viewer = await _make_agent(db_pool, enums, "log-job-viewer", ["public"])
    job = await _make_job(db_pool, enums, "Owner Job", owner["id"], ["personal"])
    log_row = await _make_log(db_pool, enums)
    await _attach_relationship(
        db_pool, enums, "log", log_row["id"], "job", job["id"], "related-to"
    )

    app.dependency_overrides[require_auth] = _auth_override(viewer["id"], enums)
    app.state.pool = db_pool
    app.state.enums = enums

    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        get_resp = await client.get(f"/api/logs/{log_row['id']}")
        list_resp = await client.get("/api/logs/")
        patch_resp = await client.patch(
            f"/api/logs/{log_row['id']}",
            json={"metadata": {"note": "hijack"}},
        )
    app.dependency_overrides.pop(require_auth, None)

    assert get_resp.status_code == 403
    assert list_resp.status_code == 200
    ids = {row["id"] for row in list_resp.json()["data"]}
    assert str(log_row["id"]) not in ids
    assert patch_resp.status_code == 403
