"""Red team tests for invalid UUID handling in MCP resource tools."""

# Third-Party
import pytest

# Local
from nebula_mcp.models import (
    GetFileInput,
    GetLogInput,
    LinkContextInput,
    UpdateLogInput,
)
from nebula_mcp.server import get_file, get_log, link_context_to_owner, update_log


@pytest.mark.asyncio
async def test_get_log_rejects_invalid_uuid(untrusted_mcp_context):
    """Invalid UUIDs should not crash get_log."""

    payload = GetLogInput(log_id="not-a-uuid")
    with pytest.raises(ValueError):
        await get_log(payload, untrusted_mcp_context)


@pytest.mark.asyncio
async def test_update_log_rejects_invalid_uuid(untrusted_mcp_context):
    """Invalid UUIDs should not crash update_log."""

    payload = UpdateLogInput(id="not-a-uuid", metadata={"note": "bad"})
    with pytest.raises(ValueError):
        await update_log(payload, untrusted_mcp_context)


@pytest.mark.asyncio
async def test_get_file_rejects_invalid_uuid(untrusted_mcp_context):
    """Invalid UUIDs should not crash get_file."""

    payload = GetFileInput(file_id="not-a-uuid")
    with pytest.raises(ValueError):
        await get_file(payload, untrusted_mcp_context)


@pytest.mark.asyncio
async def test_link_context_rejects_invalid_uuid(untrusted_mcp_context):
    """Invalid UUIDs should not crash link_context_to_owner."""

    payload = LinkContextInput(
        context_id="not-a-uuid",
        owner_type="entity",
        owner_id="also-not-a-uuid",
    )
    with pytest.raises(ValueError):
        await link_context_to_owner(payload, untrusted_mcp_context)
