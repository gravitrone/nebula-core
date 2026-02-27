"""Unit tests for log route edge branches."""

# Standard Library
from types import SimpleNamespace
from unittest.mock import AsyncMock
from uuid import uuid4

# Third-Party
import pytest
from fastapi import HTTPException

# Local
from nebula_api.routes.logs import (
    CreateLogBody,
    UpdateLogBody,
    _coerce_json_value,
    _log_visible,
    create_log,
    query_logs,
    update_log,
)


pytestmark = pytest.mark.unit


def _request(pool, enums):
    """Build a minimal request carrying app state values."""

    return SimpleNamespace(app=SimpleNamespace(state=SimpleNamespace(pool=pool, enums=enums)))


def _admin_auth(mock_enums):
    """Build an admin-scoped auth payload."""

    return {"scopes": [mock_enums.scopes.name_to_id["admin"]]}


def test_coerce_json_value_none_returns_fallback():
    """None payloads should return fallback values."""

    assert _coerce_json_value(None, {"x": 1}) == {"x": 1}


def test_coerce_json_value_dict_returns_as_is():
    """Object payloads should pass through unchanged."""

    payload = {"x": 1}
    assert _coerce_json_value(payload, {}) == payload


def test_coerce_json_value_unexpected_type_returns_fallback():
    """Unsupported payload types should return fallback values."""

    assert _coerce_json_value(7, {"x": 1}) == {"x": 1}


def test_coerce_json_value_invalid_json_returns_fallback():
    """Invalid JSON text should return fallback values."""

    assert _coerce_json_value("{bad", {"x": 1}) == {"x": 1}


@pytest.mark.asyncio
async def test_log_visible_admin_short_circuits(mock_enums):
    """Admin callers should bypass relationship visibility checks."""

    pool = SimpleNamespace(fetch=AsyncMock(), fetchrow=AsyncMock())

    visible = await _log_visible(pool, mock_enums, _admin_auth(mock_enums), str(uuid4()))

    assert visible is True
    pool.fetch.assert_not_awaited()


@pytest.mark.asyncio
async def test_log_visible_false_when_related_entity_missing(mock_enums):
    """Missing related entities should hide logs for non-admin callers."""

    pool = SimpleNamespace(
        fetch=AsyncMock(
            return_value=[
                {
                    "source_type": "entity",
                    "source_id": str(uuid4()),
                    "target_type": "log",
                    "target_id": str(uuid4()),
                }
            ]
        ),
        fetchrow=AsyncMock(return_value=None),
    )

    visible = await _log_visible(pool, mock_enums, {"scopes": []}, str(uuid4()))
    assert visible is False


@pytest.mark.asyncio
async def test_log_visible_false_when_related_context_missing(mock_enums):
    """Missing related context rows should hide logs for non-admin callers."""

    pool = SimpleNamespace(
        fetch=AsyncMock(
            return_value=[
                {
                    "source_type": "context",
                    "source_id": str(uuid4()),
                    "target_type": "log",
                    "target_id": str(uuid4()),
                }
            ]
        ),
        fetchrow=AsyncMock(return_value=None),
    )

    visible = await _log_visible(pool, mock_enums, {"scopes": []}, str(uuid4()))
    assert visible is False


@pytest.mark.asyncio
async def test_log_visible_false_when_related_job_missing(mock_enums):
    """Missing related job rows should hide logs for non-admin callers."""

    pool = SimpleNamespace(
        fetch=AsyncMock(
            return_value=[
                {
                    "source_type": "job",
                    "source_id": str(uuid4()),
                    "target_type": "log",
                    "target_id": str(uuid4()),
                }
            ]
        ),
        fetchrow=AsyncMock(return_value=None),
    )

    visible = await _log_visible(pool, mock_enums, {"scopes": []}, str(uuid4()))
    assert visible is False


@pytest.mark.asyncio
async def test_create_log_executor_valueerror_maps_400(monkeypatch, mock_enums):
    """Create executor ValueErrors should map to HTTP 400."""

    pool = SimpleNamespace()
    payload = CreateLogBody(log_type="note", status="active")

    monkeypatch.setattr(
        "nebula_api.routes.logs.maybe_check_agent_approval",
        AsyncMock(return_value=None),
    )
    monkeypatch.setattr(
        "nebula_api.routes.logs.execute_create_log",
        AsyncMock(side_effect=ValueError("create failed")),
    )

    with pytest.raises(HTTPException) as exc:
        await create_log(payload, _request(pool, mock_enums), auth={"scopes": []})

    assert exc.value.status_code == 400
    assert exc.value.detail["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
async def test_create_log_metadata_validation_error_maps_400(monkeypatch, mock_enums):
    """Invalid metadata payloads should return HTTP 400."""

    pool = SimpleNamespace()
    payload = CreateLogBody(log_type="note", status="active", metadata={"bad": True})

    monkeypatch.setattr(
        "nebula_api.routes.logs.validate_metadata_payload",
        lambda _v: (_ for _ in ()).throw(ValueError("bad metadata")),
    )

    with pytest.raises(HTTPException) as exc:
        await create_log(payload, _request(pool, mock_enums), auth={"scopes": []})

    assert exc.value.status_code == 400
    assert exc.value.detail["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
async def test_create_log_approval_short_circuit(monkeypatch, mock_enums):
    """Create should return approval envelopes without calling executor."""

    pool = SimpleNamespace()
    payload = CreateLogBody(log_type="note", status="active")
    execute = AsyncMock()

    monkeypatch.setattr(
        "nebula_api.routes.logs.maybe_check_agent_approval",
        AsyncMock(return_value={"status": "approval_required", "approval_request_id": "a1"}),
    )
    monkeypatch.setattr("nebula_api.routes.logs.execute_create_log", execute)

    result = await create_log(payload, _request(pool, mock_enums), auth={"scopes": []})

    assert result["status"] == "approval_required"
    execute.assert_not_awaited()


@pytest.mark.asyncio
async def test_query_logs_admin_returns_all_rows(mock_enums):
    """Admin callers should receive rows without visibility filtering."""

    pool = SimpleNamespace(
        fetch=AsyncMock(
            return_value=[{"id": str(uuid4()), "value": '{"k":1}', "metadata": "{}"}]
        )
    )
    result = await query_logs(_request(pool, mock_enums), auth=_admin_auth(mock_enums))

    assert len(result["data"]) == 1
    assert result["data"][0]["value"] == {"k": 1}


@pytest.mark.asyncio
async def test_update_log_metadata_validation_error_maps_400(monkeypatch, mock_enums):
    """Invalid metadata payloads should return HTTP 400 on update."""

    log_id = str(uuid4())
    pool = SimpleNamespace()
    payload = UpdateLogBody(metadata={"bad": True})

    monkeypatch.setattr(
        "nebula_api.routes.logs.validate_metadata_payload",
        lambda _v: (_ for _ in ()).throw(ValueError("bad metadata")),
    )

    with pytest.raises(HTTPException) as exc:
        await update_log(log_id, payload, _request(pool, mock_enums), auth={"scopes": []})

    assert exc.value.status_code == 400
    assert exc.value.detail["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
async def test_update_log_forbidden_when_not_visible(monkeypatch, mock_enums):
    """Hidden logs should return HTTP 403 on update."""

    log_id = str(uuid4())
    pool = SimpleNamespace()
    payload = UpdateLogBody(status="active")

    monkeypatch.setattr("nebula_api.routes.logs._log_visible", AsyncMock(return_value=False))

    with pytest.raises(HTTPException) as exc:
        await update_log(log_id, payload, _request(pool, mock_enums), auth={"scopes": []})

    assert exc.value.status_code == 403
    assert exc.value.detail["error"]["code"] == "FORBIDDEN"


@pytest.mark.asyncio
async def test_update_log_applies_payload_log_type_before_executor(monkeypatch, mock_enums):
    """Update should copy payload.log_type into executor payload."""

    log_id = str(uuid4())
    pool = SimpleNamespace()
    payload = UpdateLogBody(log_type="note")
    execute = AsyncMock(return_value={"id": log_id, "value": {}, "metadata": {}})

    monkeypatch.setattr("nebula_api.routes.logs._log_visible", AsyncMock(return_value=True))
    monkeypatch.setattr(
        "nebula_api.routes.logs.maybe_check_agent_approval",
        AsyncMock(return_value=None),
    )
    monkeypatch.setattr("nebula_api.routes.logs.execute_update_log", execute)

    await update_log(log_id, payload, _request(pool, mock_enums), auth={"scopes": []})

    data = execute.await_args.args[2]
    assert data["log_type"] == "note"


@pytest.mark.asyncio
async def test_update_log_approval_short_circuit(monkeypatch, mock_enums):
    """Update should return approval envelopes without calling executor."""

    log_id = str(uuid4())
    pool = SimpleNamespace()
    payload = UpdateLogBody(status="active")
    execute = AsyncMock()

    monkeypatch.setattr("nebula_api.routes.logs._log_visible", AsyncMock(return_value=True))
    monkeypatch.setattr(
        "nebula_api.routes.logs.maybe_check_agent_approval",
        AsyncMock(return_value={"status": "approval_required", "approval_request_id": "a2"}),
    )
    monkeypatch.setattr("nebula_api.routes.logs.execute_update_log", execute)

    result = await update_log(log_id, payload, _request(pool, mock_enums), auth={"scopes": []})

    assert result["status"] == "approval_required"
    execute.assert_not_awaited()


@pytest.mark.asyncio
async def test_update_log_executor_valueerror_maps_400(monkeypatch, mock_enums):
    """Update executor ValueErrors should map to HTTP 400."""

    log_id = str(uuid4())
    pool = SimpleNamespace()
    payload = UpdateLogBody(status="active")

    monkeypatch.setattr("nebula_api.routes.logs._log_visible", AsyncMock(return_value=True))
    monkeypatch.setattr(
        "nebula_api.routes.logs.maybe_check_agent_approval",
        AsyncMock(return_value=None),
    )
    monkeypatch.setattr(
        "nebula_api.routes.logs.execute_update_log",
        AsyncMock(side_effect=ValueError("update failed")),
    )

    with pytest.raises(HTTPException) as exc:
        await update_log(log_id, payload, _request(pool, mock_enums), auth={"scopes": []})

    assert exc.value.status_code == 400
    assert exc.value.detail["error"]["code"] == "INVALID_INPUT"
