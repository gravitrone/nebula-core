"""Unit tests for import route helper edge branches."""

# Standard Library
from types import SimpleNamespace
from unittest.mock import AsyncMock
from uuid import uuid4

# Third-Party
import pytest

# Local
from nebula_api.routes.imports import (
    BulkImportBody,
    _has_write_scopes,
    _require_context_write_access,
    _require_entity_write_access,
    _require_job_owner,
    _run_import,
    _validate_relationship_node,
)

pytestmark = pytest.mark.unit


def _request(pool, enums):
    """Build a minimal request carrying app state values."""

    return SimpleNamespace(app=SimpleNamespace(state=SimpleNamespace(pool=pool, enums=enums)))


def _admin_auth(mock_enums):
    """Build admin-scoped auth payload."""

    return {"scopes": [mock_enums.scopes.name_to_id["admin"]], "caller_type": "user"}


def test_has_write_scopes_allows_empty_node_scopes():
    """Empty node scopes should always be writable."""

    assert _has_write_scopes([], []) is True


def test_has_write_scopes_denies_when_agent_has_no_scopes():
    """Missing caller scopes should deny non-empty node scopes."""

    assert _has_write_scopes([], ["public"]) is False


@pytest.mark.asyncio
async def test_require_entity_write_access_admin_short_circuits(mock_enums):
    """Admin callers should bypass entity write-scope lookup."""

    pool = SimpleNamespace(fetchrow=AsyncMock())

    await _require_entity_write_access(pool, mock_enums, _admin_auth(mock_enums), str(uuid4()))

    pool.fetchrow.assert_not_awaited()


@pytest.mark.asyncio
async def test_require_context_write_access_admin_short_circuits(mock_enums):
    """Admin callers should bypass context write-scope lookup."""

    pool = SimpleNamespace(fetchrow=AsyncMock())

    await _require_context_write_access(pool, mock_enums, _admin_auth(mock_enums), str(uuid4()))

    pool.fetchrow.assert_not_awaited()


@pytest.mark.asyncio
async def test_require_context_write_access_missing_context_raises(mock_enums):
    """Missing context rows should raise clear not-found errors."""

    pool = SimpleNamespace(fetchrow=AsyncMock(return_value=None))
    auth = {
        "scopes": [mock_enums.scopes.name_to_id["public"]],
        "caller_type": "user",
    }

    with pytest.raises(ValueError, match="Context not found"):
        await _require_context_write_access(pool, mock_enums, auth, str(uuid4()))


@pytest.mark.asyncio
async def test_require_entity_write_access_scope_denied_raises(mock_enums):
    """Entity writes should fail when caller lacks node scopes."""

    private_scope = mock_enums.scopes.name_to_id["private"]
    public_scope = mock_enums.scopes.name_to_id["public"]
    pool = SimpleNamespace(fetchrow=AsyncMock(return_value={"privacy_scope_ids": [private_scope]}))
    auth = {"scopes": [public_scope], "caller_type": "user"}

    with pytest.raises(ValueError, match="Access denied"):
        await _require_entity_write_access(pool, mock_enums, auth, str(uuid4()))


@pytest.mark.asyncio
async def test_require_context_write_access_scope_denied_raises(mock_enums):
    """Context writes should fail when caller lacks node scopes."""

    private_scope = mock_enums.scopes.name_to_id["private"]
    public_scope = mock_enums.scopes.name_to_id["public"]
    pool = SimpleNamespace(fetchrow=AsyncMock(return_value={"privacy_scope_ids": [private_scope]}))
    auth = {"scopes": [public_scope], "caller_type": "user"}

    with pytest.raises(ValueError, match="Access denied"):
        await _require_context_write_access(pool, mock_enums, auth, str(uuid4()))


@pytest.mark.asyncio
async def test_require_job_owner_missing_job_raises():
    """Missing job rows should raise clear not-found errors."""

    pool = SimpleNamespace(fetchrow=AsyncMock(return_value=None))
    auth = {"scopes": [], "caller_type": "user"}

    with pytest.raises(ValueError, match="Job not found"):
        await _require_job_owner(pool, auth, str(uuid4()))


@pytest.mark.asyncio
async def test_require_job_owner_scope_denied_raises(mock_enums):
    """Job writes should fail when caller lacks job scopes."""

    private_scope = mock_enums.scopes.name_to_id["private"]
    public_scope = mock_enums.scopes.name_to_id["public"]
    pool = SimpleNamespace(
        fetchrow=AsyncMock(return_value={"privacy_scope_ids": [private_scope], "agent_id": None})
    )
    auth = {"scopes": [public_scope], "caller_type": "user"}

    with pytest.raises(ValueError, match="Access denied"):
        await _require_job_owner(pool, auth, str(uuid4()))


@pytest.mark.asyncio
async def test_validate_relationship_node_context_dispatches(monkeypatch, mock_enums):
    """Context nodes should dispatch to context write-access checks."""

    marker = AsyncMock()
    monkeypatch.setattr("nebula_api.routes.imports._require_context_write_access", marker)

    await _validate_relationship_node(
        SimpleNamespace(),
        mock_enums,
        {"scopes": []},
        "context",
        str(uuid4()),
    )

    marker.assert_awaited_once()


@pytest.mark.asyncio
async def test_validate_relationship_node_job_dispatches(monkeypatch, mock_enums):
    """Job nodes should dispatch to job-owner checks."""

    marker = AsyncMock()
    monkeypatch.setattr("nebula_api.routes.imports._require_job_owner", marker)

    await _validate_relationship_node(
        SimpleNamespace(),
        mock_enums,
        {"scopes": []},
        "job",
        str(uuid4()),
    )

    marker.assert_awaited_once()


@pytest.mark.asyncio
async def test_run_import_returns_maybe_check_approval_response(monkeypatch, mock_enums):
    """Trusted/non-approval caller path should return maybe-check response when present."""

    pool = SimpleNamespace()
    request = _request(pool, mock_enums)
    payload = BulkImportBody(format="json", items=[{"name": "x"}])
    auth = {
        "caller_type": "user",
        "entity_id": str(uuid4()),
        "agent_id": None,
        "scopes": [mock_enums.scopes.name_to_id["public"]],
        "agent": {"requires_approval": False},
    }

    monkeypatch.setattr(
        "nebula_api.routes.imports.extract_items",
        lambda _format, _data, _items: [{"name": "x"}],
    )
    monkeypatch.setattr(
        "nebula_api.routes.imports.maybe_check_agent_approval",
        AsyncMock(return_value={"status": "approval_required", "approval_request_id": "a1"}),
    )

    result = await _run_import(
        request,
        auth,
        payload,
        lambda item, _defaults: item,
        AsyncMock(),
        "bulk_import_entities",
    )

    assert result["status"] == "approval_required"
    assert result["approval_request_id"] == "a1"
