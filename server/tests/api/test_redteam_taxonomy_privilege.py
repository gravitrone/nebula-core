"""Red team tests for taxonomy admin scope boundaries."""

# Third-Party
import pytest
from httpx import ASGITransport, AsyncClient

# Local
from nebula_api.app import app
from nebula_api.auth import require_auth


@pytest.mark.asyncio
async def test_taxonomy_sensitive_scope_cannot_create(api, db_pool, test_entity, enums):
    """A sensitive-only user should not be treated as taxonomy admin."""

    auth_dict = {
        "key_id": None,
        "caller_type": "user",
        "entity_id": test_entity["id"],
        "entity": test_entity,
        "agent_id": None,
        "agent": None,
        "scopes": [enums.scopes.name_to_id["sensitive"]],
    }

    async def mock_auth() -> dict:
        """Handle mock auth.

        Returns:
            Result value from the operation.
        """

        return auth_dict

    app.dependency_overrides[require_auth] = mock_auth
    app.state.pool = api._transport.app.state.pool
    app.state.enums = enums
    transport = ASGITransport(app=app)
    async with AsyncClient(
        transport=transport,
        base_url="http://test",
        follow_redirects=True,
    ) as client:
        resp = await client.post(
            "/api/taxonomy/scopes",
            json={"name": "rt-sensitive-bypass-scope"},
        )
    app.dependency_overrides.pop(require_auth, None)

    created_id: str | None = None
    if resp.status_code == 200:
        created_id = resp.json()["data"]["id"]

    try:
        assert resp.status_code == 403
    finally:
        # Taxonomy tables are not truncated by the global test cleanup fixture.
        if created_id is not None:
            await db_pool.execute(
                "DELETE FROM privacy_scopes WHERE id = $1::uuid",
                created_id,
            )


@pytest.mark.asyncio
async def test_taxonomy_sensitive_scope_cannot_list(api, test_entity, enums):
    """A sensitive-only user should not list taxonomy rows."""

    auth_dict = {
        "key_id": None,
        "caller_type": "user",
        "entity_id": test_entity["id"],
        "entity": test_entity,
        "agent_id": None,
        "agent": None,
        "scopes": [enums.scopes.name_to_id["sensitive"]],
    }

    async def mock_auth() -> dict:
        """Handle mock auth.

        Returns:
            Result value from the operation.
        """

        return auth_dict

    app.dependency_overrides[require_auth] = mock_auth
    app.state.pool = api._transport.app.state.pool
    app.state.enums = enums
    transport = ASGITransport(app=app)
    async with AsyncClient(
        transport=transport,
        base_url="http://test",
        follow_redirects=True,
    ) as client:
        resp = await client.get("/api/taxonomy/scopes")
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 403
