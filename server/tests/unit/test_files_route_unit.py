"""Unit tests for file route helper and edge branches."""

# Standard Library
from types import SimpleNamespace
from unittest.mock import AsyncMock
from uuid import uuid4

# Third-Party
import pytest
from fastapi import HTTPException

# Local
from nebula_api.routes.files import (
    CreateFileBody,
    UpdateFileBody,
    _coerce_json_value,
    _file_visible,
    create_file,
    get_file,
    list_files,
    update_file,
)


pytestmark = pytest.mark.unit


def _request(pool, enums):
    """Build a minimal request carrying app state values."""

    return SimpleNamespace(app=SimpleNamespace(state=SimpleNamespace(pool=pool, enums=enums)))


def test_coerce_json_value_none_uses_fallback():
    """None payloads should return fallback values."""

    assert _coerce_json_value(None, {"x": 1}) == {"x": 1}


def test_coerce_json_value_invalid_json_uses_fallback():
    """Invalid JSON text should return fallback values."""

    assert _coerce_json_value("{bad", {"x": 1}) == {"x": 1}


def test_coerce_json_value_unexpected_type_uses_fallback():
    """Unsupported payload types should return fallback values."""

    assert _coerce_json_value(42, {"x": 1}) == {"x": 1}


@pytest.mark.asyncio
async def test_file_visible_false_when_related_entity_missing(mock_enums):
    """Missing related entity rows should hide files for non-admin callers."""

    pool = SimpleNamespace(
        fetch=AsyncMock(
            return_value=[
                {
                    "source_type": "entity",
                    "source_id": str(uuid4()),
                    "target_type": "file",
                    "target_id": str(uuid4()),
                }
            ]
        ),
        fetchrow=AsyncMock(return_value=None),
    )

    visible = await _file_visible(pool, mock_enums, {"scopes": []}, str(uuid4()))
    assert visible is False


@pytest.mark.asyncio
async def test_file_visible_false_when_related_context_missing(mock_enums):
    """Missing related context rows should hide files for non-admin callers."""

    pool = SimpleNamespace(
        fetch=AsyncMock(
            return_value=[
                {
                    "source_type": "context",
                    "source_id": str(uuid4()),
                    "target_type": "file",
                    "target_id": str(uuid4()),
                }
            ]
        ),
        fetchrow=AsyncMock(return_value=None),
    )

    visible = await _file_visible(pool, mock_enums, {"scopes": []}, str(uuid4()))
    assert visible is False


@pytest.mark.asyncio
async def test_file_visible_false_when_related_job_missing(mock_enums):
    """Missing related job rows should hide files for non-admin callers."""

    pool = SimpleNamespace(
        fetch=AsyncMock(
            return_value=[
                {
                    "source_type": "job",
                    "source_id": str(uuid4()),
                    "target_type": "file",
                    "target_id": str(uuid4()),
                }
            ]
        ),
        fetchrow=AsyncMock(return_value=None),
    )

    visible = await _file_visible(pool, mock_enums, {"scopes": []}, str(uuid4()))
    assert visible is False


@pytest.mark.asyncio
async def test_file_visible_admin_short_circuits(mock_enums):
    """Admin callers should bypass relationship visibility checks."""

    pool = SimpleNamespace(fetch=AsyncMock(), fetchrow=AsyncMock())
    auth = {"scopes": [mock_enums.scopes.name_to_id["admin"]]}

    visible = await _file_visible(pool, mock_enums, auth, str(uuid4()))

    assert visible is True
    pool.fetch.assert_not_awaited()


@pytest.mark.asyncio
async def test_file_visible_false_when_related_entity_scope_denied(mock_enums):
    """Entity relationship scopes should deny access when no scope overlap exists."""

    private_scope = mock_enums.scopes.name_to_id["private"]
    public_scope = mock_enums.scopes.name_to_id["public"]
    pool = SimpleNamespace(
        fetch=AsyncMock(
            return_value=[
                {
                    "source_type": "entity",
                    "source_id": str(uuid4()),
                    "target_type": "file",
                    "target_id": str(uuid4()),
                }
            ]
        ),
        fetchrow=AsyncMock(return_value={"privacy_scope_ids": [private_scope]}),
    )

    visible = await _file_visible(pool, mock_enums, {"scopes": [public_scope]}, str(uuid4()))
    assert visible is False


@pytest.mark.asyncio
async def test_file_visible_true_when_no_relationships(mock_enums):
    """Unlinked files should stay visible for non-admin callers."""

    pool = SimpleNamespace(fetch=AsyncMock(return_value=[]), fetchrow=AsyncMock())

    visible = await _file_visible(pool, mock_enums, {"scopes": []}, str(uuid4()))
    assert visible is True


@pytest.mark.asyncio
async def test_list_files_admin_returns_all_rows(mock_enums):
    """Admin callers should receive full file rows without visibility filtering."""

    pool = SimpleNamespace(
        fetch=AsyncMock(
            return_value=[{"id": str(uuid4()), "filename": "a.txt", "metadata": '{"k":1}'}]
        )
    )
    auth = {"scopes": [mock_enums.scopes.name_to_id["admin"]]}

    result = await list_files(_request(pool, mock_enums), auth=auth)

    assert result["data"][0]["filename"] == "a.txt"
    assert result["data"][0]["metadata"] == {"k": 1}


@pytest.mark.asyncio
async def test_list_files_non_admin_filters_hidden_rows(monkeypatch, mock_enums):
    """Non-admin callers should only receive visible rows."""

    visible_id = str(uuid4())
    hidden_id = str(uuid4())
    pool = SimpleNamespace(
        fetch=AsyncMock(
            return_value=[
                {"id": visible_id, "filename": "visible.txt", "metadata": "{}"},
                {"id": hidden_id, "filename": "hidden.txt", "metadata": "{}"},
            ]
        )
    )
    auth = {"scopes": [mock_enums.scopes.name_to_id["public"]]}

    async def _visible(_pool, _enums, _auth, file_id):
        return file_id == visible_id

    monkeypatch.setattr("nebula_api.routes.files._file_visible", _visible)

    result = await list_files(_request(pool, mock_enums), auth=auth)

    assert [row["filename"] for row in result["data"]] == ["visible.txt"]


@pytest.mark.asyncio
async def test_get_file_invalid_uuid_maps_400(mock_enums):
    """Invalid UUID file ids should return HTTP 400."""

    pool = SimpleNamespace(fetchrow=AsyncMock())

    with pytest.raises(HTTPException) as exc:
        await get_file("not-a-uuid", _request(pool, mock_enums), auth={"scopes": []})

    assert exc.value.status_code == 400
    assert exc.value.detail["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
async def test_get_file_missing_row_maps_404(mock_enums):
    """Unknown file ids should return HTTP 404."""

    pool = SimpleNamespace(fetchrow=AsyncMock(return_value=None))

    with pytest.raises(HTTPException) as exc:
        await get_file(str(uuid4()), _request(pool, mock_enums), auth={"scopes": []})

    assert exc.value.status_code == 404
    assert exc.value.detail["error"]["code"] == "NOT_FOUND"


@pytest.mark.asyncio
async def test_get_file_forbidden_maps_403(monkeypatch, mock_enums):
    """Hidden files should return HTTP 403."""

    file_id = str(uuid4())
    pool = SimpleNamespace(fetchrow=AsyncMock(return_value={"id": file_id, "metadata": {}}))
    monkeypatch.setattr("nebula_api.routes.files._file_visible", AsyncMock(return_value=False))

    with pytest.raises(HTTPException) as exc:
        await get_file(file_id, _request(pool, mock_enums), auth={"scopes": []})

    assert exc.value.status_code == 403
    assert exc.value.detail["error"]["code"] == "FORBIDDEN"


@pytest.mark.asyncio
async def test_create_file_metadata_validation_error_maps_400(monkeypatch, mock_enums):
    """Invalid metadata payloads should return HTTP 400."""

    pool = SimpleNamespace()
    payload = CreateFileBody(filename="f.txt", uri="file:///f.txt", status="active")

    monkeypatch.setattr(
        "nebula_api.routes.files.validate_metadata_payload",
        lambda _v: (_ for _ in ()).throw(ValueError("bad metadata")),
    )

    with pytest.raises(HTTPException) as exc:
        await create_file(payload, _request(pool, mock_enums), auth={"scopes": []})

    assert exc.value.status_code == 400
    assert exc.value.detail["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
async def test_create_file_requires_uri_or_path(mock_enums):
    """Create should reject payloads without both uri and file_path."""

    pool = SimpleNamespace()
    payload = CreateFileBody(filename="f.txt", status="active")

    with pytest.raises(HTTPException) as exc:
        await create_file(payload, _request(pool, mock_enums), auth={"scopes": []})

    assert exc.value.status_code == 400
    assert "uri or file_path is required" in exc.value.detail["error"]["message"]


@pytest.mark.asyncio
async def test_create_file_copies_file_path_into_uri(monkeypatch, mock_enums):
    """Create should mirror file_path into uri when uri is omitted."""

    pool = SimpleNamespace()
    payload = CreateFileBody(filename="f.txt", file_path="/tmp/f.txt", status="active")
    execute = AsyncMock(return_value={"id": str(uuid4()), "metadata": {}})

    monkeypatch.setattr(
        "nebula_api.routes.files.maybe_check_agent_approval",
        AsyncMock(return_value=None),
    )
    monkeypatch.setattr("nebula_api.routes.files.execute_create_file", execute)

    await create_file(payload, _request(pool, mock_enums), auth={"scopes": []})

    data = execute.await_args.args[2]
    assert data["uri"] == "/tmp/f.txt"


@pytest.mark.asyncio
async def test_create_file_invalid_status_maps_400(monkeypatch, mock_enums):
    """Status validation failures should map to HTTP 400."""

    pool = SimpleNamespace()
    payload = CreateFileBody(filename="f.txt", uri="file:///f.txt", status="bad-status")

    monkeypatch.setattr(
        "nebula_api.routes.files.require_status",
        lambda *_args, **_kwargs: (_ for _ in ()).throw(ValueError("bad status")),
    )

    with pytest.raises(HTTPException) as exc:
        await create_file(payload, _request(pool, mock_enums), auth={"scopes": []})

    assert exc.value.status_code == 400
    assert "bad status" in exc.value.detail["error"]["message"]


@pytest.mark.asyncio
async def test_create_file_approval_short_circuit(monkeypatch, mock_enums):
    """Create should return approval envelopes without calling executor."""

    pool = SimpleNamespace()
    payload = CreateFileBody(filename="f.txt", uri="file:///f.txt", status="active")
    execute = AsyncMock()

    monkeypatch.setattr(
        "nebula_api.routes.files.maybe_check_agent_approval",
        AsyncMock(return_value={"status": "approval_required", "approval_request_id": "a1"}),
    )
    monkeypatch.setattr("nebula_api.routes.files.execute_create_file", execute)

    result = await create_file(payload, _request(pool, mock_enums), auth={"scopes": []})

    assert result["status"] == "approval_required"
    execute.assert_not_awaited()


@pytest.mark.asyncio
async def test_create_file_success_returns_normalized_payload(monkeypatch, mock_enums):
    """Create should return normalized metadata on success."""

    pool = SimpleNamespace()
    payload = CreateFileBody(filename="f.txt", uri="file:///f.txt", status="active")

    monkeypatch.setattr(
        "nebula_api.routes.files.maybe_check_agent_approval",
        AsyncMock(return_value=None),
    )
    monkeypatch.setattr(
        "nebula_api.routes.files.execute_create_file",
        AsyncMock(return_value={"id": str(uuid4()), "filename": "f.txt", "metadata": '{"k":1}'}),
    )

    result = await create_file(payload, _request(pool, mock_enums), auth={"scopes": []})

    assert result["data"]["metadata"] == {"k": 1}


@pytest.mark.asyncio
async def test_create_file_executor_value_error_maps_400(monkeypatch, mock_enums):
    """Executor ValueErrors should map to HTTP 400 responses."""

    pool = SimpleNamespace()
    payload = CreateFileBody(filename="f.txt", uri="file:///f.txt", status="active")

    monkeypatch.setattr(
        "nebula_api.routes.files.maybe_check_agent_approval",
        AsyncMock(return_value=None),
    )
    monkeypatch.setattr(
        "nebula_api.routes.files.execute_create_file",
        AsyncMock(side_effect=ValueError("create failed")),
    )

    with pytest.raises(HTTPException) as exc:
        await create_file(payload, _request(pool, mock_enums), auth={"scopes": []})

    assert exc.value.status_code == 400
    assert exc.value.detail["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
async def test_update_file_invalid_uuid_maps_400(mock_enums):
    """Invalid UUID file ids should return HTTP 400 on update."""

    pool = SimpleNamespace()
    payload = UpdateFileBody(filename="new.txt")

    with pytest.raises(HTTPException) as exc:
        await update_file("not-a-uuid", payload, _request(pool, mock_enums), auth={"scopes": []})

    assert exc.value.status_code == 400
    assert exc.value.detail["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
async def test_update_file_metadata_validation_error_maps_400(monkeypatch, mock_enums):
    """Invalid update metadata should return HTTP 400."""

    file_id = str(uuid4())
    pool = SimpleNamespace()
    payload = UpdateFileBody(metadata={"x": 1})

    monkeypatch.setattr(
        "nebula_api.routes.files.validate_metadata_payload",
        lambda _v: (_ for _ in ()).throw(ValueError("bad metadata")),
    )

    with pytest.raises(HTTPException) as exc:
        await update_file(file_id, payload, _request(pool, mock_enums), auth={"scopes": []})

    assert exc.value.status_code == 400
    assert exc.value.detail["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
async def test_update_file_forbidden_maps_403(monkeypatch, mock_enums):
    """Invisible files should return HTTP 403 on update."""

    file_id = str(uuid4())
    pool = SimpleNamespace()
    payload = UpdateFileBody(filename="new.txt")

    monkeypatch.setattr("nebula_api.routes.files._file_visible", AsyncMock(return_value=False))

    with pytest.raises(HTTPException) as exc:
        await update_file(file_id, payload, _request(pool, mock_enums), auth={"scopes": []})

    assert exc.value.status_code == 403
    assert exc.value.detail["error"]["code"] == "FORBIDDEN"


@pytest.mark.asyncio
async def test_update_file_invalid_status_maps_400(monkeypatch, mock_enums):
    """Status validation failures should return HTTP 400 on update."""

    file_id = str(uuid4())
    pool = SimpleNamespace()
    payload = UpdateFileBody(status="bad-status")

    monkeypatch.setattr("nebula_api.routes.files._file_visible", AsyncMock(return_value=True))
    monkeypatch.setattr(
        "nebula_api.routes.files.require_status",
        lambda *_args, **_kwargs: (_ for _ in ()).throw(ValueError("bad status")),
    )

    with pytest.raises(HTTPException) as exc:
        await update_file(file_id, payload, _request(pool, mock_enums), auth={"scopes": []})

    assert exc.value.status_code == 400
    assert exc.value.detail["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
async def test_update_file_approval_short_circuit(monkeypatch, mock_enums):
    """Update should return approval payloads without calling executor."""

    file_id = str(uuid4())
    pool = SimpleNamespace()
    payload = UpdateFileBody(filename="new.txt")
    execute = AsyncMock()

    monkeypatch.setattr("nebula_api.routes.files._file_visible", AsyncMock(return_value=True))
    monkeypatch.setattr(
        "nebula_api.routes.files.maybe_check_agent_approval",
        AsyncMock(return_value={"status": "approval_required", "approval_request_id": "a2"}),
    )
    monkeypatch.setattr("nebula_api.routes.files.execute_update_file", execute)

    result = await update_file(file_id, payload, _request(pool, mock_enums), auth={"scopes": []})

    assert result["status"] == "approval_required"
    execute.assert_not_awaited()


@pytest.mark.asyncio
async def test_update_file_copies_file_path_into_uri(monkeypatch, mock_enums):
    """Update should mirror file_path into uri when uri is omitted."""

    file_id = str(uuid4())
    pool = SimpleNamespace()
    payload = UpdateFileBody(file_path="/tmp/a.txt")
    execute = AsyncMock(return_value={"id": file_id, "file_path": "/tmp/a.txt", "metadata": {}})

    monkeypatch.setattr("nebula_api.routes.files._file_visible", AsyncMock(return_value=True))
    monkeypatch.setattr(
        "nebula_api.routes.files.maybe_check_agent_approval",
        AsyncMock(return_value=None),
    )
    monkeypatch.setattr("nebula_api.routes.files.execute_update_file", execute)

    await update_file(file_id, payload, _request(pool, mock_enums), auth={"scopes": []})

    data = execute.await_args.args[2]
    assert data["uri"] == "/tmp/a.txt"


@pytest.mark.asyncio
async def test_update_file_copies_uri_into_file_path(monkeypatch, mock_enums):
    """Update should mirror uri into file_path when file_path is omitted."""

    file_id = str(uuid4())
    pool = SimpleNamespace()
    payload = UpdateFileBody(uri="file:///tmp/a.txt")
    execute = AsyncMock(
        return_value={"id": file_id, "uri": "file:///tmp/a.txt", "file_path": "file:///tmp/a.txt", "metadata": {}}
    )

    monkeypatch.setattr("nebula_api.routes.files._file_visible", AsyncMock(return_value=True))
    monkeypatch.setattr(
        "nebula_api.routes.files.maybe_check_agent_approval",
        AsyncMock(return_value=None),
    )
    monkeypatch.setattr("nebula_api.routes.files.execute_update_file", execute)

    await update_file(file_id, payload, _request(pool, mock_enums), auth={"scopes": []})

    data = execute.await_args.args[2]
    assert data["file_path"] == "file:///tmp/a.txt"


@pytest.mark.asyncio
async def test_update_file_executor_value_error_maps_400(monkeypatch, mock_enums):
    """Update executor ValueErrors should map to HTTP 400 responses."""

    file_id = str(uuid4())
    pool = SimpleNamespace()
    payload = UpdateFileBody(filename="new.txt")

    monkeypatch.setattr("nebula_api.routes.files._file_visible", AsyncMock(return_value=True))
    monkeypatch.setattr(
        "nebula_api.routes.files.maybe_check_agent_approval",
        AsyncMock(return_value=None),
    )
    monkeypatch.setattr(
        "nebula_api.routes.files.execute_update_file",
        AsyncMock(side_effect=ValueError("update failed")),
    )

    with pytest.raises(HTTPException) as exc:
        await update_file(file_id, payload, _request(pool, mock_enums), auth={"scopes": []})

    assert exc.value.status_code == 400
    assert exc.value.detail["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
async def test_update_file_missing_row_maps_404(monkeypatch, mock_enums):
    """Empty update results should map to HTTP 404 responses."""

    file_id = str(uuid4())
    pool = SimpleNamespace()
    payload = UpdateFileBody(filename="new.txt")

    monkeypatch.setattr("nebula_api.routes.files._file_visible", AsyncMock(return_value=True))
    monkeypatch.setattr(
        "nebula_api.routes.files.maybe_check_agent_approval",
        AsyncMock(return_value=None),
    )
    monkeypatch.setattr(
        "nebula_api.routes.files.execute_update_file",
        AsyncMock(return_value={}),
    )

    with pytest.raises(HTTPException) as exc:
        await update_file(file_id, payload, _request(pool, mock_enums), auth={"scopes": []})

    assert exc.value.status_code == 404
    assert exc.value.detail["error"]["code"] == "NOT_FOUND"
