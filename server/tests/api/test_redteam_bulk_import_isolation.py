"""Red team API tests for bulk import isolation."""

# Standard Library
import json

# Third-Party
import pytest
from httpx import ASGITransport, AsyncClient

# Local
from nebula_api.app import app
from nebula_api.auth import require_auth


async def _make_agent(db_pool, enums, name, scopes, requires_approval):
    """Insert an agent for bulk import tests."""

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
    """Insert an entity for bulk import tests."""

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


async def _make_job(db_pool, enums, title, agent_id, scopes):
    """Insert a job for bulk import relationship isolation tests."""

    status_id = enums.statuses.name_to_id["active"]
    scope_ids = [enums.scopes.name_to_id[s] for s in scopes]
    row = await db_pool.fetchrow(
        """
        INSERT INTO jobs (title, status_id, agent_id, privacy_scope_ids)
        VALUES ($1, $2, $3, $4)
        RETURNING *
        """,
        title,
        status_id,
        agent_id,
        scope_ids,
    )
    return dict(row)


async def _make_context(db_pool, enums, title, scopes):
    """Insert a context item for bulk import relationship isolation tests."""

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
        "context body",
        scope_ids,
        status_id,
        ["test"],
    )
    return dict(row)


def _auth_override(agent_id, scopes, enums):
    """Build auth override for agent scoped requests."""

    scope_ids = [enums.scopes.name_to_id[s] for s in scopes]
    auth_dict = {
        "key_id": None,
        "caller_type": "agent",
        "entity_id": None,
        "entity": None,
        "agent_id": agent_id,
        "agent": {"id": agent_id, "requires_approval": False},
        "scopes": scope_ids,
    }

    async def mock_auth():
        """Mock auth for public agent."""

        return auth_dict

    return mock_auth


def _user_auth_override(entity_row, scopes, enums):
    """Build auth override for user scoped requests."""

    scope_ids = [enums.scopes.name_to_id[s] for s in scopes]
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
        """Mock auth for user caller."""

        return auth_dict

    return mock_auth


@pytest.mark.asyncio
async def test_bulk_import_entities_scope_escalation(db_pool, enums):
    """Agents should not bulk import entities with private scopes."""

    agent = await _make_agent(db_pool, enums, "bulk-import-viewer", ["public"], False)
    app.dependency_overrides[require_auth] = _auth_override(
        agent["id"], ["public"], enums
    )
    app.state.pool = db_pool
    app.state.enums = enums

    payload = {
        "format": "json",
        "items": [
            {
                "name": "Escalated Entity",
                "type": "person",
                "status": "active",
                "scopes": ["private"],
                "tags": ["redteam"],
            }
        ],
    }

    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.post("/api/import/entities", json=payload)
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 200
    data = resp.json()["data"]
    items = data.get("items", [])
    if not items:
        assert data.get("failed", 0) >= 1 or data.get("created", 0) == 0
        return
    scopes = items[0]["scopes"]
    assert "private" not in scopes


@pytest.mark.asyncio
async def test_bulk_import_jobs_agent_spoofing(db_pool, enums):
    """Agents should not bulk import jobs for other agents."""

    owner = await _make_agent(db_pool, enums, "bulk-owner", ["public"], False)
    viewer = await _make_agent(db_pool, enums, "bulk-viewer", ["public"], False)

    app.dependency_overrides[require_auth] = _auth_override(
        viewer["id"], ["public"], enums
    )
    app.state.pool = db_pool
    app.state.enums = enums

    payload = {
        "format": "json",
        "items": [
            {
                "title": "Spoofed Job",
                "agent_id": str(owner["id"]),
                "priority": "high",
            }
        ],
    }

    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.post("/api/import/jobs", json=payload)
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 200
    data = resp.json()["data"]
    items = data.get("items", [])
    if not items:
        assert data.get("failed", 0) >= 1 or data.get("created", 0) == 0
        return
    job = items[0]
    assert job["agent_id"] == str(viewer["id"])


@pytest.mark.asyncio
async def test_bulk_import_relationships_private_target(db_pool, enums):
    """Agents should not bulk import relationships to private entities."""

    private_entity = await _make_entity(db_pool, enums, "Private", ["sensitive"])
    public_entity = await _make_entity(db_pool, enums, "Public", ["public"])
    viewer = await _make_agent(db_pool, enums, "bulk-linker", ["public"], False)

    app.dependency_overrides[require_auth] = _auth_override(
        viewer["id"], ["public"], enums
    )
    app.state.pool = db_pool
    app.state.enums = enums

    payload = {
        "format": "json",
        "items": [
            {
                "source_type": "entity",
                "source_id": str(public_entity["id"]),
                "target_type": "entity",
                "target_id": str(private_entity["id"]),
                "relationship_type": "related-to",
            }
        ],
    }

    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.post("/api/import/relationships", json=payload)
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 200
    data = resp.json()["data"]
    items = data.get("items", [])
    if not items:
        assert data.get("failed", 0) >= 1 or data.get("created", 0) == 0
        return
    rel = items[0]
    assert rel["target_id"] != str(private_entity["id"])


@pytest.mark.asyncio
async def test_bulk_import_context_scope_escalation(db_pool, enums):
    """Agents should not bulk import context with private scopes."""

    agent = await _make_agent(db_pool, enums, "bulk-context-viewer", ["public"], False)
    app.dependency_overrides[require_auth] = _auth_override(
        agent["id"], ["public"], enums
    )
    app.state.pool = db_pool
    app.state.enums = enums

    payload = {
        "format": "json",
        "items": [
            {
                "title": "Escalated Context",
                "source_type": "note",
                "content": "secret",
                "scopes": ["private"],
                "tags": ["redteam"],
            }
        ],
    }

    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.post("/api/import/context", json=payload)
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 200
    data = resp.json()["data"]
    items = data.get("items", [])
    if not items:
        assert data.get("failed", 0) >= 1 or data.get("created", 0) == 0
        return
    scopes = items[0]["scopes"]
    assert "private" not in scopes


@pytest.mark.asyncio
@pytest.mark.parametrize(
    ("source_type", "target_type", "source_private", "target_private"),
    [
        ("context", "entity", True, False),
        ("entity", "context", False, True),
    ],
)
async def test_bulk_import_relationships_private_context_denied_for_agent(
    db_pool,
    enums,
    source_type,
    target_type,
    source_private,
    target_private,
):
    """Public agents should not import relationships that mutate private context."""

    private_context = await _make_context(
        db_pool, enums, "Private Agent Context", ["sensitive"]
    )
    public_entity = await _make_entity(
        db_pool, enums, "Public Agent Entity", ["public"]
    )
    viewer = await _make_agent(db_pool, enums, "bulk-context-linker", ["public"], False)

    app.dependency_overrides[require_auth] = _auth_override(
        viewer["id"], ["public"], enums
    )
    app.state.pool = db_pool
    app.state.enums = enums

    payload = {
        "format": "json",
        "items": [
            {
                "source_type": source_type,
                "source_id": (
                    str(private_context["id"])
                    if source_private
                    else str(public_entity["id"])
                ),
                "target_type": target_type,
                "target_id": (
                    str(private_context["id"])
                    if target_private
                    else str(public_entity["id"])
                ),
                "relationship_type": "related-to",
            }
        ],
    }

    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.post("/api/import/relationships", json=payload)
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 200
    data = resp.json()["data"]
    assert data.get("created", 0) == 0
    assert data.get("failed", 0) >= 1


@pytest.mark.asyncio
async def test_bulk_import_relationships_private_target_denied_for_user(db_pool, enums):
    """Public-scoped users should not import relationships to private entities."""

    private_entity = await _make_entity(
        db_pool, enums, "Private User Target", ["sensitive"]
    )
    public_entity = await _make_entity(db_pool, enums, "Public User Source", ["public"])
    user_entity = await _make_entity(db_pool, enums, "Import User", ["public"])

    app.dependency_overrides[require_auth] = _user_auth_override(
        user_entity, ["public"], enums
    )
    app.state.pool = db_pool
    app.state.enums = enums

    payload = {
        "format": "json",
        "items": [
            {
                "source_type": "entity",
                "source_id": str(public_entity["id"]),
                "target_type": "entity",
                "target_id": str(private_entity["id"]),
                "relationship_type": "related-to",
            }
        ],
    }

    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.post("/api/import/relationships", json=payload)
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 200
    data = resp.json()["data"]
    assert data.get("created", 0) == 0
    assert data.get("failed", 0) >= 1


@pytest.mark.asyncio
async def test_bulk_import_relationships_private_job_denied_for_user(db_pool, enums):
    """Public-scoped users should not import relationships from private jobs."""

    owner = await _make_agent(
        db_pool, enums, "bulk-import-job-owner", ["public"], False
    )
    private_job = await _make_job(
        db_pool, enums, "Private User Job", owner["id"], ["private"]
    )
    public_entity = await _make_entity(
        db_pool, enums, "Public User Target 2", ["public"]
    )
    user_entity = await _make_entity(db_pool, enums, "Import User 2", ["public"])

    app.dependency_overrides[require_auth] = _user_auth_override(
        user_entity, ["public"], enums
    )
    app.state.pool = db_pool
    app.state.enums = enums

    payload = {
        "format": "json",
        "items": [
            {
                "source_type": "job",
                "source_id": private_job["id"],
                "target_type": "entity",
                "target_id": str(public_entity["id"]),
                "relationship_type": "related-to",
            }
        ],
    }

    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.post("/api/import/relationships", json=payload)
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 200
    data = resp.json()["data"]
    assert data.get("created", 0) == 0
    assert data.get("failed", 0) >= 1


@pytest.mark.asyncio
async def test_bulk_import_relationships_private_source_context_denied_for_user(
    db_pool, enums
):
    """Public users should not import relationships from private context nodes."""

    private_context = await _make_context(
        db_pool, enums, "Private User Source Context", ["sensitive"]
    )
    public_entity = await _make_entity(
        db_pool, enums, "Public User Target 3", ["public"]
    )
    user_entity = await _make_entity(db_pool, enums, "Import User 3", ["public"])

    app.dependency_overrides[require_auth] = _user_auth_override(
        user_entity, ["public"], enums
    )
    app.state.pool = db_pool
    app.state.enums = enums

    payload = {
        "format": "json",
        "items": [
            {
                "source_type": "context",
                "source_id": str(private_context["id"]),
                "target_type": "entity",
                "target_id": str(public_entity["id"]),
                "relationship_type": "related-to",
            }
        ],
    }

    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.post("/api/import/relationships", json=payload)
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 200
    data = resp.json()["data"]
    assert data.get("created", 0) == 0
    assert data.get("failed", 0) >= 1


@pytest.mark.asyncio
async def test_bulk_import_relationships_private_target_context_denied_for_user(
    db_pool, enums
):
    """Public users should not import relationships to private context nodes."""

    public_entity = await _make_entity(
        db_pool, enums, "Public User Source 4", ["public"]
    )
    private_context = await _make_context(
        db_pool, enums, "Private User Target Context", ["private"]
    )
    user_entity = await _make_entity(db_pool, enums, "Import User 4", ["public"])

    app.dependency_overrides[require_auth] = _user_auth_override(
        user_entity, ["public"], enums
    )
    app.state.pool = db_pool
    app.state.enums = enums

    payload = {
        "format": "json",
        "items": [
            {
                "source_type": "entity",
                "source_id": str(public_entity["id"]),
                "target_type": "context",
                "target_id": str(private_context["id"]),
                "relationship_type": "related-to",
            }
        ],
    }

    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.post("/api/import/relationships", json=payload)
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 200
    data = resp.json()["data"]
    assert data.get("created", 0) == 0
    assert data.get("failed", 0) >= 1
