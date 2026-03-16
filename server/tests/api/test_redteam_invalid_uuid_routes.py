"""Red team tests for invalid UUID handling in API write routes."""

# Third-Party
import pytest


@pytest.mark.asyncio
async def test_api_update_entity_rejects_invalid_uuid(api):
    """Invalid UUIDs should not crash update entity routes."""

    resp = await api.patch(
        "/api/entities/not-a-uuid",
        json={"tags": ["bad"]},
    )
    assert resp.status_code in {400, 404}


@pytest.mark.asyncio
async def test_api_get_relationships_rejects_invalid_uuid(api):
    """Invalid UUIDs should not crash relationship list routes."""

    resp = await api.get("/api/relationships/entity/not-a-uuid")
    assert resp.status_code in {400, 404}


@pytest.mark.asyncio
async def test_api_get_relationships_accepts_job_style_ids(api):
    """Job relationship lookups should accept canonical job ids (non-UUID)."""

    resp = await api.get("/api/relationships/job/2026Q1-ABCD")
    assert resp.status_code == 200
    body = resp.json()
    assert isinstance(body.get("data"), list)


@pytest.mark.asyncio
async def test_api_update_relationship_rejects_invalid_uuid(api):
    """Invalid UUIDs should not crash relationship update routes."""

    resp = await api.patch(
        "/api/relationships/not-a-uuid",
        json={"properties": {"note": "bad"}},
    )
    assert resp.status_code in {400, 404}


@pytest.mark.asyncio
async def test_api_query_jobs_rejects_invalid_assignee(api):
    """Invalid UUIDs should not crash job query routes."""

    resp = await api.get("/api/jobs", params={"assigned_to": "not-a-uuid"})
    assert resp.status_code in {400, 404}


@pytest.mark.asyncio
async def test_api_delete_key_rejects_invalid_uuid(api):
    """Invalid UUIDs should not crash key revoke routes."""

    resp = await api.delete("/api/keys/not-a-uuid")
    assert resp.status_code in {400, 404}


@pytest.mark.asyncio
async def test_api_update_agent_rejects_invalid_uuid(api):
    """Invalid UUIDs should not crash agent update routes."""

    resp = await api.patch(
        "/api/agents/not-a-uuid",
        json={"description": "bad"},
    )
    assert resp.status_code in {400, 403, 404}


@pytest.mark.asyncio
async def test_api_approval_routes_reject_invalid_uuid(api):
    """Approval detail and state-change routes should validate UUIDs."""

    get_resp = await api.get("/api/approvals/not-a-uuid")
    approve_resp = await api.post("/api/approvals/not-a-uuid/approve")
    reject_resp = await api.post(
        "/api/approvals/not-a-uuid/reject",
        json={"review_notes": "bad"},
    )
    diff_resp = await api.get("/api/approvals/not-a-uuid/diff")

    assert get_resp.status_code in {400, 403}
    assert approve_resp.status_code in {400, 403}
    assert reject_resp.status_code in {400, 403}
    assert diff_resp.status_code in {400, 403}
