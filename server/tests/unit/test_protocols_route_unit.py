"""Unit tests for protocol API route edge branches."""

# Standard Library
from types import SimpleNamespace
from unittest.mock import AsyncMock
from uuid import uuid4

# Third-Party
import pytest
from fastapi import HTTPException
from pydantic import ValidationError

# Local
from nebula_api.routes.protocols import (
    CreateProtocolBody,
    UpdateProtocolBody,
    _validate_tag_list,
    create_protocol,
    get_protocol,
    query_protocols,
    update_protocol,
)
from nebula_mcp.models import MAX_TAG_LENGTH, MAX_TAGS

pytestmark = pytest.mark.unit


def _request(pool, enums):
    """Build a minimal request carrying app.state values."""

    return SimpleNamespace(app=SimpleNamespace(state=SimpleNamespace(pool=pool, enums=enums)))


def _row(name: str, trusted: bool = False) -> dict:
    """Build protocol-like row payload used by route tests."""

    return {
        "id": str(uuid4()),
        "name": name,
        "title": "Protocol",
        "version": "1.0.0",
        "content": "x",
        "protocol_type": "system",
        "applies_to": ["agents"],
        "status_name": "active",
        "tags": ["x"],
        "trusted": trusted,
        "notes": "",
        "source_path": None,
    }


def test_validate_tag_list_none_returns_none():
    """Validator should pass through None unchanged."""

    assert _validate_tag_list(None) is None


def test_validate_tag_list_too_many_raises():
    """Validator should reject payloads over MAX_TAGS."""

    tags = [f"t{i}" for i in range(MAX_TAGS + 1)]
    with pytest.raises(ValueError, match="Too many tags"):
        _validate_tag_list(tags)


def test_validate_tag_list_too_long_raises():
    """Validator should reject tags that exceed MAX_TAG_LENGTH."""

    with pytest.raises(ValueError, match="Tag too long"):
        _validate_tag_list(["x" * (MAX_TAG_LENGTH + 1)])


def test_validate_tag_list_rejects_non_list_payload():
    """Validator should enforce list-only payloads."""

    with pytest.raises(ValueError, match="Tags must be a list"):
        _validate_tag_list("public")  # type: ignore[arg-type]


def test_validate_tag_list_rejects_non_string_item():
    """Validator should reject non-string tag items."""

    with pytest.raises(ValueError, match="Tags must contain only strings"):
        _validate_tag_list(["ok", 1])  # type: ignore[list-item]


def test_update_protocol_body_runs_tag_cleaner():
    """Update body validator should trim tags and drop empty values."""

    payload = UpdateProtocolBody(tags=[" a ", "", "b", "   "])
    assert payload.tags == ["a", "b"]


def test_create_protocol_body_runs_tag_cleaner():
    """Create body validator should trim tags and drop empty values."""

    payload = CreateProtocolBody(
        name="p",
        title="Protocol",
        content="body",
        tags=[" a ", "", "b", "   "],
    )
    assert payload.tags == ["a", "b"]


def test_create_protocol_body_rejects_non_string_tag_item():
    """Create body should fail fast when tags include non-string items."""

    with pytest.raises(ValidationError, match="Tags must contain only strings"):
        CreateProtocolBody(
            name="p",
            title="Protocol",
            content="body",
            tags=[1],  # type: ignore[list-item]
        )


@pytest.mark.asyncio
async def test_query_protocols_sets_admin_flag_true(mock_enums):
    """Protocol query should pass admin visibility flag when caller is admin."""

    rows = [_row("trusted-protocol", trusted=True)]
    pool = SimpleNamespace(fetch=AsyncMock(return_value=rows))
    auth = {"scopes": [mock_enums.scopes.name_to_id["admin"]]}

    result = await query_protocols(
        _request(pool, mock_enums),
        auth=auth,
        status_category="active",
        protocol_type="system",
        search="trusted",
        limit=7,
    )

    assert result["data"][0]["name"] == "trusted-protocol"
    assert result["meta"]["count"] == 1
    assert pool.fetch.await_args.args[5] is True


@pytest.mark.asyncio
async def test_query_protocols_sets_admin_flag_false(mock_enums):
    """Protocol query should keep trusted filtering for non-admin callers."""

    pool = SimpleNamespace(fetch=AsyncMock(return_value=[]))
    auth = {"scopes": [mock_enums.scopes.name_to_id["public"]]}

    result = await query_protocols(_request(pool, mock_enums), auth=auth, limit=5)

    assert result["data"] == []
    assert pool.fetch.await_args.args[5] is False


@pytest.mark.asyncio
async def test_get_protocol_success_returns_payload(mock_enums):
    """Fetching a non-trusted protocol should return success envelope."""

    pool = SimpleNamespace(fetchrow=AsyncMock(return_value=_row("plain-protocol")))
    auth = {"scopes": []}

    result = await get_protocol("plain-protocol", _request(pool, mock_enums), auth=auth)

    assert result["data"]["name"] == "plain-protocol"


@pytest.mark.asyncio
async def test_get_protocol_not_found_maps_404(mock_enums):
    """Missing protocol rows should map to HTTP 404."""

    pool = SimpleNamespace(fetchrow=AsyncMock(return_value=None))
    auth = {"scopes": []}

    with pytest.raises(HTTPException) as exc:
        await get_protocol("missing", _request(pool, mock_enums), auth=auth)

    assert exc.value.status_code == 404
    assert exc.value.detail == "Not Found"


@pytest.mark.asyncio
async def test_get_protocol_trusted_forbidden_for_non_admin(mock_enums):
    """Trusted protocol reads should be forbidden for non-admin callers."""

    pool = SimpleNamespace(fetchrow=AsyncMock(return_value=_row("trusted", trusted=True)))
    auth = {"scopes": [mock_enums.scopes.name_to_id["public"]]}

    with pytest.raises(HTTPException) as exc:
        await get_protocol("trusted", _request(pool, mock_enums), auth=auth)

    assert exc.value.status_code == 403
    assert exc.value.detail == "Forbidden"


@pytest.mark.asyncio
async def test_create_protocol_approval_short_circuit(monkeypatch, mock_enums):
    """Create route should return approval payload before executor call."""

    pool = SimpleNamespace()
    auth = {"caller_type": "agent", "agent": {"id": str(uuid4())}, "scopes": []}
    payload = CreateProtocolBody(name="p1", title="P1", content="content", status="active")
    execute = AsyncMock()

    monkeypatch.setattr(
        "nebula_api.routes.protocols.maybe_check_agent_approval",
        AsyncMock(return_value={"status": "approval_required", "approval_request_id": "a1"}),
    )
    monkeypatch.setattr("nebula_api.routes.protocols.execute_create_protocol", execute)

    result = await create_protocol(payload, _request(pool, mock_enums), auth=auth)

    assert result["status"] == "approval_required"
    execute.assert_not_awaited()


@pytest.mark.asyncio
async def test_create_protocol_executor_valueerror_maps_400(monkeypatch, mock_enums):
    """Create route should map executor ValueError into HTTP 400."""

    pool = SimpleNamespace()
    auth = {"caller_type": "user", "scopes": []}
    payload = CreateProtocolBody(name="p2", title="P2", content="content", status="active")

    monkeypatch.setattr(
        "nebula_api.routes.protocols.maybe_check_agent_approval",
        AsyncMock(return_value=None),
    )
    monkeypatch.setattr(
        "nebula_api.routes.protocols.execute_create_protocol",
        AsyncMock(side_effect=ValueError("bad protocol create")),
    )

    with pytest.raises(HTTPException) as exc:
        await create_protocol(payload, _request(pool, mock_enums), auth=auth)

    assert exc.value.status_code == 400
    assert "bad protocol create" in str(exc.value.detail)


@pytest.mark.asyncio
async def test_create_protocol_invalid_status_maps_400(monkeypatch, mock_enums):
    """Create route should map status validation errors to HTTP 400."""

    pool = SimpleNamespace()
    auth = {"caller_type": "user", "scopes": []}
    payload = CreateProtocolBody(name="p3", title="P3", content="content", status="invalid")

    monkeypatch.setattr(
        "nebula_api.routes.protocols.require_status",
        lambda *_args, **_kwargs: (_ for _ in ()).throw(ValueError("Unknown status: invalid")),
    )

    with pytest.raises(HTTPException) as exc:
        await create_protocol(payload, _request(pool, mock_enums), auth=auth)

    assert exc.value.status_code == 400
    assert "Unknown status: invalid" in str(exc.value.detail)


@pytest.mark.asyncio
async def test_create_protocol_non_admin_normalizes_payload(monkeypatch, mock_enums):
    """Create route should force trusted false and default metadata for non-admin."""

    pool = SimpleNamespace()
    auth = {"caller_type": "user", "scopes": []}
    payload = CreateProtocolBody(
        name="p4",
        title="P4",
        content="content",
        status="active",
        trusted=True,
        metadata=None,
    )
    execute = AsyncMock(return_value={"id": str(uuid4()), "name": "p4"})

    monkeypatch.setattr(
        "nebula_api.routes.protocols.require_status",
        lambda *_args, **_kwargs: None,
    )
    monkeypatch.setattr(
        "nebula_api.routes.protocols.maybe_check_agent_approval",
        AsyncMock(return_value=None),
    )
    monkeypatch.setattr("nebula_api.routes.protocols.execute_create_protocol", execute)

    result = await create_protocol(payload, _request(pool, mock_enums), auth=auth)

    data = execute.await_args.args[2]
    assert data["trusted"] is False
    assert data["notes"] == ""
    assert result["data"]["name"] == "p4"


@pytest.mark.asyncio
async def test_update_protocol_approval_short_circuit(monkeypatch, mock_enums):
    """Update route should return approval payload before executor call."""

    pool = SimpleNamespace()
    auth = {"caller_type": "agent", "agent": {"id": str(uuid4())}, "scopes": []}
    payload = UpdateProtocolBody(title="updated")
    execute = AsyncMock()

    monkeypatch.setattr(
        "nebula_api.routes.protocols.maybe_check_agent_approval",
        AsyncMock(return_value={"status": "approval_required", "approval_request_id": "a2"}),
    )
    monkeypatch.setattr("nebula_api.routes.protocols.execute_update_protocol", execute)

    result = await update_protocol("p1", payload, _request(pool, mock_enums), auth=auth)

    assert result["status"] == "approval_required"
    execute.assert_not_awaited()


@pytest.mark.asyncio
async def test_update_protocol_executor_valueerror_maps_400(monkeypatch, mock_enums):
    """Update route should map executor ValueError into HTTP 400."""

    pool = SimpleNamespace()
    auth = {"caller_type": "user", "scopes": []}
    payload = UpdateProtocolBody(title="updated")

    monkeypatch.setattr(
        "nebula_api.routes.protocols.maybe_check_agent_approval",
        AsyncMock(return_value=None),
    )
    monkeypatch.setattr(
        "nebula_api.routes.protocols.execute_update_protocol",
        AsyncMock(side_effect=ValueError("bad protocol update")),
    )

    with pytest.raises(HTTPException) as exc:
        await update_protocol("p1", payload, _request(pool, mock_enums), auth=auth)

    assert exc.value.status_code == 400
    assert "bad protocol update" in str(exc.value.detail)


@pytest.mark.asyncio
async def test_update_protocol_invalid_status_maps_400(monkeypatch, mock_enums):
    """Update route should map status validation errors to HTTP 400."""

    pool = SimpleNamespace()
    auth = {"caller_type": "user", "scopes": []}
    payload = UpdateProtocolBody(status="invalid")

    monkeypatch.setattr(
        "nebula_api.routes.protocols.require_status",
        lambda *_args, **_kwargs: (_ for _ in ()).throw(ValueError("Unknown status: invalid")),
    )

    with pytest.raises(HTTPException) as exc:
        await update_protocol("p1", payload, _request(pool, mock_enums), auth=auth)

    assert exc.value.status_code == 400
    assert "Unknown status: invalid" in str(exc.value.detail)


@pytest.mark.asyncio
async def test_update_protocol_non_admin_forces_trusted_false(monkeypatch, mock_enums):
    """Update route should clamp trusted flag for non-admin callers."""

    pool = SimpleNamespace()
    auth = {"caller_type": "user", "scopes": []}
    payload = UpdateProtocolBody(trusted=True)
    execute = AsyncMock(return_value={"id": str(uuid4()), "name": "p1", "trusted": False})

    monkeypatch.setattr(
        "nebula_api.routes.protocols.maybe_check_agent_approval",
        AsyncMock(return_value=None),
    )
    monkeypatch.setattr("nebula_api.routes.protocols.execute_update_protocol", execute)

    result = await update_protocol("p1", payload, _request(pool, mock_enums), auth=auth)

    data = execute.await_args.args[2]
    assert data["name"] == "p1"
    assert data["trusted"] is False
    assert result["data"]["trusted"] is False
