"""Unit tests for context route helper and validator branches."""

# Standard Library
from types import SimpleNamespace
from unittest.mock import AsyncMock
from uuid import uuid4

# Third-Party
import pytest
from fastapi import HTTPException

# Local
from nebula_api.routes.context import (
    UpdateContextBody,
    _has_write_scopes,
    _require_context_write_access,
    _require_entity_write_access,
    _validate_tag_list,
)
from nebula_mcp.models import MAX_TAG_LENGTH


pytestmark = pytest.mark.unit


def test_has_write_scopes_true_when_node_scopes_empty():
    """Nodes without scopes should be writable by any caller."""

    assert _has_write_scopes([], []) is True


def test_has_write_scopes_false_when_agent_scopes_empty():
    """Scoped nodes should not be writable by callers without scopes."""

    assert _has_write_scopes([], [str(uuid4())]) is False


@pytest.mark.asyncio
async def test_require_entity_write_access_admin_short_circuits(mock_enums):
    """Admin callers should bypass entity lookups."""

    pool = SimpleNamespace(fetchrow=AsyncMock())
    auth = {"scopes": [mock_enums.scopes.name_to_id["admin"]]}

    await _require_entity_write_access(pool, mock_enums, auth, str(uuid4()))

    pool.fetchrow.assert_not_awaited()


@pytest.mark.asyncio
async def test_require_entity_write_access_missing_entity_maps_404(mock_enums):
    """Unknown entity ids should return 404."""

    pool = SimpleNamespace(fetchrow=AsyncMock(return_value=None))
    auth = {"scopes": [mock_enums.scopes.name_to_id["public"]]}

    with pytest.raises(HTTPException) as exc:
        await _require_entity_write_access(pool, mock_enums, auth, str(uuid4()))

    assert exc.value.status_code == 404
    assert exc.value.detail == "Not Found"


@pytest.mark.asyncio
async def test_require_context_write_access_missing_context_maps_404(mock_enums):
    """Unknown context ids should return 404."""

    pool = SimpleNamespace(fetchrow=AsyncMock(return_value=None))
    auth = {"scopes": [mock_enums.scopes.name_to_id["public"]]}

    with pytest.raises(HTTPException) as exc:
        await _require_context_write_access(pool, mock_enums, auth, str(uuid4()))

    assert exc.value.status_code == 404
    assert exc.value.detail == "Not Found"


def test_validate_tag_list_rejects_too_long_tag():
    """Overlong tags should be rejected."""

    with pytest.raises(ValueError, match="Tag too long"):
        _validate_tag_list(["x" * (MAX_TAG_LENGTH + 1)])


def test_update_context_body_url_empty_string_roundtrips():
    """Empty URL values should pass through without scheme validation."""

    payload = UpdateContextBody(url="")
    assert payload.url == ""
