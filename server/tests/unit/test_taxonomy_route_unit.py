"""Unit tests for taxonomy route edge branches."""

# Standard Library
from types import SimpleNamespace
from unittest.mock import AsyncMock
from uuid import uuid4

# Third-Party
import pytest
from fastapi import HTTPException

# Local
from nebula_api.routes.taxonomy import (
    KIND_MAP,
    TaxonomyCreateBody,
    TaxonomyUpdateBody,
    _kind_or_error,
    _require_uuid,
    _usage_count,
    _validate_payload,
    activate_taxonomy,
    create_taxonomy,
    update_taxonomy,
)


pytestmark = pytest.mark.unit


def _request(pool, enums):
    """Build a minimal request carrying app state values."""

    return SimpleNamespace(app=SimpleNamespace(state=SimpleNamespace(pool=pool, enums=enums)))


def _admin_auth(mock_enums):
    """Build an admin-scoped auth payload."""

    return {"scopes": [mock_enums.scopes.name_to_id["admin"]]}


def test_require_uuid_invalid_value_maps_400():
    """Invalid UUID inputs should map to INVALID_INPUT errors."""

    with pytest.raises(HTTPException) as exc:
        _require_uuid("bad-uuid", "scopes")

    assert exc.value.status_code == 400
    assert exc.value.detail["error"]["code"] == "INVALID_INPUT"


def test_kind_or_error_unknown_kind_maps_400():
    """Unknown taxonomy kinds should map to INVALID_INPUT errors."""

    with pytest.raises(HTTPException) as exc:
        _kind_or_error("bad-kind")

    assert exc.value.status_code == 400
    assert exc.value.detail["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
async def test_usage_count_none_returns_zero():
    """Missing usage counts should normalize to zero."""

    pool = SimpleNamespace(fetchval=AsyncMock(return_value=None))
    value = await _usage_count(pool, KIND_MAP["scopes"], str(uuid4()))
    assert value == 0


def test_validate_payload_rejects_is_symmetric_for_scopes():
    """Non-relationship kinds should reject is_symmetric fields."""

    payload = TaxonomyCreateBody(name="x", is_symmetric=True)
    with pytest.raises(HTTPException) as exc:
        _validate_payload("scopes", payload)

    assert exc.value.status_code == 400
    assert "is_symmetric" in exc.value.detail["error"]["message"]


def test_validate_payload_rejects_value_schema_for_scopes():
    """Non-log kinds should reject value_schema fields."""

    payload = TaxonomyCreateBody(name="x", value_schema={"k": "v"})
    with pytest.raises(HTTPException) as exc:
        _validate_payload("scopes", payload)

    assert exc.value.status_code == 400
    assert "value_schema" in exc.value.detail["error"]["message"]


@pytest.mark.asyncio
async def test_create_taxonomy_blank_name_maps_400(monkeypatch, mock_enums):
    """Blank taxonomy names should return INVALID_INPUT."""

    pool = SimpleNamespace(fetchrow=AsyncMock())
    monkeypatch.setattr("nebula_api.routes.taxonomy._refresh_enums", AsyncMock())
    payload = TaxonomyCreateBody(name="   ")

    with pytest.raises(HTTPException) as exc:
        await create_taxonomy(
            "scopes",
            payload,
            _request(pool, mock_enums),
            auth=_admin_auth(mock_enums),
        )

    assert exc.value.status_code == 400
    assert exc.value.detail["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
async def test_create_taxonomy_duplicate_maps_409(monkeypatch, mock_enums):
    """Unique violations should map to DUPLICATE responses."""

    class _FakeUniqueViolation(Exception):
        pass

    pool = SimpleNamespace(fetchrow=AsyncMock(side_effect=_FakeUniqueViolation()))
    monkeypatch.setattr("nebula_api.routes.taxonomy.UniqueViolationError", _FakeUniqueViolation)
    monkeypatch.setattr("nebula_api.routes.taxonomy._refresh_enums", AsyncMock())
    payload = TaxonomyCreateBody(name="public")

    with pytest.raises(HTTPException) as exc:
        await create_taxonomy(
            "scopes",
            payload,
            _request(pool, mock_enums),
            auth=_admin_auth(mock_enums),
        )

    assert exc.value.status_code == 409
    assert exc.value.detail["error"]["code"] == "DUPLICATE"


@pytest.mark.asyncio
async def test_update_taxonomy_missing_current_row_maps_404(monkeypatch, mock_enums):
    """Updating missing rows should return NOT_FOUND."""

    pool = SimpleNamespace(fetchrow=AsyncMock())
    monkeypatch.setattr(
        "nebula_api.routes.taxonomy._fetch_taxonomy_row",
        AsyncMock(return_value=None),
    )
    payload = TaxonomyUpdateBody(description="x")

    with pytest.raises(HTTPException) as exc:
        await update_taxonomy(
            "scopes",
            str(uuid4()),
            payload,
            _request(pool, mock_enums),
            auth=_admin_auth(mock_enums),
        )

    assert exc.value.status_code == 404
    assert exc.value.detail["error"]["code"] == "NOT_FOUND"


@pytest.mark.asyncio
async def test_update_taxonomy_empty_name_maps_400(monkeypatch, mock_enums):
    """Empty update names should return INVALID_INPUT."""

    pool = SimpleNamespace(fetchrow=AsyncMock())
    monkeypatch.setattr(
        "nebula_api.routes.taxonomy._fetch_taxonomy_row",
        AsyncMock(return_value={"is_builtin": False, "name": "public"}),
    )
    payload = TaxonomyUpdateBody(name="   ")

    with pytest.raises(HTTPException) as exc:
        await update_taxonomy(
            "scopes",
            str(uuid4()),
            payload,
            _request(pool, mock_enums),
            auth=_admin_auth(mock_enums),
        )

    assert exc.value.status_code == 400
    assert exc.value.detail["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
async def test_update_taxonomy_duplicate_maps_409(monkeypatch, mock_enums):
    """Unique violations during update should map to DUPLICATE."""

    class _FakeUniqueViolation(Exception):
        pass

    pool = SimpleNamespace(fetchrow=AsyncMock(side_effect=_FakeUniqueViolation()))
    monkeypatch.setattr("nebula_api.routes.taxonomy.UniqueViolationError", _FakeUniqueViolation)
    monkeypatch.setattr(
        "nebula_api.routes.taxonomy._fetch_taxonomy_row",
        AsyncMock(return_value={"is_builtin": False, "name": "public"}),
    )
    monkeypatch.setattr("nebula_api.routes.taxonomy._refresh_enums", AsyncMock())
    payload = TaxonomyUpdateBody(name="private")

    with pytest.raises(HTTPException) as exc:
        await update_taxonomy(
            "scopes",
            str(uuid4()),
            payload,
            _request(pool, mock_enums),
            auth=_admin_auth(mock_enums),
        )

    assert exc.value.status_code == 409
    assert exc.value.detail["error"]["code"] == "DUPLICATE"


@pytest.mark.asyncio
async def test_update_taxonomy_none_row_maps_404(monkeypatch, mock_enums):
    """None update results should map to NOT_FOUND."""

    pool = SimpleNamespace(fetchrow=AsyncMock(return_value=None))
    monkeypatch.setattr(
        "nebula_api.routes.taxonomy._fetch_taxonomy_row",
        AsyncMock(return_value={"is_builtin": False, "name": "public"}),
    )
    monkeypatch.setattr("nebula_api.routes.taxonomy._refresh_enums", AsyncMock())
    payload = TaxonomyUpdateBody(description="x")

    with pytest.raises(HTTPException) as exc:
        await update_taxonomy(
            "scopes",
            str(uuid4()),
            payload,
            _request(pool, mock_enums),
            auth=_admin_auth(mock_enums),
        )

    assert exc.value.status_code == 404
    assert exc.value.detail["error"]["code"] == "NOT_FOUND"


@pytest.mark.asyncio
async def test_activate_taxonomy_missing_row_maps_404(mock_enums):
    """Missing activate targets should map to NOT_FOUND."""

    pool = SimpleNamespace(fetchrow=AsyncMock(return_value=None))

    with pytest.raises(HTTPException) as exc:
        await activate_taxonomy(
            "scopes",
            str(uuid4()),
            _request(pool, mock_enums),
            auth=_admin_auth(mock_enums),
        )

    assert exc.value.status_code == 404
    assert exc.value.detail["error"]["code"] == "NOT_FOUND"
