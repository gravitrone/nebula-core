"""Unit tests for context route helper and validator branches."""

# Standard Library
from types import SimpleNamespace
from unittest.mock import AsyncMock
from uuid import uuid4

# Third-Party
import pytest
from fastapi import HTTPException
from pydantic import ValidationError

# Local
from nebula_api.routes.context import (
    CreateContextBody,
    UpdateContextBody,
    _has_write_scopes,
    _require_context_write_access,
    _require_entity_write_access,
    _validate_tag_list,
)
from nebula_mcp.models import MAX_TAG_LENGTH


pytestmark = pytest.mark.unit


def _request(pool, enums):
    """Build a minimal request with app state."""

    return SimpleNamespace(app=SimpleNamespace(state=SimpleNamespace(pool=pool, enums=enums)))


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


@pytest.mark.asyncio
async def test_require_context_write_access_scope_denied_maps_403(mock_enums):
    """Context scope mismatches should return 403."""

    private_scope = mock_enums.scopes.name_to_id["private"]
    public_scope = mock_enums.scopes.name_to_id["public"]
    pool = SimpleNamespace(fetchrow=AsyncMock(return_value={"privacy_scope_ids": [private_scope]}))
    auth = {"scopes": [public_scope]}

    with pytest.raises(HTTPException) as exc:
        await _require_context_write_access(pool, mock_enums, auth, str(uuid4()))

    assert exc.value.status_code == 403
    assert exc.value.detail == "Forbidden"


def test_validate_tag_list_rejects_too_long_tag():
    """Overlong tags should be rejected."""

    with pytest.raises(ValueError, match="Tag too long"):
        _validate_tag_list(["x" * (MAX_TAG_LENGTH + 1)])


def test_validate_tag_list_rejects_non_list_payload():
    """Tag validator should enforce list-only payloads."""

    with pytest.raises(ValueError, match="Tags must be a list"):
        _validate_tag_list("public")  # type: ignore[arg-type]


def test_validate_tag_list_rejects_non_string_item():
    """Tag validator should reject non-string list items."""

    with pytest.raises(ValueError, match="Tags must contain only strings"):
        _validate_tag_list(["ok", 1])  # type: ignore[list-item]


def test_update_context_body_url_empty_string_roundtrips():
    """Empty URL values should pass through without scheme validation."""

    payload = UpdateContextBody(url="")
    assert payload.url == ""


def test_create_context_body_url_none_roundtrips():
    """Create context payload should allow omitted URL values."""

    payload = CreateContextBody(title="note", scopes=["public"], url=None)
    assert payload.url is None


def test_update_context_body_url_none_roundtrips():
    """Update context payload should allow null URL values."""

    payload = UpdateContextBody(url=None)
    assert payload.url is None


def test_create_context_body_url_rejects_non_string_payload():
    """Create context payload should reject non-string URL values."""

    with pytest.raises(ValidationError, match="URL must be a string"):
        CreateContextBody(
            title="note",
            scopes=["public"],
            url=123,  # type: ignore[arg-type]
        )


def test_update_context_body_url_rejects_non_string_payload():
    """Update context payload should reject non-string URL values."""

    with pytest.raises(ValidationError, match="URL must be a string"):
        UpdateContextBody(url=123)  # type: ignore[arg-type]
