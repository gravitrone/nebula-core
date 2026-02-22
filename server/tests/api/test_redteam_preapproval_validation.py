"""Red team tests ensuring invalid enum/taxonomy inputs are rejected pre-approval."""

# Standard Library
import json

# Third-Party
from httpx import ASGITransport, AsyncClient
import pytest

# Local
from nebula_api.app import app
from nebula_api.auth import require_auth


def _untrusted_auth_override(agent_row: dict, enums: object, scopes: list[str]):
    """Override require_auth to simulate an untrusted agent caller."""

    scope_ids = [enums.scopes.name_to_id[s] for s in scopes]
    auth_dict = {
        "key_id": None,
        "caller_type": "agent",
        "entity_id": None,
        "entity": None,
        "agent_id": agent_row["id"],
        "agent": {**agent_row, "requires_approval": True},
        "scopes": scope_ids,
    }

    async def mock_auth():
        """Mock agent auth for pre-approval validation tests."""

        return auth_dict

    return mock_auth


@pytest.mark.asyncio
async def test_update_job_status_invalid_status_rejected_before_approval(
    db_pool, enums, untrusted_agent_row
):
    """Invalid status should return 4xx and not create an approval request."""

    in_progress = enums.statuses.name_to_id["in-progress"]
    public_scope_id = enums.scopes.name_to_id["public"]
    job = await db_pool.fetchrow(
        """
        INSERT INTO jobs (title, agent_id, status_id, priority, metadata, privacy_scope_ids)
        VALUES ($1, $2::uuid, $3::uuid, $4, $5::jsonb, $6::uuid[])
        RETURNING id
        """,
        "preapproval-status-test",
        untrusted_agent_row["id"],
        in_progress,
        "medium",
        json.dumps({}),
        [public_scope_id],
    )
    job_id = job["id"]

    app.state.pool = db_pool
    app.state.enums = enums
    app.dependency_overrides[require_auth] = _untrusted_auth_override(
        untrusted_agent_row, enums, ["public"]
    )

    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.patch(
            f"/api/jobs/{job_id}/status",
            json={"status": "todo"},
        )

    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 400, resp.text
    body = resp.json()
    assert body["detail"]["error"]["code"] == "INVALID_INPUT"

    count = await db_pool.fetchval(
        """
        SELECT COUNT(*)
        FROM approval_requests
        WHERE requested_by = $1::uuid
          AND request_type = 'update_job_status'
          AND change_details->>'job_id' = $2
        """,
        untrusted_agent_row["id"],
        job_id,
    )
    assert int(count or 0) == 0


@pytest.mark.asyncio
@pytest.mark.parametrize(
    ("path", "payload", "request_type"),
    [
        (
            "/api/entities/",
            {
                "name": "visibility-entity",
                "type": "person",
                "status": "active",
                "scopes": ["public"],
                "metadata": {"visibility": "private"},
            },
            "create_entity",
        ),
        (
            "/api/context/",
            {
                "title": "visibility-context",
                "source_type": "note",
                "scopes": ["public"],
                "metadata": {"visibility": "private"},
            },
            "create_context",
        ),
    ],
)
async def test_visibility_metadata_key_rejected_before_approval(
    db_pool, enums, untrusted_agent_row, path, payload, request_type
):
    """Visibility metadata key should be rejected pre-approval with 4xx."""

    app.state.pool = db_pool
    app.state.enums = enums
    app.dependency_overrides[require_auth] = _untrusted_auth_override(
        untrusted_agent_row, enums, ["public"]
    )

    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.post(path, json=payload)

    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 400, resp.text
    body = resp.json()
    detail = body.get("detail")
    if isinstance(detail, dict):
        assert detail["error"]["code"] == "INVALID_INPUT"
        message = detail["error"]["message"]
    else:
        message = str(detail)
    assert "visibility" in message.lower()

    count = await db_pool.fetchval(
        """
        SELECT COUNT(*)
        FROM approval_requests
        WHERE requested_by = $1::uuid
          AND request_type = $2
          AND status = 'pending'
        """,
        untrusted_agent_row["id"],
        request_type,
    )
    assert int(count or 0) == 0


@pytest.mark.asyncio
async def test_update_entity_visibility_metadata_key_rejected_before_approval(
    db_pool, enums, untrusted_agent_row
):
    """Entity metadata updates should reject visibility keys before queuing approvals."""

    status_id = enums.statuses.name_to_id["active"]
    type_id = enums.entity_types.name_to_id["person"]
    public_scope_id = enums.scopes.name_to_id["public"]
    entity = await db_pool.fetchrow(
        """
        INSERT INTO entities
            (name, type_id, status_id, privacy_scope_ids, tags, metadata)
        VALUES
            ($1, $2, $3, $4::uuid[], $5, $6::jsonb)
        RETURNING id
        """,
        "visibility-update-entity",
        type_id,
        status_id,
        [public_scope_id],
        [],
        "{}",
    )
    assert entity is not None

    app.state.pool = db_pool
    app.state.enums = enums
    app.dependency_overrides[require_auth] = _untrusted_auth_override(
        untrusted_agent_row, enums, ["public"]
    )

    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.patch(
            f"/api/entities/{entity['id']}",
            json={"metadata": {"visibility": "private"}},
        )

    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 400, resp.text
    body = resp.json()
    detail = body.get("detail")
    if isinstance(detail, dict):
        assert detail["error"]["code"] == "INVALID_INPUT"
        message = detail["error"]["message"]
    else:
        message = str(detail)
    assert "visibility" in message.lower()

    count = await db_pool.fetchval(
        """
        SELECT COUNT(*)
        FROM approval_requests
        WHERE requested_by = $1::uuid
          AND request_type = 'update_entity'
          AND status = 'pending'
        """,
        untrusted_agent_row["id"],
    )
    assert int(count or 0) == 0
