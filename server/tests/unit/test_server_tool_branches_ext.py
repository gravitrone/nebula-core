"""Extra unit coverage for branch-heavy MCP server tool wrappers."""

# Standard Library
from types import SimpleNamespace
from unittest.mock import AsyncMock
from uuid import uuid4

# Third-Party
import pytest

# Local
import nebula_mcp.server as server_mod
from nebula_mcp.models import (
    AgentEnrollStartInput,
    ApproveRequestInput,
    BulkImportInput,
    CreateContextInput,
    CreateAPIKeyInput,
    CreateTaxonomyInput,
    GetApprovalDiffInput,
    ListAllKeysInput,
    RejectRequestInput,
    RevokeKeyInput,
    UpdateContextInput,
    UpdateTaxonomyInput,
)
from nebula_mcp.server import (
    _run_bulk_import,
    approve_request,
    create_api_key,
    create_context,
    create_taxonomy,
    get_approval_diff,
    lifespan,
    list_all_api_keys,
    register_agent,
    reject_request,
    revoke_api_key,
    update_context,
    update_taxonomy,
)


pytestmark = pytest.mark.unit


class _AsyncCM:
    """Minimal async context-manager wrapper."""

    def __init__(self, value=None):
        self._value = value

    async def __aenter__(self):
        return self._value

    async def __aexit__(self, exc_type, exc, tb):
        return False


class _ConnStub:
    """Connection stub with queued fetchrow responses."""

    def __init__(self, *, fetchrow_rows=None):
        self._fetchrow_rows = list(fetchrow_rows or [])
        self.fetchrow_calls = []

    async def fetchrow(self, query, *args):
        self.fetchrow_calls.append((query, args))
        if self._fetchrow_rows:
            return self._fetchrow_rows.pop(0)
        return None

    def transaction(self):
        return _AsyncCM()


class _PoolStub:
    """Pool stub with queued fetch/fetchrow/execute responses."""

    def __init__(
        self,
        *,
        conn=None,
        fetch_rows=None,
        fetchrow_rows=None,
        execute_rows=None,
    ):
        self._conn = conn
        self._fetch_rows = list(fetch_rows or [])
        self._fetchrow_rows = list(fetchrow_rows or [])
        self._execute_rows = list(execute_rows or [])
        self.fetch_calls = []
        self.fetchrow_calls = []
        self.execute_calls = []

    async def fetch(self, query, *args):
        self.fetch_calls.append((query, args))
        if self._fetch_rows:
            return self._fetch_rows.pop(0)
        return []

    async def fetchrow(self, query, *args):
        self.fetchrow_calls.append((query, args))
        if self._fetchrow_rows:
            return self._fetchrow_rows.pop(0)
        return None

    async def execute(self, query, *args):
        self.execute_calls.append((query, args))
        if self._execute_rows:
            return self._execute_rows.pop(0)
        return "UPDATE 1"

    def acquire(self):
        return _AsyncCM(self._conn)


def _ctx(pool, enums, agent):
    """Build an MCP context object with lifespan context payload."""

    return SimpleNamespace(
        request_context=SimpleNamespace(
            lifespan_context={
                "pool": pool,
                "enums": enums,
                "agent": agent,
            }
        )
    )


def _admin_agent(mock_enums):
    """Build an admin-scoped agent for tool auth checks."""

    return {
        "id": str(uuid4()),
        "scopes": [mock_enums.scopes.name_to_id["admin"]],
        "requires_approval": False,
    }


def _public_agent(mock_enums, *, requires_approval=False):
    """Build a non-admin agent for scope and approval checks."""

    return {
        "id": str(uuid4()),
        "scopes": [mock_enums.scopes.name_to_id["public"]],
        "requires_approval": requires_approval,
    }


@pytest.mark.asyncio
async def test_lifespan_yields_context_and_closes_pool(monkeypatch, mock_enums):
    """lifespan should yield initialized context and always close the pool."""

    pool = SimpleNamespace(close=AsyncMock())
    agent = {"id": str(uuid4()), "scopes": []}

    monkeypatch.setattr("nebula_mcp.server.get_pool", AsyncMock(return_value=pool))
    monkeypatch.setattr("nebula_mcp.server.load_enums", AsyncMock(return_value=mock_enums))
    monkeypatch.setattr(
        "nebula_mcp.server.authenticate_agent_optional",
        AsyncMock(return_value=(agent, True)),
    )

    async with lifespan(server_mod.mcp) as state:
        assert state["pool"] is pool
        assert state["enums"] is mock_enums
        assert state["agent"] == agent
        assert state["bootstrap_mode"] is True

    pool.close.assert_awaited_once()


@pytest.mark.asyncio
async def test_run_bulk_import_context_scope_validation_error_is_collected(
    monkeypatch, mock_enums
):
    """bulk_import_context should report invalid scope rows in approval mode."""

    pool = _PoolStub()
    agent = _public_agent(mock_enums, requires_approval=True)
    payload = BulkImportInput(format="json", items=[{"title": "bad"}])

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr("nebula_mcp.server.ensure_approval_capacity", AsyncMock())
    monkeypatch.setattr("nebula_mcp.server.create_approval_request", AsyncMock())

    def _normalizer(_item, _defaults):
        return {"title": "bad", "scopes": ["does-not-exist"]}

    result = await _run_bulk_import(
        payload,
        SimpleNamespace(),
        _normalizer,
        AsyncMock(),
        "bulk_import_context",
    )

    assert result["created"] == 0
    assert result["failed"] == 1
    assert result["status"] == "approval_required"
    assert "Requested scopes exceed allowed scopes" in result["errors"][0]["error"]


@pytest.mark.asyncio
async def test_run_bulk_import_relationships_validates_both_nodes(
    monkeypatch, mock_enums
):
    """bulk_import_relationships should validate source and target nodes."""

    pool = _PoolStub()
    agent = _public_agent(mock_enums, requires_approval=True)
    payload = BulkImportInput(
        format="json",
        items=[
            {
                "source_type": "entity",
                "source_id": str(uuid4()),
                "target_type": "entity",
                "target_id": str(uuid4()),
            }
        ],
    )
    validate_node = AsyncMock()
    create_approval = AsyncMock(return_value={"id": uuid4()})

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr("nebula_mcp.server.ensure_approval_capacity", AsyncMock())
    monkeypatch.setattr("nebula_mcp.server._validate_relationship_node", validate_node)
    monkeypatch.setattr("nebula_mcp.server.create_approval_request", create_approval)

    def _normalizer(item, _defaults):
        item = dict(item)
        item["relationship_type"] = "related-to"
        return item

    result = await _run_bulk_import(
        payload,
        SimpleNamespace(),
        _normalizer,
        AsyncMock(),
        "bulk_import_relationships",
    )

    assert result["failed"] == 0
    assert len(result["approvals"]) == 1
    assert validate_node.await_count == 2
    create_approval.assert_awaited_once()


@pytest.mark.asyncio
async def test_run_bulk_import_jobs_invalid_priority_is_collected(
    monkeypatch, mock_enums
):
    """bulk_import_jobs should reject invalid priority before approval enqueue."""

    pool = _PoolStub()
    agent = _public_agent(mock_enums, requires_approval=True)
    payload = BulkImportInput(format="json", items=[{"title": "job"}])

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr("nebula_mcp.server.ensure_approval_capacity", AsyncMock())
    monkeypatch.setattr("nebula_mcp.server.create_approval_request", AsyncMock())

    def _normalizer(_item, _defaults):
        return {"title": "job", "priority": "urgent"}

    result = await _run_bulk_import(
        payload,
        SimpleNamespace(),
        _normalizer,
        AsyncMock(),
        "bulk_import_jobs",
    )

    assert result["failed"] == 1
    assert "Invalid priority: urgent" in result["errors"][0]["error"]


@pytest.mark.asyncio
async def test_run_bulk_import_jobs_non_admin_forces_agent_id(monkeypatch, mock_enums):
    """bulk_import_jobs should overwrite agent_id for non-admin agents."""

    pool = _PoolStub()
    agent = _public_agent(mock_enums, requires_approval=True)
    payload = BulkImportInput(format="json", items=[{"title": "job"}])
    captured = {}

    async def _capture_approval(_pool, _agent_id, _action, normalized):
        captured.update(normalized)
        return {"id": uuid4()}

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr("nebula_mcp.server.ensure_approval_capacity", AsyncMock())
    monkeypatch.setattr("nebula_mcp.server.create_approval_request", _capture_approval)

    def _normalizer(_item, _defaults):
        return {"title": "job", "priority": "high", "agent_id": "spoofed"}

    result = await _run_bulk_import(
        payload,
        SimpleNamespace(),
        _normalizer,
        AsyncMock(),
        "bulk_import_jobs",
    )

    assert result["failed"] == 0
    assert captured["agent_id"] == agent["id"]


@pytest.mark.asyncio
async def test_create_api_key_entity_create_failure_raises(monkeypatch, mock_enums):
    """create_api_key should fail when DB insert returns no key row."""

    pool = _PoolStub(
        fetchrow_rows=[
            {"id": str(uuid4())},  # owner entity exists
            None,  # create query failed
        ]
    )
    agent = _admin_agent(mock_enums)
    payload = CreateAPIKeyInput(entity_id=str(uuid4()), name="x")

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr(
        "nebula_mcp.server.generate_api_key",
        lambda: ("raw-key", "nbl_test", "hash"),
    )

    with pytest.raises(ValueError, match="Failed to create API key"):
        await create_api_key(payload, _ctx(pool, mock_enums, agent))


@pytest.mark.asyncio
async def test_create_api_key_agent_create_failure_raises(monkeypatch, mock_enums):
    """create_api_key should fail when agent-owned key insert returns no row."""

    pool = _PoolStub(fetchrow_rows=[None])
    agent = _admin_agent(mock_enums)
    payload = CreateAPIKeyInput(name="agent-key")

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr(
        "nebula_mcp.server.generate_api_key",
        lambda: ("raw-key", "nbl_test", "hash"),
    )

    with pytest.raises(ValueError, match="Failed to create API key"):
        await create_api_key(payload, _ctx(pool, mock_enums, agent))


@pytest.mark.asyncio
async def test_list_all_api_keys_admin_success(monkeypatch, mock_enums):
    """Admin callers should receive list_all_api_keys rows."""

    rows = [{"id": str(uuid4()), "name": "k1"}]
    pool = _PoolStub(fetch_rows=[rows])
    agent = _admin_agent(mock_enums)

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )

    result = await list_all_api_keys(
        ListAllKeysInput(limit=10, offset=5),
        _ctx(pool, mock_enums, agent),
    )

    assert result == rows


@pytest.mark.asyncio
async def test_revoke_api_key_admin_path_uses_global_query(monkeypatch, mock_enums):
    """Admin revoke path should execute revoke_any and return revoked=true."""

    pool = _PoolStub(execute_rows=["UPDATE 1"])
    agent = _admin_agent(mock_enums)

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )

    result = await revoke_api_key(
        RevokeKeyInput(key_id=str(uuid4())),
        _ctx(pool, mock_enums, agent),
    )

    assert result == {"revoked": True}
    assert pool.execute_calls[0][0] == server_mod.QUERIES["api_keys/revoke_any"]


@pytest.mark.asyncio
async def test_approve_request_register_grant_requires_approval(monkeypatch, mock_enums):
    """approve_request should pass grant_requires_approval into review details."""

    approval_id = str(uuid4())
    pool = _PoolStub(fetchrow_rows=[{"id": approval_id, "request_type": "register_agent"}])
    agent = _admin_agent(mock_enums)
    approve_mock = AsyncMock(return_value={"approved": True})

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr("nebula_mcp.server.do_approve", approve_mock)

    result = await approve_request(
        ApproveRequestInput(
            approval_id=approval_id,
            grant_requires_approval=False,
            review_notes="ship it",
        ),
        _ctx(pool, mock_enums, agent),
    )

    assert result == {"approved": True}
    assert approve_mock.await_args.kwargs["review_details"]["grant_requires_approval"] is False


@pytest.mark.asyncio
async def test_approve_request_not_found(monkeypatch, mock_enums):
    """approve_request should fail cleanly for unknown approval ids."""

    pool = _PoolStub(fetchrow_rows=[None])
    agent = _admin_agent(mock_enums)

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )

    with pytest.raises(ValueError, match="Approval request not found"):
        await approve_request(
            ApproveRequestInput(approval_id=str(uuid4())),
            _ctx(pool, mock_enums, agent),
        )


@pytest.mark.asyncio
async def test_reject_request_invalid_reviewer_uuid(monkeypatch, mock_enums):
    """reject_request should validate reviewed_by when provided."""

    pool = _PoolStub()
    agent = _admin_agent(mock_enums)

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )

    with pytest.raises(ValueError, match="Invalid reviewer id"):
        await reject_request(
            RejectRequestInput(
                approval_id=str(uuid4()),
                reviewed_by="bad",
                review_notes="nope",
            ),
            _ctx(pool, mock_enums, agent),
        )


@pytest.mark.asyncio
async def test_get_approval_diff_not_found(monkeypatch, mock_enums):
    """get_approval_diff should return a clear not-found error for unknown id."""

    pool = _PoolStub(fetchrow_rows=[None])
    agent = _admin_agent(mock_enums)

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )

    with pytest.raises(ValueError, match="Approval request not found"):
        await get_approval_diff(
            GetApprovalDiffInput(approval_id=str(uuid4())),
            _ctx(pool, mock_enums, agent),
        )


@pytest.mark.asyncio
async def test_get_approval_diff_denies_foreign_non_admin(monkeypatch, mock_enums):
    """get_approval_diff should reject non-admin callers for other agents' approvals."""

    pool = _PoolStub(fetchrow_rows=[{"requested_by": str(uuid4())}])
    agent = _public_agent(mock_enums)

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )

    with pytest.raises(ValueError, match="Access denied"):
        await get_approval_diff(
            GetApprovalDiffInput(approval_id=str(uuid4())),
            _ctx(pool, mock_enums, agent),
        )


@pytest.mark.asyncio
async def test_update_context_missing_row_raises(monkeypatch, mock_enums):
    """update_context should raise not-found when executor returns no row."""

    pool = _PoolStub()
    agent = _public_agent(mock_enums)
    payload = UpdateContextInput(context_id=str(uuid4()))

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr("nebula_mcp.server._validate_relationship_node", AsyncMock())
    monkeypatch.setattr("nebula_mcp.server.maybe_require_approval", AsyncMock(return_value=None))
    monkeypatch.setattr("nebula_mcp.server.execute_update_context", AsyncMock(return_value=None))

    with pytest.raises(ValueError, match="Context not found"):
        await update_context(payload, _ctx(pool, mock_enums, agent))


@pytest.mark.asyncio
async def test_create_context_invalid_url_guard_via_model_construct(
    monkeypatch, mock_enums
):
    """Server-level URL guard should catch invalid urls when model validation is bypassed."""

    pool = _PoolStub()
    agent = _public_agent(mock_enums)
    payload = CreateContextInput.model_construct(
        title="ctx",
        source_type="note",
        scopes=["public"],
        tags=[],
        metadata={},
        content=None,
        url="javascript:alert(1)",
    )

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )

    with pytest.raises(ValueError, match="URL must start with http:// or https://"):
        await create_context(payload, _ctx(pool, mock_enums, agent))


@pytest.mark.asyncio
async def test_create_context_approval_short_circuit(monkeypatch, mock_enums):
    """create_context should return approval payload without calling executor."""

    pool = _PoolStub()
    agent = _public_agent(mock_enums)
    payload = CreateContextInput(
        title="ctx",
        source_type="note",
        scopes=["public"],
    )
    execute_create = AsyncMock()

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr(
        "nebula_mcp.server.maybe_require_approval",
        AsyncMock(return_value={"status": "approval_required", "approval_request_id": "a1"}),
    )
    monkeypatch.setattr("nebula_mcp.server.execute_create_context", execute_create)

    result = await create_context(payload, _ctx(pool, mock_enums, agent))

    assert result["status"] == "approval_required"
    execute_create.assert_not_awaited()


@pytest.mark.asyncio
async def test_update_context_invalid_url_guard_via_model_construct(
    monkeypatch, mock_enums
):
    """Server-level update URL guard should reject invalid protocols."""

    pool = _PoolStub()
    agent = _public_agent(mock_enums)
    payload = UpdateContextInput.model_construct(
        context_id=str(uuid4()),
        url="file:///tmp/x",
        status=None,
        scopes=None,
        title=None,
        source_type=None,
        content=None,
        tags=None,
        metadata=None,
    )

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr("nebula_mcp.server._validate_relationship_node", AsyncMock())
    monkeypatch.setattr("nebula_mcp.server.maybe_require_approval", AsyncMock(return_value=None))
    monkeypatch.setattr("nebula_mcp.server.execute_update_context", AsyncMock(return_value={"id": payload.context_id}))

    with pytest.raises(ValueError, match="URL must start with http:// or https://"):
        await update_context(payload, _ctx(pool, mock_enums, agent))


@pytest.mark.asyncio
async def test_update_context_status_scope_and_approval_short_circuit(
    monkeypatch, mock_enums
):
    """update_context should run status/scope checks and return approval payload."""

    pool = _PoolStub()
    agent = _public_agent(mock_enums)
    payload = UpdateContextInput.model_construct(
        context_id=str(uuid4()),
        status="active",
        scopes=["public"],
        title=None,
        source_type=None,
        content=None,
        tags=None,
        metadata=None,
        url=None,
    )
    maybe_approval = AsyncMock(return_value={"status": "approval_required", "approval_request_id": "x"})

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr("nebula_mcp.server._validate_relationship_node", AsyncMock())
    monkeypatch.setattr("nebula_mcp.server.require_status", lambda name, enums: name)
    monkeypatch.setattr("nebula_mcp.server.enforce_scope_subset", lambda scopes, allowed: scopes)
    monkeypatch.setattr("nebula_mcp.server.require_scopes", lambda scopes, enums: scopes)
    monkeypatch.setattr("nebula_mcp.server.maybe_require_approval", maybe_approval)
    monkeypatch.setattr("nebula_mcp.server.execute_update_context", AsyncMock())

    result = await update_context(payload, _ctx(pool, mock_enums, agent))

    assert result["status"] == "approval_required"


@pytest.mark.asyncio
async def test_register_agent_rejects_when_already_authenticated(
    monkeypatch, mock_enums
):
    """register_agent should fail when an authenticated agent is present."""

    pool = _PoolStub()
    agent = _public_agent(mock_enums)
    payload = AgentEnrollStartInput(name="already-authenticated")

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )

    with pytest.raises(ValueError, match="Agent already authenticated"):
        await register_agent(payload, _ctx(pool, mock_enums, agent))


@pytest.mark.asyncio
async def test_register_agent_rejects_duplicate_name(monkeypatch, mock_enums):
    """register_agent should fail when name uniqueness check finds a row."""

    conn = _ConnStub(fetchrow_rows=[{"id": str(uuid4())}])
    pool = _PoolStub(conn=conn)
    payload = AgentEnrollStartInput(name="duplicate-agent")

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, None)),
    )

    with pytest.raises(ValueError, match="already exists"):
        await register_agent(payload, _ctx(pool, mock_enums, None))


@pytest.mark.asyncio
async def test_register_agent_success_returns_pending_payload(monkeypatch, mock_enums):
    """register_agent should return enrollment details after successful creation."""

    created_agent_id = str(uuid4())
    approval_id = str(uuid4())
    registration_id = str(uuid4())
    conn = _ConnStub(
        fetchrow_rows=[
            None,  # check_name
            {"id": created_agent_id},  # create agent
        ]
    )
    pool = _PoolStub(conn=conn)
    payload = AgentEnrollStartInput(name="new-agent")

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, None)),
    )
    monkeypatch.setattr(
        "nebula_mcp.server.create_approval_request",
        AsyncMock(return_value={"id": approval_id}),
    )
    monkeypatch.setattr(
        "nebula_mcp.server.create_enrollment_session",
        AsyncMock(
            return_value={
                "id": registration_id,
                "enrollment_token": "token-123",
            }
        ),
    )

    result = await register_agent(payload, _ctx(pool, mock_enums, None))

    assert result["agent_id"] == created_agent_id
    assert result["approval_request_id"] == approval_id
    assert result["registration_id"] == registration_id
    assert result["enrollment_token"] == "token-123"
    assert result["status"] == "pending_approval"


@pytest.mark.asyncio
async def test_create_taxonomy_blank_name_rejected(monkeypatch, mock_enums):
    """create_taxonomy should reject blank names after trim."""

    pool = _PoolStub()
    agent = _admin_agent(mock_enums)
    payload = CreateTaxonomyInput.model_construct(
        kind="scopes",
        name="   ",
        description=None,
        metadata=None,
        is_symmetric=None,
        value_schema=None,
    )

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )

    with pytest.raises(ValueError, match="Taxonomy name required"):
        await create_taxonomy(payload, _ctx(pool, mock_enums, agent))


@pytest.mark.asyncio
async def test_update_taxonomy_blank_name_rejected(monkeypatch, mock_enums):
    """update_taxonomy should reject explicit empty names."""

    pool = _PoolStub()
    agent = _admin_agent(mock_enums)
    payload = UpdateTaxonomyInput.model_construct(
        kind="scopes",
        item_id=str(uuid4()),
        name="   ",
        description=None,
        metadata=None,
        is_symmetric=None,
        value_schema=None,
    )

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr(
        "nebula_mcp.server._get_taxonomy_row",
        AsyncMock(return_value={"id": str(uuid4()), "name": "public", "is_builtin": False}),
    )

    with pytest.raises(ValueError, match="Taxonomy name cannot be empty"):
        await update_taxonomy(payload, _ctx(pool, mock_enums, agent))


@pytest.mark.asyncio
async def test_update_taxonomy_missing_after_update_raises(monkeypatch, mock_enums):
    """update_taxonomy should fail when update query returns no row."""

    pool = _PoolStub(fetchrow_rows=[None])
    agent = _admin_agent(mock_enums)
    payload = UpdateTaxonomyInput(
        kind="scopes",
        item_id=str(uuid4()),
        name="renamed",
    )

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr(
        "nebula_mcp.server._get_taxonomy_row",
        AsyncMock(return_value={"id": str(uuid4()), "name": "public", "is_builtin": False}),
    )

    with pytest.raises(ValueError, match="scopes entry not found"):
        await update_taxonomy(payload, _ctx(pool, mock_enums, agent))
