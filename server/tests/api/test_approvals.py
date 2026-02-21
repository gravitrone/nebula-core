"""Approval route tests."""

# Standard Library
import json

# Third-Party
import pytest

# Local
import nebula_api.routes.approvals as approvals_route


@pytest.fixture
async def untrusted_agent(db_pool, enums):
    """Create an untrusted agent that triggers approvals."""

    status_id = enums.statuses.name_to_id["active"]
    scope_ids = [enums.scopes.name_to_id["public"]]
    row = await db_pool.fetchrow(
        """
        INSERT INTO agents (name, description, scopes, requires_approval, status_id)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING *
        """,
        "api-untrusted",
        "Untrusted agent for API tests",
        scope_ids,
        True,
        status_id,
    )
    return dict(row)


@pytest.fixture
async def pending_approval(db_pool, untrusted_agent):
    """Create a pending approval request."""

    row = await db_pool.fetchrow(
        """
        INSERT INTO approval_requests (request_type, requested_by, change_details, status)
        VALUES ($1, $2, $3::jsonb, $4)
        RETURNING *
        """,
        "create_entity",
        untrusted_agent["id"],
        json.dumps(
            {
                "name": "ApprovalTest",
                "type": "person",
                "scopes": ["public"],
                "tags": [],
                "metadata": {},
                "source_path": None,
                "status": "active",
            }
        ),
        "pending",
    )
    return dict(row)


@pytest.mark.asyncio
async def test_get_pending(api, pending_approval, auth_override, enums):
    """Test get pending."""

    auth_override["scopes"] = [enums.scopes.name_to_id["admin"]]

    r = await api.get("/api/approvals/pending")
    assert r.status_code == 200
    data = r.json()["data"]
    assert len(data) >= 1


@pytest.mark.asyncio
async def test_get_approval(api, pending_approval, auth_override, enums):
    """Test get approval."""

    auth_override["scopes"] = [enums.scopes.name_to_id["admin"]]

    r = await api.get(f"/api/approvals/{pending_approval['id']}")
    assert r.status_code == 200
    assert r.json()["data"]["request_type"] == "create_entity"


@pytest.mark.asyncio
async def test_get_approval_enriches_bulk_entity_scope_targets(
    api, db_pool, auth_override, enums, test_entity, untrusted_agent
):
    """Bulk entity scope approvals should expose readable target entity names."""

    auth_override["scopes"] = [enums.scopes.name_to_id["admin"]]

    approval = await db_pool.fetchrow(
        """
        INSERT INTO approval_requests (request_type, requested_by, change_details, status)
        VALUES ($1, $2::uuid, $3::jsonb, $4)
        RETURNING *
        """,
        "bulk_update_entity_scopes",
        str(untrusted_agent["id"]),
        json.dumps(
            {
                "entity_ids": [str(test_entity["id"])],
                "scopes": ["public", "admin"],
                "op": "add",
            }
        ),
        "pending",
    )

    r = await api.get(f"/api/approvals/{approval['id']}")
    assert r.status_code == 200
    body = r.json()["data"]
    details = body["change_details"]
    assert details["entity_names"] == [test_entity["name"]]
    assert details["entity_name"] == test_entity["name"]
    assert body["requested_by_name"] == untrusted_agent["name"]


@pytest.mark.asyncio
async def test_get_approval_enriches_relationship_endpoints_with_labels(
    api, db_pool, auth_override, enums, test_entity, untrusted_agent
):
    """Relationship approvals should include source/target labels for inbox readability."""

    auth_override["scopes"] = [enums.scopes.name_to_id["admin"]]

    target = await db_pool.fetchrow(
        """
        INSERT INTO entities
            (name, type_id, status_id, privacy_scope_ids, tags, metadata)
        VALUES
            ($1, $2, $3, $4, $5, $6::jsonb)
        RETURNING *
        """,
        "Approval target",
        enums.entity_types.name_to_id["person"],
        enums.statuses.name_to_id["active"],
        [enums.scopes.name_to_id["public"]],
        [],
        "{}",
    )

    approval = await db_pool.fetchrow(
        """
        INSERT INTO approval_requests (request_type, requested_by, change_details, status)
        VALUES ($1, $2::uuid, $3::jsonb, $4)
        RETURNING *
        """,
        "create_relationship",
        str(untrusted_agent["id"]),
        json.dumps(
            {
                "relationship_type": "related-to",
                "source_type": "entity",
                "source_id": str(test_entity["id"]),
                "target_type": "entity",
                "target_id": str(target["id"]),
            }
        ),
        "pending",
    )

    r = await api.get(f"/api/approvals/{approval['id']}")
    assert r.status_code == 200
    details = r.json()["data"]["change_details"]
    assert details["source_name"] == test_entity["name"]
    assert details["target_name"] == target["name"]


@pytest.mark.asyncio
async def test_approve_request(api, pending_approval, auth_override, enums):
    """Test approve request."""

    auth_override["scopes"] = [enums.scopes.name_to_id["admin"]]

    r = await api.post(f"/api/approvals/{pending_approval['id']}/approve")
    assert r.status_code == 200


@pytest.mark.asyncio
async def test_approve_non_register_rejects_grant_fields(
    api, pending_approval, auth_override, enums
):
    """Grant override fields should be rejected for non-register approvals."""

    auth_override["scopes"] = [enums.scopes.name_to_id["admin"]]
    r = await api.post(
        f"/api/approvals/{pending_approval['id']}/approve",
        json={
            "grant_scopes": ["public"],
            "grant_requires_approval": False,
        },
    )
    assert r.status_code == 400
    body = r.json()
    assert body["detail"]["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
async def test_approve_register_agent_accepts_grant_fields(
    api, db_pool, auth_override, enums
):
    """Reviewer grants should override requested register_agent scopes and trust."""

    auth_override["scopes"] = [enums.scopes.name_to_id["admin"]]
    inactive_status_id = enums.statuses.name_to_id["inactive"]
    active_status_id = enums.statuses.name_to_id["active"]

    agent = await db_pool.fetchrow(
        """
        INSERT INTO agents (name, description, scopes, requires_approval, status_id)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING *
        """,
        "approval-register-agent",
        "pending enrollment",
        [enums.scopes.name_to_id["public"]],
        True,
        inactive_status_id,
    )

    approval = await db_pool.fetchrow(
        """
        INSERT INTO approval_requests (request_type, requested_by, change_details, status)
        VALUES ($1, $2, $3::jsonb, $4)
        RETURNING *
        """,
        "register_agent",
        agent["id"],
        json.dumps(
            {
                "agent_id": str(agent["id"]),
                "name": "approval-register-agent",
                "requested_scopes": ["public"],
                "requested_requires_approval": True,
                "capabilities": [],
            }
        ),
        "pending",
    )

    r = await api.post(
        f"/api/approvals/{approval['id']}/approve",
        json={
            "grant_scopes": ["public", "private"],
            "grant_requires_approval": False,
            "review_notes": "approved with expanded scopes",
        },
    )
    assert r.status_code == 200

    refreshed = await db_pool.fetchrow(
        "SELECT status_id, scopes, requires_approval FROM agents WHERE id = $1::uuid",
        agent["id"],
    )
    assert refreshed["status_id"] == active_status_id
    assert refreshed["requires_approval"] is False
    assert set(refreshed["scopes"]) == {
        enums.scopes.name_to_id["public"],
        enums.scopes.name_to_id["private"],
    }

    approval_after = await db_pool.fetchrow(
        "SELECT review_details, review_notes FROM approval_requests WHERE id = $1::uuid",
        approval["id"],
    )
    assert approval_after["review_notes"] == "approved with expanded scopes"
    review_details = approval_after["review_details"]
    if isinstance(review_details, str):
        review_details = json.loads(review_details)
    assert review_details["grant_scopes"] == ["public", "private"]
    assert review_details["grant_requires_approval"] is False


@pytest.mark.asyncio
async def test_approve_register_agent_invalid_grant_scope_returns_4xx(
    api, db_pool, auth_override, enums
):
    """Register-agent grant override should reject unknown scope names."""

    auth_override["scopes"] = [enums.scopes.name_to_id["admin"]]
    inactive_status_id = enums.statuses.name_to_id["inactive"]

    agent = await db_pool.fetchrow(
        """
        INSERT INTO agents (name, description, scopes, requires_approval, status_id)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING *
        """,
        "approval-register-agent-invalid-grant",
        "pending enrollment",
        [enums.scopes.name_to_id["public"]],
        True,
        inactive_status_id,
    )

    approval = await db_pool.fetchrow(
        """
        INSERT INTO approval_requests (request_type, requested_by, change_details, status)
        VALUES ($1, $2, $3::jsonb, $4)
        RETURNING *
        """,
        "register_agent",
        agent["id"],
        json.dumps(
            {
                "agent_id": str(agent["id"]),
                "name": "approval-register-agent-invalid-grant",
                "requested_scopes": ["public"],
                "requested_requires_approval": True,
                "capabilities": [],
            }
        ),
        "pending",
    )

    r = await api.post(
        f"/api/approvals/{approval['id']}/approve",
        json={"grant_scopes": ["public", "not-real"]},
    )
    assert r.status_code == 400
    assert r.json()["detail"]["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
async def test_approve_revert_entity_request_executes(
    api, db_pool, auth_override, enums, untrusted_agent, test_entity
):
    """Approving revert_entity via API should execute and restore entity state."""

    auth_override["scopes"] = [enums.scopes.name_to_id["admin"]]
    entity_id = str(test_entity["id"])

    await db_pool.execute(
        "UPDATE entities SET name = $2 WHERE id = $1::uuid",
        entity_id,
        "Revert Midpoint",
    )
    audit_id = await db_pool.fetchval(
        """
        SELECT id
        FROM audit_log
        WHERE table_name = 'entities' AND record_id = $1
        ORDER BY changed_at DESC
        LIMIT 1
        """,
        entity_id,
    )
    assert audit_id is not None

    await db_pool.execute(
        "UPDATE entities SET name = $2 WHERE id = $1::uuid",
        entity_id,
        "Revert Current",
    )

    approval = await db_pool.fetchrow(
        """
        INSERT INTO approval_requests (request_type, requested_by, change_details, status)
        VALUES ($1, $2, $3::jsonb, $4)
        RETURNING *
        """,
        "revert_entity",
        untrusted_agent["id"],
        json.dumps({"entity_id": entity_id, "audit_id": str(audit_id)}),
        "pending",
    )
    assert approval is not None

    resp = await api.post(f"/api/approvals/{approval['id']}/approve")
    assert resp.status_code == 200

    refreshed = await db_pool.fetchrow(
        "SELECT name FROM entities WHERE id = $1::uuid",
        entity_id,
    )
    assert refreshed["name"] == "Revert Midpoint"


@pytest.mark.asyncio
async def test_reject_register_agent_updates_enrollment_status_and_reason(
    api, db_pool, auth_override, enums
):
    """Rejecting register_agent should mark linked enrollment as rejected with reason."""

    auth_override["scopes"] = [enums.scopes.name_to_id["admin"]]
    register = await api.post(
        "/api/agents/register",
        json={
            "name": "approval-reject-enrollment-agent",
            "requested_scopes": ["public"],
        },
    )
    assert register.status_code == 201
    approval_id = register.json()["data"]["approval_request_id"]

    rejected = await api.post(
        f"/api/approvals/{approval_id}/reject",
        json={"review_notes": "insufficient trust evidence"},
    )
    assert rejected.status_code == 200

    enrollment = await db_pool.fetchrow(
        """
        SELECT status, rejected_reason
        FROM agent_enrollment_sessions
        WHERE approval_request_id = $1::uuid
        """,
        approval_id,
    )
    assert enrollment["status"] == "rejected"
    assert enrollment["rejected_reason"] == "insufficient trust evidence"


@pytest.mark.asyncio
async def test_approve_register_agent_persists_review_details_shape(
    api, db_pool, auth_override, enums
):
    """Approval review_details should persist both grant names and grant scope ids."""

    auth_override["scopes"] = [enums.scopes.name_to_id["admin"]]
    inactive_status_id = enums.statuses.name_to_id["inactive"]

    agent = await db_pool.fetchrow(
        """
        INSERT INTO agents (name, description, scopes, requires_approval, status_id)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING *
        """,
        "approval-register-agent-review-shape",
        "pending enrollment",
        [enums.scopes.name_to_id["public"]],
        True,
        inactive_status_id,
    )

    approval = await db_pool.fetchrow(
        """
        INSERT INTO approval_requests (request_type, requested_by, change_details, status)
        VALUES ($1, $2, $3::jsonb, $4)
        RETURNING *
        """,
        "register_agent",
        agent["id"],
        json.dumps(
            {
                "agent_id": str(agent["id"]),
                "name": "approval-register-agent-review-shape",
                "requested_scopes": ["public"],
                "requested_requires_approval": True,
                "capabilities": [],
            }
        ),
        "pending",
    )

    r = await api.post(
        f"/api/approvals/{approval['id']}/approve",
        json={
            "grant_scopes": ["public", "private"],
            "grant_requires_approval": False,
        },
    )
    assert r.status_code == 200

    approval_after = await db_pool.fetchrow(
        "SELECT review_details FROM approval_requests WHERE id = $1::uuid",
        approval["id"],
    )
    review_details = approval_after["review_details"]
    if isinstance(review_details, str):
        review_details = json.loads(review_details)

    assert review_details["grant_scopes"] == ["public", "private"]
    assert review_details["grant_requires_approval"] is False
    assert len(review_details["grant_scope_ids"]) == 2
    assert set(review_details["grant_scope_ids"]) == {
        str(enums.scopes.name_to_id["public"]),
        str(enums.scopes.name_to_id["private"]),
    }


@pytest.mark.asyncio
async def test_reject_request(api, pending_approval, auth_override, enums):
    """Test reject request."""

    auth_override["scopes"] = [enums.scopes.name_to_id["admin"]]

    r = await api.post(
        f"/api/approvals/{pending_approval['id']}/reject",
        json={
            "review_notes": "nah bro",
        },
    )
    assert r.status_code == 200


@pytest.mark.asyncio
async def test_get_approval_not_found(api, auth_override, enums):
    """Test get approval not found."""

    auth_override["scopes"] = [enums.scopes.name_to_id["admin"]]

    r = await api.get("/api/approvals/00000000-0000-0000-0000-000000000000")
    assert r.status_code == 404


@pytest.mark.asyncio
@pytest.mark.parametrize(
    ("method", "path", "payload"),
    [
        ("get", "/api/approvals/not-a-uuid", None),
        ("get", "/api/approvals/not-a-uuid/diff", None),
        ("post", "/api/approvals/not-a-uuid/approve", None),
        ("post", "/api/approvals/not-a-uuid/reject", {"review_notes": "x"}),
    ],
)
async def test_approval_routes_validate_uuid_for_admin(
    api, auth_override, enums, method, path, payload
):
    """Admin calls should still reject malformed approval UUIDs."""

    auth_override["scopes"] = [enums.scopes.name_to_id["admin"]]
    request_fn = getattr(api, method)
    kwargs = {"json": payload} if payload is not None else {}
    r = await request_fn(path, **kwargs)
    assert r.status_code == 400
    body = r.json()
    assert body["detail"]["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
async def test_approve_request_not_found(api, auth_override, enums):
    """Approve should return 404 when approval row does not exist."""

    auth_override["scopes"] = [enums.scopes.name_to_id["admin"]]
    r = await api.post("/api/approvals/00000000-0000-0000-0000-000000000000/approve")
    assert r.status_code == 404


@pytest.mark.asyncio
async def test_approve_request_handles_executor_value_error(
    api, pending_approval, auth_override, enums, monkeypatch
):
    """Executor validation errors should map to controlled 400 responses."""

    auth_override["scopes"] = [enums.scopes.name_to_id["admin"]]

    async def _raise_value_error(*args, **kwargs):
        """Handle raise value error.

        Args:
            *args: Input parameter for _raise_value_error.
            **kwargs: Input parameter for _raise_value_error.
        """

        raise ValueError("forced-value-error")

    monkeypatch.setattr(approvals_route, "do_approve", _raise_value_error)
    r = await api.post(f"/api/approvals/{pending_approval['id']}/approve")
    assert r.status_code == 400
    body = r.json()
    assert body["detail"]["error"]["code"] == "EXECUTION_FAILED"
    assert "forced-value-error" in body["detail"]["error"]["message"]


@pytest.mark.asyncio
async def test_approve_request_handles_executor_runtime_error(
    api, pending_approval, auth_override, enums, monkeypatch
):
    """Unexpected executor errors should map to controlled 409 responses."""

    auth_override["scopes"] = [enums.scopes.name_to_id["admin"]]

    async def _raise_runtime_error(*args, **kwargs):
        """Handle raise runtime error.

        Args:
            *args: Input parameter for _raise_runtime_error.
            **kwargs: Input parameter for _raise_runtime_error.
        """

        raise RuntimeError("forced-runtime-error")

    monkeypatch.setattr(approvals_route, "do_approve", _raise_runtime_error)
    r = await api.post(f"/api/approvals/{pending_approval['id']}/approve")
    assert r.status_code == 409
    body = r.json()
    assert body["detail"]["error"]["code"] == "EXECUTION_FAILED"


@pytest.mark.asyncio
async def test_get_approval_diff_create_job(
    api, db_pool, untrusted_agent, auth_override, enums
):
    """Approval diff should include create_job fields."""

    auth_override["scopes"] = [enums.scopes.name_to_id["admin"]]

    row = await db_pool.fetchrow(
        """
        INSERT INTO approval_requests (request_type, requested_by, change_details, status)
        VALUES ($1, $2, $3::jsonb, $4)
        RETURNING *
        """,
        "create_job",
        untrusted_agent["id"],
        json.dumps({"title": "Diff Job", "priority": "high"}),
        "pending",
    )

    r = await api.get(f"/api/approvals/{row['id']}/diff")
    assert r.status_code == 200
    data = r.json()["data"]
    assert data["request_type"] == "create_job"
    assert data["changes"]["title"]["to"] == "Diff Job"


@pytest.mark.asyncio
async def test_get_approval_diff_create_context(
    api, db_pool, untrusted_agent, auth_override, enums
):
    """Approval diff should include create_context fields."""

    auth_override["scopes"] = [enums.scopes.name_to_id["admin"]]

    row = await db_pool.fetchrow(
        """
        INSERT INTO approval_requests (request_type, requested_by, change_details, status)
        VALUES ($1, $2, $3::jsonb, $4)
        RETURNING *
        """,
        "create_context",
        untrusted_agent["id"],
        json.dumps(
            {"title": "Diff Context", "source_type": "note", "scopes": ["public"]}
        ),
        "pending",
    )

    r = await api.get(f"/api/approvals/{row['id']}/diff")
    assert r.status_code == 200
    data = r.json()["data"]
    assert data["request_type"] == "create_context"
    assert data["changes"]["title"]["to"] == "Diff Context"


@pytest.mark.asyncio
async def test_get_approval_diff_update_relationship(
    api, db_pool, enums, untrusted_agent, auth_override
):
    """Approval diff should include relationship updates."""

    auth_override["scopes"] = [enums.scopes.name_to_id["admin"]]

    status_id = enums.statuses.name_to_id["active"]
    type_id = enums.relationship_types.name_to_id["related-to"]

    source = await db_pool.fetchrow(
        """
        INSERT INTO entities (name, type_id, status_id, privacy_scope_ids)
        VALUES ($1, $2, $3, $4)
        RETURNING *
        """,
        "Diff Source",
        enums.entity_types.name_to_id["person"],
        status_id,
        [enums.scopes.name_to_id["public"]],
    )
    target = await db_pool.fetchrow(
        """
        INSERT INTO entities (name, type_id, status_id, privacy_scope_ids)
        VALUES ($1, $2, $3, $4)
        RETURNING *
        """,
        "Diff Target",
        enums.entity_types.name_to_id["person"],
        status_id,
        [enums.scopes.name_to_id["public"]],
    )

    relationship = await db_pool.fetchrow(
        """
        INSERT INTO relationships (source_type, source_id, target_type, target_id, type_id, status_id, properties)
        VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb)
        RETURNING *
        """,
        "entity",
        str(source["id"]),
        "entity",
        str(target["id"]),
        type_id,
        status_id,
        json.dumps({"note": "old"}),
    )

    row = await db_pool.fetchrow(
        """
        INSERT INTO approval_requests (request_type, requested_by, change_details, status)
        VALUES ($1, $2, $3::jsonb, $4)
        RETURNING *
        """,
        "update_relationship",
        untrusted_agent["id"],
        json.dumps(
            {
                "relationship_id": str(relationship["id"]),
                "properties": {"note": "new"},
            }
        ),
        "pending",
    )

    r = await api.get(f"/api/approvals/{row['id']}/diff")
    assert r.status_code == 200
    data = r.json()["data"]
    assert data["request_type"] == "update_relationship"
    assert data["changes"]["properties"]["to"] == {"note": "new"}


@pytest.mark.asyncio
async def test_get_approval_diff_update_job_status(
    api, db_pool, enums, untrusted_agent, auth_override
):
    """Approval diff should include job status updates."""

    auth_override["scopes"] = [enums.scopes.name_to_id["admin"]]

    status_id = enums.statuses.name_to_id["in-progress"]
    job = await db_pool.fetchrow(
        """
        INSERT INTO jobs (title, status_id, priority)
        VALUES ($1, $2, $3)
        RETURNING *
        """,
        "Diff Job Status",
        status_id,
        "medium",
    )

    row = await db_pool.fetchrow(
        """
        INSERT INTO approval_requests (request_type, requested_by, change_details, status)
        VALUES ($1, $2, $3::jsonb, $4)
        RETURNING *
        """,
        "update_job_status",
        untrusted_agent["id"],
        json.dumps(
            {
                "job_id": job["id"],
                "status": "completed",
                "status_reason": "done",
            }
        ),
        "pending",
    )

    r = await api.get(f"/api/approvals/{row['id']}/diff")
    assert r.status_code == 200
    data = r.json()["data"]
    assert data["request_type"] == "update_job_status"
    assert data["changes"]["status"]["to"] == "completed"


@pytest.mark.asyncio
async def test_get_approval_diff_missing_approval_returns_clean_not_found(
    api, auth_override, enums
):
    """Missing approval IDs should return controlled 404/400 responses."""

    auth_override["scopes"] = [enums.scopes.name_to_id["admin"]]
    r = await api.get("/api/approvals/00000000-0000-0000-0000-000000000001/diff")
    assert r.status_code in {400, 404}
    body = r.json()
    assert body.get("detail", {}).get("error", {}).get("code") in {
        "NOT_FOUND",
        "INVALID_INPUT",
    }


@pytest.mark.asyncio
async def test_get_approval_diff_missing_relationship_reference_returns_clean_error(
    api, db_pool, untrusted_agent, auth_override, enums
):
    """Diff generation should fail safely when referenced relationship is missing."""

    auth_override["scopes"] = [enums.scopes.name_to_id["admin"]]

    row = await db_pool.fetchrow(
        """
        INSERT INTO approval_requests (request_type, requested_by, change_details, status)
        VALUES ($1, $2, $3::jsonb, $4)
        RETURNING *
        """,
        "update_relationship",
        untrusted_agent["id"],
        json.dumps(
            {
                "relationship_id": "00000000-0000-0000-0000-000000000001",
                "properties": {"note": "new"},
            }
        ),
        "pending",
    )

    r = await api.get(f"/api/approvals/{row['id']}/diff")
    assert r.status_code in {400, 404}
    body = r.json()
    assert body.get("detail", {}).get("error", {}).get("code") in {
        "NOT_FOUND",
        "INVALID_INPUT",
    }
