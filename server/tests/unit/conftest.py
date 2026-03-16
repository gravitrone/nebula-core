"""Unit test fixtures: mock pool, mock enums, mock context."""

# Standard Library
# Third-Party
import sys
from pathlib import Path
from unittest.mock import AsyncMock, MagicMock
from uuid import uuid4

import pytest

# Ensure source is importable
SRC_DIR = Path(__file__).resolve().parents[2] / "src"
sys.path.insert(0, str(SRC_DIR))

from nebula_mcp.enums import EnumRegistry, EnumSection


def _make_section(names: list[str]) -> EnumSection:
    """Build an EnumSection with deterministic UUIDs from a list of names."""

    name_to_id = {n: uuid4() for n in names}
    id_to_name = {v: k for k, v in name_to_id.items()}
    return EnumSection(name_to_id=name_to_id, id_to_name=id_to_name)


@pytest.fixture
def mock_enums():
    """Build a fake EnumRegistry with known names for unit testing."""

    return EnumRegistry(
        statuses=_make_section(
            [
                "active",
                "in-progress",
                "planning",
                "on-hold",
                "completed",
                "abandoned",
                "replaced",
                "deleted",
                "inactive",
            ]
        ),
        scopes=_make_section(
            [
                "public",
                "private",
                "sensitive",
                "admin",
            ]
        ),
        relationship_types=_make_section(
            [
                "related-to",
                "depends-on",
                "references",
                "blocks",
                "assigned-to",
                "owns",
                "context-of",
            ]
        ),
        entity_types=_make_section(
            [
                "person",
                "organization",
                "project",
                "tool",
                "document",
            ]
        ),
        log_types=_make_section(
            [
                "event",
                "note",
                "metric",
            ]
        ),
    )


@pytest.fixture
def mock_pool():
    """AsyncMock of asyncpg.Pool for unit tests."""

    pool = AsyncMock()
    pool.fetchrow = AsyncMock(return_value=None)
    pool.fetch = AsyncMock(return_value=[])
    pool.execute = AsyncMock()
    return pool


@pytest.fixture
def mock_context(mock_pool, mock_enums, mock_agent):
    """Mock MCP Context with lifespan_context containing pool, enums, and agent."""

    ctx = MagicMock()
    ctx.request_context.lifespan_context = {
        "pool": mock_pool,
        "enums": mock_enums,
        "agent": mock_agent,
    }
    return ctx


@pytest.fixture
def mock_agent():
    """A mock agent dict (trusted)."""

    return {
        "id": uuid4(),
        "name": "test-agent",
        "scopes": [],
        "requires_approval": False,
        "status_id": uuid4(),
    }


@pytest.fixture
def mock_untrusted_agent():
    """A mock agent dict (untrusted, requires approval)."""

    return {
        "id": uuid4(),
        "name": "untrusted-agent",
        "scopes": [],
        "requires_approval": True,
        "status_id": uuid4(),
    }
