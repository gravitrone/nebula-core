"""Red team tests for graph traversal privacy leaks."""

# Standard Library
import json

# Third-Party
import pytest

# Local
from nebula_mcp.models import GraphNeighborsInput, GraphShortestPathInput
from nebula_mcp.server import graph_neighbors, graph_shortest_path


@pytest.mark.asyncio
async def test_graph_neighbors_hides_private_nodes(
    db_pool, enums, test_entity, untrusted_mcp_context
):
    """Graph traversal should not expose nodes outside agent scopes."""

    status_id = enums.statuses.name_to_id["active"]
    type_id = enums.entity_types.name_to_id["person"]
    private_scope_id = enums.scopes.name_to_id["sensitive"]

    private_entity = await db_pool.fetchrow(
        """
        INSERT INTO entities (name, type_id, status_id, privacy_scope_ids, tags)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING *
        """,
        "Private Node",
        type_id,
        status_id,
        [private_scope_id],
        ["private"],
    )

    relationship_type_id = enums.relationship_types.name_to_id["related-to"]

    await db_pool.execute(
        """
        INSERT INTO relationships (source_type, source_id, target_type, target_id, type_id, status_id, properties)
        VALUES ('entity', $1, 'entity', $2, $3, $4, $5::jsonb)
        """,
        str(test_entity["id"]),
        str(private_entity["id"]),
        relationship_type_id,
        status_id,
        json.dumps({"note": "secret link"}),
    )

    payload = GraphNeighborsInput(
        source_type="entity",
        source_id=str(test_entity["id"]),
        max_hops=1,
        limit=10,
    )
    results = await graph_neighbors(payload, untrusted_mcp_context)
    leaked_ids = {row["node_id"] for row in results}

    assert str(private_entity["id"]) not in leaked_ids


@pytest.mark.asyncio
async def test_graph_neighbors_hides_private_context(
    db_pool, enums, test_entity, untrusted_mcp_context
):
    """Graph traversal should not expose context outside agent scopes."""

    status_id = enums.statuses.name_to_id["active"]
    private_scope_id = enums.scopes.name_to_id["sensitive"]

    context = await db_pool.fetchrow(
        """
        INSERT INTO context_items (title, source_type, content, privacy_scope_ids, status_id, tags)
        VALUES ($1, $2, $3, $4, $5, $6)
        RETURNING *
        """,
        "Private Context",
        "note",
        "secret",
        [private_scope_id],
        status_id,
        ["private"],
    )

    relationship_type_id = enums.relationship_types.name_to_id["related-to"]

    await db_pool.execute(
        """
        INSERT INTO relationships (source_type, source_id, target_type, target_id, type_id, status_id, properties)
        VALUES ('entity', $1, 'context', $2, $3, $4, $5::jsonb)
        """,
        str(test_entity["id"]),
        str(context["id"]),
        relationship_type_id,
        status_id,
        json.dumps({"note": "secret context"}),
    )

    payload = GraphNeighborsInput(
        source_type="entity",
        source_id=str(test_entity["id"]),
        max_hops=1,
        limit=10,
    )
    results = await graph_neighbors(payload, untrusted_mcp_context)
    leaked_ids = {row["node_id"] for row in results}

    assert str(context["id"]) not in leaked_ids


@pytest.mark.asyncio
async def test_graph_shortest_path_hides_private_entity(
    db_pool, enums, test_entity, untrusted_mcp_context
):
    """Shortest path should not expose private entity nodes."""

    status_id = enums.statuses.name_to_id["active"]
    type_id = enums.entity_types.name_to_id["person"]
    private_scope_id = enums.scopes.name_to_id["sensitive"]

    private_entity = await db_pool.fetchrow(
        """
        INSERT INTO entities (name, type_id, status_id, privacy_scope_ids, tags)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING *
        """,
        "Private Path Node",
        type_id,
        status_id,
        [private_scope_id],
        ["private"],
    )

    relationship_type_id = enums.relationship_types.name_to_id["related-to"]

    await db_pool.execute(
        """
        INSERT INTO relationships (source_type, source_id, target_type, target_id, type_id, status_id, properties)
        VALUES ('entity', $1, 'entity', $2, $3, $4, $5::jsonb)
        """,
        str(test_entity["id"]),
        str(private_entity["id"]),
        relationship_type_id,
        status_id,
        json.dumps({"note": "secret link"}),
    )

    payload = GraphShortestPathInput(
        source_type="entity",
        source_id=str(test_entity["id"]),
        target_type="entity",
        target_id=str(private_entity["id"]),
        max_hops=2,
    )

    with pytest.raises(ValueError):
        await graph_shortest_path(payload, untrusted_mcp_context)
