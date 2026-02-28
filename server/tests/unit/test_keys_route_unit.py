"""Unit tests for API key route edge branches."""

# Standard Library
from types import SimpleNamespace
from unittest.mock import AsyncMock
from uuid import uuid4

# Third-Party
import pytest
from fastapi import HTTPException

# Local
from nebula_api.routes.keys import _require_admin_scope, _require_uuid, revoke_key


pytestmark = pytest.mark.unit


def _request(pool):
    """Build a minimal request carrying app state values."""

    return SimpleNamespace(app=SimpleNamespace(state=SimpleNamespace(pool=pool)))


@pytest.mark.asyncio
async def test_revoke_key_update_zero_maps_not_found():
    """Revoke should map UPDATE 0 result to NOT_FOUND response."""

    pool = SimpleNamespace(execute=AsyncMock(return_value="UPDATE 0"))
    auth = {"entity_id": str(uuid4())}

    with pytest.raises(HTTPException) as exc:
        await revoke_key(str(uuid4()), _request(pool), auth=auth)

    assert exc.value.status_code == 404
    assert exc.value.detail["error"]["code"] == "NOT_FOUND"


def test_require_uuid_invalid_value_maps_400():
    """Invalid UUID inputs should map to INVALID_INPUT."""

    with pytest.raises(HTTPException) as exc:
        _require_uuid("bad-uuid", "key")

    assert exc.value.status_code == 400
    assert exc.value.detail["error"]["code"] == "INVALID_INPUT"


def test_require_admin_scope_missing_admin_maps_403(mock_enums):
    """Missing admin scope should map to FORBIDDEN."""

    auth = {"scopes": [mock_enums.scopes.name_to_id["public"]]}

    with pytest.raises(HTTPException) as exc:
        _require_admin_scope(auth, mock_enums)

    assert exc.value.status_code == 403
    assert exc.value.detail["error"]["code"] == "FORBIDDEN"
