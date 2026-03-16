"""Integration tests for MCP schema tool."""

# Third-Party
import pytest

from nebula_mcp.server import get_schema

pytestmark = pytest.mark.integration


async def test_get_schema_returns_taxonomy_and_constraints(mock_mcp_context):
    """MCP get_schema should return active taxonomy and constraint lists."""

    result = await get_schema(mock_mcp_context)

    assert "taxonomy" in result
    assert "constraints" in result
    assert "statuses" in result

    builtin_scopes = {
        row["name"] for row in result["taxonomy"]["scopes"] if row.get("is_builtin") is True
    }
    assert builtin_scopes == {"admin", "private", "public", "sensitive"}
