"""Red team tests for file access isolation."""

# Standard Library
import json
from unittest.mock import MagicMock

# Third-Party
import pytest

# Local
from nebula_mcp.models import GetFileInput, QueryFilesInput
from nebula_mcp.server import get_file, list_files


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
    """Insert an agent for file access tests."""

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


async def _make_entity(db_pool, enums, name, scopes):
    """Insert an entity for file access tests."""

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
    """Insert a file record for file access tests."""

    status_id = enums.statuses.name_to_id["active"]

    row = await db_pool.fetchrow(
        """
        INSERT INTO files (filename, file_path, status_id, metadata)
        VALUES ($1, $2, $3, $4::jsonb)
        RETURNING *
        """,
        "secret.pdf",
        "/vault/secret.pdf",
        status_id,
        json.dumps({"class": "private"}),
    )
    return dict(row)


async def _attach_file(db_pool, enums, file_id, entity_id):
    """Attach a file to an entity."""

    status_id = enums.statuses.name_to_id["active"]
    rel_type_id = enums.relationship_types.name_to_id["has-file"]

    await db_pool.execute(
        """
        INSERT INTO relationships (source_type, source_id, target_type, target_id, type_id, status_id, properties)
        VALUES ('entity', $1, 'file', $2, $3, $4, $5::jsonb)
        """,
        str(entity_id),
        str(file_id),
        rel_type_id,
        status_id,
        json.dumps({"note": "private file"}),
    )


@pytest.mark.asyncio
async def test_get_file_denies_private_entity(db_pool, enums):
    """File fetch should be denied when attached to private entities."""

    private_entity = await _make_entity(db_pool, enums, "Private", ["sensitive"])
    file_row = await _make_file(db_pool, enums)
    await _attach_file(db_pool, enums, file_row["id"], private_entity["id"])

    viewer = await _make_agent(db_pool, enums, "file-viewer", ["public"], False)
    ctx = _make_context(db_pool, enums, viewer)

    payload = GetFileInput(file_id=str(file_row["id"]))
    with pytest.raises(ValueError):
        await get_file(payload, ctx)


@pytest.mark.asyncio
async def test_list_files_hides_private_entity_files(db_pool, enums):
    """File list should not expose files attached to private entities."""

    private_entity = await _make_entity(db_pool, enums, "Private", ["sensitive"])
    file_row = await _make_file(db_pool, enums)
    await _attach_file(db_pool, enums, file_row["id"], private_entity["id"])

    viewer = await _make_agent(db_pool, enums, "file-viewer-2", ["public"], False)
    ctx = _make_context(db_pool, enums, viewer)

    rows = await list_files(QueryFilesInput(), ctx)
    ids = {str(row["id"]) for row in rows}
    assert str(file_row["id"]) not in ids
