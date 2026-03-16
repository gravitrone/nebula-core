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
    AttachFileInput,
    ApproveRequestInput,
    BulkImportInput,
    BulkUpdateEntityScopesInput,
    BulkUpdateEntityTagsInput,
    CreateContextInput,
    CreateAPIKeyInput,
    CreateFileInput,
    CreateJobInput,
    CreateLogInput,
    CreateProtocolInput,
    CreateRelationshipInput,
    CreateSubtaskInput,
    CreateTaxonomyInput,
    GetAgentInput,
    GetAgentInfoInput,
    GetFileInput,
    GetJobInput,
    GetLogInput,
    GetProtocolInput,
    GetContextInput,
    GetRelationshipsInput,
    GetApprovalDiffInput,
    GraphNeighborsInput,
    GraphShortestPathInput,
    QueryFilesInput,
    LinkContextInput,
    ListAllKeysInput,
    ListAgentsInput,
    QueryContextInput,
    QueryJobsInput,
    QueryProtocolsInput,
    QueryRelationshipsInput,
    QueryLogsInput,
    RejectRequestInput,
    RevertEntityInput,
    RevokeKeyInput,
    ToggleTaxonomyInput,
    UpdateAgentInput,
    UpdateFileInput,
    UpdateJobInput,
    UpdateJobStatusInput,
    UpdateProtocolInput,
    UpdateRelationshipInput,
    UpdateLogInput,
    UpdateContextInput,
    UpdateTaxonomyInput,
)
from nebula_mcp.server import (
    _run_bulk_import,
    approve_request,
    bulk_update_entity_scopes,
    bulk_update_entity_tags,
    create_api_key,
    create_context,
    create_file,
    create_job,
    create_log,
    create_protocol,
    create_relationship,
    create_subtask,
    create_taxonomy,
    get_agent,
    get_agent_info,
    get_file,
    get_context,
    get_job,
    get_log,
    get_protocol,
    get_relationships,
    get_approval_diff,
    graph_neighbors,
    graph_shortest_path,
    list_active_protocols,
    list_agents,
    list_files,
    link_context_to_owner,
    lifespan,
    list_all_api_keys,
    query_context,
    query_protocols,
    query_jobs,
    query_logs,
    query_relationships,
    register_agent,
    reject_request,
    revert_entity,
    revoke_api_key,
    update_job,
    update_job_status,
    update_agent,
    update_file,
    update_protocol,
    update_relationship,
    update_log,
    update_context,
    update_taxonomy,
    archive_taxonomy,
    activate_taxonomy,
    attach_file_to_job,
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
async def test_run_bulk_import_context_scope_validation_error_is_collected(monkeypatch, mock_enums):
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
async def test_run_bulk_import_relationships_validates_both_nodes(monkeypatch, mock_enums):
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
async def test_run_bulk_import_jobs_invalid_priority_is_collected(monkeypatch, mock_enums):
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
async def test_create_context_invalid_url_guard_via_model_construct(monkeypatch, mock_enums):
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
async def test_create_context_non_string_url_guard_via_model_construct(monkeypatch, mock_enums):
    """Server-level create guard should reject non-string URL payloads."""

    pool = _PoolStub()
    agent = _public_agent(mock_enums)
    payload = CreateContextInput.model_construct(
        title="ctx",
        source_type="note",
        scopes=["public"],
        tags=[],
        metadata={},
        content=None,
        url=123,  # type: ignore[arg-type]
    )

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )

    with pytest.raises(ValueError, match="URL must be a string"):
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
async def test_update_context_invalid_url_guard_via_model_construct(monkeypatch, mock_enums):
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
    monkeypatch.setattr(
        "nebula_mcp.server.execute_update_context",
        AsyncMock(return_value={"id": payload.context_id}),
    )

    with pytest.raises(ValueError, match="URL must start with http:// or https://"):
        await update_context(payload, _ctx(pool, mock_enums, agent))


@pytest.mark.asyncio
async def test_update_context_non_string_url_guard_via_model_construct(monkeypatch, mock_enums):
    """Server-level update guard should reject non-string URL payloads."""

    pool = _PoolStub()
    agent = _public_agent(mock_enums)
    payload = UpdateContextInput.model_construct(
        context_id=str(uuid4()),
        url=False,  # type: ignore[arg-type]
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
    monkeypatch.setattr(
        "nebula_mcp.server.execute_update_context",
        AsyncMock(return_value={"id": payload.context_id}),
    )

    with pytest.raises(ValueError, match="URL must be a string"):
        await update_context(payload, _ctx(pool, mock_enums, agent))


@pytest.mark.asyncio
async def test_update_context_status_scope_and_approval_short_circuit(monkeypatch, mock_enums):
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
    maybe_approval = AsyncMock(
        return_value={"status": "approval_required", "approval_request_id": "x"}
    )

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
async def test_create_context_valid_url_trim_direct_path(monkeypatch, mock_enums):
    """create_context should trim valid URLs and call executor on direct path."""

    pool = _PoolStub()
    agent = _public_agent(mock_enums)
    payload = CreateContextInput(
        title="ctx",
        source_type="note",
        scopes=["public"],
        url=" https://example.com/x ",
    )

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr("nebula_mcp.server.maybe_require_approval", AsyncMock(return_value=None))
    monkeypatch.setattr(
        "nebula_mcp.server.execute_create_context",
        AsyncMock(return_value={"id": str(uuid4()), "url": "https://example.com/x"}),
    )

    result = await create_context(payload, _ctx(pool, mock_enums, agent))

    assert result["url"] == "https://example.com/x"


@pytest.mark.asyncio
async def test_create_context_empty_url_allowed_via_model_construct(monkeypatch, mock_enums):
    """create_context should allow explicit empty URL to clear/omit link."""

    pool = _PoolStub()
    agent = _public_agent(mock_enums)
    payload = CreateContextInput.model_construct(
        title="ctx",
        source_type="note",
        scopes=["public"],
        url="",
        tags=[],
        metadata={},
        content=None,
    )

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr(
        "nebula_mcp.server.maybe_require_approval",
        AsyncMock(return_value=None),
    )
    execute_create = AsyncMock(return_value={"id": str(uuid4()), "url": ""})
    monkeypatch.setattr("nebula_mcp.server.execute_create_context", execute_create)

    result = await create_context(payload, _ctx(pool, mock_enums, agent))

    assert result["url"] == ""
    assert execute_create.await_args.args[2]["url"] == ""


@pytest.mark.asyncio
async def test_create_context_whitespace_url_rejected_via_model_construct(monkeypatch, mock_enums):
    """create_context should reject whitespace-only URL when validation is bypassed."""

    pool = _PoolStub()
    agent = _public_agent(mock_enums)
    payload = CreateContextInput.model_construct(
        title="ctx",
        source_type="note",
        scopes=["public"],
        url="   ",
        tags=[],
        metadata={},
        content=None,
    )

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    with pytest.raises(ValueError, match="URL must start with http:// or https://"):
        await create_context(payload, _ctx(pool, mock_enums, agent))


@pytest.mark.asyncio
async def test_update_context_valid_url_trim_direct_path(monkeypatch, mock_enums):
    """update_context should trim valid URLs and return direct executor row."""

    context_id = str(uuid4())
    pool = _PoolStub()
    agent = _public_agent(mock_enums)
    payload = UpdateContextInput.model_construct(
        context_id=context_id,
        url=" https://example.com/u ",
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
    monkeypatch.setattr(
        "nebula_mcp.server.execute_update_context",
        AsyncMock(return_value={"id": context_id, "url": "https://example.com/u"}),
    )

    result = await update_context(payload, _ctx(pool, mock_enums, agent))

    assert result["url"] == "https://example.com/u"


@pytest.mark.asyncio
async def test_update_context_empty_url_allowed_via_model_construct(monkeypatch, mock_enums):
    """update_context should allow explicit empty URL for clearing."""

    context_id = str(uuid4())
    pool = _PoolStub()
    agent = _public_agent(mock_enums)
    payload = UpdateContextInput.model_construct(
        context_id=context_id,
        url="",
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
    monkeypatch.setattr(
        "nebula_mcp.server.maybe_require_approval",
        AsyncMock(return_value=None),
    )
    execute_update = AsyncMock(return_value={"id": context_id, "url": ""})
    monkeypatch.setattr("nebula_mcp.server.execute_update_context", execute_update)

    result = await update_context(payload, _ctx(pool, mock_enums, agent))

    assert result["url"] == ""
    assert execute_update.await_args.args[2]["url"] == ""


@pytest.mark.asyncio
async def test_update_context_whitespace_url_rejected_via_model_construct(monkeypatch, mock_enums):
    """update_context should reject whitespace-only URL when validation is bypassed."""

    context_id = str(uuid4())
    pool = _PoolStub()
    agent = _public_agent(mock_enums)
    payload = UpdateContextInput.model_construct(
        context_id=context_id,
        url="   ",
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
    monkeypatch.setattr(
        "nebula_mcp.server.maybe_require_approval",
        AsyncMock(return_value=None),
    )
    monkeypatch.setattr(
        "nebula_mcp.server.execute_update_context",
        AsyncMock(return_value={"id": context_id, "url": ""}),
    )

    with pytest.raises(ValueError, match="URL must start with http:// or https://"):
        await update_context(payload, _ctx(pool, mock_enums, agent))


@pytest.mark.asyncio
async def test_get_context_not_found_raises(monkeypatch, mock_enums):
    """get_context should raise when the record is missing."""

    pool = _PoolStub(fetchrow_rows=[None])
    agent = _public_agent(mock_enums)
    payload = GetContextInput(context_id=str(uuid4()))

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr("nebula_mcp.server._scope_filter_ids", lambda _agent, _enums: [])

    with pytest.raises(ValueError, match="Context not found"):
        await get_context(payload, _ctx(pool, mock_enums, agent))


@pytest.mark.asyncio
async def test_query_context_clamps_offset(monkeypatch, mock_enums):
    """query_context should clamp negative offsets."""

    row_id = str(uuid4())
    pool = _PoolStub(fetch_rows=[[{"id": row_id}, {"id": str(uuid4())}]])
    agent = _public_agent(mock_enums)
    payload = QueryContextInput(scopes=["public"], limit=1000, offset=-25)

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr("nebula_mcp.server.enforce_scope_subset", lambda scopes, _allowed: scopes)
    monkeypatch.setattr(
        "nebula_mcp.server.require_scopes",
        lambda scopes, enums: [enums.scopes.name_to_id[s] for s in scopes],
    )
    monkeypatch.setattr("nebula_mcp.server._clamp_limit", lambda _limit: 50)

    result = await query_context(payload, _ctx(pool, mock_enums, agent))

    assert len(result) == 2
    assert pool.fetch_calls[0][1][-1] == 0


@pytest.mark.asyncio
async def test_link_context_to_owner_approval_short_circuit(monkeypatch, mock_enums):
    """link_context_to_owner should return approval payload without writer call."""

    pool = _PoolStub()
    agent = _public_agent(mock_enums)
    payload = LinkContextInput(
        context_id=str(uuid4()),
        owner_type="entity",
        owner_id=str(uuid4()),
    )
    execute_link = AsyncMock()

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr("nebula_mcp.server._validate_relationship_node", AsyncMock())
    monkeypatch.setattr(
        "nebula_mcp.server.maybe_require_approval",
        AsyncMock(return_value={"status": "approval_required", "approval_request_id": "r1"}),
    )
    monkeypatch.setattr("nebula_mcp.server.execute_create_relationship", execute_link)

    result = await link_context_to_owner(payload, _ctx(pool, mock_enums, agent))

    assert result["status"] == "approval_required"
    execute_link.assert_not_awaited()


@pytest.mark.asyncio
async def test_link_context_to_owner_direct_create(monkeypatch, mock_enums):
    """link_context_to_owner should execute relationship create on direct path."""

    pool = _PoolStub()
    agent = _public_agent(mock_enums)
    payload = LinkContextInput(
        context_id=str(uuid4()),
        owner_type="entity",
        owner_id=str(uuid4()),
    )

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr("nebula_mcp.server._validate_relationship_node", AsyncMock())
    monkeypatch.setattr("nebula_mcp.server.maybe_require_approval", AsyncMock(return_value=None))
    monkeypatch.setattr(
        "nebula_mcp.server.require_relationship_type", lambda *_args, **_kwargs: None
    )
    monkeypatch.setattr(
        "nebula_mcp.server.execute_create_relationship",
        AsyncMock(return_value={"id": str(uuid4())}),
    )

    result = await link_context_to_owner(payload, _ctx(pool, mock_enums, agent))

    assert "id" in result


@pytest.mark.asyncio
async def test_bulk_update_entity_tags_approval_short_circuit(monkeypatch, mock_enums):
    """bulk_update_entity_tags should return approval payload when required."""

    pool = _PoolStub()
    agent = _public_agent(mock_enums)
    payload = BulkUpdateEntityTagsInput(
        entity_ids=[str(uuid4())],
        tags=["a", "b"],
        op="add",
    )
    maybe_approval = AsyncMock(
        return_value={"status": "approval_required", "approval_request_id": "x"}
    )

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr("nebula_mcp.server._require_entity_write_access", AsyncMock())
    monkeypatch.setattr("nebula_mcp.server.maybe_require_approval", maybe_approval)
    monkeypatch.setattr("nebula_mcp.server.do_bulk_update_entity_tags", AsyncMock())

    result = await bulk_update_entity_tags(payload, _ctx(pool, mock_enums, agent))

    assert result["status"] == "approval_required"


@pytest.mark.asyncio
async def test_bulk_update_entity_tags_direct_update(monkeypatch, mock_enums):
    """bulk_update_entity_tags should return updated ids on direct path."""

    pool = _PoolStub()
    agent = _public_agent(mock_enums)
    entity_id = str(uuid4())
    payload = BulkUpdateEntityTagsInput(
        entity_ids=[entity_id],
        tags=["a"],
        op="replace",
    )

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr("nebula_mcp.server._require_entity_write_access", AsyncMock())
    monkeypatch.setattr("nebula_mcp.server.maybe_require_approval", AsyncMock(return_value=None))
    monkeypatch.setattr("nebula_mcp.server.normalize_bulk_operation", lambda op: op)
    monkeypatch.setattr(
        "nebula_mcp.server.do_bulk_update_entity_tags",
        AsyncMock(return_value=[entity_id]),
    )

    result = await bulk_update_entity_tags(payload, _ctx(pool, mock_enums, agent))

    assert result == {"updated": 1, "entity_ids": [entity_id]}


@pytest.mark.asyncio
async def test_bulk_update_entity_scopes_direct_update(monkeypatch, mock_enums):
    """bulk_update_entity_scopes should enforce subset then return updated ids."""

    pool = _PoolStub()
    agent = _public_agent(mock_enums)
    entity_id = str(uuid4())
    payload = BulkUpdateEntityScopesInput(
        entity_ids=[entity_id],
        scopes=["public"],
        op="add",
    )

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr("nebula_mcp.server._require_entity_write_access", AsyncMock())
    monkeypatch.setattr("nebula_mcp.server.normalize_bulk_operation", lambda op: op)
    monkeypatch.setattr("nebula_mcp.server.scope_names_from_ids", lambda ids, enums: ["public"])
    monkeypatch.setattr("nebula_mcp.server.enforce_scope_subset", lambda scopes, allowed: scopes)
    monkeypatch.setattr("nebula_mcp.server.require_scopes", lambda scopes, enums: scopes)
    monkeypatch.setattr("nebula_mcp.server.maybe_require_approval", AsyncMock(return_value=None))
    monkeypatch.setattr(
        "nebula_mcp.server.do_bulk_update_entity_scopes",
        AsyncMock(return_value=[entity_id]),
    )

    result = await bulk_update_entity_scopes(payload, _ctx(pool, mock_enums, agent))

    assert result == {"updated": 1, "entity_ids": [entity_id]}


@pytest.mark.asyncio
async def test_revert_entity_rejects_non_entity_audit(monkeypatch, mock_enums):
    """revert_entity should fail when audit row table is not entities."""

    pool = _PoolStub(fetchrow_rows=[{"table_name": "jobs", "record_id": str(uuid4())}])
    agent = _public_agent(mock_enums)
    payload = RevertEntityInput(entity_id=str(uuid4()), audit_id=str(uuid4()))

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )

    with pytest.raises(ValueError, match="Audit entry is not for entities"):
        await revert_entity(payload, _ctx(pool, mock_enums, agent))


@pytest.mark.asyncio
async def test_revert_entity_rejects_mismatched_record_id(monkeypatch, mock_enums):
    """revert_entity should fail when audit record does not match entity id."""

    pool = _PoolStub(
        fetchrow_rows=[
            {"table_name": "entities", "record_id": str(uuid4())},
        ]
    )
    agent = _public_agent(mock_enums)
    payload = RevertEntityInput(entity_id=str(uuid4()), audit_id=str(uuid4()))

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )

    with pytest.raises(ValueError, match="Audit entry does not match entity"):
        await revert_entity(payload, _ctx(pool, mock_enums, agent))


@pytest.mark.asyncio
async def test_revert_entity_approval_short_circuit(monkeypatch, mock_enums):
    """revert_entity should return approval payload before direct revert."""

    entity_id = str(uuid4())
    audit_id = str(uuid4())
    pool = _PoolStub(
        fetchrow_rows=[
            {"table_name": "entities", "record_id": entity_id},
        ]
    )
    agent = _public_agent(mock_enums)
    payload = RevertEntityInput(entity_id=entity_id, audit_id=audit_id)
    do_revert = AsyncMock()

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr(
        "nebula_mcp.server.maybe_require_approval",
        AsyncMock(return_value={"status": "approval_required", "approval_request_id": "x1"}),
    )
    monkeypatch.setattr("nebula_mcp.server.do_revert_entity", do_revert)

    result = await revert_entity(payload, _ctx(pool, mock_enums, agent))

    assert result["status"] == "approval_required"
    do_revert.assert_not_awaited()


@pytest.mark.asyncio
async def test_revert_entity_direct_path_sets_and_resets_changed_by(monkeypatch, mock_enums):
    """revert_entity direct path should set and reset changed-by session values."""

    entity_id = str(uuid4())
    audit_id = str(uuid4())

    class _ConnWithExec:
        def __init__(self):
            self.execute = AsyncMock()

    conn = _ConnWithExec()
    pool = _PoolStub(
        conn=conn,
        fetchrow_rows=[
            {"table_name": "entities", "record_id": entity_id},
        ],
    )
    agent = _public_agent(mock_enums)
    payload = RevertEntityInput(entity_id=entity_id, audit_id=audit_id)

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr("nebula_mcp.server.maybe_require_approval", AsyncMock(return_value=None))
    monkeypatch.setattr(
        "nebula_mcp.server.do_revert_entity",
        AsyncMock(return_value={"id": entity_id, "reverted": True}),
    )

    result = await revert_entity(payload, _ctx(pool, mock_enums, agent))

    assert result["reverted"] is True
    assert conn.execute.await_count == 4


@pytest.mark.asyncio
async def test_create_log_approval_short_circuit(monkeypatch, mock_enums):
    """create_log should return approval payload without executing writer."""

    pool = _PoolStub()
    agent = _public_agent(mock_enums)
    payload = CreateLogInput(
        title="note",
        content="hello",
        log_type="note",
        status="active",
        tags=[],
        metadata={},
    )
    maybe_approval = AsyncMock(
        return_value={"status": "approval_required", "approval_request_id": "a1"}
    )
    execute_log = AsyncMock()

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr("nebula_mcp.server.maybe_require_approval", maybe_approval)
    monkeypatch.setattr("nebula_mcp.server.execute_create_log", execute_log)

    result = await create_log(payload, _ctx(pool, mock_enums, agent))

    assert result["status"] == "approval_required"
    execute_log.assert_not_awaited()


@pytest.mark.asyncio
async def test_get_log_not_found_raises(monkeypatch, mock_enums):
    """get_log should raise when requested id does not exist."""

    pool = _PoolStub(fetchrow_rows=[None])
    agent = _public_agent(mock_enums)
    payload = GetLogInput(log_id=str(uuid4()))

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )

    with pytest.raises(ValueError, match="Log not found"):
        await get_log(payload, _ctx(pool, mock_enums, agent))


@pytest.mark.asyncio
async def test_get_log_access_denied_when_hidden_relationships(monkeypatch, mock_enums):
    """get_log should deny access when log has hidden related nodes."""

    pool = _PoolStub(fetchrow_rows=[{"id": uuid4(), "title": "x"}])
    agent = _public_agent(mock_enums)
    payload = GetLogInput(log_id=str(uuid4()))

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr("nebula_mcp.server._has_hidden_relationships", AsyncMock(return_value=True))

    with pytest.raises(ValueError, match="Access denied"):
        await get_log(payload, _ctx(pool, mock_enums, agent))


@pytest.mark.asyncio
async def test_query_logs_filters_hidden_rows(monkeypatch, mock_enums):
    """query_logs should skip rows that fail hidden-relationship checks."""

    hidden_id = uuid4()
    visible_id = uuid4()
    pool = _PoolStub(
        fetch_rows=[
            [
                {"id": hidden_id, "title": "hidden"},
                {"id": visible_id, "title": "visible"},
            ]
        ]
    )
    agent = _public_agent(mock_enums)
    payload = QueryLogsInput(log_type="note", limit=50, offset=0)

    async def _hidden(_pool, _enums, _agent, _node_type, node_id):
        return str(node_id) == str(hidden_id)

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr("nebula_mcp.server._has_hidden_relationships", _hidden)

    result = await query_logs(payload, _ctx(pool, mock_enums, agent))

    assert len(result) == 1
    assert str(result[0]["id"]) == str(visible_id)


@pytest.mark.asyncio
async def test_update_log_access_denied_when_hidden_relationships(monkeypatch, mock_enums):
    """update_log should reject writes when linked data is hidden."""

    pool = _PoolStub()
    agent = _public_agent(mock_enums)
    payload = UpdateLogInput(id=str(uuid4()), status="active")

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr("nebula_mcp.server._has_hidden_relationships", AsyncMock(return_value=True))

    with pytest.raises(ValueError, match="Access denied"):
        await update_log(payload, _ctx(pool, mock_enums, agent))


@pytest.mark.asyncio
async def test_update_log_approval_short_circuit(monkeypatch, mock_enums):
    """update_log should return approval payload when maybe_require_approval triggers."""

    pool = _PoolStub()
    agent = _public_agent(mock_enums)
    payload = UpdateLogInput(id=str(uuid4()), status="active")

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr(
        "nebula_mcp.server._has_hidden_relationships", AsyncMock(return_value=False)
    )
    monkeypatch.setattr(
        "nebula_mcp.server.maybe_require_approval",
        AsyncMock(return_value={"status": "approval_required", "approval_request_id": "a1"}),
    )
    monkeypatch.setattr("nebula_mcp.server.execute_update_log", AsyncMock())

    result = await update_log(payload, _ctx(pool, mock_enums, agent))

    assert result["status"] == "approval_required"


@pytest.mark.asyncio
async def test_create_log_direct_returns_executor_row(monkeypatch, mock_enums):
    """create_log should call executor directly when approval is not needed."""

    pool = _PoolStub()
    agent = _public_agent(mock_enums)
    payload = CreateLogInput(
        title="note",
        content="hello",
        log_type="note",
        status="active",
        tags=[],
        metadata={},
    )

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr("nebula_mcp.server.maybe_require_approval", AsyncMock(return_value=None))
    monkeypatch.setattr(
        "nebula_mcp.server.execute_create_log",
        AsyncMock(return_value={"id": str(uuid4())}),
    )

    result = await create_log(payload, _ctx(pool, mock_enums, agent))

    assert "id" in result


@pytest.mark.asyncio
async def test_get_log_direct_returns_row(monkeypatch, mock_enums):
    """get_log should return row when visible."""

    log_id = str(uuid4())
    pool = _PoolStub(fetchrow_rows=[{"id": log_id, "title": "ok"}])
    agent = _public_agent(mock_enums)
    payload = GetLogInput(log_id=log_id)

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr(
        "nebula_mcp.server._has_hidden_relationships", AsyncMock(return_value=False)
    )

    result = await get_log(payload, _ctx(pool, mock_enums, agent))

    assert result["id"] == log_id


@pytest.mark.asyncio
async def test_update_log_log_type_validation_and_direct_return(monkeypatch, mock_enums):
    """update_log should validate log type/status then return direct update result."""

    log_id = str(uuid4())
    pool = _PoolStub()
    agent = _public_agent(mock_enums)
    payload = UpdateLogInput(id=log_id, log_type="note", status="active")

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr(
        "nebula_mcp.server._has_hidden_relationships", AsyncMock(return_value=False)
    )
    monkeypatch.setattr("nebula_mcp.server.maybe_require_approval", AsyncMock(return_value=None))
    monkeypatch.setattr(
        "nebula_mcp.server.execute_update_log",
        AsyncMock(return_value={"id": log_id}),
    )

    result = await update_log(payload, _ctx(pool, mock_enums, agent))

    assert result["id"] == log_id


@pytest.mark.asyncio
async def test_create_relationship_approval_short_circuit(monkeypatch, mock_enums):
    """create_relationship should return approval payload before executor call."""

    pool = _PoolStub()
    agent = _public_agent(mock_enums)
    payload = CreateRelationshipInput(
        source_type="entity",
        source_id=str(uuid4()),
        target_type="entity",
        target_id=str(uuid4()),
        relationship_type="related-to",
        properties={},
    )

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr("nebula_mcp.server._validate_relationship_node", AsyncMock())
    monkeypatch.setattr(
        "nebula_mcp.server.maybe_require_approval",
        AsyncMock(return_value={"status": "approval_required", "approval_request_id": "r1"}),
    )
    monkeypatch.setattr("nebula_mcp.server.execute_create_relationship", AsyncMock())

    result = await create_relationship(payload, _ctx(pool, mock_enums, agent))

    assert result["status"] == "approval_required"


@pytest.mark.asyncio
async def test_get_relationships_filters_job_rows_by_node_allowed(monkeypatch, mock_enums):
    """get_relationships should filter blocked source/target job rows."""

    blocked_job = "2026Q1-ABCD"
    allowed_job = "2026Q1-ABCE"
    entity_id = str(uuid4())
    rows = [
        {
            "source_type": "job",
            "source_id": blocked_job,
            "target_type": "entity",
            "target_id": entity_id,
        },
        {
            "source_type": "entity",
            "source_id": entity_id,
            "target_type": "job",
            "target_id": allowed_job,
        },
        {
            "source_type": "entity",
            "source_id": entity_id,
            "target_type": "entity",
            "target_id": str(uuid4()),
        },
    ]
    pool = _PoolStub(fetch_rows=[rows])
    agent = _public_agent(mock_enums)
    payload = GetRelationshipsInput(
        source_type="entity",
        source_id=entity_id,
        direction="both",
        relationship_type=None,
    )

    async def _allowed(_pool, _enums, _agent, _node_type, node_id):
        return node_id != blocked_job

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr("nebula_mcp.server._node_allowed", _allowed)
    monkeypatch.setattr(
        "nebula_mcp.server._normalize_relationship_row",
        lambda row, _scopes: {
            "source_id": row["source_id"],
            "target_id": row["target_id"],
        },
    )

    result = await get_relationships(payload, _ctx(pool, mock_enums, agent))

    assert len(result) == 2
    assert all(item["source_id"] != blocked_job for item in result)


@pytest.mark.asyncio
async def test_query_relationships_filters_blocked_target_jobs(monkeypatch, mock_enums):
    """query_relationships should skip rows whose target job is not visible."""

    blocked_job = "2026Q2-ABCD"
    rows = [
        {
            "source_type": "entity",
            "source_id": str(uuid4()),
            "target_type": "job",
            "target_id": blocked_job,
        },
        {
            "source_type": "entity",
            "source_id": str(uuid4()),
            "target_type": "entity",
            "target_id": str(uuid4()),
        },
    ]
    pool = _PoolStub(fetch_rows=[rows])
    agent = _public_agent(mock_enums)
    payload = QueryRelationshipsInput(limit=50)

    async def _allowed(_pool, _enums, _agent, _node_type, node_id):
        return node_id != blocked_job

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr("nebula_mcp.server._node_allowed", _allowed)
    monkeypatch.setattr(
        "nebula_mcp.server._normalize_relationship_row",
        lambda row, _scopes: {"target_id": row["target_id"]},
    )

    result = await query_relationships(payload, _ctx(pool, mock_enums, agent))

    assert len(result) == 1
    assert result[0]["target_id"] != blocked_job


@pytest.mark.asyncio
async def test_update_relationship_not_found_raises(monkeypatch, mock_enums):
    """update_relationship should raise clear not-found for missing id."""

    pool = _PoolStub(fetchrow_rows=[None])
    agent = _public_agent(mock_enums)
    payload = UpdateRelationshipInput(relationship_id=str(uuid4()), status=None, properties=None)

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )

    with pytest.raises(ValueError, match="Relationship not found"):
        await update_relationship(payload, _ctx(pool, mock_enums, agent))


@pytest.mark.asyncio
async def test_update_relationship_direct_update_returns_empty_when_no_row(monkeypatch, mock_enums):
    """update_relationship should return empty dict when update query returns no row."""

    relationship_row = {
        "source_type": "entity",
        "source_id": str(uuid4()),
        "target_type": "entity",
        "target_id": str(uuid4()),
    }
    pool = _PoolStub(fetchrow_rows=[relationship_row, None])
    agent = _public_agent(mock_enums)
    payload = UpdateRelationshipInput(
        relationship_id=str(uuid4()),
        status="active",
        properties={"x": 1},
    )

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr("nebula_mcp.server._validate_relationship_node", AsyncMock())
    monkeypatch.setattr("nebula_mcp.server.maybe_require_approval", AsyncMock(return_value=None))

    result = await update_relationship(payload, _ctx(pool, mock_enums, agent))

    assert result == {}


@pytest.mark.asyncio
async def test_decode_graph_path_ignores_invalid_segments():
    """_decode_graph_path should skip malformed entries and split valid ones."""

    decoded = server_mod._decode_graph_path(["entity:1", "bad", "job:2026Q1-ABCD"])

    assert decoded == [
        {"type": "entity", "id": "1"},
        {"type": "job", "id": "2026Q1-ABCD"},
    ]
    assert server_mod._decode_graph_path([]) == []
    assert server_mod._decode_graph_path(None) == []


@pytest.mark.asyncio
async def test_graph_neighbors_denies_inaccessible_source(monkeypatch, mock_enums):
    """graph_neighbors should fail fast when source node is not readable."""

    pool = _PoolStub()
    agent = _public_agent(mock_enums)
    payload = GraphNeighborsInput(
        source_type="entity", source_id=str(uuid4()), max_hops=2, limit=10
    )

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr("nebula_mcp.server._node_allowed", AsyncMock(return_value=False))

    with pytest.raises(ValueError, match="Access denied"):
        await graph_neighbors(payload, _ctx(pool, mock_enums, agent))


@pytest.mark.asyncio
async def test_graph_neighbors_filters_rows_with_blocked_path_nodes(monkeypatch, mock_enums):
    """graph_neighbors should remove neighbor rows with blocked path nodes."""

    source_id = str(uuid4())
    blocked_id = str(uuid4())
    pool = _PoolStub(
        fetch_rows=[
            [
                {
                    "node_type": "entity",
                    "node_id": str(uuid4()),
                    "depth": 1,
                    "path": [f"entity:{source_id}", f"entity:{blocked_id}"],
                },
                {
                    "node_type": "entity",
                    "node_id": str(uuid4()),
                    "depth": 1,
                    "path": [f"entity:{source_id}", f"entity:{uuid4()}"],
                },
            ]
        ]
    )
    agent = _public_agent(mock_enums)
    payload = GraphNeighborsInput(source_type="entity", source_id=source_id, max_hops=2, limit=10)

    async def _allowed(_pool, _enums, _agent, _node_type, node_id):
        return node_id != blocked_id

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr("nebula_mcp.server._node_allowed", _allowed)

    result = await graph_neighbors(payload, _ctx(pool, mock_enums, agent))

    assert len(result) == 1
    assert all(node["id"] != blocked_id for node in result[0]["path"])


@pytest.mark.asyncio
async def test_graph_shortest_path_no_row_raises(monkeypatch, mock_enums):
    """graph_shortest_path should raise when backend returns no path."""

    source_id = str(uuid4())
    target_id = str(uuid4())
    pool = _PoolStub(fetchrow_rows=[None])
    agent = _public_agent(mock_enums)
    payload = GraphShortestPathInput(
        source_type="entity",
        source_id=source_id,
        target_type="entity",
        target_id=target_id,
        max_hops=3,
    )

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr("nebula_mcp.server._node_allowed", AsyncMock(return_value=True))

    with pytest.raises(ValueError, match="No path found"):
        await graph_shortest_path(payload, _ctx(pool, mock_enums, agent))


@pytest.mark.asyncio
async def test_graph_shortest_path_blocked_path_node_raises(monkeypatch, mock_enums):
    """graph_shortest_path should reject paths containing hidden nodes."""

    source_id = str(uuid4())
    target_id = str(uuid4())
    blocked_id = str(uuid4())
    pool = _PoolStub(
        fetchrow_rows=[
            {
                "depth": 2,
                "path": [
                    f"entity:{source_id}",
                    f"entity:{blocked_id}",
                    f"entity:{target_id}",
                ],
            }
        ]
    )
    agent = _public_agent(mock_enums)
    payload = GraphShortestPathInput(
        source_type="entity",
        source_id=source_id,
        target_type="entity",
        target_id=target_id,
        max_hops=3,
    )

    async def _allowed(_pool, _enums, _agent, _node_type, node_id):
        return node_id != blocked_id

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr("nebula_mcp.server._node_allowed", _allowed)

    with pytest.raises(ValueError, match="No path found"):
        await graph_shortest_path(payload, _ctx(pool, mock_enums, agent))


@pytest.mark.asyncio
async def test_create_job_invalid_priority_raises(monkeypatch, mock_enums):
    """create_job should reject unknown priorities before approval/executor."""

    pool = _PoolStub()
    agent = _public_agent(mock_enums)
    payload = CreateJobInput(title="t", priority="urgent")

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )

    with pytest.raises(ValueError, match="Invalid priority: urgent"):
        await create_job(payload, _ctx(pool, mock_enums, agent))


@pytest.mark.asyncio
async def test_create_job_non_admin_defaults_and_approval(monkeypatch, mock_enums):
    """create_job should default scopes and force agent_id for non-admin callers."""

    pool = _PoolStub()
    agent = _public_agent(mock_enums)
    payload = CreateJobInput(title="t", scopes=[])
    captured = {}

    async def _approval(_pool, _agent, _action, data):
        captured.update(data)
        return {"status": "approval_required", "approval_request_id": "j1"}

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr("nebula_mcp.server.scope_names_from_ids", lambda ids, enums: ["public"])
    monkeypatch.setattr("nebula_mcp.server.enforce_scope_subset", lambda scopes, allowed: scopes)
    monkeypatch.setattr("nebula_mcp.server.maybe_require_approval", _approval)

    result = await create_job(payload, _ctx(pool, mock_enums, agent))

    assert result["status"] == "approval_required"
    assert captured["scopes"] == ["public"]
    assert captured["agent_id"] == agent["id"]


@pytest.mark.asyncio
async def test_get_job_returns_row(monkeypatch, mock_enums):
    """get_job should return fetched job when read access passes."""

    job = {"id": "2026Q1-ABCD", "privacy_scope_ids": []}
    pool = _PoolStub()
    agent = _public_agent(mock_enums)
    payload = GetJobInput(job_id="2026Q1-ABCD")

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr("nebula_mcp.server._get_job_row", AsyncMock(return_value=job))
    monkeypatch.setattr("nebula_mcp.server._require_job_read", lambda a, e, j: None)

    result = await get_job(payload, _ctx(pool, mock_enums, agent))
    assert result == job


@pytest.mark.asyncio
async def test_query_jobs_validates_assigned_to(monkeypatch, mock_enums):
    """query_jobs should validate assignee UUIDs before querying."""

    pool = _PoolStub()
    agent = _public_agent(mock_enums)
    payload = QueryJobsInput(assigned_to="not-a-uuid")

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )

    with pytest.raises(ValueError, match="Invalid assignee id"):
        await query_jobs(payload, _ctx(pool, mock_enums, agent))


@pytest.mark.asyncio
async def test_update_job_invalid_assignee_raises(monkeypatch, mock_enums):
    """update_job should reject malformed assigned_to values."""

    pool = _PoolStub()
    agent = _public_agent(mock_enums)
    payload = UpdateJobInput(job_id="2026Q1-ABCD", assigned_to="bad-uuid")

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr(
        "nebula_mcp.server._get_job_row",
        AsyncMock(
            return_value={
                "id": "2026Q1-ABCD",
                "privacy_scope_ids": [],
                "agent_id": agent["id"],
            }
        ),
    )
    monkeypatch.setattr("nebula_mcp.server._require_job_owner", lambda a, e, j: None)

    with pytest.raises(ValueError, match="Invalid assignee id"):
        await update_job(payload, _ctx(pool, mock_enums, agent))


@pytest.mark.asyncio
async def test_update_job_status_not_found_raises(monkeypatch, mock_enums):
    """update_job_status should raise when update query returns no row."""

    pool = _PoolStub(fetchrow_rows=[None])
    agent = _public_agent(mock_enums)
    payload = UpdateJobStatusInput(job_id="2026Q1-ABCD", status="active")

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr(
        "nebula_mcp.server._get_job_row",
        AsyncMock(
            return_value={
                "id": "2026Q1-ABCD",
                "privacy_scope_ids": [],
                "agent_id": agent["id"],
            }
        ),
    )
    monkeypatch.setattr("nebula_mcp.server._require_job_owner", lambda a, e, j: None)
    monkeypatch.setattr("nebula_mcp.server.maybe_require_approval", AsyncMock(return_value=None))

    with pytest.raises(ValueError, match="not found"):
        await update_job_status(payload, _ctx(pool, mock_enums, agent))


@pytest.mark.asyncio
async def test_create_subtask_invalid_priority_raises(monkeypatch, mock_enums):
    """create_subtask should reject unknown priority values."""

    pool = _PoolStub()
    agent = _public_agent(mock_enums)
    payload = CreateSubtaskInput(parent_job_id="2026Q1-ABCD", title="child", priority="urgent")

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr(
        "nebula_mcp.server._get_job_row",
        AsyncMock(
            return_value={
                "id": "2026Q1-ABCD",
                "privacy_scope_ids": [],
                "agent_id": agent["id"],
            }
        ),
    )
    monkeypatch.setattr("nebula_mcp.server._require_job_owner", lambda a, e, j: None)

    with pytest.raises(ValueError, match="Invalid priority"):
        await create_subtask(payload, _ctx(pool, mock_enums, agent))


@pytest.mark.asyncio
async def test_update_relationship_approval_short_circuit(monkeypatch, mock_enums):
    """update_relationship should return approval payload when required."""

    relationship_id = str(uuid4())
    pool = _PoolStub(
        fetchrow_rows=[
            {
                "source_type": "entity",
                "source_id": str(uuid4()),
                "target_type": "entity",
                "target_id": str(uuid4()),
            }
        ]
    )
    agent = _public_agent(mock_enums)
    payload = UpdateRelationshipInput(relationship_id=relationship_id, status="active")

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr("nebula_mcp.server._validate_relationship_node", AsyncMock())
    monkeypatch.setattr(
        "nebula_mcp.server.maybe_require_approval",
        AsyncMock(return_value={"status": "approval_required", "approval_request_id": "ur1"}),
    )

    result = await update_relationship(payload, _ctx(pool, mock_enums, agent))

    assert result["status"] == "approval_required"
    assert len(pool.fetchrow_calls) == 1


@pytest.mark.asyncio
async def test_graph_shortest_path_denies_hidden_source(monkeypatch, mock_enums):
    """graph_shortest_path should deny when source node is not readable."""

    pool = _PoolStub()
    agent = _public_agent(mock_enums)
    payload = GraphShortestPathInput(
        source_type="entity",
        source_id=str(uuid4()),
        target_type="entity",
        target_id=str(uuid4()),
        max_hops=2,
    )

    async def _allowed(_pool, _enums, _agent, _node_type, node_id):
        return str(node_id) != payload.source_id

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr("nebula_mcp.server._node_allowed", _allowed)

    with pytest.raises(ValueError, match="Access denied"):
        await graph_shortest_path(payload, _ctx(pool, mock_enums, agent))


@pytest.mark.asyncio
async def test_update_job_invalid_priority_raises(monkeypatch, mock_enums):
    """update_job should reject unknown priority values."""

    pool = _PoolStub()
    agent = _public_agent(mock_enums)
    payload = UpdateJobInput(job_id="2026Q1-ABCD", priority="urgent")

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr(
        "nebula_mcp.server._get_job_row",
        AsyncMock(
            return_value={
                "id": "2026Q1-ABCD",
                "privacy_scope_ids": [],
                "agent_id": agent["id"],
            }
        ),
    )
    monkeypatch.setattr("nebula_mcp.server._require_job_owner", lambda a, e, j: None)

    with pytest.raises(ValueError, match="Invalid priority"):
        await update_job(payload, _ctx(pool, mock_enums, agent))


@pytest.mark.asyncio
async def test_update_job_status_approval_short_circuit(monkeypatch, mock_enums):
    """update_job_status should return approval payload before DB update."""

    pool = _PoolStub()
    agent = _public_agent(mock_enums)
    payload = UpdateJobStatusInput(job_id="2026Q1-ABCD", status="active")

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr(
        "nebula_mcp.server._get_job_row",
        AsyncMock(
            return_value={
                "id": "2026Q1-ABCD",
                "privacy_scope_ids": [],
                "agent_id": agent["id"],
            }
        ),
    )
    monkeypatch.setattr("nebula_mcp.server._require_job_owner", lambda a, e, j: None)
    monkeypatch.setattr(
        "nebula_mcp.server.maybe_require_approval",
        AsyncMock(return_value={"status": "approval_required", "approval_request_id": "js1"}),
    )

    result = await update_job_status(payload, _ctx(pool, mock_enums, agent))

    assert result["status"] == "approval_required"


@pytest.mark.asyncio
async def test_create_subtask_approval_short_circuit(monkeypatch, mock_enums):
    """create_subtask should return approval payload before create_job execution."""

    pool = _PoolStub()
    agent = _public_agent(mock_enums)
    payload = CreateSubtaskInput(parent_job_id="2026Q1-ABCD", title="child", priority="medium")
    execute_job = AsyncMock()

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr(
        "nebula_mcp.server._get_job_row",
        AsyncMock(
            return_value={
                "id": "2026Q1-ABCD",
                "privacy_scope_ids": [mock_enums.scopes.name_to_id["public"]],
                "agent_id": agent["id"],
            }
        ),
    )
    monkeypatch.setattr("nebula_mcp.server._require_job_owner", lambda a, e, j: None)
    monkeypatch.setattr(
        "nebula_mcp.server.maybe_require_approval",
        AsyncMock(return_value={"status": "approval_required", "approval_request_id": "sj1"}),
    )
    monkeypatch.setattr("nebula_mcp.server.execute_create_job", execute_job)

    result = await create_subtask(payload, _ctx(pool, mock_enums, agent))

    assert result["status"] == "approval_required"
    execute_job.assert_not_awaited()


@pytest.mark.asyncio
async def test_create_file_approval_short_circuit(monkeypatch, mock_enums):
    """create_file should return approval payload when approval is required."""

    pool = _PoolStub()
    agent = _public_agent(mock_enums)
    payload = CreateFileInput(filename="a.txt", status="active", uri="https://example.com/a.txt")
    execute_file = AsyncMock()

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr(
        "nebula_mcp.server.maybe_require_approval",
        AsyncMock(return_value={"status": "approval_required", "approval_request_id": "f1"}),
    )
    monkeypatch.setattr("nebula_mcp.server.execute_create_file", execute_file)

    result = await create_file(payload, _ctx(pool, mock_enums, agent))

    assert result["status"] == "approval_required"
    execute_file.assert_not_awaited()


@pytest.mark.asyncio
async def test_create_file_direct_path_returns_executor_row(monkeypatch, mock_enums):
    """create_file should call executor directly when approval is not required."""

    pool = _PoolStub()
    agent = _public_agent(mock_enums)
    payload = CreateFileInput(filename="a.txt", status="active", uri="https://example.com/a.txt")

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr("nebula_mcp.server.maybe_require_approval", AsyncMock(return_value=None))
    monkeypatch.setattr(
        "nebula_mcp.server.execute_create_file",
        AsyncMock(return_value={"id": str(uuid4()), "filename": "a.txt"}),
    )

    result = await create_file(payload, _ctx(pool, mock_enums, agent))

    assert result["filename"] == "a.txt"


@pytest.mark.asyncio
async def test_get_file_not_found_raises(monkeypatch, mock_enums):
    """get_file should raise when id does not exist."""

    pool = _PoolStub(fetchrow_rows=[None])
    agent = _public_agent(mock_enums)
    payload = GetFileInput(file_id=str(uuid4()))

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )

    with pytest.raises(ValueError, match="File not found"):
        await get_file(payload, _ctx(pool, mock_enums, agent))


@pytest.mark.asyncio
async def test_get_file_access_denied_when_hidden(monkeypatch, mock_enums):
    """get_file should deny access when hidden relationships exist."""

    pool = _PoolStub(fetchrow_rows=[{"id": str(uuid4()), "filename": "a.txt"}])
    agent = _public_agent(mock_enums)
    payload = GetFileInput(file_id=str(uuid4()))

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr("nebula_mcp.server._has_hidden_relationships", AsyncMock(return_value=True))

    with pytest.raises(ValueError, match="Access denied"):
        await get_file(payload, _ctx(pool, mock_enums, agent))


@pytest.mark.asyncio
async def test_get_file_direct_returns_row(monkeypatch, mock_enums):
    """get_file should return row when visible."""

    file_id = str(uuid4())
    pool = _PoolStub(fetchrow_rows=[{"id": file_id, "filename": "a.txt"}])
    agent = _public_agent(mock_enums)
    payload = GetFileInput(file_id=file_id)

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr(
        "nebula_mcp.server._has_hidden_relationships", AsyncMock(return_value=False)
    )

    result = await get_file(payload, _ctx(pool, mock_enums, agent))

    assert result["id"] == file_id


@pytest.mark.asyncio
async def test_list_files_filters_hidden_rows(monkeypatch, mock_enums):
    """list_files should skip hidden rows."""

    hidden_id = str(uuid4())
    visible_id = str(uuid4())
    pool = _PoolStub(
        fetch_rows=[
            [
                {"id": hidden_id, "filename": "hidden.txt"},
                {"id": visible_id, "filename": "visible.txt"},
            ]
        ]
    )
    agent = _public_agent(mock_enums)
    payload = QueryFilesInput(limit=100, offset=-3)

    async def _hidden(_pool, _enums, _agent, _node_type, node_id):
        return str(node_id) == hidden_id

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr("nebula_mcp.server._clamp_limit", lambda _v: 50)
    monkeypatch.setattr("nebula_mcp.server._has_hidden_relationships", _hidden)

    result = await list_files(payload, _ctx(pool, mock_enums, agent))

    assert len(result) == 1
    assert result[0]["filename"] == "visible.txt"


@pytest.mark.asyncio
async def test_update_file_access_denied_when_hidden(monkeypatch, mock_enums):
    """update_file should reject write when file is hidden."""

    pool = _PoolStub()
    agent = _public_agent(mock_enums)
    payload = UpdateFileInput(file_id=str(uuid4()), status="active")

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr("nebula_mcp.server._has_hidden_relationships", AsyncMock(return_value=True))

    with pytest.raises(ValueError, match="Access denied"):
        await update_file(payload, _ctx(pool, mock_enums, agent))


@pytest.mark.asyncio
async def test_update_file_approval_short_circuit(monkeypatch, mock_enums):
    """update_file should return approval payload before executor call."""

    pool = _PoolStub()
    agent = _public_agent(mock_enums)
    payload = UpdateFileInput(file_id=str(uuid4()), status="active")
    execute_file = AsyncMock()

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr(
        "nebula_mcp.server._has_hidden_relationships", AsyncMock(return_value=False)
    )
    monkeypatch.setattr(
        "nebula_mcp.server.maybe_require_approval",
        AsyncMock(return_value={"status": "approval_required", "approval_request_id": "f2"}),
    )
    monkeypatch.setattr("nebula_mcp.server.execute_update_file", execute_file)

    result = await update_file(payload, _ctx(pool, mock_enums, agent))

    assert result["status"] == "approval_required"
    execute_file.assert_not_awaited()


@pytest.mark.asyncio
async def test_update_file_missing_row_raises(monkeypatch, mock_enums):
    """update_file should raise not found when executor returns empty result."""

    pool = _PoolStub()
    agent = _public_agent(mock_enums)
    payload = UpdateFileInput(file_id=str(uuid4()), status="active")

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr(
        "nebula_mcp.server._has_hidden_relationships", AsyncMock(return_value=False)
    )
    monkeypatch.setattr("nebula_mcp.server.maybe_require_approval", AsyncMock(return_value=None))
    monkeypatch.setattr("nebula_mcp.server.execute_update_file", AsyncMock(return_value={}))

    with pytest.raises(ValueError, match="File not found"):
        await update_file(payload, _ctx(pool, mock_enums, agent))


@pytest.mark.asyncio
async def test_update_file_direct_path_returns_row(monkeypatch, mock_enums):
    """update_file should return executor row on direct path."""

    file_id = str(uuid4())
    pool = _PoolStub()
    agent = _public_agent(mock_enums)
    payload = UpdateFileInput(file_id=file_id, status="active")

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr(
        "nebula_mcp.server._has_hidden_relationships", AsyncMock(return_value=False)
    )
    monkeypatch.setattr("nebula_mcp.server.maybe_require_approval", AsyncMock(return_value=None))
    monkeypatch.setattr(
        "nebula_mcp.server.execute_update_file",
        AsyncMock(return_value={"id": file_id, "filename": "x"}),
    )

    result = await update_file(payload, _ctx(pool, mock_enums, agent))

    assert result["id"] == file_id


@pytest.mark.asyncio
async def test_attach_file_to_job_invalid_file_id_raises(monkeypatch, mock_enums):
    """attach_file_to_job should validate file uuid format."""

    pool = _PoolStub()
    agent = _public_agent(mock_enums)
    payload = AttachFileInput(
        file_id="bad-id", target_id="2026Q1-ABCD", relationship_type="references"
    )

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )

    with pytest.raises(ValueError, match="Invalid file id format"):
        await attach_file_to_job(payload, _ctx(pool, mock_enums, agent))


@pytest.mark.asyncio
async def test_attach_file_to_job_missing_file_raises(monkeypatch, mock_enums):
    """attach_file_to_job should raise when file record is missing."""

    pool = _PoolStub(fetchrow_rows=[None])
    agent = _public_agent(mock_enums)
    payload = AttachFileInput(
        file_id=str(uuid4()), target_id="2026Q1-ABCD", relationship_type="references"
    )

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )

    with pytest.raises(ValueError, match="File not found"):
        await attach_file_to_job(payload, _ctx(pool, mock_enums, agent))


@pytest.mark.asyncio
async def test_attach_file_to_job_access_denied_when_hidden(monkeypatch, mock_enums):
    """attach_file_to_job should deny when source file is hidden."""

    pool = _PoolStub(fetchrow_rows=[{"id": str(uuid4())}])
    agent = _public_agent(mock_enums)
    payload = AttachFileInput(
        file_id=str(uuid4()), target_id="2026Q1-ABCD", relationship_type="references"
    )

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr("nebula_mcp.server._has_hidden_relationships", AsyncMock(return_value=True))

    with pytest.raises(ValueError, match="Access denied"):
        await attach_file_to_job(payload, _ctx(pool, mock_enums, agent))


@pytest.mark.asyncio
async def test_attach_file_to_job_approval_short_circuit(monkeypatch, mock_enums):
    """attach_file_to_job should return approval payload before relationship create."""

    pool = _PoolStub(fetchrow_rows=[{"id": str(uuid4())}])
    agent = _public_agent(mock_enums)
    payload = AttachFileInput(
        file_id=str(uuid4()), target_id="2026Q1-ABCD", relationship_type="references"
    )
    execute_rel = AsyncMock()

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr(
        "nebula_mcp.server._has_hidden_relationships", AsyncMock(return_value=False)
    )
    monkeypatch.setattr("nebula_mcp.server._validate_relationship_node", AsyncMock())
    monkeypatch.setattr(
        "nebula_mcp.server.maybe_require_approval",
        AsyncMock(return_value={"status": "approval_required", "approval_request_id": "rf1"}),
    )
    monkeypatch.setattr("nebula_mcp.server.execute_create_relationship", execute_rel)

    result = await attach_file_to_job(payload, _ctx(pool, mock_enums, agent))

    assert result["status"] == "approval_required"
    execute_rel.assert_not_awaited()


@pytest.mark.asyncio
async def test_attach_file_to_job_direct_create(monkeypatch, mock_enums):
    """attach_file_to_job should call relationship executor on direct path."""

    pool = _PoolStub(fetchrow_rows=[{"id": str(uuid4())}])
    agent = _public_agent(mock_enums)
    payload = AttachFileInput(
        file_id=str(uuid4()), target_id="2026Q1-ABCD", relationship_type="references"
    )

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr(
        "nebula_mcp.server._has_hidden_relationships", AsyncMock(return_value=False)
    )
    monkeypatch.setattr("nebula_mcp.server._validate_relationship_node", AsyncMock())
    monkeypatch.setattr("nebula_mcp.server.maybe_require_approval", AsyncMock(return_value=None))
    monkeypatch.setattr(
        "nebula_mcp.server.execute_create_relationship",
        AsyncMock(return_value={"id": str(uuid4())}),
    )

    result = await attach_file_to_job(payload, _ctx(pool, mock_enums, agent))

    assert "id" in result


@pytest.mark.asyncio
async def test_get_protocol_not_found_raises(monkeypatch, mock_enums):
    """get_protocol should raise when name is missing."""

    pool = _PoolStub(fetchrow_rows=[None])
    agent = _public_agent(mock_enums)
    payload = GetProtocolInput(protocol_name="missing")

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )

    with pytest.raises(ValueError, match="not found"):
        await get_protocol(payload, _ctx(pool, mock_enums, agent))


@pytest.mark.asyncio
async def test_get_protocol_trusted_denied_for_non_admin(monkeypatch, mock_enums):
    """Non-admin callers should not access trusted protocols."""

    pool = _PoolStub(fetchrow_rows=[{"name": "p1", "trusted": True}])
    agent = _public_agent(mock_enums)
    payload = GetProtocolInput(protocol_name="p1")

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )

    with pytest.raises(ValueError, match="Access denied"):
        await get_protocol(payload, _ctx(pool, mock_enums, agent))


@pytest.mark.asyncio
async def test_get_protocol_admin_can_read_trusted(monkeypatch, mock_enums):
    """Admin callers should be able to fetch trusted protocols."""

    pool = _PoolStub(fetchrow_rows=[{"name": "p1", "trusted": True}])
    agent = _admin_agent(mock_enums)
    payload = GetProtocolInput(protocol_name="p1")

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )

    result = await get_protocol(payload, _ctx(pool, mock_enums, agent))

    assert result["name"] == "p1"


@pytest.mark.asyncio
async def test_create_protocol_non_admin_forces_untrusted_direct(monkeypatch, mock_enums):
    """create_protocol should coerce trusted=false for non-admin direct writes."""

    pool = _PoolStub()
    agent = _public_agent(mock_enums)
    payload = CreateProtocolInput(name="p1", title="P1", content="x", status="active", trusted=True)
    execute_protocol = AsyncMock(return_value={"name": "p1", "trusted": False})

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr("nebula_mcp.server.maybe_require_approval", AsyncMock(return_value=None))
    monkeypatch.setattr("nebula_mcp.server.execute_create_protocol", execute_protocol)

    result = await create_protocol(payload, _ctx(pool, mock_enums, agent))

    assert result["trusted"] is False
    sent = execute_protocol.await_args.args[2]
    assert sent["trusted"] is False


@pytest.mark.asyncio
async def test_create_protocol_approval_short_circuit(monkeypatch, mock_enums):
    """create_protocol should return approval payload when required."""

    pool = _PoolStub()
    agent = _public_agent(mock_enums)
    payload = CreateProtocolInput(name="p2", title="P2", content="x", status="active", trusted=True)
    execute_protocol = AsyncMock()

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr(
        "nebula_mcp.server.maybe_require_approval",
        AsyncMock(return_value={"status": "approval_required", "approval_request_id": "p2"}),
    )
    monkeypatch.setattr("nebula_mcp.server.execute_create_protocol", execute_protocol)

    result = await create_protocol(payload, _ctx(pool, mock_enums, agent))

    assert result["status"] == "approval_required"
    execute_protocol.assert_not_awaited()


@pytest.mark.asyncio
async def test_update_protocol_non_admin_forces_untrusted_approval(monkeypatch, mock_enums):
    """update_protocol should coerce trusted=false for non-admin callers."""

    pool = _PoolStub()
    agent = _public_agent(mock_enums)
    payload = UpdateProtocolInput(name="p1", trusted=True)
    execute_protocol = AsyncMock()
    maybe_approval = AsyncMock(
        return_value={"status": "approval_required", "approval_request_id": "p1"}
    )

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr("nebula_mcp.server.maybe_require_approval", maybe_approval)
    monkeypatch.setattr("nebula_mcp.server.execute_update_protocol", execute_protocol)

    result = await update_protocol(payload, _ctx(pool, mock_enums, agent))

    assert result["status"] == "approval_required"
    assert maybe_approval.await_args.args[3]["trusted"] is False
    execute_protocol.assert_not_awaited()


@pytest.mark.asyncio
async def test_update_protocol_direct_with_status(monkeypatch, mock_enums):
    """update_protocol should validate status and return direct executor result."""

    pool = _PoolStub()
    agent = _admin_agent(mock_enums)
    payload = UpdateProtocolInput(name="p3", status="active")

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr("nebula_mcp.server.maybe_require_approval", AsyncMock(return_value=None))
    monkeypatch.setattr(
        "nebula_mcp.server.execute_update_protocol",
        AsyncMock(return_value={"name": "p3"}),
    )

    result = await update_protocol(payload, _ctx(pool, mock_enums, agent))

    assert result["name"] == "p3"


@pytest.mark.asyncio
async def test_list_active_protocols_filters_trusted_for_non_admin(monkeypatch, mock_enums):
    """Non-admin list should filter out trusted protocols."""

    pool = _PoolStub(
        fetch_rows=[
            [
                {"name": "safe", "trusted": False},
                {"name": "trusted", "trusted": True},
            ]
        ]
    )
    agent = _public_agent(mock_enums)

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )

    result = await list_active_protocols(_ctx(pool, mock_enums, agent))

    assert result == [{"name": "safe", "trusted": False}]


@pytest.mark.asyncio
async def test_query_protocols_passes_admin_flag(monkeypatch, mock_enums):
    """query_protocols should pass admin flag to query layer."""

    pool = _PoolStub(fetch_rows=[[{"name": "safe"}]])
    agent = _public_agent(mock_enums)
    payload = QueryProtocolsInput(limit=200)

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr("nebula_mcp.server._clamp_limit", lambda _v: 50)

    result = await query_protocols(payload, _ctx(pool, mock_enums, agent))

    assert result == [{"name": "safe"}]
    assert pool.fetch_calls[0][1][-1] is False


@pytest.mark.asyncio
async def test_get_agent_not_found_raises(monkeypatch, mock_enums):
    """get_agent should raise when no agent row exists."""

    pool = _PoolStub(fetchrow_rows=[None])
    agent = _admin_agent(mock_enums)
    payload = GetAgentInput(agent_id=str(uuid4()))

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )

    with pytest.raises(ValueError, match="Agent not found"):
        await get_agent(payload, _ctx(pool, mock_enums, agent))


@pytest.mark.asyncio
async def test_get_agent_info_not_found_raises(monkeypatch, mock_enums):
    """get_agent_info should raise when requested agent name is missing."""

    pool = _PoolStub(fetchrow_rows=[None])
    agent = _admin_agent(mock_enums)
    payload = GetAgentInfoInput(name="ghost")

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )

    with pytest.raises(ValueError, match="not found"):
        await get_agent_info(payload, _ctx(pool, mock_enums, agent))


@pytest.mark.asyncio
async def test_list_agents_admin_returns_rows(monkeypatch, mock_enums):
    """list_agents should return rows for admin caller."""

    pool = _PoolStub(fetch_rows=[[{"id": str(uuid4()), "name": "a1"}]])
    agent = _admin_agent(mock_enums)
    payload = ListAgentsInput(status_category="active")

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )

    result = await list_agents(payload, _ctx(pool, mock_enums, agent))

    assert result[0]["name"] == "a1"


@pytest.mark.asyncio
async def test_update_agent_non_admin_other_agent_denied(monkeypatch, mock_enums):
    """Non-admin caller should not update a different agent."""

    pool = _PoolStub()
    agent = _public_agent(mock_enums)
    payload = UpdateAgentInput(agent_id=str(uuid4()), description="x")

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )

    with pytest.raises(ValueError, match="Admin scope required"):
        await update_agent(payload, _ctx(pool, mock_enums, agent))


@pytest.mark.asyncio
async def test_update_agent_missing_after_update_raises(monkeypatch, mock_enums):
    """update_agent should raise not found when update returns no row."""

    pool = _PoolStub(fetchrow_rows=[None])
    agent = _admin_agent(mock_enums)
    payload = UpdateAgentInput(agent_id=str(uuid4()), description="x", scopes=["public"])

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )

    with pytest.raises(ValueError, match="Agent not found"):
        await update_agent(payload, _ctx(pool, mock_enums, agent))


@pytest.mark.asyncio
async def test_register_agent_rejects_when_already_authenticated(monkeypatch, mock_enums):
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
async def test_register_agent_create_agent_failure_raises(monkeypatch, mock_enums):
    """register_agent should fail when agent create query returns no row."""

    conn = _ConnStub(
        fetchrow_rows=[
            None,  # check_name
            None,  # create agent
        ]
    )
    pool = _PoolStub(conn=conn)
    payload = AgentEnrollStartInput(name="new-agent-fail")

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, None)),
    )

    with pytest.raises(ValueError, match="Failed to create enrollment agent"):
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


@pytest.mark.asyncio
async def test_update_taxonomy_current_missing_raises(monkeypatch, mock_enums):
    """update_taxonomy should fail when current row lookup misses."""

    pool = _PoolStub()
    agent = _admin_agent(mock_enums)
    payload = UpdateTaxonomyInput(kind="scopes", item_id=str(uuid4()), name="renamed")

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr("nebula_mcp.server._get_taxonomy_row", AsyncMock(return_value=None))

    with pytest.raises(ValueError, match="scopes entry not found"):
        await update_taxonomy(payload, _ctx(pool, mock_enums, agent))


@pytest.mark.asyncio
async def test_create_taxonomy_relationship_type_branch_success(monkeypatch, mock_enums):
    """create_taxonomy should use relationship-types query shape when requested."""

    pool = _PoolStub(fetchrow_rows=[{"name": "supports"}])
    agent = _admin_agent(mock_enums)
    payload = CreateTaxonomyInput(
        kind="relationship-types",
        name="supports",
        description="desc",
        is_symmetric=False,
    )

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr("nebula_mcp.server._refresh_enums_in_context", AsyncMock())

    result = await create_taxonomy(payload, _ctx(pool, mock_enums, agent))

    assert result["name"] == "supports"


@pytest.mark.asyncio
async def test_create_taxonomy_log_types_branch_success(monkeypatch, mock_enums):
    """create_taxonomy should use value_schema branch for log-types."""

    pool = _PoolStub(fetchrow_rows=[{"name": "metric-plus"}])
    agent = _admin_agent(mock_enums)
    payload = CreateTaxonomyInput(
        kind="log-types",
        name="metric-plus",
        description="desc",
        value_schema={"type": "object"},
    )

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr("nebula_mcp.server._refresh_enums_in_context", AsyncMock())

    result = await create_taxonomy(payload, _ctx(pool, mock_enums, agent))

    assert result["name"] == "metric-plus"


@pytest.mark.asyncio
async def test_create_taxonomy_unique_violation_maps_to_value_error(monkeypatch, mock_enums):
    """create_taxonomy should map unique violations to user-facing errors."""

    class _FakeUniqueViolation(Exception):
        pass

    pool = _PoolStub()
    agent = _admin_agent(mock_enums)
    payload = CreateTaxonomyInput(kind="scopes", name="public")

    monkeypatch.setattr(server_mod, "UniqueViolationError", _FakeUniqueViolation)
    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr(
        pool,
        "fetchrow",
        AsyncMock(side_effect=_FakeUniqueViolation("dup")),
    )

    with pytest.raises(ValueError, match="entry already exists"):
        await create_taxonomy(payload, _ctx(pool, mock_enums, agent))


@pytest.mark.asyncio
async def test_update_taxonomy_relationship_type_branch_success(monkeypatch, mock_enums):
    """update_taxonomy should use relationship-types update query shape."""

    pool = _PoolStub(fetchrow_rows=[{"name": "supports"}])
    agent = _admin_agent(mock_enums)
    payload = UpdateTaxonomyInput(
        kind="relationship-types",
        item_id=str(uuid4()),
        name="supports",
        is_symmetric=True,
    )

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr(
        "nebula_mcp.server._get_taxonomy_row",
        AsyncMock(return_value={"id": str(uuid4()), "name": "related-to", "is_builtin": False}),
    )
    monkeypatch.setattr("nebula_mcp.server._refresh_enums_in_context", AsyncMock())

    result = await update_taxonomy(payload, _ctx(pool, mock_enums, agent))

    assert result["name"] == "supports"


@pytest.mark.asyncio
async def test_update_taxonomy_log_types_branch_success(monkeypatch, mock_enums):
    """update_taxonomy should use value_schema update branch for log-types."""

    pool = _PoolStub(fetchrow_rows=[{"name": "metric-plus"}])
    agent = _admin_agent(mock_enums)
    payload = UpdateTaxonomyInput(
        kind="log-types",
        item_id=str(uuid4()),
        name="metric-plus",
        value_schema={"type": "object"},
    )

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr(
        "nebula_mcp.server._get_taxonomy_row",
        AsyncMock(return_value={"id": str(uuid4()), "name": "note", "is_builtin": False}),
    )
    monkeypatch.setattr("nebula_mcp.server._refresh_enums_in_context", AsyncMock())

    result = await update_taxonomy(payload, _ctx(pool, mock_enums, agent))

    assert result["name"] == "metric-plus"


@pytest.mark.asyncio
async def test_update_taxonomy_unique_violation_maps_to_value_error(monkeypatch, mock_enums):
    """update_taxonomy should map unique violations to clear user-facing errors."""

    class _FakeUniqueViolation(Exception):
        pass

    pool = _PoolStub()
    agent = _admin_agent(mock_enums)
    payload = UpdateTaxonomyInput(
        kind="scopes",
        item_id=str(uuid4()),
        name="public",
    )

    monkeypatch.setattr(server_mod, "UniqueViolationError", _FakeUniqueViolation)
    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr(
        "nebula_mcp.server._get_taxonomy_row",
        AsyncMock(return_value={"id": str(uuid4()), "name": "private", "is_builtin": False}),
    )
    monkeypatch.setattr(
        pool,
        "fetchrow",
        AsyncMock(side_effect=_FakeUniqueViolation("dup")),
    )

    with pytest.raises(ValueError, match="entry already exists"):
        await update_taxonomy(payload, _ctx(pool, mock_enums, agent))


@pytest.mark.asyncio
async def test_archive_taxonomy_usage_blocked(monkeypatch, mock_enums):
    """archive_taxonomy should reject entries with active references."""

    pool = _PoolStub()
    agent = _admin_agent(mock_enums)
    payload = ToggleTaxonomyInput(kind="scopes", item_id=str(uuid4()))

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr("nebula_mcp.server._taxonomy_usage_count", AsyncMock(return_value=3))

    with pytest.raises(ValueError, match="Cannot archive"):
        await archive_taxonomy(payload, _ctx(pool, mock_enums, agent))


@pytest.mark.asyncio
async def test_archive_taxonomy_missing_row_raises(monkeypatch, mock_enums):
    """archive_taxonomy should raise when set_active returns no row."""

    pool = _PoolStub(fetchrow_rows=[None])
    agent = _admin_agent(mock_enums)
    payload = ToggleTaxonomyInput(kind="scopes", item_id=str(uuid4()))

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr("nebula_mcp.server._taxonomy_usage_count", AsyncMock(return_value=0))

    with pytest.raises(ValueError, match="cannot archive built-in entry"):
        await archive_taxonomy(payload, _ctx(pool, mock_enums, agent))


@pytest.mark.asyncio
async def test_activate_taxonomy_missing_row_raises(monkeypatch, mock_enums):
    """activate_taxonomy should raise when target row does not exist."""

    pool = _PoolStub(fetchrow_rows=[None])
    agent = _admin_agent(mock_enums)
    payload = ToggleTaxonomyInput(kind="scopes", item_id=str(uuid4()))

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )

    with pytest.raises(ValueError, match="entry not found"):
        await activate_taxonomy(payload, _ctx(pool, mock_enums, agent))
