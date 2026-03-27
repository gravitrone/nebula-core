"""Unit tests for nebula_api.app startup and router wiring."""

# Standard Library
from types import SimpleNamespace
from unittest.mock import AsyncMock

# Third-Party
import pytest

# Local
import nebula_api.app as app_mod

pytestmark = pytest.mark.unit


@pytest.mark.asyncio
async def test_health_endpoint_returns_ok():
    """Health handler should return canonical ok payload."""

    assert await app_mod.health() == {"status": "ok"}


@pytest.mark.asyncio
async def test_lifespan_sets_state_and_closes_pool(monkeypatch):
    """Lifespan should initialize app.state and close pool on exit."""

    pool = SimpleNamespace(close=AsyncMock())
    enums = {"loaded": True}
    app = SimpleNamespace(state=SimpleNamespace())

    monkeypatch.setattr("nebula_api.app.get_pool", AsyncMock(return_value=pool))
    monkeypatch.setattr("nebula_api.app.load_enums", AsyncMock(return_value=enums))

    async with app_mod.lifespan(app):
        assert app.state.pool is pool
        assert app.state.enums is enums
        pool.close.assert_not_awaited()

    pool.close.assert_awaited_once()


@pytest.mark.asyncio
async def test_lifespan_closes_pool_if_enum_load_fails(monkeypatch):
    """Startup failures after pool creation should still close the pool."""

    pool = SimpleNamespace(close=AsyncMock())
    app = SimpleNamespace(state=SimpleNamespace())

    monkeypatch.setattr("nebula_api.app.get_pool", AsyncMock(return_value=pool))
    monkeypatch.setattr(
        "nebula_api.app.load_enums",
        AsyncMock(side_effect=RuntimeError("enum load failed")),
    )

    with pytest.raises(RuntimeError, match="enum load failed"):
        async with app_mod.lifespan(app):
            pass

    pool.close.assert_awaited_once()


def test_app_has_expected_api_prefixes():
    """App should include all expected API route prefixes."""

    expected_prefixes = {
        "/api/entities",
        "/api/audit",
        "/api/context",
        "/api/relationships",
        "/api/jobs",
        "/api/search",
        "/api/files",
        "/api/logs",
        "/api/import",
        "/api/export",
        "/api/protocols",
        "/api/approvals",
        "/api/agents",
        "/api/keys",
        "/api/taxonomy",
        "/api/schema",
        "/api/health",
    }

    paths = [route.path for route in app_mod.app.routes]
    for prefix in expected_prefixes:
        assert any(path.startswith(prefix) for path in paths), prefix


def test_app_version_locked():
    """App metadata version should match the current public beta contract."""

    assert app_mod.app.version == "0.1.0"
