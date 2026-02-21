"""Red team tests for invalid UUID handling in audit API routes."""

# Third-Party
import pytest


@pytest.mark.asyncio
async def test_api_audit_rejects_invalid_actor_id(api, auth_override, enums):
    """Invalid UUIDs should not crash audit list routes."""

    auth_override["scopes"] = [enums.scopes.name_to_id["admin"]]

    resp = await api.get("/api/audit", params={"actor_id": "not-a-uuid"})
    assert resp.status_code in {400, 404}


@pytest.mark.asyncio
async def test_api_audit_rejects_invalid_scope_id(api, auth_override, enums):
    """Invalid UUIDs should not crash audit list routes."""

    auth_override["scopes"] = [enums.scopes.name_to_id["admin"]]

    resp = await api.get("/api/audit", params={"scope_id": "not-a-uuid"})
    assert resp.status_code in {400, 404}
