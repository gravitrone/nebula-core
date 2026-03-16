"""Graph cycle stress tests for traversal queries."""

# Standard Library
import asyncio
from pathlib import Path

# Third-Party
import pytest

# Local
from nebula_mcp.query_loader import QueryLoader

QUERIES = QueryLoader(Path(__file__).resolve().parents[2] / "src" / "queries")


@pytest.mark.asyncio
async def test_graph_cycle_neighbors(db_pool, enums):
    """Ensure graph neighbors query handles cycles without hanging."""

    status_id = enums.statuses.name_to_id["active"]
    type_id = enums.entity_types.name_to_id["person"]
    scope_ids = [enums.scopes.name_to_id["public"]]
    rel_type_id = next(iter(enums.relationship_types.name_to_id.values()))

    a = await db_pool.fetchval(
        "INSERT INTO entities (name, type_id, status_id, privacy_scope_ids, tags)"
        " VALUES ($1, $2, $3, $4, $5) RETURNING id",
        "node-a",
        type_id,
        status_id,
        scope_ids,
        [],
    )
    b = await db_pool.fetchval(
        "INSERT INTO entities (name, type_id, status_id, privacy_scope_ids, tags)"
        " VALUES ($1, $2, $3, $4, $5) RETURNING id",
        "node-b",
        type_id,
        status_id,
        scope_ids,
        [],
    )
    c = await db_pool.fetchval(
        "INSERT INTO entities (name, type_id, status_id, privacy_scope_ids, tags)"
        " VALUES ($1, $2, $3, $4, $5) RETURNING id",
        "node-c",
        type_id,
        status_id,
        scope_ids,
        [],
    )

    await db_pool.execute(
        "INSERT INTO relationships (source_type, source_id, target_type, target_id, type_id, status_id, properties)"
        " VALUES ('entity', $1, 'entity', $2, $3, $4, '{}'::jsonb)",
        str(a),
        str(b),
        rel_type_id,
        status_id,
    )
    await db_pool.execute(
        "INSERT INTO relationships (source_type, source_id, target_type, target_id, type_id, status_id, properties)"
        " VALUES ('entity', $1, 'entity', $2, $3, $4, '{}'::jsonb)",
        str(b),
        str(c),
        rel_type_id,
        status_id,
    )
    await db_pool.execute(
        "INSERT INTO relationships (source_type, source_id, target_type, target_id, type_id, status_id, properties)"
        " VALUES ('entity', $1, 'entity', $2, $3, $4, '{}'::jsonb)",
        str(c),
        str(a),
        rel_type_id,
        status_id,
    )

    async def run_neighbors():
        """Execute neighbors query for cycle check."""

        return await db_pool.fetch(
            QUERIES["graph/neighbors"],
            "entity",
            str(a),
            3,
            25,
        )

    rows = await asyncio.wait_for(run_neighbors(), timeout=3)
    assert len(rows) >= 2


@pytest.mark.asyncio
async def test_graph_shortest_path_cycle(db_pool, enums):
    """Ensure shortest path query handles cycles correctly."""

    status_id = enums.statuses.name_to_id["active"]
    type_id = enums.entity_types.name_to_id["person"]
    scope_ids = [enums.scopes.name_to_id["public"]]
    rel_type_id = next(iter(enums.relationship_types.name_to_id.values()))

    n1 = await db_pool.fetchval(
        "INSERT INTO entities (name, type_id, status_id, privacy_scope_ids, tags)"
        " VALUES ($1, $2, $3, $4, $5) RETURNING id",
        "path-1",
        type_id,
        status_id,
        scope_ids,
        [],
    )
    n2 = await db_pool.fetchval(
        "INSERT INTO entities (name, type_id, status_id, privacy_scope_ids, tags)"
        " VALUES ($1, $2, $3, $4, $5) RETURNING id",
        "path-2",
        type_id,
        status_id,
        scope_ids,
        [],
    )
    n3 = await db_pool.fetchval(
        "INSERT INTO entities (name, type_id, status_id, privacy_scope_ids, tags)"
        " VALUES ($1, $2, $3, $4, $5) RETURNING id",
        "path-3",
        type_id,
        status_id,
        scope_ids,
        [],
    )

    await db_pool.execute(
        "INSERT INTO relationships (source_type, source_id, target_type, target_id, type_id, status_id, properties)"
        " VALUES ('entity', $1, 'entity', $2, $3, $4, '{}'::jsonb)",
        str(n1),
        str(n2),
        rel_type_id,
        status_id,
    )
    await db_pool.execute(
        "INSERT INTO relationships (source_type, source_id, target_type, target_id, type_id, status_id, properties)"
        " VALUES ('entity', $1, 'entity', $2, $3, $4, '{}'::jsonb)",
        str(n2),
        str(n3),
        rel_type_id,
        status_id,
    )
    await db_pool.execute(
        "INSERT INTO relationships (source_type, source_id, target_type, target_id, type_id, status_id, properties)"
        " VALUES ('entity', $1, 'entity', $2, $3, $4, '{}'::jsonb)",
        str(n3),
        str(n1),
        rel_type_id,
        status_id,
    )

    async def run_path():
        """Execute shortest path query for cycle check."""

        return await db_pool.fetchrow(
            QUERIES["graph/shortest_path"],
            "entity",
            str(n1),
            "entity",
            str(n3),
            6,
        )

    row = await asyncio.wait_for(run_path(), timeout=3)
    assert row is not None
    assert row["depth"] >= 1
