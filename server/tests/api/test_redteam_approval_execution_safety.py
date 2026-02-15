"""Red team tests ensuring approval execution failures are safe and tracked."""

# Standard Library
import json

# Third-Party
import pytest


@pytest.mark.asyncio
async def test_approve_invalid_enum_marks_approved_failed(api, db_pool, auth_override, enums, untrusted_agent_row):
    """Approving a poisoned request should not 500 and should mark approved-failed."""

    auth_override["scopes"] = [enums.scopes.name_to_id["admin"]]

    in_progress = enums.statuses.name_to_id["in-progress"]
    public_scope_id = enums.scopes.name_to_id["public"]
    job = await db_pool.fetchrow(
        """
        INSERT INTO jobs (title, agent_id, status_id, priority, metadata, privacy_scope_ids)
        VALUES ($1, $2::uuid, $3::uuid, $4, $5::jsonb, $6::uuid[])
        RETURNING id
        """,
        "approval-poison-job",
        untrusted_agent_row["id"],
        in_progress,
        "medium",
        json.dumps({}),
        [public_scope_id],
    )
    job_id = job["id"]

    approval = await db_pool.fetchrow(
        """
        INSERT INTO approval_requests (request_type, requested_by, change_details, status)
        VALUES ($1, $2::uuid, $3::jsonb, $4)
        RETURNING id
        """,
        "update_job_status",
        untrusted_agent_row["id"],
        json.dumps({"job_id": job_id, "status": "todo"}),
        "pending",
    )
    approval_id = str(approval["id"])

    resp = await api.post(f"/api/approvals/{approval_id}/approve")
    assert resp.status_code == 400, resp.text
    assert resp.json()["detail"]["error"]["code"] == "EXECUTION_FAILED"

    updated = await db_pool.fetchrow(
        "SELECT status, execution_error FROM approval_requests WHERE id = $1::uuid",
        approval_id,
    )
    assert updated["status"] == "approved-failed"
    assert updated["execution_error"]

