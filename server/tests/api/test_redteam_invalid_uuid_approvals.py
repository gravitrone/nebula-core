"""Red team tests for invalid UUID handling in approvals API routes."""

# Third-Party
import pytest


@pytest.mark.asyncio
async def test_api_get_approval_rejects_invalid_uuid(api, auth_override, enums):
    """Invalid UUIDs should not crash approval detail routes."""

    auth_override["scopes"] = [enums.scopes.name_to_id["admin"]]

    resp = await api.get("/api/approvals/not-a-uuid")
    assert resp.status_code in {400, 404}


@pytest.mark.asyncio
async def test_api_get_approval_diff_rejects_invalid_uuid(api, auth_override, enums):
    """Invalid UUIDs should not crash approval diff routes."""

    auth_override["scopes"] = [enums.scopes.name_to_id["admin"]]

    resp = await api.get("/api/approvals/not-a-uuid/diff")
    assert resp.status_code in {400, 404}


@pytest.mark.asyncio
async def test_api_approve_rejects_invalid_uuid(api, auth_override, enums):
    """Invalid UUIDs should not crash approval approve routes."""

    auth_override["scopes"] = [enums.scopes.name_to_id["admin"]]

    resp = await api.post("/api/approvals/not-a-uuid/approve")
    assert resp.status_code in {400, 404}


@pytest.mark.asyncio
async def test_api_reject_rejects_invalid_uuid(api, auth_override, enums):
    """Invalid UUIDs should not crash approval reject routes."""

    auth_override["scopes"] = [enums.scopes.name_to_id["admin"]]

    resp = await api.post(
        "/api/approvals/not-a-uuid/reject",
        json={"review_notes": "bad id"},
    )
    assert resp.status_code in {400, 404}
