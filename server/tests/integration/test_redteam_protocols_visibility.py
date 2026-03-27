"""Red team tests for MCP protocol visibility controls."""

# Standard Library
import json
from unittest.mock import MagicMock

# Third-Party
import pytest

# Local
from nebula_mcp.models import GetProtocolInput
from nebula_mcp.server import get_protocol, list_active_protocols


async def _make_trusted_protocol(db_pool, enums, name: str) -> None:
    """Insert a trusted protocol row for MCP visibility tests."""

    status_id = enums.statuses.name_to_id["active"]
    await db_pool.execute(
        """
        INSERT INTO protocols (
            name,
            title,
            version,
            content,
            protocol_type,
            applies_to,
            status_id,
            tags,
            trusted,
            metadata,
            source_path
        )
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, TRUE, $9, $10)
        """,
        name,
        "Trusted Internal Protocol",
        "1.0.0",
        "internal system prompt material",
        "system",
        ["agents"],
        status_id,
        ["internal"],
        json.dumps({"classification": "internal"}),
        None,
    )


async def _make_public_protocol(db_pool, enums, name: str) -> None:
    """Insert an untrusted active protocol row for visibility tests."""

    status_id = enums.statuses.name_to_id["active"]
    await db_pool.execute(
        """
        INSERT INTO protocols (
            name,
            title,
            version,
            content,
            protocol_type,
            applies_to,
            status_id,
            tags,
            trusted,
            metadata,
            source_path
        )
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, FALSE, $9, $10)
        """,
        name,
        "Public Protocol",
        "1.0.0",
        "safe public protocol",
        "system",
        ["agents"],
        status_id,
        ["public"],
        json.dumps({"classification": "public"}),
        None,
    )


def _ctx_with_agent(db_pool, enums, agent_row: dict) -> MagicMock:
    """Build a context carrying a specific agent row."""

    ctx = MagicMock()
    ctx.request_context.lifespan_context = {
        "pool": db_pool,
        "enums": enums,
        "agent": agent_row,
    }
    return ctx


async def _make_agent(db_pool, enums, *, name: str, scopes: list[str]) -> dict:
    """Create an active agent and return dict row."""

    row = await db_pool.fetchrow(
        """
        INSERT INTO agents (name, description, scopes, requires_approval, status_id)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING *
        """,
        name,
        f"{name} visibility agent",
        [enums.scopes.name_to_id[s] for s in scopes],
        False,
        enums.statuses.name_to_id["active"],
    )
    return dict(row)


@pytest.mark.asyncio
async def test_non_admin_agent_cannot_read_trusted_protocol_content(
    db_pool,
    enums,
    untrusted_mcp_context,
):
    """Non-admin agents should not fetch trusted protocol content by name."""

    protocol_name = "rt-trusted-protocol-agent-read"
    await _make_trusted_protocol(db_pool, enums, protocol_name)

    payload = GetProtocolInput(protocol_name=protocol_name)
    with pytest.raises(ValueError, match="Access denied"):
        await get_protocol(payload, untrusted_mcp_context)


@pytest.mark.asyncio
async def test_non_admin_list_active_includes_public_protocols(db_pool, enums):
    """Non-admin active protocol list should include non-trusted active rows."""

    public_agent = await _make_agent(
        db_pool,
        enums,
        name="protocol-public-list-agent",
        scopes=["public"],
    )
    ctx = _ctx_with_agent(db_pool, enums, public_agent)

    await _make_public_protocol(db_pool, enums, "rt-public-listable")
    rows = await list_active_protocols(ctx)
    names = {row["name"] for row in rows}
    assert "rt-public-listable" in names


@pytest.mark.asyncio
async def test_admin_list_active_includes_public_and_trusted_protocols(db_pool, enums):
    """Admin active protocol list should include both trusted and non-trusted rows."""

    admin_agent = await _make_agent(
        db_pool,
        enums,
        name="protocol-admin-list-agent",
        scopes=["public", "admin"],
    )
    ctx = _ctx_with_agent(db_pool, enums, admin_agent)

    await _make_trusted_protocol(db_pool, enums, "rt-trusted-listable")
    await _make_public_protocol(db_pool, enums, "rt-public-listable-admin")

    rows = await list_active_protocols(ctx)
    names = {row["name"] for row in rows}
    assert "rt-trusted-listable" in names
    assert "rt-public-listable-admin" in names
