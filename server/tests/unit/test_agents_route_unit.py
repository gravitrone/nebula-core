"""Unit tests for agent route edge branches."""

# Standard Library
from types import SimpleNamespace
from unittest.mock import AsyncMock
from uuid import uuid4

# Third-Party
import pytest
from fastapi import HTTPException

# Local
from nebula_api.routes.agents import (
    RegisterAgentBody,
    UpdateAgentBody,
    _require_uuid,
    register_agent,
    update_agent,
)

pytestmark = pytest.mark.unit


def _request(pool, enums):
    """Build a minimal request carrying app state values."""

    return SimpleNamespace(app=SimpleNamespace(state=SimpleNamespace(pool=pool, enums=enums)))


def test_require_uuid_invalid_value_maps_400():
    """Invalid UUID inputs should map to INVALID_INPUT errors."""

    with pytest.raises(HTTPException) as exc:
        _require_uuid("bad-uuid", "agent")

    assert exc.value.status_code == 400
    assert exc.value.detail["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
async def test_register_agent_missing_inactive_status_maps_500(mock_enums):
    """Registration should fail when inactive status is not present."""

    mock_enums.statuses.name_to_id.pop("inactive", None)
    pool = SimpleNamespace()
    payload = RegisterAgentBody(name="unit-agent")

    with pytest.raises(HTTPException) as exc:
        await register_agent(payload, _request(pool, mock_enums))

    assert exc.value.status_code == 500
    assert exc.value.detail["error"]["code"] == "INTERNAL"


@pytest.mark.asyncio
async def test_update_agent_with_scopes_resolves_scope_ids(mock_enums):
    """Update route should resolve scope names when scopes are provided."""

    agent_id = str(uuid4())
    pool = SimpleNamespace(
        fetchrow=AsyncMock(return_value={"id": agent_id, "name": "a", "scopes": []})
    )
    payload = UpdateAgentBody(scopes=["public", "private"])
    auth = {"caller_type": "agent", "agent_id": agent_id, "scopes": []}

    result = await update_agent(agent_id, payload, _request(pool, mock_enums), auth=auth)

    scope_ids = pool.fetchrow.await_args.args[4]
    assert scope_ids == [
        mock_enums.scopes.name_to_id["public"],
        mock_enums.scopes.name_to_id["private"],
    ]
    assert result["data"]["id"] == agent_id


@pytest.mark.asyncio
async def test_update_agent_invalid_scope_raises_value_error(mock_enums):
    """Invalid scope names should bubble ValueError from require_scopes."""

    agent_id = str(uuid4())
    pool = SimpleNamespace(fetchrow=AsyncMock())
    payload = UpdateAgentBody(scopes=["public", "not-a-scope"])
    auth = {"caller_type": "agent", "agent_id": agent_id, "scopes": []}

    with pytest.raises(ValueError, match="Unknown scope"):
        await update_agent(agent_id, payload, _request(pool, mock_enums), auth=auth)
