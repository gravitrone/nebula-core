"""Red team tests for approvals access control."""

# Third-Party
import pytest

# Local
from nebula_mcp.helpers import create_approval_request


@pytest.mark.asyncio
async def test_pending_approvals_requires_admin(api):
    """Non-admin users should not list pending approvals."""

    resp = await api.get("/api/approvals/pending")
    assert resp.status_code == 403


@pytest.mark.asyncio
async def test_get_approval_requires_admin(api, db_pool, test_agent_row):
    """Non-admin users should not read approval details."""

    approval = await create_approval_request(
        db_pool,
        test_agent_row["id"],
        "create_entity",
        {
            "name": "Approval Test",
            "type": "person",
            "status": "active",
            "scopes": ["public"],
            "tags": ["test"],
        },
        None,
    )

    resp = await api.get(f"/api/approvals/{approval['id']}")
    assert resp.status_code == 403


@pytest.mark.asyncio
async def test_approve_requires_admin(api, db_pool, test_agent_row):
    """Non-admin users should not approve requests."""

    approval = await create_approval_request(
        db_pool,
        test_agent_row["id"],
        "create_entity",
        {
            "name": "Approval Test",
            "type": "person",
            "status": "active",
            "scopes": ["public"],
            "tags": ["test"],
        },
        None,
    )

    resp = await api.post(f"/api/approvals/{approval['id']}/approve")
    assert resp.status_code == 403


@pytest.mark.asyncio
async def test_reject_requires_admin(api, db_pool, test_agent_row):
    """Non-admin users should not reject requests."""

    approval = await create_approval_request(
        db_pool,
        test_agent_row["id"],
        "create_entity",
        {
            "name": "Approval Test",
            "type": "person",
            "status": "active",
            "scopes": ["public"],
            "tags": ["test"],
        },
        None,
    )

    resp = await api.post(
        f"/api/approvals/{approval['id']}/reject",
        json={"review_notes": "nope"},
    )
    assert resp.status_code == 403
