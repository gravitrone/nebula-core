"""Red team tests for graph traversal with job ownership."""

# Standard Library
import json
from unittest.mock import MagicMock

# Third-Party
import pytest

# Local
from nebula_mcp.models import GraphNeighborsInput, GraphShortestPathInput
from nebula_mcp.server import graph_neighbors, graph_shortest_path


def _make_context(pool, enums, agent):
    """Build MCP context with a specific agent."""

    ctx = MagicMock()
    ctx.request_context.lifespan_context = {
        "pool": pool,
        "enums": enums,
        "agent": agent,
    }
    return ctx


async def _make_agent(db_pool, enums, name, scopes):
    """Insert an agent for graph job tests."""

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
    """Insert an entity for graph job tests."""

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
    """Insert a job for graph job tests."""

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


async def _make_relationship(
    db_pool, enums, source_type, source_id, target_type, target_id
):
    """Insert a relationship for graph job tests."""

    status_id = enums.statuses.name_to_id["active"]
    type_id = enums.relationship_types.name_to_id["related-to"]

    await db_pool.execute(
        """
        INSERT INTO relationships (source_type, source_id, target_type, target_id, type_id, status_id, properties)
        VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb)
        """,
        source_type,
        source_id,
        target_type,
        target_id,
        type_id,
        status_id,
        json.dumps({"note": "job-link"}),
    )


@pytest.mark.asyncio
async def test_graph_neighbors_hides_out_of_scope_jobs(db_pool, enums):
    """Graph traversal should not expose jobs outside the agent scopes."""

    owner = await _make_agent(db_pool, enums, "job-owner", ["public"])
    viewer = await _make_agent(db_pool, enums, "job-viewer", ["public"])
    entity = await _make_entity(db_pool, enums, "Public Node", ["public"])
    job = await _make_job(db_pool, enums, "Private Job", owner["id"], ["private"])
    await _make_relationship(
        db_pool, enums, "entity", str(entity["id"]), "job", job["id"]
    )

    ctx = _make_context(db_pool, enums, viewer)
    payload = GraphNeighborsInput(
        source_type="entity",
        source_id=str(entity["id"]),
        max_hops=2,
        limit=10,
    )
    results = await graph_neighbors(payload, ctx)
    ids = {row["node_id"] for row in results}

    assert job["id"] not in ids


@pytest.mark.asyncio
async def test_graph_shortest_path_hides_out_of_scope_jobs(db_pool, enums):
    """Shortest path should deny access to job nodes outside the agent scopes."""

    owner = await _make_agent(db_pool, enums, "path-owner", ["public"])
    viewer = await _make_agent(db_pool, enums, "path-viewer", ["public"])
    entity = await _make_entity(db_pool, enums, "Path Node", ["public"])
    job = await _make_job(db_pool, enums, "Path Job", owner["id"], ["private"])
    await _make_relationship(
        db_pool, enums, "entity", str(entity["id"]), "job", job["id"]
    )

    ctx = _make_context(db_pool, enums, viewer)
    payload = GraphShortestPathInput(
        source_type="entity",
        source_id=str(entity["id"]),
        target_type="job",
        target_id=job["id"],
        max_hops=3,
    )

    with pytest.raises(ValueError):
        await graph_shortest_path(payload, ctx)


@pytest.mark.asyncio
async def test_graph_neighbors_accepts_job_id_format(db_pool, enums):
    """Graph neighbors should accept canonical job IDs as source IDs."""

    owner = await _make_agent(db_pool, enums, "job-owner-visible", ["public"])
    viewer = await _make_agent(db_pool, enums, "job-viewer-visible", ["public"])
    entity = await _make_entity(db_pool, enums, "Visible Node", ["public"])
    job = await _make_job(db_pool, enums, "Visible Job", owner["id"], ["public"])
    await _make_relationship(
        db_pool, enums, "job", job["id"], "entity", str(entity["id"])
    )

    ctx = _make_context(db_pool, enums, viewer)
    payload = GraphNeighborsInput(
        source_type="job",
        source_id=job["id"],
        max_hops=2,
        limit=10,
    )

    results = await graph_neighbors(payload, ctx)
    ids = {row["node_id"] for row in results}
    assert str(entity["id"]) in ids


@pytest.mark.asyncio
async def test_graph_shortest_path_accepts_job_id_format(db_pool, enums):
    """Shortest path should accept canonical job IDs as target IDs."""

    owner = await _make_agent(db_pool, enums, "job-owner-path-visible", ["public"])
    viewer = await _make_agent(db_pool, enums, "job-viewer-path-visible", ["public"])
    entity = await _make_entity(db_pool, enums, "Path Start", ["public"])
    job = await _make_job(db_pool, enums, "Path End Job", owner["id"], ["public"])
    await _make_relationship(
        db_pool, enums, "entity", str(entity["id"]), "job", job["id"]
    )

    ctx = _make_context(db_pool, enums, viewer)
    payload = GraphShortestPathInput(
        source_type="entity",
        source_id=str(entity["id"]),
        target_type="job",
        target_id=job["id"],
        max_hops=3,
    )

    result = await graph_shortest_path(payload, ctx)
    assert result["depth"] >= 1
