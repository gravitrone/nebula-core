"""Schema + taxonomy lifecycle integration tests."""

# Third-Party
import pytest
from httpx import ASGITransport, AsyncClient

# Local
from nebula_api.app import app
from nebula_api.auth import require_auth


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
async def test_scope_lifecycle_updates_schema_and_validation(api_admin):
    """New scopes should appear in schema, and archived scopes should be rejected."""

    scope = await api_admin.post(
        "/api/taxonomy/scopes",
        json={"name": "lifecycle-scope", "description": "for lifecycle tests"},
    )
    assert scope.status_code == 200, scope.text
    scope_row = scope.json()["data"]

    schema = await api_admin.get("/api/schema/")
    assert schema.status_code == 200, schema.text
    scopes = {row["name"] for row in schema.json()["data"]["taxonomy"]["scopes"]}
    assert "lifecycle-scope" in scopes

    created = await api_admin.post(
        "/api/entities",
        json={
            "name": "lifecycle entity",
            "type": "person",
            "status": "active",
            "scopes": ["public", "lifecycle-scope"],
            "tags": [],
        },
    )
    assert created.status_code == 200, created.text
    entity_id = created.json()["data"]["id"]

    updated = await api_admin.post(
        "/api/entities/bulk/scopes",
        json={"entity_ids": [entity_id], "scopes": ["public"], "op": "set"},
    )
    assert updated.status_code == 200, updated.text

    archived = await api_admin.post(
        f"/api/taxonomy/scopes/{scope_row['id']}/archive",
    )
    assert archived.status_code == 200, archived.text

    schema_after = await api_admin.get("/api/schema/")
    assert schema_after.status_code == 200, schema_after.text
    scopes_after = {row["name"] for row in schema_after.json()["data"]["taxonomy"]["scopes"]}
    assert "lifecycle-scope" not in scopes_after

    rejected = await api_admin.post(
        "/api/entities",
        json={
            "name": "lifecycle entity 2",
            "type": "person",
            "status": "active",
            "scopes": ["lifecycle-scope"],
            "tags": [],
        },
    )
    assert rejected.status_code == 400, rejected.text
