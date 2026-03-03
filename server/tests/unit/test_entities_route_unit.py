"""Unit tests for entities route helper and edge branches."""

# Standard Library
import json
from types import SimpleNamespace
from unittest.mock import AsyncMock, MagicMock
from uuid import uuid4

# Third-Party
import pytest
from fastapi import HTTPException
from pydantic import ValidationError

# Local
from nebula_api.routes.entities import (
    BulkUpdateScopesBody,
    CreateEntityBody,
    RevertEntityBody,
    _list_scope_ids,
    _normalize_entity_metadata,
    _require_entity_write_access,
    _validate_tag_list,
    bulk_update_entity_scopes,
    create_entity,
    get_entity,
    get_entity_history,
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


def test_validate_tag_list_rejects_non_list_payload():
    """Tag validator should enforce list-only payloads."""

    with pytest.raises(ValueError, match="Tags must be a list"):
        _validate_tag_list("public")  # type: ignore[arg-type]


def test_validate_tag_list_rejects_non_string_item():
    """Tag validator should reject non-string list items."""

    with pytest.raises(ValueError, match="Tags must contain only strings"):
        _validate_tag_list(["ok", 1])  # type: ignore[list-item]


def test_create_entity_body_rejects_non_string_tag_item():
    """Body validator should surface non-string tag items as validation errors."""

    with pytest.raises(ValidationError, match="Tags must contain only strings"):
        CreateEntityBody(
            name="alpha",
            type="person",
            scopes=["public"],
            tags=[1],  # type: ignore[list-item]
        )


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


@pytest.mark.asyncio
async def test_require_entity_write_access_invalid_id_maps_400(mock_enums):
    """Invalid entity ids should map to INVALID_INPUT."""

    pool = SimpleNamespace(fetch=AsyncMock())
    auth = {"scopes": [mock_enums.scopes.name_to_id["public"]]}

    with pytest.raises(HTTPException) as exc:
        await _require_entity_write_access(pool, mock_enums, auth, ["bad-id"])

    assert exc.value.status_code == 400
    assert exc.value.detail["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
async def test_require_entity_write_access_admin_short_circuits(mock_enums):
    """Admin callers should bypass scoped row lookups."""

    pool = SimpleNamespace(fetch=AsyncMock())
    auth = {"scopes": [mock_enums.scopes.name_to_id["admin"]]}

    await _require_entity_write_access(pool, mock_enums, auth, [str(uuid4())])

    pool.fetch.assert_not_awaited()


@pytest.mark.asyncio
async def test_create_entity_metadata_validation_error_maps_400(
    monkeypatch, mock_enums
):
    """Create metadata validation failures should map to INVALID_INPUT."""

    payload = CreateEntityBody(
        name="Entity",
        type="person",
        status="active",
        scopes=["public"],
        metadata={"k": "v"},
    )
    pool = SimpleNamespace()
    auth = {"caller_type": "user", "scopes": [mock_enums.scopes.name_to_id["public"]]}

    monkeypatch.setattr(
        "nebula_api.routes.entities.validate_metadata_payload",
        lambda _v: (_ for _ in ()).throw(ValueError("bad entity metadata")),
    )

    with pytest.raises(HTTPException) as exc:
        await create_entity(payload, _request(pool, mock_enums), auth=auth)

    assert exc.value.status_code == 400
    assert exc.value.detail["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
async def test_create_entity_approval_short_circuit_returns_payload(
    monkeypatch, mock_enums
):
    """Approval short-circuit payloads should return directly."""

    payload = CreateEntityBody(
        name="Entity",
        type="person",
        status="active",
        scopes=["public"],
        metadata={},
    )
    pool = SimpleNamespace()
    auth = {"caller_type": "agent", "scopes": [mock_enums.scopes.name_to_id["public"]]}
    approval = {"data": {"approval_id": "apr_1"}}

    monkeypatch.setattr(
        "nebula_api.routes.entities.maybe_check_agent_approval",
        AsyncMock(return_value=approval),
    )
    execute = AsyncMock(return_value={"id": str(uuid4())})
    monkeypatch.setattr("nebula_api.routes.entities.execute_create_entity", execute)

    result = await create_entity(payload, _request(pool, mock_enums), auth=auth)

    assert result == approval
    execute.assert_not_awaited()


@pytest.mark.asyncio
async def test_get_entity_forbidden_scope_maps_403(mock_enums):
    """Entity reads outside caller scopes should map to FORBIDDEN."""

    entity_id = str(uuid4())
    private_scope = mock_enums.scopes.name_to_id["private"]
    public_scope = mock_enums.scopes.name_to_id["public"]
    pool = SimpleNamespace(
        fetchrow=AsyncMock(return_value={"id": entity_id, "privacy_scope_ids": [private_scope]})
    )
    auth = {"scopes": [public_scope]}

    with pytest.raises(HTTPException) as exc:
        await get_entity(entity_id, _request(pool, mock_enums), auth=auth)

    assert exc.value.status_code == 403
    assert exc.value.detail["error"]["code"] == "FORBIDDEN"


@pytest.mark.asyncio
async def test_get_entity_history_forbidden_scope_maps_403(mock_enums):
    """Entity history reads outside caller scopes should map to FORBIDDEN."""

    entity_id = str(uuid4())
    private_scope = mock_enums.scopes.name_to_id["private"]
    public_scope = mock_enums.scopes.name_to_id["public"]
    pool = SimpleNamespace(
        fetchrow=AsyncMock(return_value={"id": entity_id, "privacy_scope_ids": [private_scope]})
    )
    auth = {"scopes": [public_scope]}

    with pytest.raises(HTTPException) as exc:
        await get_entity_history(entity_id, _request(pool, mock_enums), auth=auth)

    assert exc.value.status_code == 403
    assert exc.value.detail["error"]["code"] == "FORBIDDEN"


@pytest.mark.asyncio
async def test_update_entity_metadata_validation_error_maps_400(
    monkeypatch, mock_enums
):
    """Update metadata validation failures should map to INVALID_INPUT."""

    class _Payload:
        def model_dump(self):
            return {"metadata": {"k": "v"}, "scopes": None, "status": None}

    entity_id = str(uuid4())
    pool = SimpleNamespace()
    auth = {"caller_type": "user", "scopes": [mock_enums.scopes.name_to_id["public"]]}

    monkeypatch.setattr(
        "nebula_api.routes.entities._require_entity_write_access", AsyncMock()
    )
    monkeypatch.setattr(
        "nebula_api.routes.entities.validate_metadata_payload",
        lambda _v: (_ for _ in ()).throw(ValueError("bad update entity metadata")),
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


@pytest.mark.asyncio
async def test_bulk_update_entity_scopes_direct_update_returns_counts(
    monkeypatch, mock_enums
):
    """Bulk scope updates should return updated counts on direct path."""

    entity_id = str(uuid4())
    payload = BulkUpdateScopesBody(entity_ids=[entity_id], scopes=["public"], op="add")
    pool = SimpleNamespace()
    auth = {"caller_type": "user", "scopes": [mock_enums.scopes.name_to_id["public"]]}

    monkeypatch.setattr(
        "nebula_api.routes.entities._require_entity_write_access", AsyncMock()
    )
    monkeypatch.setattr(
        "nebula_api.routes.entities.maybe_check_agent_approval",
        AsyncMock(return_value=None),
    )
    monkeypatch.setattr(
        "nebula_api.routes.entities.do_bulk_update_entity_scopes",
        AsyncMock(return_value=[entity_id]),
    )

    result = await bulk_update_entity_scopes(
        payload,
        _request(pool, mock_enums),
        auth=auth,
    )

    assert result["data"]["updated"] == 1
    assert result["data"]["entity_ids"] == [entity_id]
