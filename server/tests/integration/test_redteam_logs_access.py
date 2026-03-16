"""Red team tests for log access isolation."""

# Standard Library
import json
from datetime import UTC, datetime
from unittest.mock import MagicMock

# Third-Party
import pytest

# Local
from nebula_mcp.models import GetLogInput, QueryLogsInput, UpdateLogInput
from nebula_mcp.server import get_log, query_logs, update_log


def _make_context(pool, enums, agent):
    """Build a mock MCP context for log access tests."""

    ctx = MagicMock()
    ctx.request_context.lifespan_context = {
        "pool": pool,
        "enums": enums,
        "agent": agent,
    }
    return ctx


async def _make_agent(db_pool, enums, name, scopes, requires_approval):
    """Insert a test agent for log access scenarios."""

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
    """Insert a test entity for log access scenarios."""

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


async def _make_log(db_pool, enums):
    """Insert a test log entry for access control tests."""

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
        json.dumps({"note": "private"}),
        status_id,
        json.dumps({"class": "sensitive"}),
    )
    return dict(row)


async def _attach_log(db_pool, enums, log_id, entity_id):
    """Attach a log to an entity via relationships."""

    status_id = enums.statuses.name_to_id["active"]
    rel_type_id = enums.relationship_types.name_to_id["related-to"]

    await db_pool.execute(
        """
        INSERT INTO relationships (source_type, source_id, target_type, target_id, type_id, status_id, properties)
        VALUES ('log', $1, 'entity', $2, $3, $4, $5::jsonb)
        """,
        str(log_id),
        str(entity_id),
        rel_type_id,
        status_id,
        json.dumps({"note": "private log"}),
    )


@pytest.mark.asyncio
async def test_get_log_denies_private_entity(db_pool, enums):
    """Log fetch should be denied when attached to private entities."""

    private_entity = await _make_entity(db_pool, enums, "Private", ["sensitive"])
    log_row = await _make_log(db_pool, enums)
    await _attach_log(db_pool, enums, log_row["id"], private_entity["id"])

    viewer = await _make_agent(db_pool, enums, "log-viewer", ["public"], False)
    ctx = _make_context(db_pool, enums, viewer)

    payload = GetLogInput(log_id=str(log_row["id"]))
    with pytest.raises(ValueError):
        await get_log(payload, ctx)


@pytest.mark.asyncio
async def test_query_logs_hides_private_entity_logs(db_pool, enums):
    """Log list should not expose logs attached to private entities."""

    private_entity = await _make_entity(db_pool, enums, "Private", ["sensitive"])
    log_row = await _make_log(db_pool, enums)
    await _attach_log(db_pool, enums, log_row["id"], private_entity["id"])

    viewer = await _make_agent(db_pool, enums, "log-viewer-2", ["public"], False)
    ctx = _make_context(db_pool, enums, viewer)

    rows = await query_logs(QueryLogsInput(), ctx)
    ids = {row["id"] for row in rows}
    assert str(log_row["id"]) not in ids


@pytest.mark.asyncio
async def test_update_log_denies_private_entity(db_pool, enums):
    """Log updates should be denied when attached to private entities."""

    private_entity = await _make_entity(db_pool, enums, "Private", ["sensitive"])
    log_row = await _make_log(db_pool, enums)
    await _attach_log(db_pool, enums, log_row["id"], private_entity["id"])

    viewer = await _make_agent(db_pool, enums, "log-updater", ["public"], False)
    ctx = _make_context(db_pool, enums, viewer)

    payload = UpdateLogInput(id=str(log_row["id"]), metadata={"note": "hijack"})
    with pytest.raises(ValueError):
        await update_log(payload, ctx)
