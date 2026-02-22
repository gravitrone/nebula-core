"""Integration tests for the approval workflow helpers."""

# Standard Library
import asyncio
import json

# Third-Party
import pytest

# Local
from nebula_mcp.helpers import (
    approve_request,
    create_approval_request,
    get_pending_approvals_all,
    reject_request,
)

pytestmark = pytest.mark.integration


# --- create_approval_request ---


async def test_create_approval_request_returns_pending(db_pool, enums, untrusted_agent):
    """Creating an approval request should return a row with pending status."""

    result = await create_approval_request(
        db_pool,
        str(untrusted_agent["id"]),
        "create_entity",
        {
            "name": "Pending Entity",
            "type": "project",
            "status": "active",
            "scopes": ["public"],
        },
    )

    assert result["status"] == "pending"


# --- get_pending_approvals_all ---


async def test_get_pending_approvals_count(db_pool, enums, untrusted_agent):
    """get_pending_approvals_all should return the correct number of pending requests."""

    await create_approval_request(
        db_pool,
        str(untrusted_agent["id"]),
        "create_entity",
        {
            "name": "A",
            "type": "project",
            "status": "active",
            "scopes": ["public"],
        },
    )
    await create_approval_request(
        db_pool,
        str(untrusted_agent["id"]),
        "create_entity",
        {
            "name": "B",
            "type": "project",
            "status": "active",
            "scopes": ["public"],
        },
    )

    pending = await get_pending_approvals_all(db_pool)
    assert len(pending) == 2


async def test_get_pending_approvals_enriches_relationship_endpoint_names(
    db_pool, enums, untrusted_agent, test_entity
):
    """Relationship approvals should include readable endpoint labels."""

    status_id = enums.statuses.name_to_id["active"]
    type_id = enums.entity_types.name_to_id["person"]
    scope_ids = [enums.scopes.name_to_id["public"]]

    target = await db_pool.fetchrow(
        """
        INSERT INTO entities (name, type_id, status_id, privacy_scope_ids, tags, metadata)
        VALUES ($1, $2, $3, $4, $5, $6::jsonb)
        RETURNING id, name
        """,
        "Target Person",
        type_id,
        status_id,
        scope_ids,
        ["test"],
        "{}",
    )

    await create_approval_request(
        db_pool,
        str(untrusted_agent["id"]),
        "create_relationship",
        {
            "relationship_type": "owns",
            "source_type": "entity",
            "source_id": str(test_entity["id"]),
            "target_type": "entity",
            "target_id": str(target["id"]),
        },
    )

    pending = await get_pending_approvals_all(db_pool)
    assert len(pending) == 1

    details = pending[0]["change_details"]
    assert details["source_name"] == "Test Person"
    assert details["target_name"] == "Target Person"


async def test_get_pending_approvals_enriches_bulk_entity_names(
    db_pool, enums, untrusted_agent, test_entity
):
    """Bulk entity approvals should include readable entity names."""

    await create_approval_request(
        db_pool,
        str(untrusted_agent["id"]),
        "bulk_update_entity_scopes",
        {
            "entity_ids": [str(test_entity["id"])],
            "scopes": ["admin"],
            "op": "add",
        },
    )

    pending = await get_pending_approvals_all(db_pool)
    assert len(pending) == 1

    details = pending[0]["change_details"]
    assert details["entity_names"] == ["Test Person"]
    assert details["entity_name"] == "Test Person"


# --- approve_request ---


async def test_approve_request_creates_entity_and_links_audit(
    db_pool,
    enums,
    untrusted_agent,
    test_entity,
):
    """Approving a request should create the entity and link an audit trail."""

    approval = await create_approval_request(
        db_pool,
        str(untrusted_agent["id"]),
        "create_entity",
        {
            "name": "Approved Entity",
            "type": "project",
            "status": "active",
            "scopes": ["public"],
        },
    )

    result = await approve_request(
        db_pool,
        enums,
        str(approval["id"]),
        str(test_entity["id"]),
    )

    assert "entity" in result
    assert result["entity"]["name"] == "Approved Entity"
    assert "approval" in result


async def test_approve_nonexistent_raises(db_pool, enums, test_entity):
    """Approving a nonexistent approval ID should raise ValueError."""

    with pytest.raises(
        ValueError, match="Approval request not found or already processed"
    ):
        await approve_request(
            db_pool,
            enums,
            "00000000-0000-0000-0000-000000000000",
            str(test_entity["id"]),
        )


async def test_approve_twice_raises(db_pool, enums, untrusted_agent, test_entity):
    """Approving the same request twice should raise ValueError."""

    approval = await create_approval_request(
        db_pool,
        str(untrusted_agent["id"]),
        "create_entity",
        {
            "name": "Double Approve Entity",
            "type": "project",
            "status": "active",
            "scopes": ["public"],
        },
    )

    await approve_request(
        db_pool,
        enums,
        str(approval["id"]),
        str(test_entity["id"]),
    )

    with pytest.raises(ValueError, match="already processed"):
        await approve_request(
            db_pool,
            enums,
            str(approval["id"]),
            str(test_entity["id"]),
        )


async def test_approve_request_race_only_one_executes(
    db_pool,
    enums,
    untrusted_agent,
    test_entity,
):
    """Concurrent approvals should only execute once."""

    approval = await create_approval_request(
        db_pool,
        str(untrusted_agent["id"]),
        "create_entity",
        {
            "name": "Race Approval",
            "type": "project",
            "status": "active",
            "scopes": ["public"],
        },
    )

    async def do_approve():
        """Run a single approval attempt."""

        try:
            return await approve_request(
                db_pool,
                enums,
                str(approval["id"]),
                str(test_entity["id"]),
            )
        except Exception as exc:  # noqa: BLE001 - redteam capture
            return exc

    results = await asyncio.gather(do_approve(), do_approve())
    successes = [r for r in results if isinstance(r, dict)]
    failures = [r for r in results if isinstance(r, Exception)]

    assert len(successes) == 1
    assert len(failures) == 1


async def test_approve_unknown_executor_type_raises(
    db_pool, enums, untrusted_agent, test_entity
):
    """Approving a request with an unknown executor type should raise ValueError."""

    approval = await create_approval_request(
        db_pool,
        str(untrusted_agent["id"]),
        "nonexistent_action",
        {},
    )

    with pytest.raises(ValueError, match="No executor for"):
        await approve_request(
            db_pool,
            enums,
            str(approval["id"]),
            str(test_entity["id"]),
        )


async def test_approve_with_bad_data_marks_failed(
    db_pool, enums, untrusted_agent, test_entity
):
    """Approving a request with invalid data should mark it as approved-failed."""

    approval = await create_approval_request(
        db_pool,
        str(untrusted_agent["id"]),
        "create_entity",
        {
            "name": "Will Fail",
            "type": "INVALID_TYPE",
            "status": "active",
            "scopes": ["public"],
        },
    )

    with pytest.raises(ValueError):
        await approve_request(
            db_pool,
            enums,
            str(approval["id"]),
            str(test_entity["id"]),
        )

    # Verify the approval_requests row was marked as failed
    row = await db_pool.fetchrow(
        "SELECT status FROM approval_requests WHERE id = $1::uuid",
        str(approval["id"]),
    )
    assert row["status"] == "approved-failed"


async def test_approve_bulk_scope_update_does_not_require_single_entity_id(
    db_pool, enums, untrusted_agent, test_entity
):
    """Bulk approve path should not crash when executor returns no top-level id."""

    approval = await create_approval_request(
        db_pool,
        str(untrusted_agent["id"]),
        "bulk_update_entity_scopes",
        {
            "entity_ids": [str(test_entity["id"])],
            "scopes": ["admin"],
            "op": "add",
        },
    )

    result = await approve_request(
        db_pool,
        enums,
        str(approval["id"]),
        str(test_entity["id"]),
    )

    assert result["approval"]["status"] == "approved"
    assert result["entity"]["updated"] == 1
    assert result["entity"]["entity_ids"] == [str(test_entity["id"])]


async def test_approve_mixed_entity_updates_do_not_fail_with_missing_id(
    db_pool, enums, untrusted_agent, test_entity
):
    """Sequentially approving mixed entity mutations should not raise KeyError('id')."""

    bulk_approval = await create_approval_request(
        db_pool,
        str(untrusted_agent["id"]),
        "bulk_update_entity_scopes",
        {
            "entity_ids": [str(test_entity["id"])],
            "scopes": ["admin"],
            "op": "add",
        },
    )
    update_approval = await create_approval_request(
        db_pool,
        str(untrusted_agent["id"]),
        "update_entity",
        {
            "entity_id": str(test_entity["id"]),
            "tags": ["bulk-approved"],
            "status": "active",
        },
    )

    reviewer = str(test_entity["id"])
    bulk_result = await approve_request(
        db_pool,
        enums,
        str(bulk_approval["id"]),
        reviewer,
    )
    update_result = await approve_request(
        db_pool,
        enums,
        str(update_approval["id"]),
        reviewer,
    )

    assert bulk_result["approval"]["status"] == "approved"
    assert bulk_result["entity"]["updated"] == 1
    assert update_result["approval"]["status"] == "approved"
    assert "bulk-approved" in (update_result["entity"].get("tags") or [])


async def test_approve_update_entity_metadata_patch_merges_existing(
    db_pool, enums, untrusted_agent, test_entity
):
    """update_entity approvals should deep-merge metadata patches."""

    status_id = enums.statuses.name_to_id["active"]
    type_id = enums.entity_types.name_to_id["person"]
    scope_ids = [enums.scopes.name_to_id["public"]]

    row = await db_pool.fetchrow(
        """
        INSERT INTO entities (name, type_id, status_id, privacy_scope_ids, tags, metadata)
        VALUES ($1, $2::uuid, $3::uuid, $4::uuid[], $5, $6::jsonb)
        RETURNING id
        """,
        "Metadata Merge Approval Entity",
        type_id,
        status_id,
        scope_ids,
        [],
        json.dumps(
            {
                "owner": "alxx",
                "profile": {"timezone": "Europe/Warsaw", "language": "en"},
            }
        ),
    )

    approval = await create_approval_request(
        db_pool,
        str(untrusted_agent["id"]),
        "update_entity",
        {
            "entity_id": str(row["id"]),
            "metadata": {"profile": {"timezone": "UTC"}, "seed_version": "v2"},
        },
    )

    result = await approve_request(
        db_pool,
        enums,
        str(approval["id"]),
        str(test_entity["id"]),
    )
    assert result["approval"]["status"] == "approved"

    stored = await db_pool.fetchval(
        "SELECT metadata FROM entities WHERE id = $1::uuid",
        str(row["id"]),
    )
    decoded = json.loads(stored) if isinstance(stored, str) else stored
    assert decoded["owner"] == "alxx"
    assert decoded["profile"]["language"] == "en"
    assert decoded["profile"]["timezone"] == "UTC"
    assert decoded["seed_version"] == "v2"


async def test_approve_revert_entity_executes(
    db_pool, enums, untrusted_agent, test_entity
):
    """revert_entity approvals should execute via executor registry."""

    await db_pool.execute(
        "UPDATE entities SET name = $2 WHERE id = $1::uuid",
        str(test_entity["id"]),
        "Mid Name",
    )
    audit_id = await db_pool.fetchval(
        """
        SELECT id
        FROM audit_log
        WHERE table_name = 'entities' AND record_id = $1::text
        ORDER BY changed_at DESC
        LIMIT 1
        """,
        str(test_entity["id"]),
    )
    assert audit_id is not None

    await db_pool.execute(
        "UPDATE entities SET name = $2 WHERE id = $1::uuid",
        str(test_entity["id"]),
        "Current Name",
    )

    approval = await create_approval_request(
        db_pool,
        str(untrusted_agent["id"]),
        "revert_entity",
        {
            "entity_id": str(test_entity["id"]),
            "audit_id": str(audit_id),
        },
    )

    result = await approve_request(
        db_pool,
        enums,
        str(approval["id"]),
        str(test_entity["id"]),
    )

    assert result["approval"]["status"] == "approved"
    assert str(result["entity"]["id"]) == str(test_entity["id"])
    assert result["entity"]["name"] == "Mid Name"


# --- reject_request ---


async def test_reject_request_success(db_pool, enums, untrusted_agent, test_entity):
    """Rejecting a request should set status to rejected and store review notes."""

    approval = await create_approval_request(
        db_pool,
        str(untrusted_agent["id"]),
        "create_entity",
        {
            "name": "To Reject",
            "type": "project",
            "status": "active",
            "scopes": ["public"],
        },
    )

    result = await reject_request(
        db_pool,
        str(approval["id"]),
        str(test_entity["id"]),
        "Not needed",
    )

    assert result["status"] == "rejected"
    assert result["review_notes"] == "Not needed"


async def test_reject_nonexistent_raises(db_pool, enums, test_entity):
    """Rejecting a nonexistent approval ID should raise ValueError."""

    with pytest.raises(
        ValueError, match="Approval request not found or already processed"
    ):
        await reject_request(
            db_pool,
            "00000000-0000-0000-0000-000000000000",
            str(test_entity["id"]),
            "Does not exist",
        )
