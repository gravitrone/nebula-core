"""Unit tests for job route edge branches."""

# Standard Library
from types import SimpleNamespace
from unittest.mock import AsyncMock
from uuid import uuid4

# Third-Party
import pytest
from fastapi import HTTPException

# Local
from nebula_api.routes.jobs import (
    CreateJobBody,
    CreateSubtaskBody,
    UpdateJobBody,
    UpdateJobStatusBody,
    _require_job_write,
    create_job,
    create_subtask,
    update_job,
    update_job_status,
)


pytestmark = pytest.mark.unit


def _request(pool, enums):
    """Build a minimal request carrying app state values."""

    return SimpleNamespace(app=SimpleNamespace(state=SimpleNamespace(pool=pool, enums=enums)))


def _admin_auth(mock_enums):
    """Build an admin-scoped auth payload."""

    return {
        "caller_type": "user",
        "entity_id": str(uuid4()),
        "agent_id": None,
        "agent": {"requires_approval": False},
        "scopes": [mock_enums.scopes.name_to_id["admin"]],
    }


def test_require_job_write_user_assigned_to_other_entity_forbidden(mock_enums):
    """User callers should be denied when assigned_to belongs to someone else."""

    job = {
        "privacy_scope_ids": [mock_enums.scopes.name_to_id["public"]],
        "agent_id": None,
        "assigned_to": str(uuid4()),
    }
    auth = {
        "caller_type": "user",
        "entity_id": str(uuid4()),
        "scopes": [mock_enums.scopes.name_to_id["public"]],
    }

    with pytest.raises(HTTPException) as exc:
        _require_job_write(auth, mock_enums, job)

    assert exc.value.status_code == 403
    assert exc.value.detail["error"]["code"] == "FORBIDDEN"


def test_require_job_write_user_cannot_write_agent_job(mock_enums):
    """User callers should be denied when job belongs to an agent."""

    job = {
        "privacy_scope_ids": [mock_enums.scopes.name_to_id["public"]],
        "agent_id": str(uuid4()),
        "assigned_to": None,
    }
    auth = {
        "caller_type": "user",
        "entity_id": str(uuid4()),
        "scopes": [mock_enums.scopes.name_to_id["public"]],
    }

    with pytest.raises(HTTPException) as exc:
        _require_job_write(auth, mock_enums, job)

    assert exc.value.status_code == 403
    assert exc.value.detail["error"]["code"] == "FORBIDDEN"


def test_require_job_write_agent_mismatch_forbidden(mock_enums):
    """Agent callers should be denied for foreign-agent jobs."""

    job = {
        "privacy_scope_ids": [mock_enums.scopes.name_to_id["public"]],
        "agent_id": str(uuid4()),
        "assigned_to": None,
    }
    auth = {
        "caller_type": "agent",
        "agent_id": str(uuid4()),
        "scopes": [mock_enums.scopes.name_to_id["public"]],
    }

    with pytest.raises(HTTPException) as exc:
        _require_job_write(auth, mock_enums, job)

    assert exc.value.status_code == 403
    assert exc.value.detail["error"]["code"] == "FORBIDDEN"


@pytest.mark.asyncio
async def test_create_job_agent_scope_subset_error_maps_400(monkeypatch, mock_enums):
    """Agent job create should map enforce_scope_subset failures to HTTP 400."""

    pool = SimpleNamespace()
    payload = CreateJobBody(title="job", scopes=["private"])
    auth = {
        "caller_type": "agent",
        "agent_id": str(uuid4()),
        "agent": {"requires_approval": False},
        "scopes": [mock_enums.scopes.name_to_id["public"]],
    }

    monkeypatch.setattr(
        "nebula_api.routes.jobs.enforce_scope_subset",
        lambda *_args, **_kwargs: (_ for _ in ()).throw(ValueError("scope denied")),
    )

    with pytest.raises(HTTPException) as exc:
        await create_job(payload, _request(pool, mock_enums), auth=auth)

    assert exc.value.status_code == 400
    assert exc.value.detail["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
async def test_create_job_invalid_scopes_map_400(monkeypatch, mock_enums):
    """Scope validation errors should map to HTTP 400 during create."""

    pool = SimpleNamespace()
    payload = CreateJobBody(title="job", scopes=["public"])
    auth = _admin_auth(mock_enums)

    monkeypatch.setattr(
        "nebula_api.routes.jobs.require_scopes",
        lambda *_args, **_kwargs: (_ for _ in ()).throw(ValueError("bad scope")),
    )

    with pytest.raises(HTTPException) as exc:
        await create_job(payload, _request(pool, mock_enums), auth=auth)

    assert exc.value.status_code == 400
    assert exc.value.detail["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
async def test_create_job_metadata_validation_error_maps_400(monkeypatch, mock_enums):
    """Metadata validation failures should map to HTTP 400 during create."""

    pool = SimpleNamespace()
    payload = CreateJobBody(title="job", scopes=["public"], metadata={"bad": True})
    auth = _admin_auth(mock_enums)

    monkeypatch.setattr(
        "nebula_api.routes.jobs.validate_metadata_payload",
        lambda _v: (_ for _ in ()).throw(ValueError("bad metadata")),
    )

    with pytest.raises(HTTPException) as exc:
        await create_job(payload, _request(pool, mock_enums), auth=auth)

    assert exc.value.status_code == 400
    assert exc.value.detail["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
async def test_create_job_user_cannot_set_agent_id(mock_enums):
    """User callers should be forbidden from setting agent_id explicitly."""

    pool = SimpleNamespace()
    payload = CreateJobBody(title="job", scopes=["public"], agent_id=str(uuid4()))
    auth = {
        "caller_type": "user",
        "entity_id": str(uuid4()),
        "agent_id": None,
        "agent": {"requires_approval": False},
        "scopes": [mock_enums.scopes.name_to_id["public"]],
    }

    with pytest.raises(HTTPException) as exc:
        await create_job(payload, _request(pool, mock_enums), auth=auth)

    assert exc.value.status_code == 403
    assert exc.value.detail["error"]["code"] == "FORBIDDEN"


@pytest.mark.asyncio
async def test_create_job_executor_valueerror_maps_400(monkeypatch, mock_enums):
    """Executor ValueErrors should map to HTTP 400 during create."""

    pool = SimpleNamespace()
    payload = CreateJobBody(title="job", scopes=["public"])
    auth = _admin_auth(mock_enums)

    monkeypatch.setattr(
        "nebula_api.routes.jobs.maybe_check_agent_approval",
        AsyncMock(return_value=None),
    )
    monkeypatch.setattr(
        "nebula_api.routes.jobs.execute_create_job",
        AsyncMock(side_effect=ValueError("create failed")),
    )

    with pytest.raises(HTTPException) as exc:
        await create_job(payload, _request(pool, mock_enums), auth=auth)

    assert exc.value.status_code == 400
    assert exc.value.detail["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
async def test_update_job_status_missing_job_maps_404(mock_enums):
    """Status updates should return NOT_FOUND when job lookup fails."""

    pool = SimpleNamespace(fetchrow=AsyncMock(return_value=None))
    payload = UpdateJobStatusBody(status="active")

    with pytest.raises(HTTPException) as exc:
        await update_job_status(str(uuid4()), payload, _request(pool, mock_enums), auth=_admin_auth(mock_enums))

    assert exc.value.status_code == 404
    assert exc.value.detail["error"]["code"] == "NOT_FOUND"


@pytest.mark.asyncio
async def test_update_job_status_second_lookup_missing_maps_404(monkeypatch, mock_enums):
    """Status updates should return NOT_FOUND when update query returns no row."""

    pool = SimpleNamespace(
        fetchrow=AsyncMock(
            side_effect=[
                {"id": str(uuid4()), "privacy_scope_ids": [], "agent_id": None},
                None,
            ]
        )
    )
    payload = UpdateJobStatusBody(status="active")

    monkeypatch.setattr(
        "nebula_api.routes.jobs.maybe_check_agent_approval",
        AsyncMock(return_value=None),
    )
    monkeypatch.setattr(
        "nebula_api.routes.jobs.require_status",
        lambda *_args, **_kwargs: uuid4(),
    )

    with pytest.raises(HTTPException) as exc:
        await update_job_status(str(uuid4()), payload, _request(pool, mock_enums), auth=_admin_auth(mock_enums))

    assert exc.value.status_code == 404
    assert exc.value.detail["error"]["code"] == "NOT_FOUND"


@pytest.mark.asyncio
async def test_update_job_missing_job_maps_404(mock_enums):
    """Job updates should return NOT_FOUND when base row is missing."""

    pool = SimpleNamespace(fetchrow=AsyncMock(return_value=None))
    payload = UpdateJobBody(description="x")

    with pytest.raises(HTTPException) as exc:
        await update_job(str(uuid4()), payload, _request(pool, mock_enums), auth=_admin_auth(mock_enums))

    assert exc.value.status_code == 404
    assert exc.value.detail["error"]["code"] == "NOT_FOUND"


@pytest.mark.asyncio
async def test_update_job_invalid_status_maps_400(monkeypatch, mock_enums):
    """Invalid status values should map to HTTP 400 during update."""

    pool = SimpleNamespace(
        fetchrow=AsyncMock(return_value={"id": str(uuid4()), "privacy_scope_ids": [], "agent_id": None})
    )
    payload = UpdateJobBody(status="bad")

    monkeypatch.setattr(
        "nebula_api.routes.jobs.require_status",
        lambda *_args, **_kwargs: (_ for _ in ()).throw(ValueError("bad status")),
    )

    with pytest.raises(HTTPException) as exc:
        await update_job(str(uuid4()), payload, _request(pool, mock_enums), auth=_admin_auth(mock_enums))

    assert exc.value.status_code == 400
    assert exc.value.detail["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
async def test_update_job_metadata_validation_error_maps_400(monkeypatch, mock_enums):
    """Metadata validation failures should map to HTTP 400 during update."""

    pool = SimpleNamespace(
        fetchrow=AsyncMock(return_value={"id": str(uuid4()), "privacy_scope_ids": [], "agent_id": None})
    )
    payload = UpdateJobBody(metadata={"bad": True})

    monkeypatch.setattr(
        "nebula_api.routes.jobs.validate_metadata_payload",
        lambda _v: (_ for _ in ()).throw(ValueError("bad metadata")),
    )

    with pytest.raises(HTTPException) as exc:
        await update_job(str(uuid4()), payload, _request(pool, mock_enums), auth=_admin_auth(mock_enums))

    assert exc.value.status_code == 400
    assert exc.value.detail["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
async def test_update_job_executor_valueerror_maps_400(monkeypatch, mock_enums):
    """Executor ValueErrors should map to HTTP 400 during update."""

    pool = SimpleNamespace(
        fetchrow=AsyncMock(return_value={"id": str(uuid4()), "privacy_scope_ids": [], "agent_id": None})
    )
    payload = UpdateJobBody(description="x")

    monkeypatch.setattr(
        "nebula_api.routes.jobs.maybe_check_agent_approval",
        AsyncMock(return_value=None),
    )
    monkeypatch.setattr(
        "nebula_api.routes.jobs.execute_update_job",
        AsyncMock(side_effect=ValueError("update failed")),
    )

    with pytest.raises(HTTPException) as exc:
        await update_job(str(uuid4()), payload, _request(pool, mock_enums), auth=_admin_auth(mock_enums))

    assert exc.value.status_code == 400
    assert exc.value.detail["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
async def test_create_subtask_executor_valueerror_maps_400(monkeypatch, mock_enums):
    """Subtask create should map executor ValueErrors to HTTP 400."""

    parent_id = str(uuid4())
    pool = SimpleNamespace(
        fetchrow=AsyncMock(return_value={"id": parent_id, "privacy_scope_ids": [], "agent_id": None})
    )
    payload = CreateSubtaskBody(title="child")

    monkeypatch.setattr(
        "nebula_api.routes.jobs.maybe_check_agent_approval",
        AsyncMock(return_value=None),
    )
    monkeypatch.setattr(
        "nebula_api.routes.jobs.execute_create_job",
        AsyncMock(side_effect=ValueError("subtask failed")),
    )

    with pytest.raises(HTTPException) as exc:
        await create_subtask(parent_id, payload, _request(pool, mock_enums), auth=_admin_auth(mock_enums))

    assert exc.value.status_code == 400
    assert exc.value.detail["error"]["code"] == "INVALID_INPUT"
