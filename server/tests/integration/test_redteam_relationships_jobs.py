"""Red team tests for relationship access involving jobs."""

# Standard Library
import json
from unittest.mock import MagicMock

# Third-Party
import pytest

# Local
from nebula_mcp.models import (
    CreateRelationshipInput,
    GetRelationshipsInput,
    QueryRelationshipsInput,
)
from nebula_mcp.server import (
    create_relationship,
    get_relationships,
    query_relationships,
)


def _make_context(pool, enums, agent):
    """Build a mock MCP context for relationship job tests."""

    ctx = MagicMock()
    ctx.request_context.lifespan_context = {
        "pool": pool,
        "enums": enums,
        "agent": agent,
    }
    return ctx


async def _make_agent(db_pool, enums, name, scopes):
    """Insert a test agent for relationship job scenarios."""

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
        False,
        status_id,
    )
    return dict(row)


async def _make_entity(db_pool, enums, name, scopes):
    """Insert a test entity for relationship job scenarios."""

    status_id = enums.statuses.name_to_id["active"]
    type_id = enums.entity_types.name_to_id["person"]
    scope_ids = [enums.scopes.name_to_id[s] for s in scopes]

    row = await db_pool.fetchrow(
        """
        INSERT INTO entities (name, type_id, status_id, privacy_scope_ids, tags)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING *
        """,
        name,
        type_id,
        status_id,
        scope_ids,
        ["test"],
    )
    return dict(row)


async def _make_job(db_pool, enums, title, agent_id, scopes):
    """Insert a test job for relationship job scenarios."""

    status_id = enums.statuses.name_to_id["active"]
    scope_ids = [enums.scopes.name_to_id[s] for s in scopes]

    row = await db_pool.fetchrow(
        """
        INSERT INTO jobs (title, status_id, agent_id, privacy_scope_ids)
        VALUES ($1, $2, $3, $4)
        RETURNING *
        """,
        title,
        status_id,
        agent_id,
        scope_ids,
    )
    return dict(row)


async def _make_relationship(db_pool, enums, source_type, source_id, target_type, target_id):
    """Insert a relationship linking entities/jobs for access checks."""

    status_id = enums.statuses.name_to_id["active"]
    type_id = enums.relationship_types.name_to_id["related-to"]

    row = await db_pool.fetchrow(
        """
        INSERT INTO relationships (source_type, source_id, target_type, target_id, type_id, status_id, properties)
        VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb)
        RETURNING *
        """,
        source_type,
        source_id,
        target_type,
        target_id,
        type_id,
        status_id,
        json.dumps({"note": "job-link"}),
    )
    return dict(row)


@pytest.mark.asyncio
async def test_get_relationships_hides_out_of_scope_job_links(db_pool, enums):
    """Agents should not see job relationships for jobs outside their scopes."""

    owner = await _make_agent(db_pool, enums, "rel-owner", ["public"])
    viewer = await _make_agent(db_pool, enums, "rel-viewer", ["public"])
    entity = await _make_entity(db_pool, enums, "Public Node", ["public"])
    job = await _make_job(db_pool, enums, "Private Job", owner["id"], ["private"])
    rel = await _make_relationship(db_pool, enums, "entity", str(entity["id"]), "job", job["id"])

    ctx = _make_context(db_pool, enums, viewer)
    payload = GetRelationshipsInput(
        source_type="entity",
        source_id=str(entity["id"]),
        direction="both",
        relationship_type=None,
    )
    results = await get_relationships(payload, ctx)
    ids = {row["id"] for row in results}

    assert rel["id"] not in ids


@pytest.mark.asyncio
async def test_query_relationships_hides_out_of_scope_job_links(db_pool, enums):
    """Querying relationships should not expose jobs outside the agent scopes."""

    owner = await _make_agent(db_pool, enums, "rel-owner-2", ["public"])
    viewer = await _make_agent(db_pool, enums, "rel-viewer-2", ["public"])
    entity = await _make_entity(db_pool, enums, "Public Node 2", ["public"])
    job = await _make_job(db_pool, enums, "Private Job 2", owner["id"], ["private"])
    rel = await _make_relationship(db_pool, enums, "job", job["id"], "entity", str(entity["id"]))

    ctx = _make_context(db_pool, enums, viewer)
    payload = QueryRelationshipsInput(
        source_type=None,
        target_type=None,
        relationship_types=[],
        status_category="active",
        limit=50,
    )
    results = await query_relationships(payload, ctx)
    ids = {row["id"] for row in results}

    assert rel["id"] not in ids


@pytest.mark.asyncio
async def test_create_relationship_accepts_job_id_format(db_pool, enums):
    """Relationship create should accept canonical job IDs for job nodes."""

    owner = await _make_agent(db_pool, enums, "rel-owner-3", ["public"])
    entity = await _make_entity(db_pool, enums, "Public Node 3", ["public"])
    job = await _make_job(db_pool, enums, "Public Job 3", owner["id"], ["public"])

    ctx = _make_context(db_pool, enums, owner)
    payload = CreateRelationshipInput(
        source_type="entity",
        source_id=str(entity["id"]),
        target_type="job",
        target_id=job["id"],
        relationship_type="related-to",
        properties={},
    )
    result = await create_relationship(payload, ctx)
    assert result["target_type"] == "job"
    assert result["target_id"] == job["id"]
