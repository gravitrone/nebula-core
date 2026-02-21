"""Context route tests."""

# Third-Party
from httpx import ASGITransport, AsyncClient
import pytest

# Local
from nebula_api.app import app
from nebula_api.auth import require_auth
from nebula_mcp.models import MAX_TAGS

LEGACY_SCOPE_NAMES = ("work", "code", "vault-only", "blacklisted")


def _agent_auth_override(agent_row: dict, scope_ids: list[int]) -> callable:
    """Build an auth override that simulates an agent caller."""

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
async def test_create_context(api):
    """Test create context."""

    r = await api.post(
        "/api/context",
        json={
            "title": "Test Article",
            "url": "https://example.com/article",
            "source_type": "article",
            "content": "some content",
            "scopes": ["public"],
            "tags": ["test"],
        },
    )
    assert r.status_code == 200
    data = r.json()["data"]
    assert data["title"] == "Test Article"


@pytest.mark.asyncio
async def test_query_context(api):
    """Test query context."""

    await api.post(
        "/api/context",
        json={
            "title": "QueryContext",
            "source_type": "video",
            "scopes": ["public"],
        },
    )
    r = await api.get("/api/context", params={"source_type": "video"})
    assert r.status_code == 200
    data = r.json()["data"]
    assert len(data) >= 1


@pytest.mark.asyncio
async def test_link_context_to_entity(api):
    """Test link context to entity."""

    kr = await api.post(
        "/api/context",
        json={
            "title": "LinkTest",
            "scopes": ["public"],
        },
    )
    k_id = kr.json()["data"]["id"]

    er = await api.post(
        "/api/entities",
        json={
            "name": "LinkTarget",
            "type": "person",
            "scopes": ["public"],
        },
    )
    e_id = er.json()["data"]["id"]

    r = await api.post(
        f"/api/context/{k_id}/link",
        json={
            "entity_id": str(e_id),
            "relationship_type": "related-to",
        },
    )
    assert r.status_code == 200


@pytest.mark.asyncio
async def test_query_context_pagination(api):
    """Test query context pagination."""

    for i in range(3):
        await api.post(
            "/api/context",
            json={
                "title": f"KPage{i}",
                "scopes": ["public"],
            },
        )
    r = await api.get("/api/context", params={"limit": 2})
    assert r.status_code == 200
    meta = r.json()["meta"]
    assert meta["limit"] == 2


@pytest.mark.asyncio
async def test_create_context_validation_errors(api):
    """Create route should reject invalid URL, tags, and scopes."""

    invalid_url = await api.post(
        "/api/context",
        json={"title": "BadUrl", "url": "ftp://bad", "scopes": ["public"]},
    )
    assert invalid_url.status_code == 422

    too_many_tags = await api.post(
        "/api/context",
        json={
            "title": "TooManyTags",
            "scopes": ["public"],
            "tags": [f"t{i}" for i in range(MAX_TAGS + 1)],
        },
    )
    assert too_many_tags.status_code == 422

    for scope in ("not-a-scope", *LEGACY_SCOPE_NAMES):
        bad_scope = await api.post(
            "/api/context",
            json={"title": f"BadScope-{scope}", "scopes": [scope]},
        )
        assert bad_scope.status_code == 400


@pytest.mark.asyncio
async def test_get_context_validation_and_not_found(api):
    """Get route should validate context ids."""

    invalid = await api.get("/api/context/not-a-uuid")
    assert invalid.status_code == 400

    missing = await api.get("/api/context/00000000-0000-0000-0000-000000000001")
    assert missing.status_code == 404


@pytest.mark.asyncio
async def test_link_context_validation_and_relationship_type_errors(api):
    """Link route should reject invalid ids and unknown relationship types."""

    context = (
        await api.post("/api/context", json={"title": "CtxLink", "scopes": ["public"]})
    ).json()["data"]
    entity = (
        await api.post(
            "/api/entities",
            json={"name": "EntityLink", "type": "person", "scopes": ["public"]},
        )
    ).json()["data"]

    invalid_ids = await api.post(
        f"/api/context/{context['id']}/link",
        json={"entity_id": "not-a-uuid", "relationship_type": "related-to"},
    )
    assert invalid_ids.status_code == 400

    bad_rel_type = await api.post(
        f"/api/context/{context['id']}/link",
        json={"entity_id": entity["id"], "relationship_type": "does-not-exist"},
    )
    assert bad_rel_type.status_code == 400


@pytest.mark.asyncio
async def test_update_context_validation_errors(api):
    """Update route should validate ids, URL, status, and scopes."""

    context = (
        await api.post(
            "/api/context",
            json={"title": "CtxUpdate", "url": "https://ok", "scopes": ["public"]},
        )
    ).json()["data"]

    bad_id = await api.patch("/api/context/not-a-uuid", json={"title": "X"})
    assert bad_id.status_code == 400

    bad_url = await api.patch(
        f"/api/context/{context['id']}", json={"url": "file://bad"}
    )
    assert bad_url.status_code == 422

    bad_status = await api.patch(
        f"/api/context/{context['id']}",
        json={"status": "does-not-exist"},
    )
    assert bad_status.status_code == 400

    bad_scope = await api.patch(
        f"/api/context/{context['id']}",
        json={"scopes": ["invalid-scope"]},
    )
    assert bad_scope.status_code == 400


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "invalid_scope",
    ("invalid-scope", *LEGACY_SCOPE_NAMES),
)
async def test_update_context_rejects_legacy_scope_names(api, invalid_scope):
    """Update route should reject inactive legacy scopes by default."""

    context = (
        await api.post(
            "/api/context",
            json={"title": "CtxLegacyScope", "url": "https://ok", "scopes": ["public"]},
        )
    ).json()["data"]

    bad_scope = await api.patch(
        f"/api/context/{context['id']}",
        json={"scopes": [invalid_scope]},
    )
    assert bad_scope.status_code == 400


@pytest.mark.asyncio
async def test_update_context_agent_scope_subset_enforced(api_agent_auth):
    """Agent updates should reject scope expansion outside caller scopes."""

    created = await api_agent_auth.post(
        "/api/context",
        json={"title": "AgentCtx", "scopes": ["public"]},
    )
    assert created.status_code == 200
    context_id = created.json()["data"]["id"]

    expanded = await api_agent_auth.patch(
        f"/api/context/{context_id}",
        json={"scopes": ["public", "sensitive"]},
    )
    assert expanded.status_code == 400


@pytest.mark.asyncio
async def test_create_context_agent_scope_subset_enforced(api_agent_auth):
    """Agent creates should reject scopes outside the caller subset."""

    created = await api_agent_auth.post(
        "/api/context",
        json={"title": "TooWide", "scopes": ["public", "sensitive"]},
    )
    assert created.status_code == 400
    assert "scope" in created.json()["detail"].lower()


@pytest.mark.asyncio
async def test_create_context_untrusted_agent_returns_approval_required(
    db_pool, enums, untrusted_agent_row
):
    """Untrusted agents should queue context creates for approval."""

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
        resp = await client.post(
            "/api/context",
            json={"title": "Queued Context", "scopes": ["public"]},
        )
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 202
    assert resp.json()["status"] == "approval_required"


@pytest.mark.asyncio
async def test_link_context_untrusted_agent_returns_approval_required(
    db_pool, enums, untrusted_agent_row
):
    """Untrusted agents should queue context link requests."""

    context = await db_pool.fetchrow(
        """
        INSERT INTO context_items (title, source_type, privacy_scope_ids, status_id, tags, metadata)
        VALUES ($1, $2, $3::uuid[], $4::uuid, $5, $6::jsonb)
        RETURNING id
        """,
        "Ctx Queue Link",
        "note",
        [enums.scopes.name_to_id["public"]],
        enums.statuses.name_to_id["active"],
        [],
        "{}",
    )
    entity = await db_pool.fetchrow(
        """
        INSERT INTO entities (name, type_id, status_id, privacy_scope_ids, tags, metadata)
        VALUES ($1, $2::uuid, $3::uuid, $4::uuid[], $5, $6::jsonb)
        RETURNING id
        """,
        "Entity Queue Link",
        enums.entity_types.name_to_id["person"],
        enums.statuses.name_to_id["active"],
        [enums.scopes.name_to_id["public"]],
        [],
        "{}",
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
        resp = await client.post(
            f"/api/context/{context['id']}/link",
            json={"entity_id": str(entity["id"]), "relationship_type": "related-to"},
        )
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 202
    assert resp.json()["status"] == "approval_required"


@pytest.mark.asyncio
async def test_link_context_entity_scope_forbidden_for_agent(db_pool, enums):
    """Agent link writes should fail when target entity scopes exceed caller scopes."""

    context = await db_pool.fetchrow(
        """
        INSERT INTO context_items (title, source_type, privacy_scope_ids, status_id, tags, metadata)
        VALUES ($1, $2, $3::uuid[], $4::uuid, $5, $6::jsonb)
        RETURNING id
        """,
        "Ctx Scope Guard",
        "note",
        [enums.scopes.name_to_id["public"]],
        enums.statuses.name_to_id["active"],
        [],
        "{}",
    )
    entity = await db_pool.fetchrow(
        """
        INSERT INTO entities (name, type_id, status_id, privacy_scope_ids, tags, metadata)
        VALUES ($1, $2::uuid, $3::uuid, $4::uuid[], $5, $6::jsonb)
        RETURNING id
        """,
        "Entity Scope Guard",
        enums.entity_types.name_to_id["person"],
        enums.statuses.name_to_id["active"],
        [enums.scopes.name_to_id["public"], enums.scopes.name_to_id["sensitive"]],
        [],
        "{}",
    )

    status_id = enums.statuses.name_to_id["active"]
    agent = await db_pool.fetchrow(
        """
        INSERT INTO agents (name, description, scopes, requires_approval, status_id)
        VALUES ($1, $2, $3::uuid[], $4, $5::uuid)
        RETURNING *
        """,
        "context-scope-agent",
        "scope guard",
        [enums.scopes.name_to_id["public"]],
        False,
        status_id,
    )

    app.dependency_overrides[require_auth] = _agent_auth_override(
        dict(agent),
        [enums.scopes.name_to_id["public"]],
    )
    app.state.pool = db_pool
    app.state.enums = enums
    transport = ASGITransport(app=app)
    async with AsyncClient(
        transport=transport, base_url="http://test", follow_redirects=True
    ) as client:
        resp = await client.post(
            f"/api/context/{context['id']}/link",
            json={"entity_id": str(entity["id"]), "relationship_type": "related-to"},
        )
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 403
    assert resp.json()["detail"] == "Forbidden"


@pytest.mark.asyncio
async def test_update_context_admin_agent_can_bypass_scope_guard(db_pool, enums):
    """Admin agent callers should bypass context scope write restrictions."""

    context = await db_pool.fetchrow(
        """
        INSERT INTO context_items (title, source_type, privacy_scope_ids, status_id, tags, metadata)
        VALUES ($1, $2, $3::uuid[], $4::uuid, $5, $6::jsonb)
        RETURNING id
        """,
        "Ctx Admin Guard",
        "note",
        [enums.scopes.name_to_id["sensitive"]],
        enums.statuses.name_to_id["active"],
        [],
        "{}",
    )
    admin_scope = enums.scopes.name_to_id.get("admin")
    if not admin_scope:
        pytest.skip("admin scope unavailable in taxonomy")

    status_id = enums.statuses.name_to_id["active"]
    agent = await db_pool.fetchrow(
        """
        INSERT INTO agents (name, description, scopes, requires_approval, status_id)
        VALUES ($1, $2, $3::uuid[], $4, $5::uuid)
        RETURNING *
        """,
        "context-admin-agent",
        "admin",
        [admin_scope],
        False,
        status_id,
    )

    app.dependency_overrides[require_auth] = _agent_auth_override(
        dict(agent),
        [admin_scope],
    )
    app.state.pool = db_pool
    app.state.enums = enums
    transport = ASGITransport(app=app)
    async with AsyncClient(
        transport=transport, base_url="http://test", follow_redirects=True
    ) as client:
        resp = await client.patch(
            f"/api/context/{context['id']}",
            json={"title": "Ctx Admin Updated"},
        )
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 200
    assert resp.json()["data"]["title"] == "Ctx Admin Updated"


@pytest.mark.asyncio
async def test_update_context_untrusted_agent_returns_approval_required(
    db_pool, enums, untrusted_agent_row
):
    """Untrusted agents should queue context updates for approval."""

    context = await db_pool.fetchrow(
        """
        INSERT INTO context_items (title, source_type, privacy_scope_ids, status_id, tags, metadata)
        VALUES ($1, $2, $3::uuid[], $4::uuid, $5, $6::jsonb)
        RETURNING id
        """,
        "Ctx Queue Update",
        "note",
        [enums.scopes.name_to_id["public"]],
        enums.statuses.name_to_id["active"],
        [],
        "{}",
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
        resp = await client.patch(
            f"/api/context/{context['id']}",
            json={"title": "Queued Updated"},
        )
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 202
    assert resp.json()["status"] == "approval_required"


@pytest.mark.asyncio
async def test_create_context_blank_url_allowed(api):
    """Create should allow blank URL values after validator normalization."""

    resp = await api.post(
        "/api/context",
        json={"title": "Blank URL", "url": "", "scopes": ["public"]},
    )
    assert resp.status_code == 200


@pytest.mark.asyncio
async def test_update_context_accepts_valid_url_and_null_tags(api):
    """Update should accept valid URLs and explicit null tags."""

    created = (
        await api.post(
            "/api/context",
            json={"title": "Ctx URL Update", "scopes": ["public"]},
        )
    ).json()["data"]

    resp = await api.patch(
        f"/api/context/{created['id']}",
        json={"url": "https://example.com/new", "tags": None},
    )
    assert resp.status_code == 200
    assert resp.json()["data"]["url"] == "https://example.com/new"


@pytest.mark.asyncio
async def test_create_context_executor_value_error_returns_400(api, monkeypatch):
    """Create should convert executor value errors to 400 responses."""

    async def _boom(*_args, **_kwargs):
        """Handle boom.

        Args:
            *_args: Input parameter for _boom.
            **_kwargs: Input parameter for _boom.
        """

        raise ValueError("ctx create failed")

    monkeypatch.setattr("nebula_api.routes.context.execute_create_context", _boom)
    resp = await api.post(
        "/api/context",
        json={"title": "CtxFail", "scopes": ["public"]},
    )
    assert resp.status_code == 400
    assert resp.json()["detail"] == "ctx create failed"


@pytest.mark.asyncio
async def test_link_context_executor_value_error_returns_400(api, monkeypatch):
    """Link should convert executor value errors to 400 responses."""

    context = (
        await api.post(
            "/api/context", json={"title": "CtxLinkFail", "scopes": ["public"]}
        )
    ).json()["data"]
    entity = (
        await api.post(
            "/api/entities",
            json={"name": "EntityLinkFail", "type": "person", "scopes": ["public"]},
        )
    ).json()["data"]

    async def _boom(*_args, **_kwargs):
        """Handle boom.

        Args:
            *_args: Input parameter for _boom.
            **_kwargs: Input parameter for _boom.
        """

        raise ValueError("ctx link failed")

    monkeypatch.setattr("nebula_api.routes.context.execute_create_relationship", _boom)
    resp = await api.post(
        f"/api/context/{context['id']}/link",
        json={"entity_id": entity["id"], "relationship_type": "related-to"},
    )
    assert resp.status_code == 400
    assert resp.json()["detail"] == "ctx link failed"


@pytest.mark.asyncio
async def test_update_context_executor_value_error_returns_400(api, monkeypatch):
    """Update should convert executor value errors to 400 responses."""

    context = (
        await api.post(
            "/api/context",
            json={"title": "CtxUpdateFail", "scopes": ["public"]},
        )
    ).json()["data"]

    async def _boom(*_args, **_kwargs):
        """Handle boom.

        Args:
            *_args: Input parameter for _boom.
            **_kwargs: Input parameter for _boom.
        """

        raise ValueError("ctx update failed")

    monkeypatch.setattr("nebula_api.routes.context.execute_update_context", _boom)
    resp = await api.patch(
        f"/api/context/{context['id']}",
        json={"title": "new title"},
    )
    assert resp.status_code == 400
    assert resp.json()["detail"] == "ctx update failed"
