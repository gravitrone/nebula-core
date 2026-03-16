"""Integration test fixtures: real DB agent factories, MCP context mock."""

# Standard Library
# Third-Party
import sys
from pathlib import Path
from unittest.mock import MagicMock

import pytest

SRC_DIR = Path(__file__).resolve().parents[2] / "src"
sys.path.insert(0, str(SRC_DIR))


# --- Agent Fixtures ---


@pytest.fixture
async def test_agent(db_pool, enums):
    """Create a trusted test agent (requires_approval=False) and return its row."""

    status_id = enums.statuses.name_to_id["active"]
    scope_ids = [
        enums.scopes.name_to_id["public"],
        enums.scopes.name_to_id["private"],
        enums.scopes.name_to_id["sensitive"],
        enums.scopes.name_to_id["admin"],
    ]

    row = await db_pool.fetchrow(
        """
        INSERT INTO agents (name, description, scopes, requires_approval, status_id)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING *
        """,
        "test-agent",
        "Trusted test agent",
        scope_ids,
        False,
        status_id,
    )
    return dict(row)


@pytest.fixture
async def untrusted_agent(db_pool, enums):
    """Create an untrusted test agent (requires_approval=True)."""

    status_id = enums.statuses.name_to_id["active"]
    scope_ids = [enums.scopes.name_to_id["public"]]

    row = await db_pool.fetchrow(
        """
        INSERT INTO agents (name, description, scopes, requires_approval, status_id)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING *
        """,
        "untrusted-agent",
        "Untrusted test agent",
        scope_ids,
        True,
        status_id,
    )
    return dict(row)


# --- Entity Fixtures ---


@pytest.fixture
async def test_entity(db_pool, enums):
    """Create a minimal entity for integration test references."""

    status_id = enums.statuses.name_to_id["active"]
    type_id = enums.entity_types.name_to_id["person"]
    scope_ids = [enums.scopes.name_to_id["public"]]

    row = await db_pool.fetchrow(
        """
        INSERT INTO entities (name, type_id, status_id, privacy_scope_ids, tags)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING *
        """,
        "Test Person",
        type_id,
        status_id,
        scope_ids,
        ["test"],
    )
    return dict(row)


# --- Mock Context ---


@pytest.fixture
def mock_mcp_context(db_pool, enums, test_agent):
    """Mock MCP Context pointing to real pool, enums, and agent for tool testing."""

    ctx = MagicMock()
    ctx.request_context.lifespan_context = {
        "pool": db_pool,
        "enums": enums,
        "agent": test_agent,
    }
    return ctx


@pytest.fixture
def untrusted_mcp_context(db_pool, enums, untrusted_agent):
    """Mock MCP Context with untrusted agent for approval testing."""

    ctx = MagicMock()
    ctx.request_context.lifespan_context = {
        "pool": db_pool,
        "enums": enums,
        "agent": untrusted_agent,
    }
    return ctx


@pytest.fixture
def bootstrap_mcp_context(db_pool, enums):
    """Mock MCP Context for bootstrap mode with no authenticated agent."""

    ctx = MagicMock()
    ctx.request_context.lifespan_context = {
        "pool": db_pool,
        "enums": enums,
        "agent": None,
        "bootstrap_mode": True,
    }
    return ctx
