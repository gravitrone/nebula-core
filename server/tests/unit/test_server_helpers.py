"""Unit coverage for pure/branch-heavy helpers in nebula_mcp.server."""

# Standard Library
from types import SimpleNamespace
from unittest.mock import AsyncMock
from uuid import uuid4

# Third-Party
import pytest

# Local
from nebula_mcp.server import (
    MAX_GRAPH_HOPS,
    MAX_PAGE_LIMIT,
    _clamp_hops,
    _clamp_limit,
    _context_semantic_candidate,
    _entity_semantic_candidate,
    _export_response_rows,
    _flatten_csv_value,
    _get_job_row,
    _get_taxonomy_row,
    _has_hidden_relationships,
    _has_write_scopes,
    _is_admin,
    _node_allowed,
    _normalize_relationship_row,
    _require_admin,
    _require_entity_write_access,
    _require_job_id,
    _require_job_read,
    _require_job_write,
    _require_node_id,
    _require_uuid,
    _resolve_scope_ids_for_export,
    _rows_to_csv,
    _scope_filter_ids,
    _taxonomy_kind_or_error,
    _taxonomy_usage_count,
    _validate_relationship_node,
    _validate_taxonomy_payload,
    _visible_scope_names,
)


def _agent_with_scopes(mock_enums, *scope_names):
    """Return an agent payload with selected scope ids."""

    return {
        "id": str(uuid4()),
        "scopes": [mock_enums.scopes.name_to_id[name] for name in scope_names],
    }


def test_clamp_limit_bounds():
    """_clamp_limit enforces [1, MAX_PAGE_LIMIT]."""

    assert _clamp_limit(-5) == 1
    assert _clamp_limit(0) == 1
    assert _clamp_limit(1) == 1
    assert _clamp_limit(MAX_PAGE_LIMIT) == MAX_PAGE_LIMIT
    assert _clamp_limit(MAX_PAGE_LIMIT + 99) == MAX_PAGE_LIMIT


def test_clamp_hops_bounds():
    """_clamp_hops enforces [1, MAX_GRAPH_HOPS]."""

    assert _clamp_hops(-2) == 1
    assert _clamp_hops(0) == 1
    assert _clamp_hops(1) == 1
    assert _clamp_hops(MAX_GRAPH_HOPS) == MAX_GRAPH_HOPS
    assert _clamp_hops(MAX_GRAPH_HOPS + 7) == MAX_GRAPH_HOPS


def test_require_uuid_validation():
    """_require_uuid accepts UUID values and rejects malformed ids."""

    _require_uuid(str(uuid4()), "entity")
    with pytest.raises(ValueError, match="Invalid entity id"):
        _require_uuid("not-a-uuid", "entity")


def test_require_job_id_validation():
    """_require_job_id enforces the Nebula job id format."""

    _require_job_id("2026Q1-ABCD", "job")
    _require_job_id("2026q4-abc2", "job")

    with pytest.raises(ValueError, match="Invalid job id"):
        _require_job_id("job-123", "job")


def test_require_node_id_dispatches_uuid_and_job():
    """_require_node_id routes to job-id and uuid validators."""

    _require_node_id("job", "2026Q2-ABCD", "job")
    _require_node_id("entity", str(uuid4()), "entity")

    with pytest.raises(ValueError, match="Invalid job id"):
        _require_node_id("job", "bad", "job")

    with pytest.raises(ValueError, match="Invalid entity id"):
        _require_node_id("entity", "bad", "entity")


def test_flatten_csv_value_shapes():
    """_flatten_csv_value normalizes common scalar/collection payloads."""

    assert _flatten_csv_value(None) == ""
    assert _flatten_csv_value(["a", 2]) == "a,2"
    assert _flatten_csv_value({"b": 2, "a": 1}) == '{"a": 1, "b": 2}'
    assert _flatten_csv_value(42) == "42"


def test_rows_to_csv_handles_escaping():
    """_rows_to_csv emits CSV headers and escapes commas/newlines/quotes."""

    rows = [
        {"id": "1", "name": "alpha"},
        {"id": "2", "name": 'hello, "nebula"\nworld'},
    ]
    csv_text = _rows_to_csv(rows)

    assert "id,name" in csv_text
    assert "1,alpha" in csv_text
    assert '"hello, ""nebula""' in csv_text
    assert "\nworld" in csv_text
    assert _rows_to_csv([]) == ""


def test_export_response_rows_formats():
    """_export_response_rows switches between CSV and JSON envelopes."""

    rows = [{"id": "a1"}, {"id": "a2"}]
    csv_resp = _export_response_rows(rows, "csv")
    assert csv_resp["format"] == "csv"
    assert csv_resp["count"] == 2
    assert "id" in csv_resp["content"]

    json_resp = _export_response_rows(rows, "json")
    assert json_resp == {"format": "json", "items": rows, "count": 2}


def test_taxonomy_kind_or_error():
    """Known taxonomy kinds resolve, unknown kinds fail loudly."""

    cfg = _taxonomy_kind_or_error("scopes")
    assert cfg["create"] == "taxonomy/create_scope"

    with pytest.raises(ValueError, match="Unknown taxonomy kind"):
        _taxonomy_kind_or_error("unknown-kind")


def test_validate_taxonomy_payload_support_matrix():
    """is_symmetric/value_schema only apply to supported taxonomy kinds."""

    _validate_taxonomy_payload("relationship-types", is_symmetric=True, value_schema=None)
    _validate_taxonomy_payload("log-types", is_symmetric=None, value_schema={"type": "object"})
    _validate_taxonomy_payload("scopes", is_symmetric=None, value_schema=None)

    with pytest.raises(ValueError, match="is_symmetric is only valid"):
        _validate_taxonomy_payload("scopes", is_symmetric=True, value_schema=None)
    with pytest.raises(ValueError, match="value_schema is only valid"):
        _validate_taxonomy_payload("entity-types", is_symmetric=None, value_schema={})


def test_admin_detection_and_scope_filters(mock_enums):
    """_is_admin/_scope_filter_ids/_visible_scope_names honor admin scopes."""

    admin_agent = _agent_with_scopes(mock_enums, "admin", "public")
    user_agent = _agent_with_scopes(mock_enums, "public", "private")

    assert _is_admin(admin_agent, mock_enums) is True
    assert _is_admin(user_agent, mock_enums) is False

    assert _scope_filter_ids(admin_agent, mock_enums) is None
    assert _scope_filter_ids(user_agent, mock_enums) == user_agent["scopes"]

    assert _visible_scope_names(admin_agent, mock_enums) == sorted(
        mock_enums.scopes.name_to_id.keys()
    )
    explicit = _visible_scope_names(
        user_agent,
        mock_enums,
        [mock_enums.scopes.name_to_id["private"]],
    )
    assert explicit == ["private"]


def test_require_admin_enforces_admin_scope(mock_enums):
    """_require_admin allows admin agents and rejects non-admin agents."""

    _require_admin(_agent_with_scopes(mock_enums, "admin"), mock_enums)
    with pytest.raises(ValueError, match="Admin scope required"):
        _require_admin(_agent_with_scopes(mock_enums, "public"), mock_enums)


def test_semantic_candidate_formatters_build_expected_fields():
    """Semantic candidate builders include snippet/title/text fields."""

    entity = _entity_semantic_candidate(
        {
            "id": "ent-1",
            "name": "Alpha Project",
            "type": "project",
            "tags": ["x", "y"],
        }
    )
    assert entity["kind"] == "entity"
    assert entity["title"] == "Alpha Project"
    assert "project" in entity["snippet"]
    assert "x" in entity["text"]

    context = _context_semantic_candidate(
        {
            "id": "ctx-1",
            "title": "Runbook",
            "source_type": "note",
            "content": "x" * 200,
            "tags": ["ops"],
        }
    )
    assert context["kind"] == "context"
    assert context["title"] == "Runbook"
    assert context["subtitle"] == "note"
    assert "..." in context["snippet"]


def test_has_write_scopes_matrix():
    """Write-scope checks allow public-by-default and subset writes."""

    assert _has_write_scopes([], []) is True
    assert _has_write_scopes([], [1]) is False
    assert _has_write_scopes([1, 2], [1]) is True
    assert _has_write_scopes([1], [1, 2]) is False


def test_require_job_read_and_write_matrix(mock_enums):
    """Job read/write auth checks enforce scope and ownership rules."""

    public = mock_enums.scopes.name_to_id["public"]
    private = mock_enums.scopes.name_to_id["private"]

    owner = _agent_with_scopes(mock_enums, "public")
    owner["id"] = "agent-1"
    outsider = _agent_with_scopes(mock_enums, "public")
    outsider["id"] = "agent-2"
    admin = _agent_with_scopes(mock_enums, "admin")
    admin["id"] = "agent-3"

    job = {"privacy_scope_ids": [public], "agent_id": "agent-1"}
    _require_job_read(owner, mock_enums, job)
    _require_job_write(owner, mock_enums, job)
    _require_job_write(admin, mock_enums, {"privacy_scope_ids": [private], "agent_id": "x"})

    with pytest.raises(ValueError, match="Job not in your scopes"):
        _require_job_read(
            owner, mock_enums, {"privacy_scope_ids": [private], "agent_id": "agent-1"}
        )
    with pytest.raises(ValueError, match="Access denied"):
        _require_job_write(outsider, mock_enums, job)


def test_normalize_relationship_row_scopes_properties():
    """Relationship normalization preserves shape and filters properties."""

    row = {
        "id": "rel-1",
        "properties": {"context_segments": [{"text": "ok", "scopes": ["public"]}]},
    }
    out = _normalize_relationship_row(row, ["public"])
    assert out["id"] == "rel-1"
    assert out["properties"]["context_segments"][0]["text"] == "ok"


def test_resolve_scope_ids_for_export_subset_enforced(mock_enums):
    """Export scope resolution defaults to caller scopes and enforces subset."""

    agent = _agent_with_scopes(mock_enums, "public", "private")
    caller_scope_ids = agent["scopes"]

    assert _resolve_scope_ids_for_export(agent, mock_enums, []) == caller_scope_ids
    resolved = _resolve_scope_ids_for_export(agent, mock_enums, ["public"])
    assert resolved == [mock_enums.scopes.name_to_id["public"]]

    with pytest.raises(ValueError, match="Requested scopes exceed allowed scopes"):
        _resolve_scope_ids_for_export(agent, mock_enums, ["admin"])


@pytest.mark.asyncio
async def test_get_taxonomy_row_returns_dict_or_none():
    """_get_taxonomy_row returns dict payloads and None when absent."""

    fake_pool = AsyncMock()
    fake_pool.fetchrow = AsyncMock(return_value={"id": "scope-1", "name": "public"})

    row = await _get_taxonomy_row(fake_pool, "scopes", "scope-1")
    assert row == {"id": "scope-1", "name": "public"}

    fake_pool.fetchrow = AsyncMock(return_value=None)
    row = await _get_taxonomy_row(fake_pool, "scopes", "scope-1")
    assert row is None


@pytest.mark.asyncio
async def test_taxonomy_usage_count_handles_none_and_int():
    """_taxonomy_usage_count coerces DB values and normalizes NULL to zero."""

    cfg = {"usage": "taxonomy/count_scope_usage"}
    pool = AsyncMock()

    pool.fetchval = AsyncMock(return_value=None)
    assert await _taxonomy_usage_count(pool, cfg, "scope-1") == 0

    pool.fetchval = AsyncMock(return_value="7")
    assert await _taxonomy_usage_count(pool, cfg, "scope-1") == 7


def test_visible_scope_names_uses_agent_scopes_when_scope_ids_omitted(mock_enums):
    """Non-admin fallback uses agent scopes when explicit ids are absent."""

    agent = _agent_with_scopes(mock_enums, "sensitive", "public")
    names = _visible_scope_names(agent, mock_enums)
    assert set(names) == {"sensitive", "public"}


def test_scope_filter_ids_handles_missing_scope_list(mock_enums):
    """Missing scopes normalize to an empty filter for non-admin users."""

    no_scope_agent = {"id": str(uuid4()), "scopes": None}
    assert _scope_filter_ids(no_scope_agent, mock_enums) == []


def test_require_admin_with_minimal_enums_shape():
    """_require_admin only needs a scopes section with id/name mappings."""

    admin_id = uuid4()
    scopes = SimpleNamespace(
        name_to_id={"admin": admin_id},
        id_to_name={admin_id: "admin"},
    )
    enums = SimpleNamespace(scopes=scopes)
    _require_admin({"scopes": [admin_id]}, enums)


class _ServerPoolStub:
    """Async pool stub with queued fetch/fetchrow responses."""

    def __init__(self, *, fetch_queue=None, fetchrow_map=None):
        self.fetch_queue = list(fetch_queue or [])
        self.fetchrow_map = dict(fetchrow_map or {})
        self.fetch_calls = []
        self.fetchrow_calls = []

    async def fetch(self, query, *args):
        self.fetch_calls.append((query, args))
        if self.fetch_queue:
            return self.fetch_queue.pop(0)
        return []

    async def fetchrow(self, query, *args):
        self.fetchrow_calls.append((query, args))
        key = args[0] if args else None
        return self.fetchrow_map.get(key)


@pytest.mark.asyncio
async def test_get_job_row_success_and_not_found():
    """_get_job_row returns row dict or raises clear not-found error."""

    job_id = "2026Q1-ABCD"
    pool = _ServerPoolStub(fetchrow_map={job_id: {"id": job_id, "privacy_scope_ids": []}})

    row = await _get_job_row(pool, job_id)
    assert row["id"] == job_id

    with pytest.raises(ValueError, match="not found"):
        await _get_job_row(_ServerPoolStub(), job_id)


@pytest.mark.asyncio
async def test_require_entity_write_access_matrix(mock_enums):
    """Entity write access helper enforces id validity, existence, and scopes."""

    public_id = mock_enums.scopes.name_to_id["public"]
    private_id = mock_enums.scopes.name_to_id["private"]

    admin = _agent_with_scopes(mock_enums, "admin")
    user = _agent_with_scopes(mock_enums, "public")

    # invalid UUID is rejected before DB calls
    with pytest.raises(ValueError, match="Invalid entity id"):
        await _require_entity_write_access(
            _ServerPoolStub(),
            mock_enums,
            user,
            ["not-a-uuid"],
        )

    # admin bypasses row scope checks
    await _require_entity_write_access(_ServerPoolStub(), mock_enums, admin, [])

    # non-admin missing rows => entity not found
    entity_id = str(uuid4())
    with pytest.raises(ValueError, match="Entity not found"):
        await _require_entity_write_access(
            _ServerPoolStub(fetch_queue=[[]]),
            mock_enums,
            user,
            [entity_id],
        )

    # non-admin row outside scopes => access denied
    with pytest.raises(ValueError, match="Access denied"):
        await _require_entity_write_access(
            _ServerPoolStub(
                fetch_queue=[[{"id": str(uuid4()), "privacy_scope_ids": [private_id]}]]
            ),
            mock_enums,
            user,
            [str(uuid4())],
        )

    # happy path
    await _require_entity_write_access(
        _ServerPoolStub(fetch_queue=[[{"id": str(uuid4()), "privacy_scope_ids": [public_id]}]]),
        mock_enums,
        user,
        [str(uuid4())],
    )


@pytest.mark.asyncio
async def test_has_hidden_relationships_non_file_log_branches(mock_enums):
    """Non file/log node hidden-relationship checks cover count and job branches."""

    public_id = mock_enums.scopes.name_to_id["public"]
    private_id = mock_enums.scopes.name_to_id["private"]
    user = _agent_with_scopes(mock_enums, "public")
    admin = _agent_with_scopes(mock_enums, "admin")

    # admin bypass
    assert (
        await _has_hidden_relationships(
            _ServerPoolStub(), mock_enums, admin, "entity", str(uuid4())
        )
        is False
    )

    # no rows
    assert (
        await _has_hidden_relationships(
            _ServerPoolStub(fetch_queue=[[]]),
            mock_enums,
            user,
            "entity",
            str(uuid4()),
        )
        is False
    )

    # scoped subset smaller than full set => hidden
    all_rows = [{"source_type": "entity", "target_type": "entity"}]
    assert (
        await _has_hidden_relationships(
            _ServerPoolStub(fetch_queue=[all_rows, []]),
            mock_enums,
            user,
            "entity",
            str(uuid4()),
        )
        is True
    )

    # job relationship with missing job row => hidden
    rels = [
        {
            "source_type": "job",
            "source_id": "2026Q1-ABCD",
            "target_type": "entity",
            "target_id": str(uuid4()),
        }
    ]
    assert (
        await _has_hidden_relationships(
            _ServerPoolStub(fetch_queue=[rels, rels], fetchrow_map={}),
            mock_enums,
            user,
            "entity",
            str(uuid4()),
        )
        is True
    )

    # job relationship with out-of-scope job => hidden
    assert (
        await _has_hidden_relationships(
            _ServerPoolStub(
                fetch_queue=[rels, rels],
                fetchrow_map={
                    "2026Q1-ABCD": {
                        "id": "2026Q1-ABCD",
                        "privacy_scope_ids": [private_id],
                        "agent_id": user["id"],
                    }
                },
            ),
            mock_enums,
            user,
            "entity",
            str(uuid4()),
        )
        is True
    )

    # no hidden relationships
    assert (
        await _has_hidden_relationships(
            _ServerPoolStub(
                fetch_queue=[rels, rels],
                fetchrow_map={
                    "2026Q1-ABCD": {
                        "id": "2026Q1-ABCD",
                        "privacy_scope_ids": [public_id],
                        "agent_id": user["id"],
                    }
                },
            ),
            mock_enums,
            user,
            "entity",
            str(uuid4()),
        )
        is False
    )


@pytest.mark.asyncio
async def test_has_hidden_relationships_file_log_branches(mock_enums):
    """File/log branch checks entity/context/job visibility for linked nodes."""

    public_id = mock_enums.scopes.name_to_id["public"]
    private_id = mock_enums.scopes.name_to_id["private"]
    user = _agent_with_scopes(mock_enums, "public")
    file_id = str(uuid4())

    rel = {
        "source_type": "entity",
        "source_id": str(uuid4()),
        "target_type": "file",
        "target_id": file_id,
    }

    # missing entity row => hidden
    assert (
        await _has_hidden_relationships(
            _ServerPoolStub(fetch_queue=[[rel], [rel]], fetchrow_map={}),
            mock_enums,
            user,
            "file",
            file_id,
        )
        is True
    )

    # entity row with private scope => hidden
    entity_id = rel["source_id"]
    assert (
        await _has_hidden_relationships(
            _ServerPoolStub(
                fetch_queue=[[rel], [rel]],
                fetchrow_map={entity_id: {"privacy_scope_ids": [private_id]}},
            ),
            mock_enums,
            user,
            "file",
            file_id,
        )
        is True
    )

    # fully visible entity link => not hidden
    assert (
        await _has_hidden_relationships(
            _ServerPoolStub(
                fetch_queue=[[rel], [rel]],
                fetchrow_map={entity_id: {"privacy_scope_ids": [public_id]}},
            ),
            mock_enums,
            user,
            "file",
            file_id,
        )
        is False
    )


@pytest.mark.asyncio
async def test_node_allowed_matrix(mock_enums):
    """_node_allowed enforces per-node visibility semantics."""

    public_id = mock_enums.scopes.name_to_id["public"]
    private_id = mock_enums.scopes.name_to_id["private"]
    user = _agent_with_scopes(mock_enums, "public")
    admin = _agent_with_scopes(mock_enums, "admin")

    assert await _node_allowed(_ServerPoolStub(), mock_enums, admin, "entity", str(uuid4()))

    entity_id = str(uuid4())
    assert not await _node_allowed(
        _ServerPoolStub(fetchrow_map={}),
        mock_enums,
        user,
        "entity",
        entity_id,
    )
    assert await _node_allowed(
        _ServerPoolStub(fetchrow_map={entity_id: {"privacy_scope_ids": []}}),
        mock_enums,
        user,
        "entity",
        entity_id,
    )
    assert await _node_allowed(
        _ServerPoolStub(fetchrow_map={entity_id: {"privacy_scope_ids": [public_id]}}),
        mock_enums,
        user,
        "entity",
        entity_id,
    )
    assert not await _node_allowed(
        _ServerPoolStub(fetchrow_map={entity_id: {"privacy_scope_ids": [private_id]}}),
        mock_enums,
        user,
        "entity",
        entity_id,
    )

    context_id = str(uuid4())
    assert await _node_allowed(
        _ServerPoolStub(fetchrow_map={context_id: {"id": context_id}}),
        mock_enums,
        user,
        "context",
        context_id,
    )
    assert not await _node_allowed(
        _ServerPoolStub(fetchrow_map={}),
        mock_enums,
        user,
        "context",
        context_id,
    )

    job_id = "2026Q1-ABCD"
    assert await _node_allowed(
        _ServerPoolStub(
            fetchrow_map={
                job_id: {
                    "id": job_id,
                    "privacy_scope_ids": [public_id],
                    "agent_id": user["id"],
                }
            }
        ),
        mock_enums,
        user,
        "job",
        job_id,
    )
    assert not await _node_allowed(
        _ServerPoolStub(fetchrow_map={}),
        mock_enums,
        user,
        "job",
        job_id,
    )

    # Unknown node types default allow (guarded at higher layers).
    assert await _node_allowed(_ServerPoolStub(), mock_enums, user, "protocol", "x")


@pytest.mark.asyncio
async def test_validate_relationship_node_matrix(mock_enums):
    """Relationship node validation covers entity/context/job error branches."""

    public_id = mock_enums.scopes.name_to_id["public"]
    private_id = mock_enums.scopes.name_to_id["private"]
    user = _agent_with_scopes(mock_enums, "public")

    entity_id = str(uuid4())
    context_id = str(uuid4())
    job_id = "2026Q1-ABCD"

    with pytest.raises(ValueError, match="Unsupported source type"):
        await _validate_relationship_node(
            _ServerPoolStub(),
            mock_enums,
            user,
            "protocol",
            "x",
            "Source",
        )

    with pytest.raises(ValueError, match="Source entity not found"):
        await _validate_relationship_node(
            _ServerPoolStub(fetchrow_map={}),
            mock_enums,
            user,
            "entity",
            entity_id,
            "Source",
        )

    with pytest.raises(ValueError, match="Access denied"):
        await _validate_relationship_node(
            _ServerPoolStub(fetchrow_map={entity_id: {"privacy_scope_ids": [private_id]}}),
            mock_enums,
            user,
            "entity",
            entity_id,
            "Source",
            require_write=True,
        )

    with pytest.raises(ValueError, match="Target context not found"):
        await _validate_relationship_node(
            _ServerPoolStub(fetchrow_map={}),
            mock_enums,
            user,
            "context",
            context_id,
            "Target",
        )

    with pytest.raises(ValueError, match="Access denied"):
        await _validate_relationship_node(
            _ServerPoolStub(fetchrow_map={context_id: {"privacy_scope_ids": [private_id]}}),
            mock_enums,
            user,
            "context",
            context_id,
            "Target",
            require_write=True,
        )

    with pytest.raises(ValueError, match="Target job not found"):
        await _validate_relationship_node(
            _ServerPoolStub(fetchrow_map={}),
            mock_enums,
            user,
            "job",
            job_id,
            "Target",
        )

    with pytest.raises(ValueError, match="Job not in your scopes"):
        await _validate_relationship_node(
            _ServerPoolStub(
                fetchrow_map={
                    job_id: {
                        "id": job_id,
                        "privacy_scope_ids": [private_id],
                        "agent_id": user["id"],
                    }
                }
            ),
            mock_enums,
            user,
            "job",
            job_id,
            "Target",
        )

    # happy path: writable entity + readable context + writable job
    await _validate_relationship_node(
        _ServerPoolStub(fetchrow_map={entity_id: {"privacy_scope_ids": [public_id]}}),
        mock_enums,
        user,
        "entity",
        entity_id,
        "Source",
        require_write=True,
    )
    await _validate_relationship_node(
        _ServerPoolStub(fetchrow_map={context_id: {"privacy_scope_ids": [public_id]}}),
        mock_enums,
        user,
        "context",
        context_id,
        "Target",
    )
    await _validate_relationship_node(
        _ServerPoolStub(
            fetchrow_map={
                job_id: {
                    "id": job_id,
                    "privacy_scope_ids": [public_id],
                    "agent_id": user["id"],
                }
            }
        ),
        mock_enums,
        user,
        "job",
        job_id,
        "Target",
        require_write=True,
    )
