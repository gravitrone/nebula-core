"""Job route tests."""

# Third-Party
import pytest
from httpx import ASGITransport, AsyncClient

# Local
from nebula_api.app import app
from nebula_api.auth import require_auth


@pytest.mark.asyncio
async def test_create_job(api):
    """Test create job."""

    r = await api.post(
        "/api/jobs",
        json={
            "title": "Test Job",
            "description": "A test job",
            "priority": "medium",
        },
    )
    assert r.status_code == 200
    data = r.json()["data"]
    assert data["title"] == "Test Job"
    assert "id" in data


@pytest.mark.asyncio
async def test_create_job_invalid_priority_returns_400(api):
    """Create job should reject unsupported priorities."""

    r = await api.post(
        "/api/jobs",
        json={"title": "Bad Priority", "priority": "urgent"},
    )
    assert r.status_code == 400
    assert r.json()["detail"]["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
async def test_create_job_invalid_due_at_returns_400(api):
    """Create job should reject invalid due_at values."""

    r = await api.post(
        "/api/jobs",
        json={"title": "Bad Due", "due_at": "not-a-datetime"},
    )
    assert r.status_code == 400
    assert r.json()["detail"]["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
async def test_create_job_accepts_iso_due_at(api):
    """Create job should accept ISO due_at strings."""

    r = await api.post(
        "/api/jobs",
        json={
            "title": "Timed Job",
            "due_at": "2026-02-18T18:00:00Z",
        },
    )
    assert r.status_code == 200
    data = r.json()["data"]
    assert data["title"] == "Timed Job"
    assert data["due_at"] is not None


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "due_at",
    [
        "2026-02-18T18:00:00Z",
        "2026-02-18T18:00:00+00:00",
        "2026-02-18T19:30:00+01:30",
        "2026-02-18",
    ],
)
async def test_create_job_due_at_timezone_matrix(api, due_at):
    """Create job should accept ISO due_at values across timezone formats."""

    r = await api.post(
        "/api/jobs",
        json={"title": f"Timed Job {due_at}", "due_at": due_at},
    )
    assert r.status_code == 200
    assert r.json()["data"]["due_at"] is not None


@pytest.mark.asyncio
async def test_get_job(api):
    """Test get job."""

    cr = await api.post("/api/jobs", json={"title": "GetJob"})
    job_id = cr.json()["data"]["id"]

    r = await api.get(f"/api/jobs/{job_id}")
    assert r.status_code == 200
    assert r.json()["data"]["title"] == "GetJob"


@pytest.mark.asyncio
async def test_get_job_not_found(api):
    """Test get job not found."""

    r = await api.get("/api/jobs/00000000-0000-0000-0000-000000000000")
    assert r.status_code == 404


@pytest.mark.asyncio
async def test_get_job_forbidden_when_scope_mismatch(api):
    """Get job should enforce scope visibility."""

    create = await api.post(
        "/api/jobs",
        json={"title": "Sensitive Job", "scopes": ["sensitive"]},
    )
    assert create.status_code == 200
    job_id = create.json()["data"]["id"]

    resp = await api.get(f"/api/jobs/{job_id}")
    assert resp.status_code == 403
    assert resp.json()["detail"]["error"]["code"] == "FORBIDDEN"


@pytest.mark.asyncio
async def test_query_jobs(api):
    """Test query jobs."""

    await api.post("/api/jobs", json={"title": "QueryJob", "priority": "high"})
    r = await api.get("/api/jobs", params={"priority": "high"})
    assert r.status_code == 200
    assert len(r.json()["data"]) >= 1


@pytest.mark.asyncio
async def test_query_jobs_accepts_iso_due_filters(api):
    """Query jobs should parse ISO due filter params without 500 errors."""

    await api.post(
        "/api/jobs",
        json={
            "title": "Due Filter Job",
            "due_at": "2026-02-18T18:00:00Z",
        },
    )
    r = await api.get(
        "/api/jobs",
        params={"due_before": "2026-12-31T00:00:00Z"},
    )
    assert r.status_code == 200
    assert isinstance(r.json()["data"], list)


@pytest.mark.asyncio
async def test_query_jobs_invalid_due_filter_returns_400(api):
    """Invalid due filter should return INVALID_INPUT."""

    r = await api.get("/api/jobs", params={"due_before": "not-a-date"})
    assert r.status_code == 400
    body = r.json()
    assert body["detail"]["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
async def test_query_jobs_invalid_assignee_returns_400(api):
    """Query jobs should reject malformed assignee ids."""

    r = await api.get("/api/jobs", params={"assigned_to": "not-a-uuid"})
    assert r.status_code == 400
    assert r.json()["detail"]["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
async def test_update_job_status(api):
    """Test update job status."""

    cr = await api.post("/api/jobs", json={"title": "StatusJob"})
    job_id = cr.json()["data"]["id"]

    r = await api.patch(
        f"/api/jobs/{job_id}/status",
        json={
            "status": "completed",
        },
    )
    assert r.status_code == 200


@pytest.mark.asyncio
async def test_update_job_status_invalid_status_returns_400(api):
    """Status updates should reject unknown status names."""

    cr = await api.post("/api/jobs", json={"title": "Bad Status Job"})
    job_id = cr.json()["data"]["id"]

    r = await api.patch(
        f"/api/jobs/{job_id}/status",
        json={"status": "todo"},
    )
    assert r.status_code == 400
    assert r.json()["detail"]["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
async def test_update_job_status_invalid_completed_at_returns_400(api):
    """Status updates should reject invalid completed_at values."""

    cr = await api.post("/api/jobs", json={"title": "Bad CompletedAt"})
    job_id = cr.json()["data"]["id"]

    r = await api.patch(
        f"/api/jobs/{job_id}/status",
        json={"status": "completed", "completed_at": "nope"},
    )
    assert r.status_code == 400
    assert r.json()["detail"]["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
async def test_update_job_status_accepts_iso_completed_at(api):
    """Status updates should accept ISO completed_at values."""

    cr = await api.post("/api/jobs", json={"title": "Status Date Job"})
    job_id = cr.json()["data"]["id"]

    r = await api.patch(
        f"/api/jobs/{job_id}/status",
        json={
            "status": "completed",
            "completed_at": "2026-02-18T18:00:00Z",
        },
    )
    assert r.status_code == 200
    assert r.json()["data"]["completed_at"] is not None


@pytest.mark.asyncio
async def test_update_job_invalid_priority_returns_400(api):
    """Job patch should reject unsupported priorities."""

    cr = await api.post("/api/jobs", json={"title": "Patch Priority"})
    job_id = cr.json()["data"]["id"]
    r = await api.patch(f"/api/jobs/{job_id}", json={"priority": "urgent"})
    assert r.status_code == 400
    assert r.json()["detail"]["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
async def test_update_job_invalid_assigned_to_returns_400(api):
    """Job patch should reject malformed assignee ids."""

    cr = await api.post("/api/jobs", json={"title": "Patch Assignee"})
    job_id = cr.json()["data"]["id"]
    r = await api.patch(f"/api/jobs/{job_id}", json={"assigned_to": "bad-id"})
    assert r.status_code == 400
    assert r.json()["detail"]["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
async def test_update_job_invalid_due_at_returns_400(api):
    """Job patch should reject invalid due_at values."""

    cr = await api.post("/api/jobs", json={"title": "Patch Due"})
    job_id = cr.json()["data"]["id"]
    r = await api.patch(f"/api/jobs/{job_id}", json={"due_at": "bad-date"})
    assert r.status_code == 400
    assert r.json()["detail"]["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
async def test_update_job_due_at_omitted_preserves_existing_value(api):
    """Patching other fields should keep due_at when due_at is omitted."""

    created = await api.post(
        "/api/jobs",
        json={
            "title": "Due Preserve",
            "due_at": "2026-02-18T18:00:00Z",
        },
    )
    assert created.status_code == 200
    job_id = created.json()["data"]["id"]

    patched = await api.patch(f"/api/jobs/{job_id}", json={"title": "Due Preserve Patched"})
    assert patched.status_code == 200
    assert patched.json()["data"]["due_at"] is not None


@pytest.mark.asyncio
async def test_update_job_due_at_null_clears_existing_value(api):
    """Explicit due_at null should clear an existing due date."""

    created = await api.post(
        "/api/jobs",
        json={
            "title": "Due Clear",
            "due_at": "2026-02-18T18:00:00Z",
        },
    )
    assert created.status_code == 200
    job_id = created.json()["data"]["id"]

    patched = await api.patch(f"/api/jobs/{job_id}", json={"due_at": None})
    assert patched.status_code == 200
    assert patched.json()["data"]["due_at"] is None


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "due_at",
    [
        "2026-02-18T18:00:00Z",
        "2026-02-18T18:00:00+00:00",
        "2026-02-18T21:15:00+03:15",
        "2026-02-18",
    ],
)
async def test_update_job_due_at_timezone_matrix(api, due_at):
    """Job patch should accept due_at values across timezone formats."""

    created = await api.post("/api/jobs", json={"title": "Due TZ Matrix"})
    assert created.status_code == 200
    job_id = created.json()["data"]["id"]

    patched = await api.patch(f"/api/jobs/{job_id}", json={"due_at": due_at})
    assert patched.status_code == 200
    assert patched.json()["data"]["due_at"] is not None


@pytest.mark.asyncio
async def test_update_job_due_at_clear_then_set_roundtrip(api):
    """Job patch should allow clear then set transitions for due_at."""

    created = await api.post(
        "/api/jobs",
        json={"title": "Due Roundtrip", "due_at": "2026-02-18T18:00:00Z"},
    )
    assert created.status_code == 200
    job_id = created.json()["data"]["id"]

    cleared = await api.patch(f"/api/jobs/{job_id}", json={"due_at": None})
    assert cleared.status_code == 200
    assert cleared.json()["data"]["due_at"] is None

    reset = await api.patch(
        f"/api/jobs/{job_id}",
        json={"due_at": "2026-02-19T09:00:00+02:00"},
    )
    assert reset.status_code == 200
    assert reset.json()["data"]["due_at"] is not None


@pytest.mark.asyncio
async def test_create_subtask(api):
    """Test create subtask."""

    cr = await api.post("/api/jobs", json={"title": "ParentJob"})
    parent_id = cr.json()["data"]["id"]

    r = await api.post(
        f"/api/jobs/{parent_id}/subtasks",
        json={
            "title": "Child Task",
            "priority": "low",
        },
    )
    assert r.status_code == 200


@pytest.mark.asyncio
async def test_create_subtask_parent_not_found(api):
    """Subtask creation should 404 for missing parent."""

    resp = await api.post(
        "/api/jobs/DOES-NOT-EXIST/subtasks",
        json={"title": "Child"},
    )
    assert resp.status_code == 404


@pytest.mark.asyncio
async def test_create_subtask_invalid_priority_returns_400(api):
    """Subtask creation should reject invalid priority."""

    cr = await api.post("/api/jobs", json={"title": "Parent"})
    parent_id = cr.json()["data"]["id"]
    resp = await api.post(
        f"/api/jobs/{parent_id}/subtasks",
        json={"title": "Child", "priority": "urgent"},
    )
    assert resp.status_code == 400
    assert resp.json()["detail"]["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
async def test_create_subtask_invalid_due_at_returns_400(api):
    """Subtask creation should reject invalid due_at."""

    cr = await api.post("/api/jobs", json={"title": "Parent"})
    parent_id = cr.json()["data"]["id"]
    resp = await api.post(
        f"/api/jobs/{parent_id}/subtasks",
        json={"title": "Child", "due_at": "bad-date"},
    )
    assert resp.status_code == 400
    assert resp.json()["detail"]["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
async def test_agent_cannot_update_job_outside_scopes(api, db_pool, enums):
    """Non-admin agents should be blocked from patching out-of-scope jobs."""

    create = await api.post(
        "/api/jobs",
        json={"title": "Private User Job", "scopes": ["private"]},
    )
    assert create.status_code == 200
    job_id = create.json()["data"]["id"]

    status_id = enums.statuses.name_to_id["active"]
    public_scope = enums.scopes.name_to_id["public"]
    agent = await db_pool.fetchrow(
        """
        INSERT INTO agents (name, description, scopes, requires_approval, status_id)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING *
        """,
        "jobs-scope-guard-agent",
        "scope guard",
        [public_scope],
        False,
        status_id,
    )

    auth_dict = {
        "key_id": None,
        "caller_type": "agent",
        "entity_id": None,
        "entity": None,
        "agent_id": agent["id"],
        "agent": dict(agent),
        "scopes": [public_scope],
    }

    async def mock_auth():
        """Handle mock auth.

        Returns:
            Result value from the operation.
        """

        return auth_dict

    app.dependency_overrides[require_auth] = mock_auth
    app.state.pool = db_pool
    app.state.enums = enums
    transport = ASGITransport(app=app)
    async with AsyncClient(
        transport=transport, base_url="http://test", follow_redirects=True
    ) as client:
        resp = await client.patch(
            f"/api/jobs/{job_id}/status",
            json={"status": "completed"},
        )
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 403
    assert resp.json()["detail"]["error"]["code"] == "FORBIDDEN"


def _agent_auth_override(agent_row: dict, scope_ids: list):
    """Build a dependency override for agent auth."""

    auth_dict = {
        "key_id": None,
        "caller_type": "agent",
        "entity_id": None,
        "entity": None,
        "agent_id": agent_row["id"],
        "agent": agent_row,
        "scopes": scope_ids,
    }

    async def mock_auth():
        """Handle mock auth.

        Returns:
            Result value from the operation.
        """

        return auth_dict

    return mock_auth


@pytest.mark.asyncio
async def test_untrusted_agent_create_job_returns_approval_required(
    db_pool, enums, untrusted_agent_row
):
    """Untrusted agent job creates should queue approvals."""

    app.dependency_overrides[require_auth] = _agent_auth_override(
        {**untrusted_agent_row, "requires_approval": True},
        [enums.scopes.name_to_id["public"]],
    )
    app.state.pool = db_pool
    app.state.enums = enums
    transport = ASGITransport(app=app)
    async with AsyncClient(
        transport=transport, base_url="http://test", follow_redirects=True
    ) as client:
        resp = await client.post("/api/jobs", json={"title": "Queued Job"})
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 202
    assert resp.json()["status"] == "approval_required"


@pytest.mark.asyncio
async def test_untrusted_agent_update_job_status_returns_approval_required(
    db_pool, enums, untrusted_agent_row
):
    """Untrusted agent status updates should queue approvals."""

    in_progress = enums.statuses.name_to_id["in-progress"]
    public_scope = enums.scopes.name_to_id["public"]
    job = await db_pool.fetchrow(
        """
        INSERT INTO jobs (title, agent_id, status_id, priority, privacy_scope_ids)
        VALUES ($1, $2::uuid, $3::uuid, $4, $5::uuid[])
        RETURNING id
        """,
        "Status Approval Job",
        untrusted_agent_row["id"],
        in_progress,
        "medium",
        [public_scope],
    )
    job_id = job["id"]

    app.dependency_overrides[require_auth] = _agent_auth_override(
        {**untrusted_agent_row, "requires_approval": True},
        [enums.scopes.name_to_id["public"]],
    )
    app.state.pool = db_pool
    app.state.enums = enums
    transport = ASGITransport(app=app)
    async with AsyncClient(
        transport=transport, base_url="http://test", follow_redirects=True
    ) as client:
        resp = await client.patch(
            f"/api/jobs/{job_id}/status",
            json={"status": "completed"},
        )
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 202
    assert resp.json()["status"] == "approval_required"


@pytest.mark.asyncio
async def test_untrusted_agent_update_job_returns_approval_required(
    db_pool, enums, untrusted_agent_row
):
    """Untrusted agent job patches should queue approvals."""

    in_progress = enums.statuses.name_to_id["in-progress"]
    public_scope = enums.scopes.name_to_id["public"]
    job = await db_pool.fetchrow(
        """
        INSERT INTO jobs (title, agent_id, status_id, priority, privacy_scope_ids)
        VALUES ($1, $2::uuid, $3::uuid, $4, $5::uuid[])
        RETURNING id
        """,
        "Patch Approval Job",
        untrusted_agent_row["id"],
        in_progress,
        "medium",
        [public_scope],
    )
    job_id = job["id"]

    app.dependency_overrides[require_auth] = _agent_auth_override(
        {**untrusted_agent_row, "requires_approval": True},
        [enums.scopes.name_to_id["public"]],
    )
    app.state.pool = db_pool
    app.state.enums = enums
    transport = ASGITransport(app=app)
    async with AsyncClient(
        transport=transport, base_url="http://test", follow_redirects=True
    ) as client:
        resp = await client.patch(
            f"/api/jobs/{job_id}",
            json={"description": "queued update"},
        )
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 202
    assert resp.json()["status"] == "approval_required"


@pytest.mark.asyncio
async def test_untrusted_agent_create_subtask_returns_approval_required(
    db_pool, enums, untrusted_agent_row
):
    """Untrusted agent subtask creates should queue approvals."""

    in_progress = enums.statuses.name_to_id["in-progress"]
    public_scope = enums.scopes.name_to_id["public"]
    parent = await db_pool.fetchrow(
        """
        INSERT INTO jobs (title, agent_id, status_id, priority, privacy_scope_ids)
        VALUES ($1, $2::uuid, $3::uuid, $4, $5::uuid[])
        RETURNING id
        """,
        "Parent Queue Job",
        untrusted_agent_row["id"],
        in_progress,
        "medium",
        [public_scope],
    )
    parent_id = parent["id"]

    app.dependency_overrides[require_auth] = _agent_auth_override(
        {**untrusted_agent_row, "requires_approval": True},
        [enums.scopes.name_to_id["public"]],
    )
    app.state.pool = db_pool
    app.state.enums = enums
    transport = ASGITransport(app=app)
    async with AsyncClient(
        transport=transport, base_url="http://test", follow_redirects=True
    ) as client:
        resp = await client.post(
            f"/api/jobs/{parent_id}/subtasks",
            json={"title": "Queued Child"},
        )
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 202
    assert resp.json()["status"] == "approval_required"


@pytest.mark.asyncio
async def test_admin_agent_can_update_foreign_job(api, db_pool, enums):
    """Admin-scoped agents should bypass job owner guard."""

    create = await api.post("/api/jobs", json={"title": "Foreign Job"})
    job_id = create.json()["data"]["id"]
    status_id = enums.statuses.name_to_id["active"]
    admin_scope = enums.scopes.name_to_id.get("admin")
    if not admin_scope:
        pytest.skip("admin scope unavailable in this taxonomy")
    admin_agent = await db_pool.fetchrow(
        """
        INSERT INTO agents (name, description, scopes, requires_approval, status_id)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING *
        """,
        "jobs-admin-agent",
        "admin",
        [admin_scope],
        False,
        status_id,
    )

    app.dependency_overrides[require_auth] = _agent_auth_override(
        dict(admin_agent),
        [admin_scope],
    )
    app.state.pool = db_pool
    app.state.enums = enums
    transport = ASGITransport(app=app)
    async with AsyncClient(
        transport=transport, base_url="http://test", follow_redirects=True
    ) as client:
        resp = await client.patch(
            f"/api/jobs/{job_id}/status",
            json={"status": "completed"},
        )
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 200
