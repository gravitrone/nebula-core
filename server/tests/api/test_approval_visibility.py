"""Regression tests for post-approval visibility on agent-created records."""

# Standard Library
from dataclasses import dataclass
import json

# Third-Party
import pytest
from httpx import ASGITransport, AsyncClient

# Local
from nebula_api.app import app
from nebula_api.auth import require_auth
from nebula_mcp.helpers import approve_request

pytestmark = pytest.mark.api


@dataclass(frozen=True)
class ApprovalVisibilityCase:
    """Defines one approval-create flow and list endpoint expectation."""

    request_type: str
    create_path: str
    create_payload: dict
    list_path: str
    table_name: str
    id_where_clause: str = "id = $1::uuid"


CASES = (
    ApprovalVisibilityCase(
        request_type="create_entity",
        create_path="/api/entities",
        create_payload={
            "name": "Queue Persist Entity",
            "type": "project",
            "status": "active",
            "scopes": ["public"],
        },
        list_path="/api/entities/",
        table_name="entities",
    ),
    ApprovalVisibilityCase(
        request_type="create_context",
        create_path="/api/context",
        create_payload={
            "title": "Queue Persist Context",
            "source_type": "note",
            "scopes": ["public"],
        },
        list_path="/api/context",
        table_name="context_items",
    ),
    ApprovalVisibilityCase(
        request_type="create_job",
        create_path="/api/jobs",
        create_payload={
            "title": "Queue Persist Job",
            "priority": "medium",
        },
        list_path="/api/jobs",
        table_name="jobs",
        id_where_clause="id = $1",
    ),
    ApprovalVisibilityCase(
        request_type="create_log",
        create_path="/api/logs",
        create_payload={
            "log_type": "note",
            "value_text": "Queue Persist Log",
            "status": "active",
            "scopes": ["public"],
        },
        list_path="/api/logs",
        table_name="logs",
    ),
)


async def _agent_auth_override(agent_row: dict, scope_ids: list):
    """Returns an auth override dependency for a specific agent context."""

    auth_dict = {
        "key_id": None,
        "caller_type": "agent",
        "entity_id": None,
        "entity": None,
        "agent_id": agent_row["id"],
        "agent": agent_row,
        "scopes": scope_ids,
    }

    async def mock_auth():
        """Returns mocked auth context."""

        return auth_dict

    return mock_auth


async def _make_agent(
    db_pool,
    enums,
    name: str,
    requires_approval: bool,
    scopes: list[str] | None = None,
):
    """Creates a test agent and returns it as a plain dict."""

    status_id = enums.statuses.name_to_id["active"]
    scope_names = scopes or ["public"]
    scope_ids = [enums.scopes.name_to_id[scope_name] for scope_name in scope_names]
    row = await db_pool.fetchrow(
        """
        INSERT INTO agents (name, description, scopes, requires_approval, status_id)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING *
        """,
        name,
        "approval visibility regression agent",
        scope_ids,
        requires_approval,
        status_id,
    )
    return dict(row)


async def _make_reviewer(db_pool, enums):
    """Creates a reviewer entity for approval actions."""

    status_id = enums.statuses.name_to_id["active"]
    type_id = enums.entity_types.name_to_id["person"]
    public_scope = enums.scopes.name_to_id["public"]
    row = await db_pool.fetchrow(
        """
        INSERT INTO entities (name, type_id, status_id, privacy_scope_ids, tags)
        VALUES ($1, $2, $3, $4::uuid[], $5)
        RETURNING id
        """,
        "approval-visibility-reviewer",
        type_id,
        status_id,
        [public_scope],
        ["test"],
    )
    return str(row["id"])


@pytest.mark.parametrize("case", CASES)
async def test_untrusted_create_remains_visible_after_approval(
    db_pool,
    enums,
    case: ApprovalVisibilityCase,
):
    """Queued creates should still be visible to the agent after approval + refresh."""

    public_scope = enums.scopes.name_to_id["public"]
    untrusted_agent = await _make_agent(
        db_pool,
        enums,
        f"approval-queue-agent-{case.request_type}",
        True,
    )
    reviewer_id = await _make_reviewer(db_pool, enums)

    app.state.pool = db_pool
    app.state.enums = enums
    transport = ASGITransport(app=app)
    try:
        app.dependency_overrides[require_auth] = await _agent_auth_override(
            untrusted_agent, [public_scope]
        )
        async with AsyncClient(
            transport=transport, base_url="http://test", follow_redirects=True
        ) as client:
            queued = await client.post(case.create_path, json=case.create_payload)
        assert queued.status_code == 202, queued.text
        assert queued.json()["status"] == "approval_required"

        approval = await db_pool.fetchrow(
            """
            SELECT id
            FROM approval_requests
            WHERE requested_by = $1::uuid
              AND request_type = $2
              AND status = 'pending'
            ORDER BY created_at DESC
            LIMIT 1
            """,
            untrusted_agent["id"],
            case.request_type,
        )
        assert approval is not None

        approved = await approve_request(db_pool, enums, str(approval["id"]), reviewer_id)
        created = approved["entity"]
        created_id = str(created["id"])

        persisted = await db_pool.fetchval(
            f"SELECT COUNT(*) FROM {case.table_name} WHERE {case.id_where_clause}",
            created_id,
        )
        assert persisted == 1

        trusted_agent = dict(untrusted_agent)
        trusted_agent["requires_approval"] = False
        app.dependency_overrides[require_auth] = await _agent_auth_override(
            trusted_agent, [public_scope]
        )
        async with AsyncClient(
            transport=transport, base_url="http://test", follow_redirects=True
        ) as client:
            refreshed = await client.get(case.list_path)

        assert refreshed.status_code == 200, refreshed.text
        rows = refreshed.json()["data"]
        assert any(str(row.get("id")) == created_id for row in rows), (
            f"approved {case.request_type} record should be visible after list refresh"
        )
    finally:
        app.dependency_overrides.pop(require_auth, None)


async def test_queued_create_log_preserves_metadata_after_approval(db_pool, enums):
    """Queued log creates should preserve nested metadata after approval."""

    public_scope = enums.scopes.name_to_id["public"]
    untrusted_agent = await _make_agent(
        db_pool,
        enums,
        "approval-queue-agent-log-metadata",
        True,
        scopes=["public"],
    )
    reviewer_id = await _make_reviewer(db_pool, enums)

    payload = {
        "log_type": "note",
        "value": {"text": "queued metadata log"},
        "status": "active",
        "tags": ["metadata", "log"],
        "metadata": {
            "owner": "alxx",
            "profile": {"timezone": "Europe/Warsaw"},
            "context_segments": [
                {"text": "public log context", "scopes": ["public"]},
                {"text": "private log context", "scopes": ["private"]},
            ],
        },
    }

    app.state.pool = db_pool
    app.state.enums = enums
    transport = ASGITransport(app=app)
    try:
        app.dependency_overrides[require_auth] = await _agent_auth_override(
            untrusted_agent,
            [public_scope],
        )
        async with AsyncClient(
            transport=transport, base_url="http://test", follow_redirects=True
        ) as client:
            queued = await client.post("/api/logs/", json=payload)
        assert queued.status_code == 202, queued.text

        approval = await db_pool.fetchrow(
            """
            SELECT id
            FROM approval_requests
            WHERE requested_by = $1::uuid
              AND request_type = 'create_log'
              AND status = 'pending'
            ORDER BY created_at DESC
            LIMIT 1
            """,
            untrusted_agent["id"],
        )
        assert approval is not None
        approved = await approve_request(db_pool, enums, str(approval["id"]), reviewer_id)
        created_id = str(approved["entity"]["id"])

        row = await db_pool.fetchrow(
            "SELECT metadata FROM logs WHERE id = $1::uuid",
            created_id,
        )
        assert row is not None
        persisted_metadata = row["metadata"]
        if isinstance(persisted_metadata, str):
            persisted_metadata = json.loads(persisted_metadata)
        assert persisted_metadata == payload["metadata"]

        app.dependency_overrides[require_auth] = await _agent_auth_override(
            {**untrusted_agent, "requires_approval": False},
            [public_scope],
        )
        async with AsyncClient(
            transport=transport, base_url="http://test", follow_redirects=True
        ) as client:
            response = await client.get(f"/api/logs/{created_id}")

        assert response.status_code == 200, response.text
        assert response.json()["data"]["metadata"] == payload["metadata"]
    finally:
        app.dependency_overrides.pop(require_auth, None)


async def test_queued_create_file_preserves_metadata_after_approval(db_pool, enums):
    """Queued file creates should preserve nested metadata after approval."""

    public_scope = enums.scopes.name_to_id["public"]
    untrusted_agent = await _make_agent(
        db_pool,
        enums,
        "approval-queue-agent-file-metadata",
        True,
        scopes=["public"],
    )
    reviewer_id = await _make_reviewer(db_pool, enums)

    payload = {
        "filename": "queued-file.txt",
        "uri": "path:queued-file.txt",
        "status": "active",
        "tags": ["metadata", "file"],
        "metadata": {
            "owner": "alxx",
            "profile": {"timezone": "Europe/Warsaw"},
            "notes": "queued file metadata",
        },
    }

    app.state.pool = db_pool
    app.state.enums = enums
    transport = ASGITransport(app=app)
    try:
        app.dependency_overrides[require_auth] = await _agent_auth_override(
            untrusted_agent,
            [public_scope],
        )
        async with AsyncClient(
            transport=transport, base_url="http://test", follow_redirects=True
        ) as client:
            queued = await client.post("/api/files/", json=payload)
        assert queued.status_code == 202, queued.text

        approval = await db_pool.fetchrow(
            """
            SELECT id
            FROM approval_requests
            WHERE requested_by = $1::uuid
              AND request_type = 'create_file'
              AND status = 'pending'
            ORDER BY created_at DESC
            LIMIT 1
            """,
            untrusted_agent["id"],
        )
        assert approval is not None
        approved = await approve_request(db_pool, enums, str(approval["id"]), reviewer_id)
        created_id = str(approved["entity"]["id"])

        row = await db_pool.fetchrow(
            "SELECT metadata FROM files WHERE id = $1::uuid",
            created_id,
        )
        assert row is not None
        persisted_metadata = row["metadata"]
        if isinstance(persisted_metadata, str):
            persisted_metadata = json.loads(persisted_metadata)
        assert persisted_metadata == payload["metadata"]

        app.dependency_overrides[require_auth] = await _agent_auth_override(
            {**untrusted_agent, "requires_approval": False},
            [public_scope],
        )
        async with AsyncClient(
            transport=transport, base_url="http://test", follow_redirects=True
        ) as client:
            response = await client.get(f"/api/files/{created_id}")

        assert response.status_code == 200, response.text
        assert response.json()["data"]["metadata"] == payload["metadata"]
    finally:
        app.dependency_overrides.pop(require_auth, None)


async def test_queued_update_log_preserves_metadata_after_approval(db_pool, enums):
    """Queued log updates should preserve metadata and return object payloads."""

    public_scope = enums.scopes.name_to_id["public"]
    untrusted_agent = await _make_agent(
        db_pool,
        enums,
        "approval-queue-agent-update-log-metadata",
        True,
        scopes=["public"],
    )
    reviewer_id = await _make_reviewer(db_pool, enums)

    status_id = enums.statuses.name_to_id["active"]
    log_type_id = enums.log_types.name_to_id["note"]
    existing = await db_pool.fetchrow(
        """
        INSERT INTO logs (log_type_id, timestamp, value, status_id, tags, metadata)
        VALUES ($1, now(), $2::jsonb, $3, $4, $5::jsonb)
        RETURNING id
        """,
        log_type_id,
        json.dumps({"text": "before"}),
        status_id,
        ["test"],
        json.dumps({"owner": "before"}),
    )
    assert existing is not None
    log_id = str(existing["id"])

    payload = {
        "metadata": {
            "owner": "alxx",
            "profile": {"timezone": "Europe/Warsaw"},
            "notes": "queued log metadata update",
        }
    }

    app.state.pool = db_pool
    app.state.enums = enums
    transport = ASGITransport(app=app)
    try:
        app.dependency_overrides[require_auth] = await _agent_auth_override(
            untrusted_agent,
            [public_scope],
        )
        async with AsyncClient(
            transport=transport, base_url="http://test", follow_redirects=True
        ) as client:
            queued = await client.patch(f"/api/logs/{log_id}", json=payload)
        assert queued.status_code == 202, queued.text
        assert queued.json()["status"] == "approval_required"

        approval = await db_pool.fetchrow(
            """
            SELECT id
            FROM approval_requests
            WHERE requested_by = $1::uuid
              AND request_type = 'update_log'
              AND status = 'pending'
            ORDER BY created_at DESC
            LIMIT 1
            """,
            untrusted_agent["id"],
        )
        assert approval is not None
        await approve_request(db_pool, enums, str(approval["id"]), reviewer_id)

        row = await db_pool.fetchrow(
            "SELECT metadata FROM logs WHERE id = $1::uuid",
            log_id,
        )
        assert row is not None
        persisted_metadata = row["metadata"]
        if isinstance(persisted_metadata, str):
            persisted_metadata = json.loads(persisted_metadata)
        assert persisted_metadata == payload["metadata"]

        app.dependency_overrides[require_auth] = await _agent_auth_override(
            {**untrusted_agent, "requires_approval": False},
            [public_scope],
        )
        async with AsyncClient(
            transport=transport, base_url="http://test", follow_redirects=True
        ) as client:
            response = await client.get(f"/api/logs/{log_id}")
            listing = await client.get("/api/logs", params={"log_type": "note"})

        assert response.status_code == 200, response.text
        assert response.json()["data"]["metadata"] == payload["metadata"]
        assert isinstance(response.json()["data"]["value"], dict)

        assert listing.status_code == 200, listing.text
        listed_row = next(
            (item for item in listing.json()["data"] if str(item.get("id")) == log_id),
            None,
        )
        assert listed_row is not None
        assert listed_row["metadata"] == payload["metadata"]
        assert isinstance(listed_row["value"], dict)
    finally:
        app.dependency_overrides.pop(require_auth, None)


async def test_queued_update_file_preserves_metadata_after_approval(db_pool, enums):
    """Queued file updates should preserve metadata and return object payloads."""

    public_scope = enums.scopes.name_to_id["public"]
    untrusted_agent = await _make_agent(
        db_pool,
        enums,
        "approval-queue-agent-update-file-metadata",
        True,
        scopes=["public"],
    )
    reviewer_id = await _make_reviewer(db_pool, enums)

    status_id = enums.statuses.name_to_id["active"]
    existing = await db_pool.fetchrow(
        """
        INSERT INTO files (filename, uri, file_path, status_id, tags, metadata)
        VALUES ($1, $2, $3, $4, $5, $6::jsonb)
        RETURNING id
        """,
        "queued-update-file.txt",
        "path:queued-update-file.txt",
        "path:queued-update-file.txt",
        status_id,
        ["test"],
        json.dumps({"owner": "before"}),
    )
    assert existing is not None
    file_id = str(existing["id"])

    payload = {
        "metadata": {
            "owner": "alxx",
            "profile": {"timezone": "Europe/Warsaw"},
            "notes": "queued file metadata update",
        }
    }

    app.state.pool = db_pool
    app.state.enums = enums
    transport = ASGITransport(app=app)
    try:
        app.dependency_overrides[require_auth] = await _agent_auth_override(
            untrusted_agent,
            [public_scope],
        )
        async with AsyncClient(
            transport=transport, base_url="http://test", follow_redirects=True
        ) as client:
            queued = await client.patch(f"/api/files/{file_id}", json=payload)
        assert queued.status_code == 202, queued.text
        assert queued.json()["status"] == "approval_required"

        approval = await db_pool.fetchrow(
            """
            SELECT id
            FROM approval_requests
            WHERE requested_by = $1::uuid
              AND request_type = 'update_file'
              AND status = 'pending'
            ORDER BY created_at DESC
            LIMIT 1
            """,
            untrusted_agent["id"],
        )
        assert approval is not None
        await approve_request(db_pool, enums, str(approval["id"]), reviewer_id)

        row = await db_pool.fetchrow(
            "SELECT metadata FROM files WHERE id = $1::uuid",
            file_id,
        )
        assert row is not None
        persisted_metadata = row["metadata"]
        if isinstance(persisted_metadata, str):
            persisted_metadata = json.loads(persisted_metadata)
        assert persisted_metadata == payload["metadata"]

        app.dependency_overrides[require_auth] = await _agent_auth_override(
            {**untrusted_agent, "requires_approval": False},
            [public_scope],
        )
        async with AsyncClient(
            transport=transport, base_url="http://test", follow_redirects=True
        ) as client:
            response = await client.get(f"/api/files/{file_id}")
            listing = await client.get("/api/files")

        assert response.status_code == 200, response.text
        assert response.json()["data"]["metadata"] == payload["metadata"]

        assert listing.status_code == 200, listing.text
        listed_row = next(
            (item for item in listing.json()["data"] if str(item.get("id")) == file_id),
            None,
        )
        assert listed_row is not None
        assert listed_row["metadata"] == payload["metadata"]
    finally:
        app.dependency_overrides.pop(require_auth, None)
