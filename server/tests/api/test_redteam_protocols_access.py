"""Red team tests for protocol trusted access control."""

# Third-Party
import pytest


@pytest.mark.asyncio
async def test_create_protocol_forces_trusted_false_for_non_admin(api):
    """Non-admin users should not persist trusted protocols on create."""

    resp = await api.post(
        "/api/protocols/",
        json={
            "name": "rt-proto-trusted-create",
            "title": "RT Trusted Create",
            "version": "1.0.0",
            "content": "trusted escalation create",
            "protocol_type": "security",
            "applies_to": ["agents"],
            "status": "active",
            "tags": ["redteam"],
            "notes": "source: redteam",
            "trusted": True,
        },
    )
    assert resp.status_code == 200
    assert resp.json()["data"]["trusted"] is False


@pytest.mark.asyncio
async def test_update_protocol_forces_trusted_false_for_non_admin(api):
    """Non-admin users should not persist trusted protocols on update."""

    create_resp = await api.post(
        "/api/protocols/",
        json={
            "name": "rt-proto-trusted-update",
            "title": "RT Trusted Update",
            "version": "1.0.0",
            "content": "trusted escalation update",
            "protocol_type": "security",
            "applies_to": ["agents"],
            "status": "active",
            "tags": ["redteam"],
            "notes": "source: redteam",
            "trusted": False,
        },
    )
    assert create_resp.status_code == 200

    update_resp = await api.patch(
        "/api/protocols/rt-proto-trusted-update",
        json={"trusted": True},
    )
    assert update_resp.status_code == 200
    assert update_resp.json()["data"]["trusted"] is False
