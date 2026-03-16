"""Auth middleware tests."""

# Third-Party
import pytest


@pytest.mark.asyncio
async def test_missing_auth_header(api_no_auth):
    """Test missing auth header."""

    r = await api_no_auth.get("/api/health")
    assert r.status_code == 200

    r = await api_no_auth.get("/api/entities")
    assert r.status_code == 401


@pytest.mark.asyncio
async def test_invalid_bearer_format(api_no_auth):
    """Test invalid bearer format."""

    r = await api_no_auth.get("/api/entities", headers={"Authorization": "Basic abc"})
    assert r.status_code == 401


@pytest.mark.asyncio
async def test_short_key(api_no_auth):
    """Test short key."""

    r = await api_no_auth.get("/api/entities", headers={"Authorization": "Bearer abc"})
    assert r.status_code == 401


@pytest.mark.asyncio
async def test_valid_key(api_no_auth, api_key_row):
    """Test valid key."""

    raw_key, row = api_key_row
    r = await api_no_auth.get("/api/entities", headers={"Authorization": f"Bearer {raw_key}"})
    assert r.status_code == 200


@pytest.mark.asyncio
async def test_wrong_key_hash(api_no_auth, api_key_row):
    """Test wrong key hash."""

    raw_key, row = api_key_row
    # same prefix but wrong body
    bad_key = raw_key[:8] + "x" * (len(raw_key) - 8)
    r = await api_no_auth.get("/api/entities", headers={"Authorization": f"Bearer {bad_key}"})
    assert r.status_code == 401


@pytest.mark.asyncio
async def test_revoked_key(api_no_auth, api_key_row, db_pool):
    """Test revoked key."""

    raw_key, row = api_key_row
    await db_pool.execute("UPDATE api_keys SET revoked_at = NOW() WHERE id = $1", row["id"])
    r = await api_no_auth.get("/api/entities", headers={"Authorization": f"Bearer {raw_key}"})
    assert r.status_code == 401


@pytest.mark.asyncio
async def test_expired_key(api_no_auth, api_key_row, db_pool):
    """Test expired key."""

    raw_key, row = api_key_row
    await db_pool.execute(
        "UPDATE api_keys SET expires_at = NOW() - INTERVAL '1 hour' WHERE id = $1",
        row["id"],
    )
    r = await api_no_auth.get("/api/entities", headers={"Authorization": f"Bearer {raw_key}"})
    assert r.status_code == 401


@pytest.mark.asyncio
async def test_last_used_at_updated(api_no_auth, api_key_row, db_pool):
    """Test last used at updated."""

    raw_key, row = api_key_row
    assert row["last_used_at"] is None
    await api_no_auth.get("/api/entities", headers={"Authorization": f"Bearer {raw_key}"})
    updated = await db_pool.fetchval("SELECT last_used_at FROM api_keys WHERE id = $1", row["id"])
    assert updated is not None


@pytest.mark.asyncio
async def test_nonexistent_prefix(api_no_auth):
    """Test nonexistent prefix."""

    r = await api_no_auth.get(
        "/api/entities",
        headers={"Authorization": "Bearer nbl_zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"},
    )
    assert r.status_code == 401
