"""Unit tests for entities route helper and edge branches."""

# Standard Library
import json
from types import SimpleNamespace
from unittest.mock import AsyncMock, MagicMock
from uuid import uuid4

# Third-Party
import pytest
from fastapi import HTTPException

# Local
from nebula_api.routes.entities import (
    RevertEntityBody,
    _list_scope_ids,
    _normalize_entity_metadata,
    _validate_tag_list,
    revert_entity,
    update_entity,
)
from nebula_mcp.models import MAX_TAG_LENGTH, MAX_TAGS


pytestmark = pytest.mark.unit


def _request(pool, enums):
    """Build a minimal request with app state."""

    return SimpleNamespace(app=SimpleNamespace(state=SimpleNamespace(pool=pool, enums=enums)))


def test_normalize_entity_metadata_double_encoded_invalid_inner_json():
    """Double encoded metadata with invalid inner JSON should normalize to an object."""

    entity = {"metadata": json.dumps("{bad")}
    out = _normalize_entity_metadata(entity)
    assert out["metadata"] == {}


def test_list_scope_ids_user_falls_back_to_public_scope(mock_enums):
    """User callers with empty scopes should fall back to public scope."""

    auth = {"caller_type": "user", "scopes": []}
    assert _list_scope_ids(auth, mock_enums) == [mock_enums.scopes.name_to_id["public"]]


def test_list_scope_ids_empty_for_non_user_without_scopes(mock_enums):
    """Non-user callers should not get public fallback scopes."""

    auth = {"caller_type": "agent", "scopes": []}
    assert _list_scope_ids(auth, mock_enums) == []


def test_validate_tag_list_rejects_too_many_tags():
    """Tag lists over the max count should fail."""

    with pytest.raises(ValueError, match="Too many tags"):
        _validate_tag_list([f"t{i}" for i in range(MAX_TAGS + 1)])


def test_validate_tag_list_rejects_too_long_tag():
    """Tag values over max length should fail."""

    with pytest.raises(ValueError, match="Tag too long"):
        _validate_tag_list(["x" * (MAX_TAG_LENGTH + 1)])


class _AcquireCtx:
    """Simple async context manager wrapper for a mocked connection."""

    def __init__(self, conn):
        self._conn = conn

    async def __aenter__(self):
        return self._conn

    async def __aexit__(self, exc_type, exc, tb):
        return False


@pytest.mark.asyncio
async def test_revert_entity_sets_and_resets_runtime_markers(monkeypatch, mock_enums):
    """Revert should always set and reset runtime changed-by markers."""

    entity_id = str(uuid4())
    audit_id = str(uuid4())
    auth = {"caller_type": "user", "entity_id": uuid4()}
    conn = SimpleNamespace(execute=AsyncMock())
    pool = SimpleNamespace(acquire=MagicMock(return_value=_AcquireCtx(conn)))

    monkeypatch.setattr(
        "nebula_api.routes.entities.do_revert_entity",
        AsyncMock(return_value={"id": entity_id, "reverted_from_audit_id": audit_id}),
    )

    result = await revert_entity(
        entity_id,
        RevertEntityBody(audit_id=audit_id),
        _request(pool, mock_enums),
        auth=auth,
    )

    assert result["data"]["id"] == entity_id
    assert conn.execute.await_count == 4


@pytest.mark.asyncio
async def test_update_entity_agent_scope_subset_error_maps_400(
    monkeypatch, mock_enums
):
    """Agent scope update failures should map to INVALID_INPUT."""

    class _Payload:
        def model_dump(self):
            return {"metadata": None, "scopes": ["private"]}

    entity_id = str(uuid4())
    pool = SimpleNamespace()
    auth = {
        "caller_type": "agent",
        "scopes": [mock_enums.scopes.name_to_id["public"]],
    }

    monkeypatch.setattr(
        "nebula_api.routes.entities._require_entity_write_access", AsyncMock()
    )
    monkeypatch.setattr(
        "nebula_api.routes.entities.enforce_scope_subset",
        lambda scopes, allowed: (_ for _ in ()).throw(ValueError("scope denied")),
    )

    with pytest.raises(HTTPException) as exc:
        await update_entity(
            entity_id,
            _Payload(),
            _request(pool, mock_enums),
            auth=auth,
        )

    assert exc.value.status_code == 400
    assert exc.value.detail["error"]["code"] == "INVALID_INPUT"
