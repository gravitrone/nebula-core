"""Red team tests for context metadata privacy filtering."""

# Standard Library
import json
from unittest.mock import MagicMock
from uuid import UUID, uuid4

# Third-Party
import pytest

# Local
from nebula_mcp.models import QueryContextInput
from nebula_mcp.server import query_context


async def _make_context(pool, enums, agent):
    """Build MCP context with a specific agent."""

    normalized_agent = dict(agent)
    try:
        agent_id = str(UUID(str(normalized_agent.get("id", ""))))
    except (TypeError, ValueError):
        agent_id = str(uuid4())
    normalized_agent["id"] = agent_id
    normalized_agent.setdefault("name", f"rt-{agent_id[:8]}")
    normalized_agent.setdefault("requires_approval", False)
    normalized_agent.setdefault("scopes", [enums.scopes.name_to_id["public"]])

    await pool.execute(
        """
        INSERT INTO agents (id, name, description, scopes, requires_approval, status_id)
        VALUES ($1::uuid, $2, $3, $4, $5, $6::uuid)
        ON CONFLICT (id) DO UPDATE
        SET name = EXCLUDED.name,
            scopes = EXCLUDED.scopes,
            requires_approval = EXCLUDED.requires_approval,
            status_id = EXCLUDED.status_id
        """,
        agent_id,
        normalized_agent["name"],
        "test context metadata helper",
        normalized_agent["scopes"],
        normalized_agent["requires_approval"],
        enums.statuses.name_to_id["active"],
    )

    ctx = MagicMock()
    ctx.request_context.lifespan_context = {
        "pool": pool,
        "enums": enums,
        "agent": normalized_agent,
    }
    return ctx


async def _make_context_item(db_pool, enums, title, scopes, metadata):
    """Insert a context item for metadata filtering tests."""

    status_id = enums.statuses.name_to_id["active"]
    scope_ids = [enums.scopes.name_to_id[s] for s in scopes]

    row = await db_pool.fetchrow(
        """
        INSERT INTO context_items (title, source_type, content, privacy_scope_ids, status_id, tags, metadata)
        VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb)
        RETURNING *
        """,
        title,
        "note",
        "secret",
        scope_ids,
        status_id,
        ["test"],
        json.dumps(metadata),
    )
    return dict(row)


@pytest.mark.asyncio
async def test_query_context_filters_context_segments(db_pool, enums):
    """Query results should not include context segments outside agent scopes."""

    metadata = {
        "context_segments": [
            {"text": "public info", "scopes": ["public"]},
            {"text": "private info", "scopes": ["private"]},
        ]
    }
    await _make_context_item(
        db_pool, enums, "Mixed Scope", ["public", "private"], metadata
    )

    public_agent = {
        "id": "public-agent",
        "scopes": [enums.scopes.name_to_id["public"]],
    }
    ctx = await _make_context(db_pool, enums, public_agent)

    payload = QueryContextInput(scopes=["public"])
    rows = await query_context(payload, ctx)
    assert rows
    segments = rows[0]["metadata"].get("context_segments", [])

    assert all("private" not in seg.get("scopes", []) for seg in segments)
