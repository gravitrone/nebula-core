"""Unit tests for MCP database pool creation error translation."""

# Third-Party
import asyncpg
import pytest

from nebula_mcp.db import get_pool

pytestmark = pytest.mark.unit


class TestGetPoolErrorTranslation:
    """Tests for get_pool error translation behavior."""

    async def test_connection_refused_translates_to_runtimeerror(self, monkeypatch):
        """Translate connection refused errors into a friendly RuntimeError."""

        monkeypatch.setenv("POSTGRES_PASSWORD", "pw")

        async def fake_create_pool(**_kwargs):
            """Raise a connection error."""

            raise ConnectionRefusedError("connection refused")

        monkeypatch.setattr(asyncpg, "create_pool", fake_create_pool)

        with pytest.raises(RuntimeError, match="Database connection failed. Is Docker running\\?"):
            await get_pool()

    async def test_non_connection_postgres_error_reraises(self, monkeypatch):
        """Re-raise non-connection Postgres errors unchanged."""

        monkeypatch.setenv("POSTGRES_PASSWORD", "pw")

        async def fake_create_pool(**_kwargs):
            """Raise a generic Postgres error."""

            raise asyncpg.PostgresError("boom")

        monkeypatch.setattr(asyncpg, "create_pool", fake_create_pool)

        with pytest.raises(asyncpg.PostgresError, match="boom"):
            await get_pool()
