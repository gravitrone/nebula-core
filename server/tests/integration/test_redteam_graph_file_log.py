"""Red team tests for graph traversal privacy with file and log nodes."""

# Standard Library
import json
from datetime import UTC, datetime
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
    """Insert an agent for graph privacy tests."""

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
    """Insert an entity for graph privacy tests."""

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


async def _make_file(db_pool, enums):
    """Insert a file node for graph privacy tests."""

    status_id = enums.statuses.name_to_id["active"]

    row = await db_pool.fetchrow(
        """
        INSERT INTO files (filename, file_path, status_id, metadata)
        VALUES ($1, $2, $3, $4::jsonb)
        RETURNING *
        """,
        "graph-secret.txt",
        "/vault/graph-secret.txt",
        status_id,
        json.dumps({"note": "secret"}),
    )
    return dict(row)


async def _make_log(db_pool, enums):
    """Insert a log node for graph privacy tests."""

    status_id = enums.statuses.name_to_id["active"]
    log_type_id = enums.log_types.name_to_id["note"]

    row = await db_pool.fetchrow(
        """
        INSERT INTO logs (log_type_id, timestamp, value, status_id, metadata)
        VALUES ($1, $2, $3::jsonb, $4, $5::jsonb)
        RETURNING *
        """,
        log_type_id,
        datetime.now(UTC),
        json.dumps({"note": "secret"}),
        status_id,
        json.dumps({"class": "sensitive"}),
    )
    return dict(row)


async def _attach_relationship(
    db_pool, enums, source_type, source_id, target_type, target_id, rel_name
):
    """Attach relationship between graph nodes."""

    status_id = enums.statuses.name_to_id["active"]
    rel_type_id = enums.relationship_types.name_to_id[rel_name]

    await db_pool.execute(
        """
        INSERT INTO relationships (source_type, source_id, target_type, target_id, type_id, status_id, properties)
        VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb)
        """,
        source_type,
        source_id,
        target_type,
        target_id,
        rel_type_id,
        status_id,
        json.dumps({"note": "graph link"}),
    )


@pytest.mark.asyncio
async def test_graph_neighbors_hides_hidden_file(db_pool, enums):
    """Graph neighbors should not expose files with hidden relationships."""

    public_entity = await _make_entity(db_pool, enums, "Public Node", ["public"])
    private_entity = await _make_entity(db_pool, enums, "Private Node", ["sensitive"])
    file_row = await _make_file(db_pool, enums)

    await _attach_relationship(
        db_pool,
        enums,
        "file",
        str(file_row["id"]),
        "entity",
        str(public_entity["id"]),
        "has-file",
    )
    await _attach_relationship(
        db_pool,
        enums,
        "file",
        str(file_row["id"]),
        "entity",
        str(private_entity["id"]),
        "has-file",
    )

    viewer = await _make_agent(db_pool, enums, "graph-file-viewer", ["public"])
    ctx = _make_context(db_pool, enums, viewer)

    payload = GraphNeighborsInput(
        source_type="entity",
        source_id=str(public_entity["id"]),
        max_hops=2,
        limit=10,
    )
    results = await graph_neighbors(payload, ctx)
    ids = {row["node_id"] for row in results}

    assert str(file_row["id"]) not in ids


@pytest.mark.asyncio
async def test_graph_neighbors_hides_hidden_log(db_pool, enums):
    """Graph neighbors should not expose logs with hidden relationships."""

    public_entity = await _make_entity(db_pool, enums, "Public Node", ["public"])
    private_entity = await _make_entity(db_pool, enums, "Private Node", ["sensitive"])
    log_row = await _make_log(db_pool, enums)

    await _attach_relationship(
        db_pool,
        enums,
        "log",
        str(log_row["id"]),
        "entity",
        str(public_entity["id"]),
        "related-to",
    )
    await _attach_relationship(
        db_pool,
        enums,
        "log",
        str(log_row["id"]),
        "entity",
        str(private_entity["id"]),
        "related-to",
    )

    viewer = await _make_agent(db_pool, enums, "graph-log-viewer", ["public"])
    ctx = _make_context(db_pool, enums, viewer)

    payload = GraphNeighborsInput(
        source_type="entity",
        source_id=str(public_entity["id"]),
        max_hops=2,
        limit=10,
    )
    results = await graph_neighbors(payload, ctx)
    ids = {row["node_id"] for row in results}

    assert str(log_row["id"]) not in ids


@pytest.mark.asyncio
async def test_graph_shortest_path_hides_hidden_file(db_pool, enums):
    """Shortest path should not expose files with hidden relationships."""

    public_entity = await _make_entity(db_pool, enums, "Public Node", ["public"])
    private_entity = await _make_entity(db_pool, enums, "Private Node", ["sensitive"])
    file_row = await _make_file(db_pool, enums)

    await _attach_relationship(
        db_pool,
        enums,
        "file",
        str(file_row["id"]),
        "entity",
        str(public_entity["id"]),
        "has-file",
    )
    await _attach_relationship(
        db_pool,
        enums,
        "file",
        str(file_row["id"]),
        "entity",
        str(private_entity["id"]),
        "has-file",
    )

    viewer = await _make_agent(db_pool, enums, "path-file-viewer", ["public"])
    ctx = _make_context(db_pool, enums, viewer)

    payload = GraphShortestPathInput(
        source_type="entity",
        source_id=str(public_entity["id"]),
        target_type="file",
        target_id=str(file_row["id"]),
        max_hops=3,
    )

    with pytest.raises(ValueError):
        await graph_shortest_path(payload, ctx)


@pytest.mark.asyncio
async def test_graph_shortest_path_hides_hidden_log(db_pool, enums):
    """Shortest path should not expose logs with hidden relationships."""

    public_entity = await _make_entity(db_pool, enums, "Public Node 2", ["public"])
    private_entity = await _make_entity(db_pool, enums, "Private Node 2", ["sensitive"])
    log_row = await _make_log(db_pool, enums)

    await _attach_relationship(
        db_pool,
        enums,
        "log",
        str(log_row["id"]),
        "entity",
        str(public_entity["id"]),
        "related-to",
    )
    await _attach_relationship(
        db_pool,
        enums,
        "log",
        str(log_row["id"]),
        "entity",
        str(private_entity["id"]),
        "related-to",
    )

    viewer = await _make_agent(db_pool, enums, "path-log-viewer", ["public"])
    ctx = _make_context(db_pool, enums, viewer)

    payload = GraphShortestPathInput(
        source_type="entity",
        source_id=str(public_entity["id"]),
        target_type="log",
        target_id=str(log_row["id"]),
        max_hops=3,
    )

    with pytest.raises(ValueError):
        await graph_shortest_path(payload, ctx)
