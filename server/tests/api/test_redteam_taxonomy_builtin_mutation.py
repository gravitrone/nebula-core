"""Red team tests for taxonomy built-in mutation and reserved name reuse."""

# Third-Party
import pytest
from httpx import ASGITransport, AsyncClient

from nebula_api.app import app
from nebula_api.auth import require_auth

pytestmark = pytest.mark.api


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
async def test_taxonomy_builtin_scope_name_is_immutable(api_admin, db_pool):
    """Renaming built-in scopes should be rejected (prevents reserved-name reuse)."""

    listing = await api_admin.get("/api/taxonomy/scopes", params={"search": "admin"})
    assert listing.status_code == 200, listing.text
    rows = listing.json()["data"]
    admin_scope = next((r for r in rows if r["name"] == "admin"), None)
    assert admin_scope is not None
    assert admin_scope["is_builtin"] is True

    builtin_id = admin_scope["id"]
    renamed_name = "admin-rt-renamed"

    rename = await api_admin.patch(
        f"/api/taxonomy/scopes/{builtin_id}",
        json={"name": renamed_name},
    )

    created_id: str | None = None
    try:
        # If rename succeeds, reserved name becomes reusable. Show that too.
        if rename.status_code == 200:
            create = await api_admin.post(
                "/api/taxonomy/scopes",
                json={"name": "admin", "description": "rt reserved name reuse"},
            )
            if create.status_code == 200:
                created_id = create.json()["data"]["id"]

        assert rename.status_code in {403, 404, 409}
    finally:
        # Taxonomy tables are not truncated by the global test cleanup fixture.
        if created_id is not None:
            await db_pool.execute(
                "DELETE FROM privacy_scopes WHERE id = $1::uuid",
                created_id,
            )
        if rename.status_code == 200:
            await db_pool.execute(
                "UPDATE privacy_scopes SET name = $2 WHERE id = $1::uuid",
                builtin_id,
                "admin",
            )
