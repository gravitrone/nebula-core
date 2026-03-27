"""Red team concurrency and integrity tests."""

# Standard Library
import asyncio

# Third-Party
import pytest

# Local
from nebula_mcp.models import CreateEntityInput, CreateRelationshipInput, UpdateEntityInput
from nebula_mcp.server import create_entity, create_relationship, update_entity


@pytest.mark.asyncio
async def test_concurrent_create_entity_duplicate(mock_mcp_context, db_pool):
    """Concurrent entity creates should not bypass duplicate checks."""

    payload = CreateEntityInput(
        name="Race Entity",
        type="person",
        status="active",
        scopes=["public"],
        tags=["test"],
    )

    results = await asyncio.gather(
        create_entity(payload, mock_mcp_context),
        create_entity(payload, mock_mcp_context),
        return_exceptions=True,
    )

    created = [r for r in results if isinstance(r, dict) and r.get("id")]
    assert len(created) == 1


@pytest.mark.asyncio
async def test_self_relationship_blocked(db_pool, enums, test_entity, mock_mcp_context):
    """Self referential relationships should be rejected."""

    relationship_type = "owns"
    payload = CreateRelationshipInput(
        source_type="entity",
        source_id=str(test_entity["id"]),
        target_type="entity",
        target_id=str(test_entity["id"]),
        relationship_type=relationship_type,
        properties={"note": "self"},
    )

    with pytest.raises(ValueError):
        await create_relationship(payload, mock_mcp_context)


@pytest.mark.asyncio
async def test_cycle_relationship_blocked(db_pool, enums, mock_mcp_context):
    """Cycles for cycle sensitive relationship types should be blocked."""

    status_id = enums.statuses.name_to_id["active"]
    type_id = enums.entity_types.name_to_id["person"]
    scope_ids = [enums.scopes.name_to_id["public"]]

    rows = []
    for name in ["Node A", "Node B", "Node C"]:
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
        rows.append(dict(row))

    def rel_payload(source, target):
        """Build relationship payload for cycle checks."""

        return CreateRelationshipInput(
            source_type="entity",
            source_id=str(source["id"]),
            target_type="entity",
            target_id=str(target["id"]),
            relationship_type="owns",
            properties={"note": "cycle"},
        )

    await create_relationship(rel_payload(rows[0], rows[1]), mock_mcp_context)
    await create_relationship(rel_payload(rows[1], rows[2]), mock_mcp_context)

    with pytest.raises(ValueError):
        await create_relationship(rel_payload(rows[2], rows[0]), mock_mcp_context)


@pytest.mark.asyncio
async def test_concurrent_entity_updates_do_not_error(test_entity, mock_mcp_context):
    """Concurrent updates should not crash or return errors."""

    async def do_update(i: int):
        """Run a single entity update for concurrency tests."""

        payload = UpdateEntityInput(
            entity_id=str(test_entity["id"]),
            tags=[f"iter-{i}"],
        )
        return await update_entity(payload, mock_mcp_context)

    results = await asyncio.gather(*(do_update(i) for i in range(25)))
    assert all(isinstance(row, dict) for row in results)
