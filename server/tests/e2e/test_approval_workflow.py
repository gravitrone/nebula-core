"""E2E test: full approval workflow."""

# Standard Library
import json
import pytest

from nebula_mcp.helpers import (
    approve_request,
    create_approval_request,
    reject_request,
)

pytestmark = pytest.mark.e2e


# --- Helpers ---


async def _make_agent(db_pool, enums, name, trusted=False):
    """Insert a test agent and return the row."""

    status_id = enums.statuses.name_to_id["active"]

    row = await db_pool.fetchrow(
        """
        INSERT INTO agents (name, status_id, requires_approval)
        VALUES ($1, $2, $3)
        RETURNING *
        """,
        name,
        status_id,
        not trusted,
    )
    return row


async def _make_reviewer(db_pool, enums):
    """Insert a reviewer entity and return the row."""

    status_id = enums.statuses.name_to_id["active"]
    type_id = enums.entity_types.name_to_id["person"]
    scope_ids = [enums.scopes.name_to_id["public"]]

    row = await db_pool.fetchrow(
        """
        INSERT INTO entities (privacy_scope_ids, name, type_id, status_id)
        VALUES ($1, $2, $3, $4)
        RETURNING *
        """,
        scope_ids,
        "test-reviewer",
        type_id,
        status_id,
    )
    return row


# --- Approval Workflow ---


@pytest.mark.asyncio
async def test_approve_creates_entity(db_pool, enums):
    """Untrusted agent creates approval -> approve -> entity exists in DB."""

    agent = await _make_agent(db_pool, enums, "untrusted-approve")
    reviewer = await _make_reviewer(db_pool, enums)

    change_details = {
        "name": "Approved Entity",
        "type": "project",
        "status": "active",
        "scopes": ["public"],
        "tags": ["approved"],
        "metadata": {"description": "Created via approval"},
    }

    approval = await create_approval_request(
        db_pool,
        str(agent["id"]),
        "create_entity",
        change_details,
    )
    assert approval["status"] == "pending"

    result = await approve_request(
        db_pool, enums, str(approval["id"]), str(reviewer["id"])
    )
    entity_id = str(result["entity"]["id"])

    row = await db_pool.fetchrow(
        "SELECT * FROM entities WHERE id = $1::uuid", entity_id
    )
    assert row is not None
    assert row["name"] == "Approved Entity"
    decoded = json.loads(row["metadata"])
    assert isinstance(decoded, dict)
    assert decoded.get("description") == "Created via approval"


@pytest.mark.asyncio
async def test_reject_does_not_create_entity(db_pool, enums):
    """Untrusted agent creates approval -> reject -> entity does NOT exist."""

    agent = await _make_agent(db_pool, enums, "untrusted-reject")
    reviewer = await _make_reviewer(db_pool, enums)

    change_details = {
        "name": "Rejected Entity",
        "type": "project",
        "status": "active",
        "scopes": ["public"],
        "tags": ["rejected"],
        "metadata": {},
    }

    approval = await create_approval_request(
        db_pool,
        str(agent["id"]),
        "create_entity",
        change_details,
    )

    rejected = await reject_request(
        db_pool, str(approval["id"]), str(reviewer["id"]), "Not needed"
    )
    assert rejected["status"] == "rejected"

    count = await db_pool.fetchval(
        "SELECT COUNT(*) FROM entities WHERE name = 'Rejected Entity'"
    )
    assert count == 0


@pytest.mark.asyncio
async def test_approve_bad_payload_marks_failed(db_pool, enums):
    """Approving a request with an INVALID entity type results in approved-failed status."""

    agent = await _make_agent(db_pool, enums, "untrusted-fail")
    reviewer = await _make_reviewer(db_pool, enums)

    change_details = {
        "name": "Bad Type Entity",
        "type": "INVALID",
        "status": "active",
        "scopes": ["public"],
        "tags": [],
        "metadata": {},
    }

    approval = await create_approval_request(
        db_pool,
        str(agent["id"]),
        "create_entity",
        change_details,
    )

    with pytest.raises(ValueError):
        await approve_request(db_pool, enums, str(approval["id"]), str(reviewer["id"]))

    failed = await db_pool.fetchrow(
        "SELECT status FROM approval_requests WHERE id = $1::uuid",
        str(approval["id"]),
    )
    assert failed["status"] == "approved-failed"
