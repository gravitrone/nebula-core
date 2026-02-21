"""Unit tests for the in-memory rate limiting decorator."""

# Third-Party
import pytest
from fastapi import HTTPException
from starlette.requests import Request

# Local
import nebula_api.rate_limit as rate_limit_mod

pytestmark = pytest.mark.unit


def _make_request(auth_header: str | None, client_host: str = "127.0.0.1") -> Request:
    """Build a minimal Starlette request with optional auth header."""

    headers: list[tuple[bytes, bytes]] = []
    if auth_header is not None:
        headers.append((b"authorization", auth_header.encode("utf-8")))

    scope = {
        "type": "http",
        "asgi": {"spec_version": "2.3"},
        "http_version": "1.1",
        "method": "GET",
        "scheme": "http",
        "path": "/",
        "raw_path": b"/",
        "query_string": b"",
        "headers": headers,
        "client": (client_host, 12345),
        "server": ("testserver", 80),
    }
    return Request(scope)


async def _ok_handler(*, request: Request) -> str:
    """Return a constant value for rate limit tests."""

    return "ok"


@pytest.mark.asyncio
async def test_rate_limit_enforces_max_requests(monkeypatch):
    """Rate limiter should reject requests after max_requests within window."""

    rate_limit_mod._buckets.clear()
    times = iter([0.0, 1.0, 2.0])
    monkeypatch.setattr(rate_limit_mod.time, "time", lambda: next(times))

    decorated = rate_limit_mod.rate_limit(max_requests=2, window=60)(_ok_handler)
    req = _make_request("Bearer key-1")

    assert await decorated(request=req) == "ok"
    assert await decorated(request=req) == "ok"

    with pytest.raises(HTTPException) as exc:
        await decorated(request=req)

    assert exc.value.status_code == 429
    assert exc.value.detail["error"]["code"] == "RATE_LIMITED"


@pytest.mark.asyncio
async def test_rate_limit_isolated_per_key(monkeypatch):
    """Rate limiter should track counts separately per Authorization key."""

    rate_limit_mod._buckets.clear()
    times = iter([0.0, 0.1, 0.2, 0.3])
    monkeypatch.setattr(rate_limit_mod.time, "time", lambda: next(times))

    decorated = rate_limit_mod.rate_limit(max_requests=1, window=60)(_ok_handler)
    req_a = _make_request("Bearer key-a")
    req_b = _make_request("Bearer key-b")

    assert await decorated(request=req_a) == "ok"
    assert await decorated(request=req_b) == "ok"

    with pytest.raises(HTTPException):
        await decorated(request=req_a)


@pytest.mark.asyncio
async def test_rate_limit_window_expiry_resets(monkeypatch):
    """Rate limiter should allow requests again after the window expires."""

    rate_limit_mod._buckets.clear()
    times = iter([0.0, 1.0, 61.0])
    monkeypatch.setattr(rate_limit_mod.time, "time", lambda: next(times))

    decorated = rate_limit_mod.rate_limit(max_requests=1, window=60)(_ok_handler)
    req = _make_request("Bearer key-expiry")

    assert await decorated(request=req) == "ok"

    with pytest.raises(HTTPException):
        await decorated(request=req)

    assert await decorated(request=req) == "ok"


@pytest.mark.asyncio
async def test_rate_limit_uses_client_host_when_auth_missing(monkeypatch):
    """Missing Authorization should fall back to request.client.host for bucketing."""

    rate_limit_mod._buckets.clear()
    times = iter([0.0, 0.1, 0.2])
    monkeypatch.setattr(rate_limit_mod.time, "time", lambda: next(times))

    decorated = rate_limit_mod.rate_limit(max_requests=1, window=60)(_ok_handler)
    req = _make_request(None, client_host="10.0.0.1")

    assert await decorated(request=req) == "ok"

    with pytest.raises(HTTPException):
        await decorated(request=req)


@pytest.mark.asyncio
async def test_rate_limit_host_fallback_isolated_by_host(monkeypatch):
    """Host fallback bucketing should not collide across different client hosts."""

    rate_limit_mod._buckets.clear()
    times = iter([0.0, 0.1])
    monkeypatch.setattr(rate_limit_mod.time, "time", lambda: next(times))

    decorated = rate_limit_mod.rate_limit(max_requests=1, window=60)(_ok_handler)
    req_a = _make_request(None, client_host="10.0.0.1")
    req_b = _make_request(None, client_host="10.0.0.2")

    assert await decorated(request=req_a) == "ok"
    assert await decorated(request=req_b) == "ok"
