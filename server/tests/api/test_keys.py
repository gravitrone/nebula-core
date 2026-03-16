"""API key management route tests."""

# Third-Party
import pytest


@pytest.mark.asyncio
async def test_login_creates_entity_and_key(api_no_auth):
    """Test login creates entity and key."""

    r = await api_no_auth.post("/api/keys/login", json={"username": "newuser"})
    assert r.status_code == 200
    data = r.json()["data"]
    assert data["api_key"].startswith("nbl_")
    assert data["username"] == "newuser"
    assert "entity_id" in data


@pytest.mark.asyncio
async def test_login_existing_user(api_no_auth, test_entity):
    """Test login existing user."""

    r = await api_no_auth.post("/api/keys/login", json={"username": "api-test-user"})
    assert r.status_code == 200
    data = r.json()["data"]
    assert data["entity_id"] == str(test_entity["id"])


@pytest.mark.asyncio
async def test_login_ensures_admin_scope(api_no_auth, db_pool, enums):
    """Login should always ensure the user entity has admin scope."""

    # Create an existing entity without admin in scopes.
    status_id = enums.statuses.name_to_id["active"]
    type_id = enums.entity_types.name_to_id["person"]
    public_scope = enums.scopes.name_to_id["public"]
    private_scope = enums.scopes.name_to_id["private"]
    admin_scope = enums.scopes.name_to_id["admin"]

    existing = await db_pool.fetchrow(
        """
        INSERT INTO entities (name, type_id, status_id, privacy_scope_ids, tags)
        VALUES ($1, $2, $3, $4::uuid[], $5)
        RETURNING id
        """,
        "login-admin-scope-user",
        type_id,
        status_id,
        [public_scope, private_scope],
        [],
    )

    r = await api_no_auth.post(
        "/api/keys/login", json={"username": "login-admin-scope-user"}
    )
    assert r.status_code == 200
    entity_id = r.json()["data"]["entity_id"]
    assert entity_id == str(existing["id"])

    refreshed = await db_pool.fetchrow(
        "SELECT privacy_scope_ids FROM entities WHERE id = $1::uuid", entity_id
    )
    assert admin_scope in (refreshed["privacy_scope_ids"] or [])


@pytest.mark.asyncio
async def test_login_existing_user_backfills_baseline_scopes(
    api_no_auth, db_pool, enums
):
    """Login should backfill baseline scopes for existing users."""

    status_id = enums.statuses.name_to_id["active"]
    type_id = enums.entity_types.name_to_id["person"]
    public_scope = enums.scopes.name_to_id["public"]
    baseline = {
        enums.scopes.name_to_id["public"],
        enums.scopes.name_to_id["private"],
        enums.scopes.name_to_id["sensitive"],
        enums.scopes.name_to_id["admin"],
    }

    existing = await db_pool.fetchrow(
        """
        INSERT INTO entities (name, type_id, status_id, privacy_scope_ids, tags)
        VALUES ($1, $2, $3, $4::uuid[], $5)
        RETURNING id
        """,
        "login-baseline-backfill-user",
        type_id,
        status_id,
        [public_scope],
        [],
    )

    r = await api_no_auth.post(
        "/api/keys/login", json={"username": "login-baseline-backfill-user"}
    )
    assert r.status_code == 200
    assert r.json()["data"]["entity_id"] == str(existing["id"])

    refreshed = await db_pool.fetchrow(
        "SELECT privacy_scope_ids FROM entities WHERE id = $1::uuid", existing["id"]
    )
    assert set(refreshed["privacy_scope_ids"] or []) == baseline


@pytest.mark.asyncio
async def test_login_returns_service_unavailable_when_baseline_scope_missing(
    api_no_auth, enums
):
    """Login returns 503 when required baseline scope is unavailable."""

    removed_id = enums.scopes.name_to_id.pop("admin", None)
    if removed_id is not None:
        enums.scopes.id_to_name.pop(removed_id, None)

    try:
        r = await api_no_auth.post(
            "/api/keys/login", json={"username": "missing-admin-scope-user"}
        )
        assert r.status_code == 503
        err = r.json()["detail"]["error"]
        assert err["code"] == "SERVICE_UNAVAILABLE"
        assert "required baseline taxonomy is missing" in err["message"]
    finally:
        if removed_id is not None:
            enums.scopes.name_to_id["admin"] = removed_id
            enums.scopes.id_to_name[removed_id] = "admin"


@pytest.mark.asyncio
async def test_login_returns_service_unavailable_when_person_type_missing(
    api_no_auth, enums
):
    """Login returns 503 when required person entity type is unavailable."""

    removed_id = enums.entity_types.name_to_id.pop("person", None)
    if removed_id is not None:
        enums.entity_types.id_to_name.pop(removed_id, None)

    try:
        r = await api_no_auth.post(
            "/api/keys/login", json={"username": "missing-person-type-user"}
        )
        assert r.status_code == 503
        err = r.json()["detail"]["error"]
        assert err["code"] == "SERVICE_UNAVAILABLE"
        assert "required baseline taxonomy is missing" in err["message"]
    finally:
        if removed_id is not None:
            enums.entity_types.name_to_id["person"] = removed_id
            enums.entity_types.id_to_name[removed_id] = "person"


@pytest.mark.asyncio
async def test_login_returns_service_unavailable_when_active_status_missing(
    api_no_auth, enums
):
    """Login returns 503 when required active status is unavailable."""

    removed_id = enums.statuses.name_to_id.pop("active", None)
    if removed_id is not None:
        enums.statuses.id_to_name.pop(removed_id, None)

    try:
        r = await api_no_auth.post(
            "/api/keys/login", json={"username": "missing-active-status-user"}
        )
        assert r.status_code == 503
        err = r.json()["detail"]["error"]
        assert err["code"] == "SERVICE_UNAVAILABLE"
        assert "required baseline taxonomy is missing" in err["message"]
    finally:
        if removed_id is not None:
            enums.statuses.name_to_id["active"] = removed_id
            enums.statuses.id_to_name[removed_id] = "active"


@pytest.mark.asyncio
async def test_create_additional_key(api):
    """Test create additional key."""

    r = await api.post("/api/keys", json={"name": "second-key"})
    assert r.status_code == 200
    data = r.json()["data"]
    assert data["api_key"].startswith("nbl_")
    assert data["name"] == "second-key"


@pytest.mark.asyncio
async def test_list_keys(api):
    """Test list keys."""

    await api.post("/api/keys", json={"name": "list-test"})
    r = await api.get("/api/keys")
    assert r.status_code == 200
    data = r.json()["data"]
    assert len(data) >= 1


@pytest.mark.asyncio
async def test_revoke_key(api, db_pool, auth_override):
    """Test revoke key."""

    cr = await api.post("/api/keys", json={"name": "revoke-me"})
    key_id = cr.json()["data"]["key_id"]

    r = await api.delete(f"/api/keys/{key_id}")
    assert r.status_code == 200
    assert r.json()["data"]["revoked"] is True

    row = await db_pool.fetchrow(
        "SELECT revoked_at FROM api_keys WHERE id = $1::uuid", key_id
    )
    assert row["revoked_at"] is not None


@pytest.mark.asyncio
async def test_list_all_keys(api, db_pool, test_entity, auth_override, enums):
    """Test list all keys includes user and agent keys."""

    auth_override["scopes"] = [enums.scopes.name_to_id["admin"]]

    # Create a user key
    await api.post("/api/keys", json={"name": "user-key-for-all"})

    # Create an agent + agent key directly in DB
    from nebula_api.auth import generate_api_key

    agent = await db_pool.fetchrow(
        """
        INSERT INTO agents (name, description, scopes, requires_approval, status_id)
        VALUES ($1, $2, $3, $4, (SELECT id FROM statuses WHERE name = 'active'))
        RETURNING *
        """,
        "all-keys-test-agent",
        "Agent for list_all test",
        [],
        False,
    )
    raw_key, prefix, key_hash = generate_api_key()
    await db_pool.execute(
        """
        INSERT INTO api_keys (agent_id, key_hash, key_prefix, name)
        VALUES ($1, $2, $3, $4)
        """,
        agent["id"],
        key_hash,
        prefix,
        "agent-key-for-all",
    )

    r = await api.get("/api/keys/all")
    assert r.status_code == 200
    data = r.json()["data"]
    assert len(data) >= 2

    owner_types = {k["owner_type"] for k in data}
    assert "user" in owner_types
    assert "agent" in owner_types
