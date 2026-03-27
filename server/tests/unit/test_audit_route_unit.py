"""Unit tests for audit route edge branches."""

# Standard Library
from types import SimpleNamespace
from unittest.mock import AsyncMock
from uuid import uuid4

# Third-Party
import pytest
from fastapi import HTTPException

# Local
from nebula_api.routes.audit import (
    _require_admin_scope,
    _require_uuid,
    list_actors,
    list_audit_log,
    list_scopes,
)

pytestmark = pytest.mark.unit


def _request(pool, enums):
    """Build a minimal request with app state."""

    return SimpleNamespace(app=SimpleNamespace(state=SimpleNamespace(pool=pool, enums=enums)))


def test_require_uuid_invalid_value_maps_400():
    """Invalid UUID values should map to INVALID_INPUT."""

    with pytest.raises(HTTPException) as exc:
        _require_uuid("bad", "record")

    assert exc.value.status_code == 400
    assert exc.value.detail["error"]["code"] == "INVALID_INPUT"


def test_require_admin_scope_missing_admin_maps_403(mock_enums):
    """Missing admin scope should map to FORBIDDEN."""

    auth = {"scopes": [mock_enums.scopes.name_to_id["public"]]}

    with pytest.raises(HTTPException) as exc:
        _require_admin_scope(auth, mock_enums)

    assert exc.value.status_code == 403
    assert exc.value.detail["error"]["code"] == "FORBIDDEN"


@pytest.mark.asyncio
async def test_list_audit_log_invalid_record_id_maps_400(mock_enums):
    """Invalid record UUID filters should be rejected."""

    pool = SimpleNamespace()
    auth = {"scopes": [mock_enums.scopes.name_to_id["admin"]]}

    with pytest.raises(HTTPException) as exc:
        await list_audit_log(
            _request(pool, mock_enums),
            auth=auth,
            record_id="bad-record-id",
        )

    assert exc.value.status_code == 400
    assert exc.value.detail["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
async def test_list_audit_log_invalid_actor_id_maps_400(mock_enums):
    """Invalid actor UUID filters should be rejected."""

    pool = SimpleNamespace()
    auth = {"scopes": [mock_enums.scopes.name_to_id["admin"]]}

    with pytest.raises(HTTPException) as exc:
        await list_audit_log(
            _request(pool, mock_enums),
            auth=auth,
            actor_id="bad-actor-id",
        )

    assert exc.value.status_code == 400
    assert exc.value.detail["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
async def test_list_audit_log_success_returns_paginated_rows(mock_enums):
    """Audit list should return paginated helper output."""

    rows = [{"id": str(uuid4()), "table_name": "entities"}]
    pool = SimpleNamespace()
    auth = {"scopes": [mock_enums.scopes.name_to_id["admin"]]}

    from nebula_api.routes import audit as mod

    original = mod.query_audit_log
    mod.query_audit_log = AsyncMock(return_value=rows)
    try:
        result = await list_audit_log(
            _request(pool, mock_enums),
            auth=auth,
            table="entities",
            action="UPDATE",
            limit=10,
            offset=0,
        )
    finally:
        mod.query_audit_log = original

    assert result["data"] == rows
    assert result["meta"]["count"] == 1


@pytest.mark.asyncio
async def test_list_scopes_success_returns_rows(mock_enums):
    """Scope listing should return helper payload."""

    rows = [{"scope_id": str(uuid4()), "scope_name": "public", "count": 1}]
    pool = SimpleNamespace()
    auth = {"scopes": [mock_enums.scopes.name_to_id["admin"]]}

    from nebula_api.routes import audit as mod

    original = mod.list_audit_scopes
    mod.list_audit_scopes = AsyncMock(return_value=rows)
    try:
        result = await list_scopes(_request(pool, mock_enums), auth=auth)
    finally:
        mod.list_audit_scopes = original

    assert result["data"] == rows


@pytest.mark.asyncio
async def test_list_actors_success_returns_rows(mock_enums):
    """Actor listing should return helper payload."""

    rows = [{"actor_type": "entity", "actor_id": str(uuid4()), "count": 2}]
    pool = SimpleNamespace()
    auth = {"scopes": [mock_enums.scopes.name_to_id["admin"]]}

    from nebula_api.routes import audit as mod

    original = mod.list_audit_actors
    mod.list_audit_actors = AsyncMock(return_value=rows)
    try:
        result = await list_actors(_request(pool, mock_enums), auth=auth, actor_type="entity")
    finally:
        mod.list_audit_actors = original

    assert result["data"] == rows
