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
            "status": "active",
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
            "status": "in-progress",
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
        INSERT INTO entities (name, type_id, status_id, privacy_scope_ids, tags, metadata)
        VALUES ($1, $2, $3, $4::uuid[], $5, $6::jsonb)
        RETURNING id
        """,
        "approval-visibility-reviewer",
        type_id,
        status_id,
        [public_scope],
        ["test"],
        "{}",
    )
    return str(row["id"])


@pytest.mark.asyncio
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


@pytest.mark.asyncio
async def test_queued_create_entity_preserves_metadata_after_approval(
    db_pool, enums
):
    """Queued entity creates should preserve nested metadata after approval."""

    public_scope = enums.scopes.name_to_id["public"]
    private_scope = enums.scopes.name_to_id["private"]
    untrusted_agent = await _make_agent(
        db_pool,
        enums,
        "approval-queue-agent-metadata",
        True,
        scopes=["public", "private"],
    )
    reviewer_id = await _make_reviewer(db_pool, enums)

    payload = {
        "name": "Queue Metadata Preserve Entity",
        "type": "person",
        "status": "active",
        "scopes": ["public", "private"],
        "tags": ["metadata", "approval"],
        "metadata": {
            "owner": "alxx",
            "profile": {"timezone": "Europe/Warsaw"},
            "context_segments": [
                {"text": "public summary", "scopes": ["public"]},
                {"text": "private summary", "scopes": ["private"]},
            ],
        },
    }

    app.state.pool = db_pool
    app.state.enums = enums
    transport = ASGITransport(app=app)
    try:
        app.dependency_overrides[require_auth] = await _agent_auth_override(
            untrusted_agent,
            [public_scope, private_scope],
        )
        async with AsyncClient(
            transport=transport, base_url="http://test", follow_redirects=True
        ) as client:
            queued = await client.post("/api/entities", json=payload)
        assert queued.status_code == 202, queued.text

        approval = await db_pool.fetchrow(
            """
            SELECT id
            FROM approval_requests
            WHERE requested_by = $1::uuid
              AND request_type = 'create_entity'
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
            "SELECT metadata FROM entities WHERE id = $1::uuid",
            created_id,
        )
        assert row is not None
        persisted_metadata = row["metadata"]
        if isinstance(persisted_metadata, str):
            persisted_metadata = json.loads(persisted_metadata)
        assert persisted_metadata["owner"] == "alxx"
        assert persisted_metadata["profile"]["timezone"] == "Europe/Warsaw"
        assert persisted_metadata["context_segments"] == [
            {"text": "public summary", "scopes": ["public"]},
            {"text": "private summary", "scopes": ["private"]},
        ]

        app.dependency_overrides[require_auth] = await _agent_auth_override(
            {**untrusted_agent, "requires_approval": False},
            [public_scope],
        )
        async with AsyncClient(
            transport=transport, base_url="http://test", follow_redirects=True
        ) as client:
            response = await client.get(f"/api/entities/{created_id}")

        assert response.status_code == 200, response.text
        metadata = response.json()["data"]["metadata"]
        assert metadata["owner"] == "alxx"
        assert metadata["profile"]["timezone"] == "Europe/Warsaw"
        assert metadata["context_segments"] == [
            {"text": "public summary", "scopes": ["public"]}
        ]
    finally:
        app.dependency_overrides.pop(require_auth, None)
