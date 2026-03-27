"""API tests for taxonomy management endpoints."""

# Standard Library

# Third-Party
import pytest
from httpx import ASGITransport, AsyncClient

from nebula_api.app import app
from nebula_api.auth import require_auth

pytestmark = pytest.mark.api
LEGACY_SCOPE_NAMES = ("work", "code", "vault-only", "blacklisted")


@pytest.fixture
async def admin_auth_override(test_entity, enums):
    """Override require_auth dependency with an admin-scoped user."""

    auth_dict = {
        "key_id": None,
        "caller_type": "user",
        "entity_id": test_entity["id"],
        "entity": test_entity,
        "agent_id": None,
        "agent": None,
        "scopes": [enums.scopes.name_to_id["admin"]],
    }

    async def mock_auth():
        """Handle mock auth.

        Returns:
            Result value from the operation.
        """

        return auth_dict

    app.dependency_overrides[require_auth] = mock_auth
    yield auth_dict
    app.dependency_overrides.pop(require_auth, None)


@pytest.fixture
async def api_admin(db_pool, enums, admin_auth_override):
    """API client with admin auth override enabled."""

    app.state.pool = db_pool
    app.state.enums = enums
    del admin_auth_override
    transport = ASGITransport(app=app)
    async with AsyncClient(
        transport=transport, base_url="http://test", follow_redirects=True
    ) as client:
        yield client


@pytest.mark.asyncio
async def test_taxonomy_requires_admin_scope(api):
    """Non-admin users cannot access taxonomy management."""

    resp = await api.get("/api/taxonomy/scopes")
    assert resp.status_code == 403
    body = resp.json()
    assert body["detail"]["error"]["code"] == "FORBIDDEN"


@pytest.mark.asyncio
@pytest.mark.parametrize(
    ("kind", "payload", "patch_payload"),
    [
        (
            "scopes",
            {
                "name": "sdk-scope",
                "description": "scope for SDK tests",
                "notes": "owner: tests",
            },
            {"description": "scope for SDK tests v2"},
        ),
        (
            "entity-types",
            {
                "name": "sdk-entity",
                "description": "entity type for SDK tests",
                "notes": "category: sdk",
            },
            {"description": "entity type for SDK tests v2"},
        ),
        (
            "relationship-types",
            {
                "name": "sdk-relates",
                "description": "relationship type for SDK tests",
                "is_symmetric": False,
                "notes": "kind: sdk",
            },
            {"description": "relationship type for SDK tests v2", "is_symmetric": True},
        ),
        (
            "log-types",
            {
                "name": "sdk-log",
                "description": "log type for SDK tests",
                "value_schema": {
                    "type": "object",
                    "properties": {"v": {"type": "number"}},
                },
            },
            {"description": "log type for SDK tests v2"},
        ),
    ],
)
async def test_taxonomy_crud_roundtrip(api_admin, kind, payload, patch_payload):
    """Admin can create, update, archive, and reactivate taxonomy rows."""

    create = await api_admin.post(f"/api/taxonomy/{kind}", json=payload)
    assert create.status_code == 200, create.text
    item = create.json()["data"]
    assert item["name"] == payload["name"]
    assert item["is_active"] is True

    update = await api_admin.patch(
        f"/api/taxonomy/{kind}/{item['id']}",
        json=patch_payload,
    )
    assert update.status_code == 200, update.text
    updated = update.json()["data"]
    assert updated["description"] == patch_payload["description"]

    archived = await api_admin.post(f"/api/taxonomy/{kind}/{item['id']}/archive")
    assert archived.status_code == 200, archived.text
    assert archived.json()["data"]["is_active"] is False

    activated = await api_admin.post(f"/api/taxonomy/{kind}/{item['id']}/activate")
    assert activated.status_code == 200, activated.text
    assert activated.json()["data"]["is_active"] is True


@pytest.mark.asyncio
async def test_taxonomy_list_supports_search_and_pagination(api_admin):
    """List endpoint supports include_inactive, search, limit and offset."""

    payloads = [
        {"name": "sdk-scope-alpha", "description": "alpha"},
        {"name": "sdk-scope-bravo", "description": "bravo"},
        {"name": "sdk-scope-charlie", "description": "charlie"},
    ]
    for payload in payloads:
        resp = await api_admin.post("/api/taxonomy/scopes", json=payload)
        assert resp.status_code == 200, resp.text

    first = await api_admin.get(
        "/api/taxonomy/scopes",
        params={"search": "sdk-scope-", "limit": 2, "offset": 0},
    )
    assert first.status_code == 200, first.text
    first_items = first.json()["data"]
    assert len(first_items) == 2

    second = await api_admin.get(
        "/api/taxonomy/scopes",
        params={"search": "sdk-scope-", "limit": 2, "offset": 2},
    )
    assert second.status_code == 200, second.text
    second_items = second.json()["data"]
    assert len(second_items) >= 1


@pytest.mark.asyncio
async def test_taxonomy_builtin_archive_is_rejected(api_admin):
    """Built-in taxonomy rows cannot be archived."""

    listing = await api_admin.get("/api/taxonomy/scopes", params={"search": "admin"})
    assert listing.status_code == 200, listing.text
    rows = listing.json()["data"]
    admin_scope = next((r for r in rows if r["name"] == "admin"), None)
    assert admin_scope is not None
    assert admin_scope["is_builtin"] is True

    resp = await api_admin.post(f"/api/taxonomy/scopes/{admin_scope['id']}/archive")
    assert resp.status_code == 404


@pytest.mark.asyncio
async def test_taxonomy_scope_archive_conflict_when_referenced(api_admin, db_pool, enums):
    """Archiving a scope in active use returns conflict."""

    created = await api_admin.post(
        "/api/taxonomy/scopes",
        json={"name": "sdk-scope-in-use", "description": "in use"},
    )
    assert created.status_code == 200, created.text
    scope = created.json()["data"]

    await db_pool.execute(
        """
        INSERT INTO entities (name, type_id, status_id, privacy_scope_ids, tags)
        VALUES ($1, $2, $3, $4, $5)
        """,
        "sdk-scope-holder",
        enums.entity_types.name_to_id["person"],
        enums.statuses.name_to_id["active"],
        [scope["id"]],
        ["sdk"],
    )

    resp = await api_admin.post(f"/api/taxonomy/scopes/{scope['id']}/archive")
    assert resp.status_code == 409
    body = resp.json()
    assert body["detail"]["error"]["code"] == "CONFLICT"


@pytest.mark.asyncio
async def test_taxonomy_entity_type_archive_conflict_when_referenced(api_admin, db_pool, enums):
    """Archiving an entity type in active use returns conflict."""

    created = await api_admin.post(
        "/api/taxonomy/entity-types",
        json={"name": "sdk-entity-in-use", "description": "in use"},
    )
    assert created.status_code == 200, created.text
    entity_type = created.json()["data"]

    await db_pool.execute(
        """
        INSERT INTO entities (name, type_id, status_id, privacy_scope_ids, tags)
        VALUES ($1, $2, $3, $4, $5)
        """,
        "sdk-entity-type-holder",
        entity_type["id"],
        enums.statuses.name_to_id["active"],
        [enums.scopes.name_to_id["public"]],
        ["sdk"],
    )

    resp = await api_admin.post(f"/api/taxonomy/entity-types/{entity_type['id']}/archive")
    assert resp.status_code == 409


@pytest.mark.asyncio
async def test_taxonomy_relationship_type_archive_conflict_when_referenced(
    api_admin, db_pool, enums
):
    """Archiving a relationship type in active use returns conflict."""

    created = await api_admin.post(
        "/api/taxonomy/relationship-types",
        json={"name": "sdk-rel-in-use", "description": "in use"},
    )
    assert created.status_code == 200, created.text
    rel_type = created.json()["data"]

    source = await db_pool.fetchrow(
        """
        INSERT INTO entities (name, type_id, status_id, privacy_scope_ids, tags)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING id
        """,
        "sdk-rel-source",
        enums.entity_types.name_to_id["person"],
        enums.statuses.name_to_id["active"],
        [enums.scopes.name_to_id["public"]],
        ["sdk"],
    )
    target = await db_pool.fetchrow(
        """
        INSERT INTO entities (name, type_id, status_id, privacy_scope_ids, tags)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING id
        """,
        "sdk-rel-target",
        enums.entity_types.name_to_id["person"],
        enums.statuses.name_to_id["active"],
        [enums.scopes.name_to_id["public"]],
        ["sdk"],
    )
    await db_pool.execute(
        """
        INSERT INTO relationships (source_type, source_id, target_type, target_id, type_id, status_id, notes)
        VALUES ('entity', $1, 'entity', $2, $3, $4, '{}')
        """,
        str(source["id"]),
        str(target["id"]),
        rel_type["id"],
        enums.statuses.name_to_id["active"],
    )

    resp = await api_admin.post(f"/api/taxonomy/relationship-types/{rel_type['id']}/archive")
    assert resp.status_code == 409


@pytest.mark.asyncio
async def test_taxonomy_log_type_archive_conflict_when_referenced(api_admin, db_pool, enums):
    """Archiving a log type in active use returns conflict."""

    created = await api_admin.post(
        "/api/taxonomy/log-types",
        json={"name": "sdk-log-in-use", "description": "in use"},
    )
    assert created.status_code == 200, created.text
    log_type = created.json()["data"]

    await db_pool.execute(
        """
        INSERT INTO logs (log_type_id, timestamp, content, notes, status_id)
        VALUES ($1, NOW(), '{}', '{}', $2)
        """,
        log_type["id"],
        enums.statuses.name_to_id["active"],
    )

    resp = await api_admin.post(f"/api/taxonomy/log-types/{log_type['id']}/archive")
    assert resp.status_code == 409


@pytest.mark.asyncio
@pytest.mark.parametrize("legacy_scope", LEGACY_SCOPE_NAMES)
async def test_legacy_scope_rejected_until_activated_via_taxonomy(
    api, api_admin, db_pool, legacy_scope
):
    """Legacy scopes stay invalid until explicitly activated in taxonomy."""

    denied = await api.post(
        "/api/entities",
        json={
            "name": f"legacy-denied-{legacy_scope}",
            "type": "person",
            "status": "active",
            "scopes": [legacy_scope],
        },
    )
    assert denied.status_code == 400
    assert denied.json()["detail"]["error"]["code"] == "INVALID_INPUT"

    listing = await api_admin.get(
        "/api/taxonomy/scopes",
        params={"include_inactive": True, "search": legacy_scope},
    )
    assert listing.status_code == 200, listing.text
    rows = listing.json()["data"]
    scope = next((row for row in rows if row["name"] == legacy_scope), None)
    assert scope is not None, f"missing scope row for {legacy_scope}"
    assert scope["is_active"] is False

    activate = await api_admin.post(f"/api/taxonomy/scopes/{scope['id']}/activate")
    assert activate.status_code == 200, activate.text
    assert activate.json()["data"]["is_active"] is True

    allowed = await api.post(
        "/api/entities",
        json={
            "name": f"legacy-allowed-{legacy_scope}",
            "type": "person",
            "status": "active",
            "scopes": [legacy_scope],
        },
    )
    assert allowed.status_code == 200, allowed.text
    created = allowed.json()["data"]

    await db_pool.execute("DELETE FROM entities WHERE id = $1::uuid", created["id"])

    archive = await api_admin.post(f"/api/taxonomy/scopes/{scope['id']}/archive")
    assert archive.status_code == 200, archive.text
    assert archive.json()["data"]["is_active"] is False
