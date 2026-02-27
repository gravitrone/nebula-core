"""Focused unit tests for nebula_api.auth edge branches."""

# Standard Library
import json
from types import SimpleNamespace
from unittest.mock import AsyncMock
from uuid import uuid4

# Third-Party
import pytest
from argon2.exceptions import VerifyMismatchError
from fastapi import HTTPException

# Local
from nebula_api.auth import (
    _merge_scopes,
    generate_api_key,
    maybe_check_agent_approval,
    require_auth,
)


pytestmark = pytest.mark.unit


def _request_with_pool(pool, raw_key: str):
    """Build minimal request object used by require_auth."""

    return SimpleNamespace(
        headers={"Authorization": f"Bearer {raw_key}"},
        app=SimpleNamespace(state=SimpleNamespace(pool=pool)),
    )


def _pool_stub(*, fetchrow_side_effect):
    """Build a simple pool stub with async fetch/execute methods."""

    return SimpleNamespace(
        fetchrow=AsyncMock(side_effect=fetchrow_side_effect),
        execute=AsyncMock(),
    )


def test_merge_scopes_intersects_key_and_owner_scopes():
    """Key scopes should narrow the effective owner scope set."""

    assert _merge_scopes(["public", "admin"], ["public", "private"]) == ["public"]


def test_merge_scopes_returns_owner_scopes_when_key_scopes_missing():
    """Missing key scopes should fallback to all owner scopes."""

    assert _merge_scopes(None, ["public", "private"]) == ["public", "private"]


def test_generate_api_key_returns_prefixed_key_and_hash(monkeypatch):
    """generate_api_key should produce prefixed raw key and matching prefix/hash."""

    monkeypatch.setattr("nebula_api.auth.secrets.token_urlsafe", lambda _n: "abc123")
    monkeypatch.setattr(
        "nebula_api.auth.ph",
        SimpleNamespace(hash=lambda raw: f"hash:{raw}"),
    )

    raw, prefix, key_hash = generate_api_key()

    assert raw == "nbl_abc123"
    assert prefix == raw[:8]
    assert key_hash == f"hash:{raw}"


@pytest.mark.asyncio
async def test_require_auth_missing_header_raises_401():
    """Missing Authorization header should raise HTTP 401."""

    request = SimpleNamespace(
        headers={},
        app=SimpleNamespace(state=SimpleNamespace(pool=SimpleNamespace())),
    )

    with pytest.raises(HTTPException) as exc:
        await require_auth(request)

    assert exc.value.status_code == 401
    assert exc.value.detail == "Missing API key"


@pytest.mark.asyncio
async def test_require_auth_short_key_raises_401():
    """Bearer keys shorter than 8 chars should fail fast."""

    request = _request_with_pool(SimpleNamespace(), "short")

    with pytest.raises(HTTPException) as exc:
        await require_auth(request)

    assert exc.value.status_code == 401
    assert exc.value.detail == "Invalid API key"


@pytest.mark.asyncio
async def test_require_auth_missing_prefix_row_raises_401():
    """Unknown key prefixes should return invalid key errors."""

    pool = _pool_stub(fetchrow_side_effect=[None])
    request = _request_with_pool(pool, "nbl_1234567890")

    with pytest.raises(HTTPException) as exc:
        await require_auth(request)

    assert exc.value.status_code == 401
    assert exc.value.detail == "Invalid API key"


@pytest.mark.asyncio
async def test_require_auth_hash_mismatch_raises_401(monkeypatch):
    """Argon mismatch should map to generic invalid-key response."""

    key_row = {
        "id": str(uuid4()),
        "key_hash": "hash",
        "entity_id": None,
        "agent_id": str(uuid4()),
        "scopes": None,
    }
    pool = _pool_stub(fetchrow_side_effect=[key_row])
    request = _request_with_pool(pool, "nbl_1234567890")

    monkeypatch.setattr(
        "nebula_api.auth.ph",
        SimpleNamespace(verify=lambda *_args: (_ for _ in ()).throw(VerifyMismatchError())),
    )

    with pytest.raises(HTTPException) as exc:
        await require_auth(request)

    assert exc.value.status_code == 401
    assert exc.value.detail == "Invalid API key"


@pytest.mark.asyncio
async def test_require_auth_agent_not_found_raises_401(monkeypatch):
    """Agent-owned key with missing agent row should return 401."""

    key_row = {
        "id": str(uuid4()),
        "key_hash": "hash",
        "entity_id": None,
        "agent_id": str(uuid4()),
        "scopes": None,
    }
    pool = _pool_stub(fetchrow_side_effect=[key_row, None])
    request = _request_with_pool(pool, "nbl_1234567890")

    monkeypatch.setattr(
        "nebula_api.auth.ph",
        SimpleNamespace(verify=lambda *_args: True),
    )

    with pytest.raises(HTTPException) as exc:
        await require_auth(request)

    assert exc.value.status_code == 401
    assert exc.value.detail == "Agent not found or inactive"


@pytest.mark.asyncio
async def test_require_auth_key_without_owner_raises_401(monkeypatch):
    """Keys missing both entity_id and agent_id should be rejected."""

    key_row = {
        "id": str(uuid4()),
        "key_hash": "hash",
        "entity_id": None,
        "agent_id": None,
        "scopes": None,
    }
    pool = _pool_stub(fetchrow_side_effect=[key_row])
    request = _request_with_pool(pool, "nbl_0987654321")

    monkeypatch.setattr(
        "nebula_api.auth.ph",
        SimpleNamespace(verify=lambda *_args: True),
    )

    with pytest.raises(HTTPException) as exc:
        await require_auth(request)

    assert exc.value.status_code == 401
    assert exc.value.detail == "Invalid API key"


@pytest.mark.asyncio
async def test_require_auth_user_owner_returns_user_context(monkeypatch):
    """Entity-owned key should return user caller context and merged scopes."""

    key_row = {
        "id": str(uuid4()),
        "key_hash": "hash",
        "entity_id": str(uuid4()),
        "agent_id": None,
        "scopes": ["public", "admin"],
    }
    entity_row = {"id": key_row["entity_id"], "privacy_scope_ids": ["public", "private"]}
    pool = _pool_stub(fetchrow_side_effect=[key_row, entity_row])
    request = _request_with_pool(pool, "nbl_1234567890")

    monkeypatch.setattr(
        "nebula_api.auth.ph",
        SimpleNamespace(verify=lambda *_args: True),
    )

    result = await require_auth(request)

    assert result["caller_type"] == "user"
    assert result["entity_id"] == key_row["entity_id"]
    assert result["agent_id"] is None
    assert result["scopes"] == ["public"]
    pool.execute.assert_awaited_once()


@pytest.mark.asyncio
async def test_require_auth_user_owner_missing_entity_returns_none_entity(monkeypatch):
    """Entity-owned keys should tolerate missing entity rows."""

    key_row = {
        "id": str(uuid4()),
        "key_hash": "hash",
        "entity_id": str(uuid4()),
        "agent_id": None,
        "scopes": None,
    }
    pool = _pool_stub(fetchrow_side_effect=[key_row, None])
    request = _request_with_pool(pool, "nbl_1234567890")

    monkeypatch.setattr(
        "nebula_api.auth.ph",
        SimpleNamespace(verify=lambda *_args: True),
    )

    result = await require_auth(request)

    assert result["caller_type"] == "user"
    assert result["entity"] is None
    assert result["scopes"] == []


@pytest.mark.asyncio
async def test_require_auth_agent_owner_returns_agent_context(monkeypatch):
    """Agent-owned keys should return agent caller context and merged scopes."""

    key_row = {
        "id": str(uuid4()),
        "key_hash": "hash",
        "entity_id": None,
        "agent_id": str(uuid4()),
        "scopes": ["public", "admin"],
    }
    agent_row = {"id": key_row["agent_id"], "scopes": ["public", "private"]}
    pool = _pool_stub(fetchrow_side_effect=[key_row, agent_row])
    request = _request_with_pool(pool, "nbl_1234567890")

    monkeypatch.setattr(
        "nebula_api.auth.ph",
        SimpleNamespace(verify=lambda *_args: True),
    )

    result = await require_auth(request)

    assert result["caller_type"] == "agent"
    assert result["agent_id"] == key_row["agent_id"]
    assert result["entity_id"] is None
    assert result["scopes"] == ["public"]
    pool.execute.assert_awaited_once()


@pytest.mark.asyncio
async def test_maybe_check_agent_approval_rate_limited_returns_429(monkeypatch):
    """Approval-capacity failures should map to explicit 429 response."""

    auth = {
        "caller_type": "agent",
        "agent": {"id": str(uuid4()), "requires_approval": True},
    }

    async def _raise_capacity(*_args, **_kwargs):
        raise ValueError("Approval queue limit reached")

    monkeypatch.setattr(
        "nebula_mcp.helpers.ensure_approval_capacity",
        _raise_capacity,
    )

    response = await maybe_check_agent_approval(
        pool=SimpleNamespace(),
        auth=auth,
        action="create_entity",
        payload={"name": "x"},
    )

    assert response is not None
    assert response.status_code == 429
    payload = json.loads(response.body.decode("utf-8"))
    assert payload["status"] == "rate_limited"
    assert "Approval queue limit reached" in payload["message"]


@pytest.mark.asyncio
async def test_maybe_check_agent_approval_non_agent_returns_none():
    """User caller path should bypass approval helper entirely."""

    result = await maybe_check_agent_approval(
        pool=SimpleNamespace(),
        auth={"caller_type": "user", "agent": None},
        action="create_entity",
        payload={"name": "x"},
    )

    assert result is None


@pytest.mark.asyncio
async def test_maybe_check_agent_approval_trusted_agent_returns_none():
    """Trusted agents should bypass approval queueing."""

    result = await maybe_check_agent_approval(
        pool=SimpleNamespace(),
        auth={
            "caller_type": "agent",
            "agent": {"id": str(uuid4()), "requires_approval": False},
        },
        action="create_entity",
        payload={"name": "x"},
    )

    assert result is None


@pytest.mark.asyncio
async def test_maybe_check_agent_approval_success_returns_202(monkeypatch):
    """Untrusted agent should receive approval-required response payload."""

    auth = {
        "caller_type": "agent",
        "agent": {"id": str(uuid4()), "requires_approval": True},
    }

    monkeypatch.setattr(
        "nebula_mcp.helpers.ensure_approval_capacity",
        AsyncMock(),
    )
    monkeypatch.setattr(
        "nebula_mcp.helpers.create_approval_request",
        AsyncMock(return_value={"id": str(uuid4())}),
    )

    response = await maybe_check_agent_approval(
        pool=SimpleNamespace(),
        auth=auth,
        action="create_entity",
        payload={"name": "x"},
    )

    assert response is not None
    assert response.status_code == 202
    payload = json.loads(response.body.decode("utf-8"))
    assert payload["status"] == "approval_required"
    assert payload["approval_request_id"]
