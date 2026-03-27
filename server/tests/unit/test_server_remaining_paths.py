"""Focused unit tests for remaining nebula_mcp.server branch paths."""

# Standard Library
import runpy
import sys
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import AsyncMock
from uuid import uuid4

# Third-Party
import pytest

# Local
import nebula_mcp.server as server_mod
from nebula_mcp.models import (
    AgentAuthAttachInput,
    AgentEnrollStartInput,
    ApproveRequestInput,
    BulkImportInput,
    BulkUpdateEntityScopesInput,
    ExportDataInput,
    GetAgentInput,
    GetApprovalDiffInput,
    GetApprovalInput,
    GetEntityHistoryInput,
    LoginInput,
    RejectRequestInput,
    UpdateAgentInput,
    UpdateEntityInput,
    UpdateJobInput,
)
from nebula_mcp.server import (
    _has_hidden_relationships,
    _require_entity_write_access,
    _validate_relationship_node,
    agent_auth_attach,
    agent_enroll_start,
    approve_request,
    bulk_import_context,
    bulk_update_entity_scopes,
    export_data,
    export_schema,
    get_agent,
    get_approval,
    get_approval_diff,
    get_entity_history,
    login_user,
    reject_request,
    update_agent,
    update_entity,
    update_job,
)

pytestmark = pytest.mark.unit


class _AsyncCM:
    """Minimal async context manager wrapper."""

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

    async def fetchrow(self, *_args):
        if self._fetchrow_rows:
            return self._fetchrow_rows.pop(0)
        return None

    def transaction(self):
        return _AsyncCM()


class _PoolStub:
    """Pool stub with queued fetch/fetchrow responses."""

    def __init__(self, *, conn=None, fetch_rows=None, fetchrow_rows=None):
        self._conn = conn
        self._fetch_rows = list(fetch_rows or [])
        self._fetchrow_rows = list(fetchrow_rows or [])
        self.fetch_calls: list[tuple] = []
        self.fetchrow_calls: list[tuple] = []

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

    def acquire(self):
        return _AsyncCM(self._conn)


def _ctx(pool, enums, agent):
    """Build MCP context object with lifespan payload."""

    return SimpleNamespace(
        request_context=SimpleNamespace(
            lifespan_context={
                "pool": pool,
                "enums": enums,
                "agent": agent,
            }
        )
    )


def _public_agent(mock_enums):
    """Build non-admin agent."""

    return {
        "id": str(uuid4()),
        "scopes": [mock_enums.scopes.name_to_id["public"]],
        "requires_approval": False,
    }


def _admin_agent(mock_enums):
    """Build admin-scoped agent."""

    return {
        "id": str(uuid4()),
        "scopes": [mock_enums.scopes.name_to_id["admin"]],
        "requires_approval": False,
    }


@pytest.mark.asyncio
async def test_require_entity_write_access_empty_ids_returns(mock_enums):
    """Empty entity list should no-op for non-admin agent."""

    pool = _PoolStub()
    agent = _public_agent(mock_enums)

    await _require_entity_write_access(pool, mock_enums, agent, [])

    assert pool.fetch_calls == []


@pytest.mark.asyncio
async def test_has_hidden_relationships_file_context_missing_row_returns_true(
    mock_enums,
):
    """Missing related context row should mark file as hidden."""

    rel = {
        "source_type": "context",
        "source_id": str(uuid4()),
        "target_type": "file",
        "target_id": str(uuid4()),
    }
    pool = _PoolStub(fetch_rows=[[rel], [rel]], fetchrow_rows=[None])
    agent = _public_agent(mock_enums)

    result = await _has_hidden_relationships(pool, mock_enums, agent, "file", str(uuid4()))

    assert result is True


@pytest.mark.asyncio
async def test_has_hidden_relationships_file_context_scope_denied_returns_true(
    mock_enums,
):
    """Out-of-scope related context should mark file as hidden."""

    rel = {
        "source_type": "context",
        "source_id": str(uuid4()),
        "target_type": "file",
        "target_id": str(uuid4()),
    }
    pool = _PoolStub(
        fetch_rows=[[rel], [rel]],
        fetchrow_rows=[
            {"privacy_scope_ids": [mock_enums.scopes.name_to_id["private"]]},
        ],
    )
    agent = _public_agent(mock_enums)

    result = await _has_hidden_relationships(pool, mock_enums, agent, "file", str(uuid4()))

    assert result is True


@pytest.mark.asyncio
async def test_has_hidden_relationships_file_job_missing_row_returns_true(mock_enums):
    """Missing related job row should mark file as hidden."""

    rel = {
        "source_type": "job",
        "source_id": "2026Q1-ABCD",
        "target_type": "file",
        "target_id": str(uuid4()),
    }
    pool = _PoolStub(fetch_rows=[[rel], [rel]], fetchrow_rows=[None])
    agent = _public_agent(mock_enums)

    result = await _has_hidden_relationships(pool, mock_enums, agent, "file", str(uuid4()))

    assert result is True


@pytest.mark.asyncio
async def test_has_hidden_relationships_file_job_unreadable_returns_true(mock_enums):
    """Unreadable related job should mark file as hidden."""

    rel = {
        "source_type": "job",
        "source_id": "2026Q1-ABCD",
        "target_type": "file",
        "target_id": str(uuid4()),
    }
    pool = _PoolStub(
        fetch_rows=[[rel], [rel]],
        fetchrow_rows=[{"privacy_scope_ids": [mock_enums.scopes.name_to_id["private"]]}],
    )
    agent = _public_agent(mock_enums)

    result = await _has_hidden_relationships(pool, mock_enums, agent, "file", str(uuid4()))

    assert result is True


@pytest.mark.asyncio
async def test_validate_relationship_node_entity_read_denied(monkeypatch, mock_enums):
    """Read validation should deny entity node when node_allowed says false."""

    entity_id = str(uuid4())
    pool = _PoolStub(fetchrow_rows=[{"id": entity_id, "privacy_scope_ids": []}])
    agent = _public_agent(mock_enums)

    monkeypatch.setattr("nebula_mcp.server._node_allowed", AsyncMock(return_value=False))

    with pytest.raises(ValueError, match="Access denied"):
        await _validate_relationship_node(
            pool,
            mock_enums,
            agent,
            "entity",
            entity_id,
            "Source",
            require_write=False,
        )


@pytest.mark.asyncio
async def test_export_schema_returns_contract(monkeypatch, mock_enums):
    """export_schema should read context then return export contract."""

    pool = _PoolStub()
    agent = _public_agent(mock_enums)

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr(
        "nebula_mcp.server.load_export_schema_contract",
        lambda: {"contract": "ok"},
    )

    result = await export_schema(_ctx(pool, mock_enums, agent))

    assert result == {"contract": "ok"}


@pytest.mark.asyncio
async def test_export_data_context_returns_rows(monkeypatch, mock_enums):
    """export_data(context) should return context rows."""

    pool = _PoolStub(fetch_rows=[[{"id": str(uuid4())}]])
    agent = _public_agent(mock_enums)
    payload = ExportDataInput(resource="context", format="json", params={"scopes": ["public"]})

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    result = await export_data(payload, _ctx(pool, mock_enums, agent))

    assert result["items"][0]["id"]


@pytest.mark.asyncio
async def test_agent_enroll_start_create_agent_failure_raises(monkeypatch, mock_enums):
    """agent_enroll_start should fail if create-agent query returns no row."""

    conn = _ConnStub(fetchrow_rows=[None, None])
    pool = _PoolStub(conn=conn)
    payload = AgentEnrollStartInput(name="x")

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, None)),
    )

    with pytest.raises(ValueError, match="Failed to create enrollment agent"):
        await agent_enroll_start(payload, _ctx(pool, mock_enums, None))


@pytest.mark.asyncio
async def test_agent_auth_attach_rejects_authenticated_session(monkeypatch, mock_enums):
    """agent_auth_attach should reject when already authenticated."""

    pool = _PoolStub()
    agent = _public_agent(mock_enums)
    payload = AgentAuthAttachInput(api_key="api-key")

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )

    with pytest.raises(ValueError, match="Agent already authenticated"):
        await agent_auth_attach(payload, _ctx(pool, mock_enums, agent))


@pytest.mark.asyncio
async def test_agent_auth_attach_missing_lifespan_context_raises(monkeypatch, mock_enums):
    """agent_auth_attach should fail when lifespan context is unavailable."""

    pool = _PoolStub()
    payload = AgentAuthAttachInput(api_key="api-key")
    ctx = SimpleNamespace(request_context=SimpleNamespace(lifespan_context=None))

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, None)),
    )
    monkeypatch.setattr(
        "nebula_mcp.server.authenticate_agent_with_key",
        AsyncMock(return_value={"id": str(uuid4()), "name": "a", "scopes": []}),
    )

    with pytest.raises(ValueError, match="Lifespan context not initialized"):
        await agent_auth_attach(payload, ctx)


@pytest.mark.asyncio
async def test_login_user_rejects_authenticated_session(monkeypatch, mock_enums):
    """login_user should reject when agent is already authenticated."""

    pool = _PoolStub()
    agent = _public_agent(mock_enums)
    payload = LoginInput(username="alice")

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )

    with pytest.raises(ValueError, match="Agent already authenticated"):
        await login_user(payload, _ctx(pool, mock_enums, agent))


@pytest.mark.asyncio
async def test_get_approval_not_found_raises(monkeypatch, mock_enums):
    """get_approval should raise when approval request is missing."""

    pool = _PoolStub(fetchrow_rows=[None])
    agent = _admin_agent(mock_enums)
    payload = GetApprovalInput(approval_id=str(uuid4()))

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )

    with pytest.raises(ValueError, match="Approval request not found"):
        await get_approval(payload, _ctx(pool, mock_enums, agent))


@pytest.mark.asyncio
async def test_approve_request_invalid_reviewer_uuid_raises(monkeypatch, mock_enums):
    """approve_request should validate reviewed_by UUID format."""

    pool = _PoolStub(fetchrow_rows=[{"request_type": "create_entity"}])
    agent = _admin_agent(mock_enums)
    payload = ApproveRequestInput(approval_id=str(uuid4()), reviewed_by="bad-id")

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )

    with pytest.raises(ValueError, match="Invalid reviewer id"):
        await approve_request(payload, _ctx(pool, mock_enums, agent))


@pytest.mark.asyncio
async def test_reject_request_calls_helper(monkeypatch, mock_enums):
    """reject_request should delegate to helper on valid payload."""

    pool = _PoolStub()
    agent = _admin_agent(mock_enums)
    payload = RejectRequestInput(approval_id=str(uuid4()), review_notes="no")

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr(
        "nebula_mcp.server.do_reject",
        AsyncMock(return_value={"status": "rejected"}),
    )

    result = await reject_request(payload, _ctx(pool, mock_enums, agent))

    assert result["status"] == "rejected"


@pytest.mark.asyncio
async def test_get_approval_diff_admin_calls_diff_helper(monkeypatch, mock_enums):
    """Admin caller should receive approval diff payload."""

    pool = _PoolStub(fetchrow_rows=[{"requested_by": str(uuid4())}])
    agent = _admin_agent(mock_enums)
    payload = GetApprovalDiffInput(approval_id=str(uuid4()))

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr(
        "nebula_mcp.server.compute_approval_diff",
        AsyncMock(return_value={"diff": "ok"}),
    )

    result = await get_approval_diff(payload, _ctx(pool, mock_enums, agent))

    assert result["diff"] == "ok"


@pytest.mark.asyncio
async def test_bulk_import_context_wrapper_calls_run_bulk_import(monkeypatch, mock_enums):
    """bulk_import_context wrapper should delegate to _run_bulk_import."""

    pool = _PoolStub()
    agent = _public_agent(mock_enums)
    payload = BulkImportInput(format="json", items=[{"title": "x", "source_type": "note"}])

    monkeypatch.setattr(
        "nebula_mcp.server._run_bulk_import",
        AsyncMock(return_value={"created": 1, "failed": 0, "errors": [], "items": []}),
    )

    result = await bulk_import_context(payload, _ctx(pool, mock_enums, agent))

    assert result["created"] == 1


@pytest.mark.asyncio
async def test_update_entity_approval_short_circuit(monkeypatch, mock_enums):
    """update_entity should return approval payload before executor call."""

    pool = _PoolStub()
    agent = _public_agent(mock_enums)
    payload = UpdateEntityInput(entity_id=str(uuid4()), status="active")
    execute_entity = AsyncMock()

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr("nebula_mcp.server._require_entity_write_access", AsyncMock())
    monkeypatch.setattr(
        "nebula_mcp.server.maybe_require_approval",
        AsyncMock(return_value={"status": "approval_required", "approval_request_id": "e1"}),
    )
    monkeypatch.setattr("nebula_mcp.server.execute_update_entity", execute_entity)

    result = await update_entity(payload, _ctx(pool, mock_enums, agent))

    assert result["status"] == "approval_required"
    execute_entity.assert_not_awaited()


@pytest.mark.asyncio
async def test_bulk_update_entity_scopes_approval_short_circuit(monkeypatch, mock_enums):
    """bulk_update_entity_scopes should return approval payload when required."""

    pool = _PoolStub()
    agent = _public_agent(mock_enums)
    payload = BulkUpdateEntityScopesInput(
        entity_ids=[str(uuid4())],
        scopes=["public"],
        op="add",
    )
    update_scopes = AsyncMock()

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr("nebula_mcp.server._require_entity_write_access", AsyncMock())
    monkeypatch.setattr("nebula_mcp.server.normalize_bulk_operation", lambda op: op)
    monkeypatch.setattr("nebula_mcp.server.scope_names_from_ids", lambda _ids, _enums: ["public"])
    monkeypatch.setattr("nebula_mcp.server.enforce_scope_subset", lambda scopes, _allowed: scopes)
    monkeypatch.setattr("nebula_mcp.server.require_scopes", lambda scopes, _enums: scopes)
    monkeypatch.setattr(
        "nebula_mcp.server.maybe_require_approval",
        AsyncMock(return_value={"status": "approval_required", "approval_request_id": "s1"}),
    )
    monkeypatch.setattr("nebula_mcp.server.do_bulk_update_entity_scopes", update_scopes)

    result = await bulk_update_entity_scopes(payload, _ctx(pool, mock_enums, agent))

    assert result["status"] == "approval_required"
    update_scopes.assert_not_awaited()


@pytest.mark.asyncio
async def test_get_entity_history_allowed_returns_rows(monkeypatch, mock_enums):
    """get_entity_history should return helper results when node is readable."""

    pool = _PoolStub()
    agent = _public_agent(mock_enums)
    payload = GetEntityHistoryInput(entity_id=str(uuid4()), limit=50, offset=0)

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr("nebula_mcp.server._node_allowed", AsyncMock(return_value=True))
    monkeypatch.setattr(
        "nebula_mcp.server.fetch_entity_history",
        AsyncMock(return_value=[{"id": "h1"}]),
    )

    result = await get_entity_history(payload, _ctx(pool, mock_enums, agent))

    assert result == [{"id": "h1"}]


@pytest.mark.asyncio
async def test_bulk_import_context_valid_scopes_routes_to_approvals(monkeypatch, mock_enums):
    """Valid context rows should queue approvals for untrusted agents."""

    pool = _PoolStub()
    agent = _public_agent(mock_enums)
    agent["requires_approval"] = True
    payload = BulkImportInput(
        format="json",
        items=[
            {
                "title": "Ops note",
                "source_type": "note",
                "scopes": ["public"],
                "content": "x",
            }
        ],
    )
    create_approval = AsyncMock(return_value={"id": str(uuid4())})

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr("nebula_mcp.server.ensure_approval_capacity", AsyncMock())
    monkeypatch.setattr("nebula_mcp.server.create_approval_request", create_approval)

    result = await bulk_import_context(payload, _ctx(pool, mock_enums, agent))

    assert result["status"] == "approval_required"
    assert result["failed"] == 0
    assert len(result["approvals"]) == 1
    create_approval.assert_awaited_once()


@pytest.mark.asyncio
async def test_update_job_status_field_approval_short_circuit(monkeypatch, mock_enums):
    """update_job should validate status then return approval payload."""

    pool = _PoolStub()
    agent = _public_agent(mock_enums)
    payload = UpdateJobInput(job_id="2026Q1-0001", status="active")
    execute_update = AsyncMock()
    checked: list[str] = []

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr(
        "nebula_mcp.server._get_job_row",
        AsyncMock(return_value={"id": payload.job_id}),
    )
    monkeypatch.setattr("nebula_mcp.server._require_job_owner", lambda *_args: None)
    monkeypatch.setattr(
        "nebula_mcp.server.require_status",
        lambda status, _enums: checked.append(status),
    )
    monkeypatch.setattr(
        "nebula_mcp.server.maybe_require_approval",
        AsyncMock(return_value={"status": "approval_required", "approval_request_id": "j1"}),
    )
    monkeypatch.setattr("nebula_mcp.server.execute_update_job", execute_update)

    result = await update_job(payload, _ctx(pool, mock_enums, agent))

    assert checked == ["active"]
    assert result["status"] == "approval_required"
    execute_update.assert_not_awaited()


@pytest.mark.asyncio
async def test_get_agent_admin_returns_row(monkeypatch, mock_enums):
    """get_agent should return row data for admin callers."""

    row = {"id": str(uuid4()), "name": "alpha"}
    pool = _PoolStub(fetchrow_rows=[row])
    agent = _admin_agent(mock_enums)
    payload = GetAgentInput(agent_id=row["id"])

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )

    result = await get_agent(payload, _ctx(pool, mock_enums, agent))

    assert result["id"] == row["id"]
    assert result["name"] == "alpha"


@pytest.mark.asyncio
async def test_update_agent_admin_success_returns_row(monkeypatch, mock_enums):
    """update_agent should return updated row when admin update succeeds."""

    row = {"id": str(uuid4()), "name": "alpha", "requires_approval": False}
    pool = _PoolStub(fetchrow_rows=[row])
    agent = _admin_agent(mock_enums)
    payload = UpdateAgentInput(
        agent_id=row["id"],
        description="updated",
        scopes=["public"],
        requires_approval=False,
    )

    monkeypatch.setattr(
        "nebula_mcp.server.require_context",
        AsyncMock(return_value=(pool, mock_enums, agent)),
    )
    monkeypatch.setattr(
        "nebula_mcp.server.require_scopes",
        lambda scopes, _enums: [mock_enums.scopes.name_to_id[s] for s in scopes],
    )

    result = await update_agent(payload, _ctx(pool, mock_enums, agent))

    assert result["id"] == row["id"]
    assert result["name"] == "alpha"


def test_main_runs_dotenv_then_mcp(monkeypatch):
    """main should load environment values before starting MCP runtime."""

    calls: list[str] = []

    class _MCPStub:
        def run(self):
            calls.append("run")

    monkeypatch.setattr(server_mod, "load_dotenv", lambda: calls.append("dotenv"))
    monkeypatch.setattr(server_mod, "mcp", _MCPStub())

    server_mod.main()

    assert calls[-1] == "run"
    assert calls.count("dotenv") >= 1


def test_server_script_entrypoint_uses_bootstrap_path(monkeypatch):
    """Script execution should append src path and invoke main entrypoint."""

    calls: list[str] = []
    server_path = Path(server_mod.__file__).resolve()
    expected_src = str(server_path.parents[1])

    class _FastMCPStub:
        def __init__(self, *_args, **_kwargs):
            pass

        def tool(self, *_args, **_kwargs):
            def _decorator(func):
                return func

            return _decorator

        def run(self):
            calls.append("run")

    monkeypatch.setitem(
        sys.modules,
        "dotenv",
        SimpleNamespace(load_dotenv=lambda: calls.append("dotenv")),
    )
    monkeypatch.setitem(
        sys.modules,
        "mcp.server.fastmcp",
        SimpleNamespace(Context=object, FastMCP=_FastMCPStub),
    )

    before_len = len(sys.path)
    runpy.run_path(str(server_path), run_name="__main__")

    assert calls[-1] == "run"
    assert calls.count("dotenv") >= 1
    assert len(sys.path) == before_len + 1
    assert sys.path[-1] == expected_src
