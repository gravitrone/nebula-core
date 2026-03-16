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


async def _count_pending_approvals(
    db_pool,
    agent_id: str,
    request_type: str,
    detail_key: str | None = None,
    detail_val: str | None = None,
) -> int:
    """Count pending approvals for a specific request type (optionally narrowed by change_details key)."""

    if detail_key is None:
        count = await db_pool.fetchval(
            """
            SELECT COUNT(*)
            FROM approval_requests
            WHERE requested_by = $1::uuid
              AND request_type = $2
              AND status = 'pending'
            """,
            agent_id,
            request_type,
        )
        return int(count or 0)

    count = await db_pool.fetchval(
        f"""
        SELECT COUNT(*)
        FROM approval_requests
        WHERE requested_by = $1::uuid
          AND request_type = $2
          AND status = 'pending'
          AND change_details->>'{detail_key}' = $3
        """,
        agent_id,
        request_type,
        detail_val,
    )
    return int(count or 0)


@pytest.mark.asyncio
async def test_update_job_status_invalid_status_rejected_before_approval(
    db_pool, enums, untrusted_agent_row
):
    """Invalid status should return 4xx and not create an approval request."""

    in_progress = enums.statuses.name_to_id["in-progress"]
    public_scope_id = enums.scopes.name_to_id["public"]
    job = await db_pool.fetchrow(
        """
        INSERT INTO jobs (title, agent_id, status_id, priority, privacy_scope_ids)
        VALUES ($1, $2::uuid, $3::uuid, $4, $5::uuid[])
        RETURNING id
        """,
        "preapproval-status-test",
        untrusted_agent_row["id"],
        in_progress,
        "medium",
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

    count = await _count_pending_approvals(
        db_pool,
        str(untrusted_agent_row["id"]),
        "update_job_status",
        detail_key="job_id",
        detail_val=str(job_id),
    )
    assert count == 0


@pytest.mark.asyncio
@pytest.mark.parametrize(
    ("path", "payload", "request_type"),
    [
        (
            "/api/logs/",
            {
                "log_type": "note",
                "value": {"text": "visibility-log"},
                "status": "active",
                "metadata": {"visibility": "private"},
            },
            "create_log",
        ),
        (
            "/api/files/",
            {
                "filename": "visibility-file.txt",
                "uri": "path:visibility-file.txt",
                "status": "active",
                "metadata": {"visibility": "private"},
            },
            "create_file",
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

    count = await _count_pending_approvals(
        db_pool,
        str(untrusted_agent_row["id"]),
        request_type,
    )
    assert count == 0


@pytest.mark.asyncio
async def test_update_file_visibility_metadata_key_rejected_before_approval(
    db_pool, enums, untrusted_agent_row
):
    """File metadata updates should reject visibility keys before queuing approvals."""

    file_row = await db_pool.fetchrow(
        """
        INSERT INTO files
            (filename, uri, file_path, mime_type, size_bytes, checksum, status_id, tags, metadata)
        VALUES
            ($1, $2, $3, $4, $5, $6, $7::uuid, $8, $9::jsonb)
        RETURNING id
        """,
        "visibility-update-file.txt",
        "path:visibility-update-file.txt",
        "path:visibility-update-file.txt",
        "text/plain",
        12,
        "abc",
        enums.statuses.name_to_id["active"],
        [],
        "{}",
    )
    assert file_row is not None

    app.state.pool = db_pool
    app.state.enums = enums
    app.dependency_overrides[require_auth] = _untrusted_auth_override(
        untrusted_agent_row, enums, ["public"]
    )

    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.patch(
            f"/api/files/{file_row['id']}",
            json={"metadata": {"visibility": "private"}},
        )

    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 400, resp.text
    detail = resp.json().get("detail")
    if isinstance(detail, dict):
        assert detail["error"]["code"] == "INVALID_INPUT"
        message = detail["error"]["message"]
    else:
        message = str(detail)
    assert "visibility" in message.lower()

    count = await _count_pending_approvals(
        db_pool,
        str(untrusted_agent_row["id"]),
        "update_file",
    )
    assert count == 0


@pytest.mark.asyncio
async def test_update_log_visibility_metadata_key_rejected_before_approval(
    db_pool, enums, untrusted_agent_row
):
    """Log metadata updates should reject visibility keys before queuing approvals."""

    log_row = await db_pool.fetchrow(
        """
        INSERT INTO logs (log_type_id, timestamp, value, status_id, tags, metadata)
        VALUES ($1::uuid, NOW(), $2::jsonb, $3::uuid, $4, $5::jsonb)
        RETURNING id
        """,
        enums.log_types.name_to_id["note"],
        json.dumps({"text": "visibility-update-log"}),
        enums.statuses.name_to_id["active"],
        [],
        "{}",
    )
    assert log_row is not None

    app.state.pool = db_pool
    app.state.enums = enums
    app.dependency_overrides[require_auth] = _untrusted_auth_override(
        untrusted_agent_row, enums, ["public"]
    )

    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.patch(
            f"/api/logs/{log_row['id']}",
            json={"metadata": {"visibility": "private"}},
        )

    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 400, resp.text
    detail = resp.json().get("detail")
    if isinstance(detail, dict):
        assert detail["error"]["code"] == "INVALID_INPUT"
        message = detail["error"]["message"]
    else:
        message = str(detail)
    assert "visibility" in message.lower()

    count = await _count_pending_approvals(
        db_pool,
        str(untrusted_agent_row["id"]),
        "update_log",
    )
    assert count == 0
