"""Red team unit tests for auth/context negative paths (no DB required)."""

# Standard Library
from unittest.mock import AsyncMock, MagicMock
from uuid import uuid4

# Third-Party
import argon2
import argon2.exceptions
import pytest

# Local
from nebula_mcp.context import authenticate_agent

pytestmark = pytest.mark.unit


@pytest.mark.asyncio
async def test_authenticate_agent_missing_env_var_raises(monkeypatch):
    """Missing NEBULA_API_KEY should raise ValueError before any DB calls."""

    monkeypatch.delenv("NEBULA_API_KEY", raising=False)
    pool = MagicMock()
    pool.fetchrow = AsyncMock()

    with pytest.raises(ValueError, match="NEBULA_API_KEY environment variable is required"):
        await authenticate_agent(pool)

    pool.fetchrow.assert_not_awaited()


@pytest.mark.asyncio
async def test_authenticate_agent_short_key_raises(monkeypatch):
    """Short NEBULA_API_KEY should raise ValueError before any DB calls."""

    monkeypatch.setenv("NEBULA_API_KEY", "short")
    pool = MagicMock()
    pool.fetchrow = AsyncMock()

    with pytest.raises(ValueError, match="NEBULA_API_KEY is too short"):
        await authenticate_agent(pool)

    pool.fetchrow.assert_not_awaited()


@pytest.mark.asyncio
async def test_authenticate_agent_invalid_prefix_raises(monkeypatch):
    """Unknown key prefix should raise a clean invalid/revoked error."""

    monkeypatch.setenv("NEBULA_API_KEY", "nbl_testkey_aaaaaaaaaaaaaaaa")
    pool = MagicMock()
    pool.fetchrow = AsyncMock(return_value=None)

    with pytest.raises(ValueError, match="invalid or revoked"):
        await authenticate_agent(pool)

    pool.fetchrow.assert_awaited()


@pytest.mark.asyncio
async def test_authenticate_agent_hash_mismatch_raises(monkeypatch):
    """Hash mismatch should raise ValueError without leaking raw argon2 errors."""

    class DummyHasher:
        """Stub hasher that simulates a mismatch."""

        def verify(self, *_args, **_kwargs):
            """Handle verify.

            Args:
                *_args: Input parameter for verify.
                **_kwargs: Input parameter for verify.
            """

            raise argon2.exceptions.VerifyMismatchError

    monkeypatch.setenv("NEBULA_API_KEY", "nbl_testkey_aaaaaaaaaaaaaaaa")
    monkeypatch.setattr(argon2, "PasswordHasher", DummyHasher)

    agent_id = uuid4()
    pool = MagicMock()
    pool.fetchrow = AsyncMock(
        side_effect=[
            {"key_hash": "hash", "agent_id": agent_id},
        ]
    )

    with pytest.raises(ValueError, match="hash mismatch"):
        await authenticate_agent(pool)


@pytest.mark.asyncio
async def test_authenticate_agent_non_agent_key_raises(monkeypatch):
    """Keys without an agent_id should be rejected as non-agent keys."""

    class DummyHasher:
        """Stub hasher that always verifies successfully."""

        def verify(self, *_args, **_kwargs):
            """Handle verify.

            Args:
                *_args: Input parameter for verify.
                **_kwargs: Input parameter for verify.

            Returns:
                Result value from the operation.
            """

            return True

    monkeypatch.setenv("NEBULA_API_KEY", "nbl_testkey_aaaaaaaaaaaaaaaa")
    monkeypatch.setattr(argon2, "PasswordHasher", DummyHasher)

    pool = MagicMock()
    pool.fetchrow = AsyncMock(side_effect=[{"key_hash": "hash", "agent_id": None}])

    with pytest.raises(ValueError, match="not an agent key"):
        await authenticate_agent(pool)


@pytest.mark.asyncio
async def test_authenticate_agent_inactive_agent_raises(monkeypatch):
    """Missing agent row should raise ValueError without leaking DB details."""

    class DummyHasher:
        """Stub hasher that always verifies successfully."""

        def verify(self, *_args, **_kwargs):
            """Handle verify.

            Args:
                *_args: Input parameter for verify.
                **_kwargs: Input parameter for verify.

            Returns:
                Result value from the operation.
            """

            return True

    monkeypatch.setenv("NEBULA_API_KEY", "nbl_testkey_aaaaaaaaaaaaaaaa")
    monkeypatch.setattr(argon2, "PasswordHasher", DummyHasher)

    agent_id = uuid4()
    pool = MagicMock()
    pool.fetchrow = AsyncMock(
        side_effect=[
            {"key_hash": "hash", "agent_id": agent_id},
            None,
        ]
    )

    with pytest.raises(ValueError, match="Agent not found or inactive"):
        await authenticate_agent(pool)
