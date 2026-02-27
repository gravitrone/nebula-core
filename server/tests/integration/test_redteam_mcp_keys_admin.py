"""Redteam integration coverage for MCP key/admin/login tool paths."""

# Standard Library
from typing import Any
from unittest.mock import MagicMock

# Third-Party
import pytest

# Local
from nebula_mcp.models import (
    ApproveRequestInput,
    CreateAPIKeyInput,
    CreateEntityInput,
    CreateProtocolInput,
    GetAgentInput,
    GetApprovalInput,
    ListAuditActorsInput,
    ListAllKeysInput,
    LoginInput,
    PendingApprovalsInput,
    QueryAuditLogInput,
    QueryProtocolsInput,
    RejectRequestInput,
    RevokeKeyInput,
    UpdateAgentInput,
)
from nebula_mcp.server import (
    approve_request,
    create_api_key,
    create_entity,
    create_protocol,
    get_agent,
    get_approval,
    get_pending_approvals,
    get_pending_approvals_all,
    list_audit_actors,
    list_audit_scopes,
    list_all_api_keys,
    list_api_keys,
    login_user,
    query_audit_log,
    query_protocols,
    reject_request,
    revoke_api_key,
    update_agent,
)

pytestmark = pytest.mark.integration


def _mcp_ctx(pool: Any, enums: Any, agent: dict[str, Any] | None) -> MagicMock:
    """Build an MCP context for integration tests."""

    ctx = MagicMock()
    ctx.request_context.lifespan_context = {
        "pool": pool,
        "enums": enums,
        "agent": agent,
    }
    return ctx


async def _create_agent(
    db_pool: Any,
    enums: Any,
    *,
    name: str,
    scopes: list[str],
    requires_approval: bool = False,
) -> dict[str, Any]:
    """Insert and return an active agent row."""

    row = await db_pool.fetchrow(
        """
        INSERT INTO agents (name, description, scopes, requires_approval, status_id)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING *
        """,
        name,
        f"{name} integration agent",
        [enums.scopes.name_to_id[s] for s in scopes],
        requires_approval,
        enums.statuses.name_to_id["active"],
    )
    return dict(row)


async def _create_person_entity(
    db_pool: Any, enums: Any, *, name: str
) -> dict[str, Any]:
    """Insert and return a person entity."""

    row = await db_pool.fetchrow(
        """
        INSERT INTO entities (name, type_id, status_id, privacy_scope_ids, tags, metadata)
        VALUES ($1, $2, $3, $4, $5, $6::jsonb)
        RETURNING *
        """,
        name,
        enums.entity_types.name_to_id["person"],
        enums.statuses.name_to_id["active"],
        [enums.scopes.name_to_id["public"]],
        [],
        "{}",
    )
    return dict(row)


async def test_login_user_bootstrap_creates_entity_and_key(
    bootstrap_mcp_context, db_pool, enums
):
    """login_user should create a person entity and return an API key in bootstrap mode."""

    result = await login_user(
        LoginInput(username="mcp-login-user"), bootstrap_mcp_context
    )
    assert result["username"] == "mcp-login-user"
    assert result["api_key"].startswith("nbl_")

    row = await db_pool.fetchrow(
        "SELECT * FROM entities WHERE id = $1::uuid",
        result["entity_id"],
    )
    assert row is not None
    assert row["type_id"] == enums.entity_types.name_to_id["person"]


async def test_login_user_reuses_existing_entity_and_adds_baseline_scopes(
    bootstrap_mcp_context, db_pool, enums
):
    """login_user should reuse an existing person and enforce baseline scope union."""

    row = await db_pool.fetchrow(
        """
        INSERT INTO entities (name, type_id, status_id, privacy_scope_ids, tags, metadata)
        VALUES ($1, $2, $3, $4, $5, $6::jsonb)
        RETURNING *
        """,
        "reuse-login-user",
        enums.entity_types.name_to_id["person"],
        enums.statuses.name_to_id["active"],
        [enums.scopes.name_to_id["public"]],
        [],
        "{}",
    )
    assert row is not None

    result = await login_user(
        LoginInput(username="reuse-login-user"), bootstrap_mcp_context
    )
    assert result["entity_id"] == str(row["id"])

    refreshed = await db_pool.fetchrow(
        "SELECT privacy_scope_ids FROM entities WHERE id = $1::uuid",
        row["id"],
    )
    assert refreshed is not None
    scope_names = {
        name
        for name, scope_id in enums.scopes.name_to_id.items()
        if scope_id in refreshed["privacy_scope_ids"]
    }
    assert {"public", "private", "sensitive", "admin"}.issubset(scope_names)


async def test_list_all_api_keys_requires_admin(db_pool, enums):
    """Non-admin agents should be denied list_all_api_keys."""

    public_agent = await _create_agent(
        db_pool,
        enums,
        name="keys-public-agent",
        scopes=["public"],
    )
    public_ctx = _mcp_ctx(db_pool, enums, public_agent)

    with pytest.raises(ValueError, match="Admin scope required"):
        await list_all_api_keys(ListAllKeysInput(limit=10, offset=0), public_ctx)


@pytest.mark.parametrize(
    ("tool_name", "invoker"),
    [
        (
            "list_all_api_keys",
            lambda ctx, target: list_all_api_keys(
                ListAllKeysInput(limit=1, offset=0),
                ctx,
            ),
        ),
        (
            "list_audit_scopes",
            lambda ctx, target: list_audit_scopes(ctx),
        ),
        (
            "list_audit_actors",
            lambda ctx, target: list_audit_actors(
                ListAuditActorsInput(actor_type="agent"),
                ctx,
            ),
        ),
        (
            "query_audit_log",
            lambda ctx, target: query_audit_log(
                QueryAuditLogInput(limit=1),
                ctx,
            ),
        ),
        (
            "get_pending_approvals_all",
            lambda ctx, target: get_pending_approvals_all(
                PendingApprovalsInput(limit=1),
                ctx,
            ),
        ),
        (
            "get_agent",
            lambda ctx, target: get_agent(
                GetAgentInput(agent_id=target),
                ctx,
            ),
        ),
        (
            "update_agent",
            lambda ctx, target: update_agent(
                UpdateAgentInput(
                    agent_id=target,
                    description="forbidden update",
                ),
                ctx,
            ),
        ),
    ],
)
async def test_admin_tools_forbidden_for_non_admin_matrix(
    db_pool, enums, tool_name, invoker
):
    """All admin-only MCP tools should reject non-admin agents consistently."""

    public_agent = await _create_agent(
        db_pool,
        enums,
        name=f"matrix-public-{tool_name}",
        scopes=["public"],
    )
    target_agent = await _create_agent(
        db_pool,
        enums,
        name=f"matrix-target-{tool_name}",
        scopes=["public"],
    )
    public_ctx = _mcp_ctx(db_pool, enums, public_agent)

    with pytest.raises(ValueError, match="Admin scope required"):
        await invoker(public_ctx, str(target_agent["id"]))


async def test_create_api_key_entity_path_requires_admin(db_pool, enums):
    """Non-admin agents should not be able to mint entity-owned API keys."""

    public_agent = await _create_agent(
        db_pool,
        enums,
        name="mint-public-agent",
        scopes=["public"],
    )
    target_entity = await _create_person_entity(
        db_pool,
        enums,
        name="mint-target-user",
    )
    public_ctx = _mcp_ctx(db_pool, enums, public_agent)

    with pytest.raises(ValueError, match="Admin scope required"):
        await create_api_key(
            CreateAPIKeyInput(entity_id=str(target_entity["id"]), name="forbidden"),
            public_ctx,
        )


async def test_create_api_key_entity_path_rejects_missing_entity(db_pool, enums):
    """Admin entity-key creation should return a clean error for missing owners."""

    admin_agent = await _create_agent(
        db_pool,
        enums,
        name="mint-admin-agent",
        scopes=["public", "admin"],
    )
    admin_ctx = _mcp_ctx(db_pool, enums, admin_agent)

    with pytest.raises(ValueError, match="Entity not found"):
        await create_api_key(
            CreateAPIKeyInput(
                entity_id="00000000-0000-0000-0000-000000000000",
                name="missing-owner",
            ),
            admin_ctx,
        )


async def test_revoke_api_key_denies_foreign_agent_key(db_pool, enums):
    """Non-admin agents should not revoke keys owned by another agent."""

    owner_agent = await _create_agent(
        db_pool,
        enums,
        name="owner-agent",
        scopes=["public"],
    )
    attacker_agent = await _create_agent(
        db_pool,
        enums,
        name="attacker-agent",
        scopes=["public"],
    )

    owner_ctx = _mcp_ctx(db_pool, enums, owner_agent)
    attacker_ctx = _mcp_ctx(db_pool, enums, attacker_agent)

    issued = await create_api_key(CreateAPIKeyInput(name="owner-key"), owner_ctx)
    assert issued["key_id"]

    with pytest.raises(ValueError, match="Key not found or already revoked"):
        await revoke_api_key(RevokeKeyInput(key_id=issued["key_id"]), attacker_ctx)


async def test_list_api_keys_returns_only_caller_keys(db_pool, enums):
    """list_api_keys should only return rows for the authenticated caller."""

    alpha_agent = await _create_agent(
        db_pool,
        enums,
        name="keys-alpha-agent",
        scopes=["public"],
    )
    beta_agent = await _create_agent(
        db_pool,
        enums,
        name="keys-beta-agent",
        scopes=["public"],
    )

    alpha_ctx = _mcp_ctx(db_pool, enums, alpha_agent)
    beta_ctx = _mcp_ctx(db_pool, enums, beta_agent)

    alpha_key = await create_api_key(CreateAPIKeyInput(name="alpha-key"), alpha_ctx)
    await create_api_key(CreateAPIKeyInput(name="beta-key"), beta_ctx)

    rows = await list_api_keys(alpha_ctx)
    assert rows
    assert all("agent_id" not in row for row in rows)
    assert any(str(row["id"]) == alpha_key["key_id"] for row in rows)
    assert not any(row["name"] == "beta-key" for row in rows)
    assert all("key_hash" not in row for row in rows)


async def test_get_approval_requires_admin(untrusted_mcp_context, db_pool, enums):
    """Non-admin callers should be denied get_approval even with a valid id."""

    created = await create_entity(
        CreateEntityInput(
            name="needs-approval-entity",
            type="project",
            status="active",
            scopes=["public"],
        ),
        untrusted_mcp_context,
    )
    approval_id = created["approval_request_id"]
    assert approval_id

    with pytest.raises(ValueError, match="Admin scope required"):
        await get_approval(
            GetApprovalInput(approval_id=approval_id), untrusted_mcp_context
        )

    admin_agent = await _create_agent(
        db_pool,
        enums,
        name="approval-admin-agent",
        scopes=["public", "admin"],
    )
    admin_ctx = _mcp_ctx(db_pool, enums, admin_agent)
    fetched = await get_approval(GetApprovalInput(approval_id=approval_id), admin_ctx)
    assert str(fetched["id"]) == approval_id


async def test_approve_request_rejects_grants_for_non_register_approval(db_pool, enums):
    """approve_request should reject grant fields for non-register approvals."""

    untrusted_agent = await _create_agent(
        db_pool,
        enums,
        name="approval-untrusted-agent",
        scopes=["public"],
        requires_approval=True,
    )
    untrusted_ctx = _mcp_ctx(db_pool, enums, untrusted_agent)
    created = await create_entity(
        CreateEntityInput(
            name="approval-grant-check",
            type="project",
            status="active",
            scopes=["public"],
        ),
        untrusted_ctx,
    )
    approval_id = created["approval_request_id"]
    assert approval_id

    admin_agent = await _create_agent(
        db_pool,
        enums,
        name="approval-admin",
        scopes=["public", "admin"],
    )
    admin_ctx = _mcp_ctx(db_pool, enums, admin_agent)
    with pytest.raises(ValueError, match="only valid for register_agent approvals"):
        await approve_request(
            ApproveRequestInput(
                approval_id=approval_id,
                grant_scopes=["public"],
            ),
            admin_ctx,
        )


async def test_approve_and_reject_tools_require_admin_scope(db_pool, enums):
    """Non-admin callers should not approve or reject pending approvals."""

    untrusted_agent = await _create_agent(
        db_pool,
        enums,
        name="approve-reject-untrusted-agent",
        scopes=["public"],
        requires_approval=True,
    )
    untrusted_ctx = _mcp_ctx(db_pool, enums, untrusted_agent)

    created = await create_entity(
        CreateEntityInput(
            name="approve-reject-target",
            type="project",
            status="active",
            scopes=["public"],
        ),
        untrusted_ctx,
    )
    approval_id = created["approval_request_id"]
    assert approval_id

    with pytest.raises(ValueError, match="Admin scope required"):
        await approve_request(
            ApproveRequestInput(approval_id=approval_id),
            untrusted_ctx,
        )

    with pytest.raises(ValueError, match="Admin scope required"):
        await reject_request(
            RejectRequestInput(
                approval_id=approval_id,
                review_notes="non-admin reject attempt",
            ),
            untrusted_ctx,
        )


async def test_update_agent_self_scope_escalation_is_denied(db_pool, enums):
    """Agent self-update should not allow elevating own scopes to admin."""

    self_agent = await _create_agent(
        db_pool,
        enums,
        name="self-update-agent",
        scopes=["public"],
    )
    self_ctx = _mcp_ctx(db_pool, enums, self_agent)

    with pytest.raises(ValueError, match="Admin scope required"):
        await update_agent(
            UpdateAgentInput(
                agent_id=str(self_agent["id"]),
                scopes=["public", "admin"],
            ),
            self_ctx,
        )


async def test_pending_approvals_all_requires_admin_and_paginates(db_pool, enums):
    """get_pending_approvals_all should enforce admin gate and support pagination."""

    untrusted_agent = await _create_agent(
        db_pool,
        enums,
        name="pending-agent",
        scopes=["public"],
        requires_approval=True,
    )
    untrusted_ctx = _mcp_ctx(db_pool, enums, untrusted_agent)

    for idx in range(3):
        await create_entity(
            CreateEntityInput(
                name=f"pending-{idx}",
                type="project",
                status="active",
                scopes=["public"],
            ),
            untrusted_ctx,
        )

    with pytest.raises(ValueError, match="Admin scope required"):
        await get_pending_approvals_all(
            PendingApprovalsInput(limit=2, offset=0),
            _mcp_ctx(db_pool, enums, untrusted_agent),
        )

    admin_agent = await _create_agent(
        db_pool,
        enums,
        name="pending-admin",
        scopes=["public", "admin"],
    )
    admin_ctx = _mcp_ctx(db_pool, enums, admin_agent)
    pending = await get_pending_approvals(
        _mcp_ctx(db_pool, enums, untrusted_agent),
    )
    assert len(pending) == 3

    page = await get_pending_approvals_all(
        PendingApprovalsInput(limit=2, offset=1),
        admin_ctx,
    )
    assert len(page) == 2


async def test_query_protocols_non_admin_limit_starvation_on_trusted_rows(
    db_pool, enums
):
    """Non-admin query should not starve untrusted rows when trusted rows lead."""

    admin_agent = await _create_agent(
        db_pool,
        enums,
        name="protocol-admin-agent",
        scopes=["public", "admin"],
    )
    public_agent = await _create_agent(
        db_pool,
        enums,
        name="protocol-public-agent",
        scopes=["public"],
    )
    admin_ctx = _mcp_ctx(db_pool, enums, admin_agent)
    public_ctx = _mcp_ctx(db_pool, enums, public_agent)

    await create_protocol(
        CreateProtocolInput(
            name="a-trusted-protocol",
            title="Trusted Protocol",
            content="trusted content",
            trusted=True,
            status="active",
        ),
        admin_ctx,
    )
    await create_protocol(
        CreateProtocolInput(
            name="z-public-protocol",
            title="Public Protocol",
            content="public content",
            trusted=False,
            status="active",
        ),
        admin_ctx,
    )

    rows = await query_protocols(
        QueryProtocolsInput(status_category="active", limit=1),
        public_ctx,
    )
    assert rows
    assert all(not row.get("trusted") for row in rows)


async def test_update_agent_self_requires_approval_bypass_is_denied(db_pool, enums):
    """Untrusted agents should not be able to self-disable approval requirements."""

    untrusted_self = await _create_agent(
        db_pool,
        enums,
        name="self-untrusted-agent",
        scopes=["public"],
        requires_approval=True,
    )
    self_ctx = _mcp_ctx(db_pool, enums, untrusted_self)

    with pytest.raises(ValueError, match="Admin scope required"):
        await update_agent(
            UpdateAgentInput(
                agent_id=str(untrusted_self["id"]),
                requires_approval=False,
            ),
            self_ctx,
        )


async def test_admin_only_tools_enforce_admin_gate_matrix(db_pool, enums):
    """Admin-only MCP tools should deny non-admin agents and allow admin callers."""

    public_agent = await _create_agent(
        db_pool,
        enums,
        name="admin-matrix-public-agent",
        scopes=["public"],
    )
    public_ctx = _mcp_ctx(db_pool, enums, public_agent)

    admin_agent = await _create_agent(
        db_pool,
        enums,
        name="admin-matrix-admin-agent",
        scopes=["public", "admin"],
    )
    admin_ctx = _mcp_ctx(db_pool, enums, admin_agent)

    with pytest.raises(ValueError, match="Admin scope required"):
        await list_audit_scopes(public_ctx)
    with pytest.raises(ValueError, match="Admin scope required"):
        await list_audit_actors(ListAuditActorsInput(actor_type="agent"), public_ctx)
    with pytest.raises(ValueError, match="Admin scope required"):
        await query_audit_log(QueryAuditLogInput(limit=5), public_ctx)
    with pytest.raises(ValueError, match="Admin scope required"):
        await get_agent(GetAgentInput(agent_id=str(admin_agent["id"])), public_ctx)

    scope_rows = await list_audit_scopes(admin_ctx)
    assert isinstance(scope_rows, list)

    actor_rows = await list_audit_actors(
        ListAuditActorsInput(actor_type="agent"),
        admin_ctx,
    )
    assert isinstance(actor_rows, list)

    audit_rows = await query_audit_log(QueryAuditLogInput(limit=5), admin_ctx)
    assert isinstance(audit_rows, list)

    agent_row = await get_agent(GetAgentInput(agent_id=str(public_agent["id"])), admin_ctx)
    assert str(agent_row["id"]) == str(public_agent["id"])


async def test_query_audit_log_rejects_invalid_uuid_filters(db_pool, enums):
    """query_audit_log should validate uuid-like filter ids for admin callers."""

    admin_agent = await _create_agent(
        db_pool,
        enums,
        name="audit-uuid-admin-agent",
        scopes=["public", "admin"],
    )
    admin_ctx = _mcp_ctx(db_pool, enums, admin_agent)

    with pytest.raises(ValueError, match="Invalid actor id"):
        await query_audit_log(
            QueryAuditLogInput(limit=5, actor_id="not-a-uuid"),
            admin_ctx,
        )

    with pytest.raises(ValueError, match="Invalid record id"):
        await query_audit_log(
            QueryAuditLogInput(limit=5, record_id="not-a-uuid"),
            admin_ctx,
        )

    with pytest.raises(ValueError, match="Invalid scope id"):
        await query_audit_log(
            QueryAuditLogInput(limit=5, scope_id="not-a-uuid"),
            admin_ctx,
        )


async def test_query_audit_log_accepts_valid_uuid_filters(db_pool, enums):
    """query_audit_log should allow valid uuid filters for admin callers."""

    admin_agent = await _create_agent(
        db_pool,
        enums,
        name="audit-valid-admin-agent",
        scopes=["public", "admin"],
    )
    admin_ctx = _mcp_ctx(db_pool, enums, admin_agent)
    target = await _create_person_entity(db_pool, enums, name="audit-filter-target")

    rows = await query_audit_log(
        QueryAuditLogInput(
            table_name="entities",
            actor_id=str(admin_agent["id"]),
            record_id=str(target["id"]),
            scope_id=str(enums.scopes.name_to_id["public"]),
            limit=1,
            offset=0,
        ),
        admin_ctx,
    )

    assert isinstance(rows, list)
