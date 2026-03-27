"""Entity route tests."""

# Third-Party
import pytest
from httpx import ASGITransport, AsyncClient

# Local
from nebula_api.app import app
from nebula_api.auth import require_auth

LEGACY_SCOPE_NAMES = ("work", "code", "vault-only", "blacklisted")


def _agent_auth_override(agent_row: dict, scope_ids: list[int]) -> callable:
    """Build a dependency override for agent auth."""

    auth_dict = {
        "key_id": None,
        "caller_type": "agent",
        "entity_id": None,
        "entity": None,
        "agent_id": agent_row["id"],
        "agent": agent_row,
        "scopes": scope_ids,
    }

    async def mock_auth():
        """Handle mock auth.

        Returns:
            Result value from the operation.
        """

        return auth_dict

    return mock_auth


@pytest.mark.asyncio
async def test_create_entity(api):
    """Test create entity."""

    r = await api.post(
        "/api/entities",
        json={
            "name": "New Entity",
            "type": "person",
            "scopes": ["public"],
            "tags": ["test-tag"],
        },
    )
    assert r.status_code == 200
    data = r.json()["data"]
    assert data["name"] == "New Entity"
    assert "id" in data


@pytest.mark.asyncio
async def test_get_entity(api, test_entity):
    """Test get entity."""

    r = await api.get(f"/api/entities/{test_entity['id']}")
    assert r.status_code == 200
    data = r.json()["data"]
    assert data["name"] == "api-test-user"


@pytest.mark.asyncio
async def test_get_entity_not_found(api):
    """Test get entity not found."""

    r = await api.get("/api/entities/00000000-0000-0000-0000-000000000000")
    assert r.status_code == 404


@pytest.mark.asyncio
async def test_get_entity_invalid_id_returns_400(api):
    """Entity get should reject malformed ids."""

    r = await api.get("/api/entities/not-a-uuid")
    assert r.status_code == 400
    assert r.json()["detail"]["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
async def test_query_entities(api):
    """Test query entities."""

    await api.post(
        "/api/entities",
        json={"name": "QueryTest", "type": "person", "scopes": ["public"]},
    )
    r = await api.get("/api/entities", params={"type": "person"})
    assert r.status_code == 200
    data = r.json()["data"]
    assert len(data) >= 1


@pytest.mark.asyncio
async def test_create_entity_invalid_status_returns_400(api):
    """Entity create should reject unknown statuses."""

    r = await api.post(
        "/api/entities",
        json={
            "name": "Bad Status Entity",
            "type": "person",
            "status": "todo",
            "scopes": ["public"],
        },
    )
    assert r.status_code == 400
    assert r.json()["detail"]["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "invalid_scope",
    ("does-not-exist", *LEGACY_SCOPE_NAMES),
)
@pytest.mark.asyncio
async def test_create_entity_invalid_scope_returns_400(api, invalid_scope):
    """Entity create should reject inactive or unknown scope names."""

    r = await api.post(
        "/api/entities",
        json={
            "name": "Bad Scope Entity",
            "type": "person",
            "status": "active",
            "scopes": [invalid_scope],
        },
    )
    assert r.status_code == 400
    assert r.json()["detail"]["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
async def test_create_entity_agent_scope_subset_enforced(api_agent_auth):
    """Agent creates should reject scopes outside caller scope set."""

    r = await api_agent_auth.post(
        "/api/entities",
        json={"name": "TooWide", "type": "person", "scopes": ["public", "sensitive"]},
    )
    assert r.status_code == 400
    assert r.json()["detail"]["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
async def test_create_entity_executor_value_error_returns_400(api, monkeypatch):
    """Create route should normalize executor value errors."""

    async def _boom(*_args, **_kwargs):
        """Handle boom.

        Args:
            *_args: Input parameter for _boom.
            **_kwargs: Input parameter for _boom.
        """

        raise ValueError("entity create failed")

    monkeypatch.setattr("nebula_api.routes.entities.execute_create_entity", _boom)
    r = await api.post(
        "/api/entities",
        json={"name": "Boom", "type": "person", "scopes": ["public"]},
    )
    assert r.status_code == 400
    assert r.json()["detail"]["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
async def test_update_entity(api, test_entity):
    """Test update entity."""

    r = await api.patch(
        f"/api/entities/{test_entity['id']}",
        json={
            "tags": ["updated"],
            "status": "active",
        },
    )
    assert r.status_code == 200


@pytest.mark.asyncio
async def test_update_entity_invalid_id_returns_400(api):
    """Entity update should reject malformed entity ids."""

    r = await api.patch(
        "/api/entities/not-a-uuid",
        json={"status": "active"},
    )
    assert r.status_code == 400
    assert r.json()["detail"]["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
async def test_update_entity_invalid_status_returns_400(api, test_entity):
    """Entity update should reject unknown statuses."""

    r = await api.patch(
        f"/api/entities/{test_entity['id']}",
        json={"status": "todo"},
    )
    assert r.status_code == 400
    assert r.json()["detail"]["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
async def test_update_entity_with_null_tags_is_allowed(api, test_entity):
    """Update route should treat null tags as no-op."""

    r = await api.patch(
        f"/api/entities/{test_entity['id']}",
        json={"tags": None},
    )
    assert r.status_code == 200


@pytest.mark.asyncio
async def test_update_entity_untrusted_agent_returns_approval_required(
    db_pool, enums, untrusted_agent_row
):
    """Untrusted agents should queue entity updates for approval."""

    entity = await db_pool.fetchrow(
        """
        INSERT INTO entities (name, type_id, status_id, privacy_scope_ids, tags)
        VALUES ($1, $2::uuid, $3::uuid, $4::uuid[], $5)
        RETURNING id
        """,
        "Queued Update Entity",
        enums.entity_types.name_to_id["person"],
        enums.statuses.name_to_id["active"],
        [enums.scopes.name_to_id["public"]],
        [],
    )
    app.dependency_overrides[require_auth] = _agent_auth_override(
        {**untrusted_agent_row, "requires_approval": True},
        [enums.scopes.name_to_id["public"]],
    )
    app.state.pool = db_pool
    app.state.enums = enums
    transport = ASGITransport(app=app)
    async with AsyncClient(
        transport=transport, base_url="http://test", follow_redirects=True
    ) as client:
        r = await client.patch(
            f"/api/entities/{entity['id']}",
            json={"tags": ["queued"]},
        )
    app.dependency_overrides.pop(require_auth, None)

    assert r.status_code == 202
    assert r.json()["status"] == "approval_required"


@pytest.mark.asyncio
async def test_create_entity_allows_reuse_after_archive(api):
    """Archiving should allow re-creating same name/type/scopes."""

    create_resp = await api.post(
        "/api/entities",
        json={
            "name": "Reusable Entity Name",
            "type": "project",
            "status": "active",
            "scopes": ["public"],
        },
    )
    assert create_resp.status_code == 200, create_resp.text
    entity_id = create_resp.json()["data"]["id"]

    archive_resp = await api.patch(
        f"/api/entities/{entity_id}",
        json={"status": "archived", "status_reason": "archive then reuse"},
    )
    assert archive_resp.status_code == 200, archive_resp.text

    recreate_resp = await api.post(
        "/api/entities",
        json={
            "name": "Reusable Entity Name",
            "type": "project",
            "status": "active",
            "scopes": ["public"],
        },
    )
    assert recreate_resp.status_code == 200, recreate_resp.text
    assert recreate_resp.json()["data"]["id"] != entity_id


@pytest.mark.asyncio
async def test_create_entity_still_blocks_duplicate_active_name_scope(api):
    """Active entities should still enforce same name/type/scope uniqueness."""

    first_resp = await api.post(
        "/api/entities",
        json={
            "name": "Duplicate Active Entity",
            "type": "project",
            "status": "active",
            "scopes": ["public"],
        },
    )
    assert first_resp.status_code == 200, first_resp.text

    dup_resp = await api.post(
        "/api/entities",
        json={
            "name": "Duplicate Active Entity",
            "type": "project",
            "status": "active",
            "scopes": ["public"],
        },
    )
    assert dup_resp.status_code == 400, dup_resp.text
    assert dup_resp.json()["detail"]["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
async def test_create_entity_allows_same_name_with_different_scope_set(api):
    """Same name/type should be allowed when scope set differs."""

    first_resp = await api.post(
        "/api/entities",
        json={
            "name": "Scope Variant Entity",
            "type": "project",
            "status": "active",
            "scopes": ["public"],
        },
    )
    assert first_resp.status_code == 200, first_resp.text

    second_resp = await api.post(
        "/api/entities",
        json={
            "name": "Scope Variant Entity",
            "type": "project",
            "status": "active",
            "scopes": ["private"],
        },
    )
    assert second_resp.status_code == 200, second_resp.text
    assert second_resp.json()["data"]["id"] != first_resp.json()["data"]["id"]


@pytest.mark.asyncio
async def test_create_entity_allows_same_name_with_different_type(api):
    """Same name/scope should be allowed when type differs."""

    first_resp = await api.post(
        "/api/entities",
        json={
            "name": "Type Variant Entity",
            "type": "project",
            "status": "active",
            "scopes": ["public"],
        },
    )
    assert first_resp.status_code == 200, first_resp.text

    second_resp = await api.post(
        "/api/entities",
        json={
            "name": "Type Variant Entity",
            "type": "tool",
            "status": "active",
            "scopes": ["public"],
        },
    )
    assert second_resp.status_code == 200, second_resp.text
    assert second_resp.json()["data"]["id"] != first_resp.json()["data"]["id"]


@pytest.mark.asyncio
async def test_update_entity_executor_value_error_returns_400(api, test_entity, monkeypatch):
    """Update route should normalize executor value errors."""

    async def _boom(*_args, **_kwargs):
        """Handle boom.

        Args:
            *_args: Input parameter for _boom.
            **_kwargs: Input parameter for _boom.
        """

        raise ValueError("entity update failed")

    monkeypatch.setattr("nebula_api.routes.entities.execute_update_entity", _boom)
    r = await api.patch(
        f"/api/entities/{test_entity['id']}",
        json={"status": "active"},
    )
    assert r.status_code == 400
    assert r.json()["detail"]["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
async def test_bulk_update_tags_requires_entity_ids(api):
    """Bulk tag updates should fail without entity ids."""

    r = await api.post(
        "/api/entities/bulk/tags",
        json={"entity_ids": [], "tags": ["x"], "op": "add"},
    )
    assert r.status_code == 400
    assert r.json()["detail"]["error"]["code"] == "VALIDATION_ERROR"


@pytest.mark.asyncio
async def test_bulk_update_tags_requires_tags_for_add(api, test_entity):
    """Bulk tag add should fail when tag list is empty."""

    r = await api.post(
        "/api/entities/bulk/tags",
        json={"entity_ids": [str(test_entity["id"])], "tags": [], "op": "add"},
    )
    assert r.status_code == 400
    assert r.json()["detail"]["error"]["code"] == "VALIDATION_ERROR"


@pytest.mark.asyncio
async def test_bulk_update_scopes_requires_entity_ids(api):
    """Bulk scope updates should fail without entity ids."""

    r = await api.post(
        "/api/entities/bulk/scopes",
        json={"entity_ids": [], "scopes": ["public"], "op": "add"},
    )
    assert r.status_code == 400
    assert r.json()["detail"]["error"]["code"] == "VALIDATION_ERROR"


@pytest.mark.asyncio
async def test_bulk_update_scopes_invalid_scope_returns_400(api, test_entity):
    """Bulk scope updates should reject invalid scope names."""

    r = await api.post(
        "/api/entities/bulk/scopes",
        json={
            "entity_ids": [str(test_entity["id"])],
            "scopes": ["does-not-exist"],
            "op": "add",
        },
    )
    assert r.status_code == 400
    assert r.json()["detail"]["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
async def test_bulk_update_scopes_requires_scopes_for_add(api, test_entity):
    """Bulk scope add should fail when scope list is empty."""

    r = await api.post(
        "/api/entities/bulk/scopes",
        json={"entity_ids": [str(test_entity["id"])], "scopes": [], "op": "add"},
    )
    assert r.status_code == 400
    assert r.json()["detail"]["error"]["code"] == "VALIDATION_ERROR"


@pytest.mark.asyncio
async def test_bulk_update_tags_agent_entity_not_found_returns_404(db_pool, enums):
    """Agent bulk updates should return 404 when ids are missing."""

    status_id = enums.statuses.name_to_id["active"]
    public_scope = enums.scopes.name_to_id["public"]
    agent = await db_pool.fetchrow(
        """
        INSERT INTO agents (name, description, scopes, requires_approval, status_id)
        VALUES ($1, $2, $3::uuid[], $4, $5::uuid)
        RETURNING *
        """,
        "bulk-not-found-agent",
        "agent",
        [public_scope],
        False,
        status_id,
    )
    app.dependency_overrides[require_auth] = _agent_auth_override(dict(agent), [public_scope])
    app.state.pool = db_pool
    app.state.enums = enums
    transport = ASGITransport(app=app)
    async with AsyncClient(
        transport=transport, base_url="http://test", follow_redirects=True
    ) as client:
        r = await client.post(
            "/api/entities/bulk/tags",
            json={
                "entity_ids": ["00000000-0000-0000-0000-000000000000"],
                "tags": ["x"],
                "op": "add",
            },
        )
    app.dependency_overrides.pop(require_auth, None)

    assert r.status_code == 404
    assert r.json()["detail"]["error"]["code"] == "NOT_FOUND"


@pytest.mark.asyncio
async def test_bulk_update_tags_agent_scope_guard_rejects_without_scopes(
    db_pool, enums, test_entity
):
    """Agent bulk updates should fail when caller has no write scopes."""

    status_id = enums.statuses.name_to_id["active"]
    agent = await db_pool.fetchrow(
        """
        INSERT INTO agents (name, description, scopes, requires_approval, status_id)
        VALUES ($1, $2, $3::uuid[], $4, $5::uuid)
        RETURNING *
        """,
        "bulk-no-scopes-agent",
        "agent",
        [],
        False,
        status_id,
    )
    app.dependency_overrides[require_auth] = _agent_auth_override(dict(agent), [])
    app.state.pool = db_pool
    app.state.enums = enums
    transport = ASGITransport(app=app)
    async with AsyncClient(
        transport=transport, base_url="http://test", follow_redirects=True
    ) as client:
        r = await client.post(
            "/api/entities/bulk/tags",
            json={
                "entity_ids": [str(test_entity["id"])],
                "tags": ["x"],
                "op": "add",
            },
        )
    app.dependency_overrides.pop(require_auth, None)

    assert r.status_code == 403
    assert r.json()["detail"]["error"]["code"] == "FORBIDDEN"


@pytest.mark.asyncio
async def test_bulk_update_tags_allows_entities_without_scopes(db_pool, enums):
    """Agent updates should allow entities without assigned privacy scopes."""

    status_id = enums.statuses.name_to_id["active"]
    type_id = enums.entity_types.name_to_id["person"]
    entity = await db_pool.fetchrow(
        """
        INSERT INTO entities (name, type_id, status_id, privacy_scope_ids, tags)
        VALUES ($1, $2::uuid, $3::uuid, $4::uuid[], $5)
        RETURNING id
        """,
        "NoScopeEntity",
        type_id,
        status_id,
        [],
        [],
    )
    public_scope = enums.scopes.name_to_id["public"]
    agent = await db_pool.fetchrow(
        """
        INSERT INTO agents (name, description, scopes, requires_approval, status_id)
        VALUES ($1, $2, $3::uuid[], $4, $5::uuid)
        RETURNING *
        """,
        "bulk-empty-scope-entity-agent",
        "agent",
        [public_scope],
        False,
        status_id,
    )
    app.dependency_overrides[require_auth] = _agent_auth_override(dict(agent), [public_scope])
    app.state.pool = db_pool
    app.state.enums = enums
    transport = ASGITransport(app=app)
    async with AsyncClient(
        transport=transport, base_url="http://test", follow_redirects=True
    ) as client:
        r = await client.post(
            "/api/entities/bulk/tags",
            json={"entity_ids": [str(entity["id"])], "tags": ["x"], "op": "add"},
        )
    app.dependency_overrides.pop(require_auth, None)

    assert r.status_code == 200
    assert r.json()["data"]["updated"] == 1


@pytest.mark.asyncio
async def test_bulk_update_scopes_agent_subset_enforced(api_agent_auth, test_entity):
    """Agent bulk scope updates should reject expansion outside caller scopes."""

    r = await api_agent_auth.post(
        "/api/entities/bulk/scopes",
        json={
            "entity_ids": [str(test_entity["id"])],
            "scopes": ["public", "sensitive"],
            "op": "add",
        },
    )
    assert r.status_code == 400
    assert r.json()["detail"]["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
async def test_bulk_update_scopes_untrusted_agent_returns_approval_required(
    db_pool, enums, untrusted_agent_row
):
    """Untrusted agents should queue bulk scope updates."""

    entity = await db_pool.fetchrow(
        """
        INSERT INTO entities (name, type_id, status_id, privacy_scope_ids, tags)
        VALUES ($1, $2::uuid, $3::uuid, $4::uuid[], $5)
        RETURNING id
        """,
        "Queued Scope Entity",
        enums.entity_types.name_to_id["person"],
        enums.statuses.name_to_id["active"],
        [enums.scopes.name_to_id["public"]],
        [],
    )
    app.dependency_overrides[require_auth] = _agent_auth_override(
        {**untrusted_agent_row, "requires_approval": True},
        [enums.scopes.name_to_id["public"]],
    )
    app.state.pool = db_pool
    app.state.enums = enums
    transport = ASGITransport(app=app)
    async with AsyncClient(
        transport=transport, base_url="http://test", follow_redirects=True
    ) as client:
        r = await client.post(
            "/api/entities/bulk/scopes",
            json={
                "entity_ids": [str(entity["id"])],
                "scopes": ["public"],
                "op": "set",
            },
        )
    app.dependency_overrides.pop(require_auth, None)

    assert r.status_code == 202
    assert r.json()["status"] == "approval_required"


@pytest.mark.asyncio
async def test_bulk_update_tags_untrusted_agent_returns_approval_required(
    db_pool, enums, untrusted_agent_row
):
    """Untrusted agents should queue bulk tag updates."""

    entity = await db_pool.fetchrow(
        """
        INSERT INTO entities (name, type_id, status_id, privacy_scope_ids, tags)
        VALUES ($1, $2::uuid, $3::uuid, $4::uuid[], $5)
        RETURNING id
        """,
        "Queued Tag Entity",
        enums.entity_types.name_to_id["person"],
        enums.statuses.name_to_id["active"],
        [enums.scopes.name_to_id["public"]],
        [],
    )
    app.dependency_overrides[require_auth] = _agent_auth_override(
        {**untrusted_agent_row, "requires_approval": True},
        [enums.scopes.name_to_id["public"]],
    )
    app.state.pool = db_pool
    app.state.enums = enums
    transport = ASGITransport(app=app)
    async with AsyncClient(
        transport=transport, base_url="http://test", follow_redirects=True
    ) as client:
        r = await client.post(
            "/api/entities/bulk/tags",
            json={
                "entity_ids": [str(entity["id"])],
                "tags": ["queued"],
                "op": "add",
            },
        )
    app.dependency_overrides.pop(require_auth, None)

    assert r.status_code == 202
    assert r.json()["status"] == "approval_required"


@pytest.mark.asyncio
async def test_entity_history_not_found_returns_404(api):
    """History endpoint should return 404 for missing entities."""

    r = await api.get("/api/entities/00000000-0000-0000-0000-000000000000/history")
    assert r.status_code == 404


@pytest.mark.asyncio
async def test_entity_history_success_returns_rows(api, test_entity):
    """History endpoint should return entries for existing entities."""

    await api.patch(
        f"/api/entities/{test_entity['id']}",
        json={"tags": ["history-test"]},
    )
    r = await api.get(f"/api/entities/{test_entity['id']}/history")
    assert r.status_code == 200
    assert isinstance(r.json()["data"], list)


@pytest.mark.asyncio
async def test_revert_entity_forbidden_for_agents(db_pool, enums, test_entity):
    """Entity revert should be blocked for agent callers."""

    status_id = enums.statuses.name_to_id["active"]
    public_scope = enums.scopes.name_to_id["public"]
    agent = await db_pool.fetchrow(
        """
        INSERT INTO agents (name, description, scopes, requires_approval, status_id)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING *
        """,
        "revert-agent",
        "agent",
        [public_scope],
        False,
        status_id,
    )

    async def mock_auth():
        """Handle mock auth.

        Returns:
            Result value from the operation.
        """

        return {
            "key_id": None,
            "caller_type": "agent",
            "entity_id": None,
            "entity": None,
            "agent_id": agent["id"],
            "agent": dict(agent),
            "scopes": [public_scope],
        }

    app.dependency_overrides[require_auth] = mock_auth
    app.state.pool = db_pool
    app.state.enums = enums
    transport = ASGITransport(app=app)
    async with AsyncClient(
        transport=transport, base_url="http://test", follow_redirects=True
    ) as client:
        r = await client.post(
            f"/api/entities/{test_entity['id']}/revert",
            json={"audit_id": "00000000-0000-0000-0000-000000000000"},
        )
    app.dependency_overrides.pop(require_auth, None)

    assert r.status_code == 403
    assert r.json()["detail"]["error"]["code"] == "FORBIDDEN"


@pytest.mark.asyncio
async def test_query_entities_respects_user_scopes_not_public_only(api):
    """Entity query should include private-scoped rows when caller has private scope."""

    create_resp = await api.post(
        "/api/entities",
        json={
            "name": "PrivateVisibleEntity",
            "type": "person",
            "scopes": ["private"],
        },
    )
    assert create_resp.status_code == 200

    query_resp = await api.get("/api/entities")
    assert query_resp.status_code == 200
    names = [row["name"] for row in query_resp.json()["data"]]
    assert "PrivateVisibleEntity" in names


@pytest.mark.asyncio
async def test_query_with_pagination(api):
    """Test query with pagination."""

    for i in range(3):
        await api.post(
            "/api/entities",
            json={"name": f"Page{i}", "type": "person", "scopes": ["public"]},
        )
    r = await api.get("/api/entities", params={"limit": 2, "offset": 0})
    assert r.status_code == 200
    meta = r.json()["meta"]
    assert meta["limit"] == 2
    assert meta["offset"] == 0
