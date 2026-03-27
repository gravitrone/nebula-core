"""Unit tests for approvals route edge branches."""

# Standard Library
from types import SimpleNamespace
from unittest.mock import AsyncMock
from uuid import uuid4

# Third-Party
import pytest
from fastapi import HTTPException

# Local
from nebula_api.routes.approvals import (
    ApproveBody,
    _require_admin_scope,
    _require_uuid,
    approve,
    get_approval,
    get_diff,
    get_pending,
)

pytestmark = pytest.mark.unit


def _request(pool, enums):
    """Build a minimal request with app state."""

    return SimpleNamespace(app=SimpleNamespace(state=SimpleNamespace(pool=pool, enums=enums)))


def test_require_uuid_invalid_value_maps_400():
    """Invalid UUID values should map to INVALID_INPUT."""

    with pytest.raises(HTTPException) as exc:
        _require_uuid("bad", "approval")

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
async def test_get_pending_admin_success(mock_enums):
    """Admin pending list should return helper payload."""

    rows = [{"id": str(uuid4())}]
    pool = SimpleNamespace()
    auth = {"scopes": [mock_enums.scopes.name_to_id["admin"]]}

    from nebula_api.routes import approvals as mod

    original = mod.get_pending_approvals_all
    mod.get_pending_approvals_all = AsyncMock(return_value=rows)
    try:
        result = await get_pending(_request(pool, mock_enums), auth=auth)
    finally:
        mod.get_pending_approvals_all = original

    assert result["data"] == rows


@pytest.mark.asyncio
async def test_get_approval_not_found_maps_404(mock_enums):
    """Missing approval ids should map to NOT_FOUND."""

    pool = SimpleNamespace()
    auth = {"scopes": [mock_enums.scopes.name_to_id["admin"]]}

    from nebula_api.routes import approvals as mod

    original = mod.get_approval_request
    mod.get_approval_request = AsyncMock(return_value=None)
    try:
        with pytest.raises(HTTPException) as exc:
            await get_approval(str(uuid4()), _request(pool, mock_enums), auth=auth)
    finally:
        mod.get_approval_request = original

    assert exc.value.status_code == 404
    assert exc.value.detail["error"]["code"] == "NOT_FOUND"


@pytest.mark.asyncio
async def test_approve_non_register_with_grants_maps_400(mock_enums):
    """Grant fields should be rejected for non-register approvals."""

    approval_id = str(uuid4())
    pool = SimpleNamespace()
    auth = {"scopes": [mock_enums.scopes.name_to_id["admin"]], "entity_id": uuid4()}
    payload = ApproveBody(grant_scopes=["public"])

    from nebula_api.routes import approvals as mod

    original = mod.get_approval_request
    mod.get_approval_request = AsyncMock(
        return_value={"id": approval_id, "request_type": "create_entity"}
    )
    try:
        with pytest.raises(HTTPException) as exc:
            await approve(approval_id, _request(pool, mock_enums), payload=payload, auth=auth)
    finally:
        mod.get_approval_request = original

    assert exc.value.status_code == 400
    assert exc.value.detail["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
async def test_approve_register_invalid_grant_scope_maps_400(mock_enums):
    """Unknown grant scopes should map to INVALID_INPUT."""

    approval_id = str(uuid4())
    pool = SimpleNamespace()
    auth = {"scopes": [mock_enums.scopes.name_to_id["admin"]], "entity_id": uuid4()}
    payload = ApproveBody(grant_scopes=["does-not-exist"])

    from nebula_api.routes import approvals as mod

    original = mod.get_approval_request
    mod.get_approval_request = AsyncMock(
        return_value={"id": approval_id, "request_type": "register_agent"}
    )
    try:
        with pytest.raises(HTTPException) as exc:
            await approve(approval_id, _request(pool, mock_enums), payload=payload, auth=auth)
    finally:
        mod.get_approval_request = original

    assert exc.value.status_code == 400
    assert exc.value.detail["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
async def test_get_diff_invalid_input_maps_400(mock_enums):
    """Non-not-found diff errors should map to INVALID_INPUT."""

    approval_id = str(uuid4())
    pool = SimpleNamespace()
    auth = {"scopes": [mock_enums.scopes.name_to_id["admin"]]}

    from nebula_api.routes import approvals as mod

    original = mod.compute_approval_diff
    mod.compute_approval_diff = AsyncMock(side_effect=ValueError("bad diff payload"))
    try:
        with pytest.raises(HTTPException) as exc:
            await get_diff(approval_id, _request(pool, mock_enums), auth=auth)
    finally:
        mod.compute_approval_diff = original

    assert exc.value.status_code == 400
    assert exc.value.detail["error"]["code"] == "INVALID_INPUT"
