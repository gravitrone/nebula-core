"""Red team tests for job access isolation."""

# Standard Library
import json
from unittest.mock import MagicMock

# Third-Party
import pytest

# Local
from nebula_mcp.models import (
    CreateJobInput,
    CreateSubtaskInput,
    GetJobInput,
    QueryJobsInput,
    UpdateJobStatusInput,
)
from nebula_mcp.server import (
    create_job,
    create_subtask,
    get_job,
    query_jobs,
    update_job_status,
)


def _make_context(pool, enums, agent):
    """Build MCP context with a specific agent."""

    ctx = MagicMock()
    ctx.request_context.lifespan_context = {
        "pool": pool,
        "enums": enums,
        "agent": agent,
    }
    return ctx


async def _make_agent(db_pool, enums, name, scopes, requires_approval):
    """Insert an agent for job access tests."""

    status_id = enums.statuses.name_to_id["active"]
    scope_ids = [enums.scopes.name_to_id[s] for s in scopes]

    row = await db_pool.fetchrow(
        """
        INSERT INTO agents (name, description, scopes, requires_approval, status_id)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING *
        """,
        name,
        "redteam agent",
        scope_ids,
        requires_approval,
        status_id,
    )
    return dict(row)


async def _make_job(db_pool, enums, agent_id):
    """Insert a job for job access tests."""

    status_id = enums.statuses.name_to_id["active"]
    scope_ids = [enums.scopes.name_to_id["public"]]

    row = await db_pool.fetchrow(
        """
        INSERT INTO jobs (title, status_id, agent_id, metadata, privacy_scope_ids)
        VALUES ($1, $2, $3, $4::jsonb, $5)
        RETURNING *
        """,
        "Private Job",
        status_id,
        agent_id,
        json.dumps({"secret": "job"}),
        scope_ids,
    )
    return dict(row)


@pytest.mark.asyncio
async def test_get_job_allows_other_agent_in_scope(db_pool, enums):
    """Job read should be allowed for agents with matching scopes."""

    owner = await _make_agent(db_pool, enums, "job-owner", ["public"], False)
    other = await _make_agent(db_pool, enums, "job-viewer", ["public"], False)
    job = await _make_job(db_pool, enums, owner["id"])

    ctx = _make_context(db_pool, enums, other)
    payload = GetJobInput(job_id=job["id"])

    result = await get_job(payload, ctx)
    assert result["id"] == job["id"]


@pytest.mark.asyncio
async def test_query_jobs_includes_other_agents_jobs_in_scope(db_pool, enums):
    """Job list should include jobs from other agents when scopes allow."""

    owner = await _make_agent(db_pool, enums, "job-owner-2", ["public"], False)
    other = await _make_agent(db_pool, enums, "job-viewer-2", ["public"], False)
    job = await _make_job(db_pool, enums, owner["id"])

    ctx = _make_context(db_pool, enums, other)
    payload = QueryJobsInput()
    rows = await query_jobs(payload, ctx)

    ids = {row["id"] for row in rows}
    assert job["id"] in ids


@pytest.mark.asyncio
async def test_get_job_denies_agent_outside_scopes(db_pool, enums):
    """Job read should be denied when scopes do not overlap."""

    owner = await _make_agent(db_pool, enums, "job-owner-scopes", ["personal"], False)
    other = await _make_agent(db_pool, enums, "job-viewer-scopes", ["public"], False)

    status_id = enums.statuses.name_to_id["active"]
    private_scope_ids = [enums.scopes.name_to_id["personal"]]
    job = await db_pool.fetchrow(
        """
        INSERT INTO jobs (title, status_id, agent_id, metadata, privacy_scope_ids)
        VALUES ($1, $2, $3, $4::jsonb, $5)
        RETURNING *
        """,
        "Scoped Job",
        status_id,
        owner["id"],
        json.dumps({"secret": "job"}),
        private_scope_ids,
    )

    ctx = _make_context(db_pool, enums, other)
    payload = GetJobInput(job_id=job["id"])

    with pytest.raises(ValueError):
        await get_job(payload, ctx)


@pytest.mark.asyncio
async def test_update_job_status_denies_other_agent(db_pool, enums):
    """Job status updates should require ownership."""

    owner = await _make_agent(db_pool, enums, "job-owner-3", ["public"], False)
    other = await _make_agent(db_pool, enums, "job-viewer-3", ["public"], False)
    job = await _make_job(db_pool, enums, owner["id"])

    ctx = _make_context(db_pool, enums, other)
    payload = UpdateJobStatusInput(job_id=job["id"], status="completed")

    with pytest.raises(ValueError):
        await update_job_status(payload, ctx)


@pytest.mark.asyncio
async def test_create_job_denies_agent_spoofing(db_pool, enums):
    """Agents should not create jobs on behalf of other agents."""

    owner = await _make_agent(db_pool, enums, "job-owner-4", ["public"], False)
    other = await _make_agent(db_pool, enums, "job-viewer-4", ["public"], False)

    ctx = _make_context(db_pool, enums, other)
    payload = CreateJobInput(
        title="Injected Job",
        status="active",
        priority="medium",
        agent_id=str(owner["id"]),
    )
    result = await create_job(payload, ctx)

    if result.get("status") == "approval_required":
        return

    assert str(result["agent_id"]) == str(other["id"])


@pytest.mark.asyncio
async def test_create_subtask_denies_foreign_job(db_pool, enums):
    """Agents should not create subtasks for jobs they do not own."""

    owner = await _make_agent(db_pool, enums, "job-owner-5", ["public"], False)
    other = await _make_agent(db_pool, enums, "job-viewer-5", ["public"], False)
    job = await _make_job(db_pool, enums, owner["id"])

    ctx = _make_context(db_pool, enums, other)
    payload = CreateSubtaskInput(parent_job_id=str(job["id"]), title="Injected Subtask")

    with pytest.raises(ValueError):
        await create_subtask(payload, ctx)


@pytest.mark.asyncio
async def test_create_job_handles_uuid_agent_id(db_pool, enums):
    """Agent job creation should not crash on UUID agent_id."""

    agent = await _make_agent(db_pool, enums, "job-creator", ["public"], False)
    ctx = _make_context(db_pool, enums, agent)
    payload = CreateJobInput(
        title="UUID Agent Job",
        priority="medium",
    )
    result = await create_job(payload, ctx)

    if result.get("status") == "approval_required":
        return

    assert str(result.get("agent_id")) == str(agent["id"])
