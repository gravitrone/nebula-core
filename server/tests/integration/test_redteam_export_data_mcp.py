"""Redteam tests for MCP export_data scope and payload behavior."""

# Standard Library
import json
from unittest.mock import MagicMock

# Third-Party
import pytest

# Local
from nebula_mcp.models import ExportDataInput
from nebula_mcp.server import export_data


def _ctx(pool, enums, agent):
    """Build MCP context for export_data tests."""

    ctx = MagicMock()
    ctx.request_context.lifespan_context = {
        "pool": pool,
        "enums": enums,
        "agent": agent,
    }
    return ctx


async def _make_agent(db_pool, enums, *, name: str, scopes: list[str]) -> dict:
    """Create an active agent row and return dict payload."""

    row = await db_pool.fetchrow(
        """
        INSERT INTO agents (name, description, scopes, requires_approval, status_id)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING *
        """,
        name,
        f"{name} export agent",
        [enums.scopes.name_to_id[s] for s in scopes],
        False,
        enums.statuses.name_to_id["active"],
    )
    return dict(row)


async def _make_entity(db_pool, enums, *, name: str) -> dict:
    """Create a public entity row and return dict payload."""

    row = await db_pool.fetchrow(
        """
        INSERT INTO entities (name, type_id, status_id, privacy_scope_ids, tags)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING *
        """,
        name,
        enums.entity_types.name_to_id["person"],
        enums.statuses.name_to_id["active"],
        [enums.scopes.name_to_id["public"]],
        [],
    )
    return dict(row)


async def _make_job(db_pool, enums, *, title: str, agent_id: str, scopes: list[str]) -> dict:
    """Create a job row with explicit scopes and return dict payload."""

    row = await db_pool.fetchrow(
        """
        INSERT INTO jobs (title, status_id, agent_id, privacy_scope_ids)
        VALUES ($1, $2, $3, $4)
        RETURNING *
        """,
        title,
        enums.statuses.name_to_id["active"],
        agent_id,
        [enums.scopes.name_to_id[s] for s in scopes],
    )
    return dict(row)


async def _make_relationship_with_segments(
    db_pool, enums, *, source_id: str, target_id: str
) -> dict:
    """Create relationship row with text notes."""

    row = await db_pool.fetchrow(
        """
        INSERT INTO relationships (source_type, source_id, target_type, target_id, type_id, status_id, notes)
        VALUES ('entity', $1, 'entity', $2, $3, $4, $5)
        RETURNING *
        """,
        source_id,
        target_id,
        enums.relationship_types.name_to_id["related-to"],
        enums.statuses.name_to_id["active"],
        "mixed-scope-export note",
    )
    return dict(row)


async def _make_job_relationship(
    db_pool,
    enums,
    *,
    source_type: str,
    source_id: str,
    target_type: str,
    target_id: str,
) -> dict:
    """Create relationship row linking job and non-job nodes."""

    row = await db_pool.fetchrow(
        """
        INSERT INTO relationships (source_type, source_id, target_type, target_id, type_id, status_id, notes)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
        RETURNING *
        """,
        source_type,
        source_id,
        target_type,
        target_id,
        enums.relationship_types.name_to_id["related-to"],
        enums.statuses.name_to_id["active"],
        json.dumps({"note": "job-rel"}),
    )
    return dict(row)


def _as_dict(value):
    """Normalize relationship properties payload to a dictionary."""

    if isinstance(value, dict):
        return value
    if isinstance(value, str):
        try:
            parsed = json.loads(value)
        except json.JSONDecodeError:
            return {}
        return parsed if isinstance(parsed, dict) else {}
    return {}


@pytest.mark.asyncio
async def test_mcp_export_relationships_filters_properties_context_segments(db_pool, enums):
    """MCP relationships export should hide sensitive segments from public-only agents."""

    viewer = await _make_agent(db_pool, enums, name="mcp-export-viewer", scopes=["public"])
    source = await _make_entity(db_pool, enums, name="Export Src")
    target = await _make_entity(db_pool, enums, name="Export Dst")
    rel = await _make_relationship_with_segments(
        db_pool,
        enums,
        source_id=str(source["id"]),
        target_id=str(target["id"]),
    )

    result = await export_data(
        ExportDataInput(
            resource="relationships",
            format="json",
            params={"status_category": "active", "limit": 50},
        ),
        _ctx(db_pool, enums, viewer),
    )
    rows = result["items"]
    row = next((item for item in rows if str(item["id"]) == str(rel["id"])), None)
    assert row is not None
    assert row["notes"] is not None


@pytest.mark.asyncio
async def test_mcp_export_relationships_properties_payload_is_object(db_pool, enums):
    """MCP relationships export should return properties as structured object."""

    viewer = await _make_agent(db_pool, enums, name="mcp-export-viewer-type", scopes=["public"])
    source = await _make_entity(db_pool, enums, name="Export Type Src")
    target = await _make_entity(db_pool, enums, name="Export Type Dst")
    rel = await _make_relationship_with_segments(
        db_pool,
        enums,
        source_id=str(source["id"]),
        target_id=str(target["id"]),
    )

    result = await export_data(
        ExportDataInput(
            resource="relationships",
            format="json",
            params={"status_category": "active", "limit": 50},
        ),
        _ctx(db_pool, enums, viewer),
    )
    row = next((item for item in result["items"] if str(item["id"]) == str(rel["id"])), None)
    assert row is not None
    assert isinstance(row.get("notes"), str)


@pytest.mark.asyncio
async def test_mcp_export_snapshot_filters_relationship_properties_context_segments(db_pool, enums):
    """Snapshot export should hide sensitive relationship segments for public agents."""

    viewer = await _make_agent(db_pool, enums, name="mcp-snapshot-viewer", scopes=["public"])
    source = await _make_entity(db_pool, enums, name="Snapshot Src")
    target = await _make_entity(db_pool, enums, name="Snapshot Dst")
    rel = await _make_relationship_with_segments(
        db_pool,
        enums,
        source_id=str(source["id"]),
        target_id=str(target["id"]),
    )

    result = await export_data(
        ExportDataInput(resource="snapshot", format="json", params={"limit": 50}),
        _ctx(db_pool, enums, viewer),
    )
    rows = result["items"]["relationships"]
    row = next((item for item in rows if str(item["id"]) == str(rel["id"])), None)
    assert row is not None
    assert row["notes"] is not None


@pytest.mark.asyncio
async def test_mcp_export_snapshot_relationship_properties_payload_is_object(db_pool, enums):
    """Snapshot export should return relationship properties as structured object."""

    viewer = await _make_agent(db_pool, enums, name="mcp-snapshot-viewer-type", scopes=["public"])
    source = await _make_entity(db_pool, enums, name="Snapshot Type Src")
    target = await _make_entity(db_pool, enums, name="Snapshot Type Dst")
    rel = await _make_relationship_with_segments(
        db_pool,
        enums,
        source_id=str(source["id"]),
        target_id=str(target["id"]),
    )

    result = await export_data(
        ExportDataInput(resource="snapshot", format="json", params={"limit": 50}),
        _ctx(db_pool, enums, viewer),
    )
    row = next(
        (item for item in result["items"]["relationships"] if str(item["id"]) == str(rel["id"])),
        None,
    )
    assert row is not None
    assert isinstance(row.get("notes"), str)


@pytest.mark.asyncio
async def test_mcp_export_relationships_hides_out_of_scope_job_links(db_pool, enums):
    """Relationships export should not include links to private jobs."""

    owner = await _make_agent(db_pool, enums, name="mcp-export-job-owner", scopes=["public"])
    viewer = await _make_agent(db_pool, enums, name="mcp-export-job-viewer", scopes=["public"])
    entity = await _make_entity(db_pool, enums, name="MCP Export Job Entity")
    private_job = await _make_job(
        db_pool,
        enums,
        title="MCP Private Export Job",
        agent_id=owner["id"],
        scopes=["private"],
    )
    rel = await _make_job_relationship(
        db_pool,
        enums,
        source_type="entity",
        source_id=str(entity["id"]),
        target_type="job",
        target_id=private_job["id"],
    )

    result = await export_data(
        ExportDataInput(
            resource="relationships",
            format="json",
            params={"status_category": "active", "limit": 50},
        ),
        _ctx(db_pool, enums, viewer),
    )
    ids = {str(item["id"]) for item in result["items"]}
    assert str(rel["id"]) not in ids


@pytest.mark.asyncio
async def test_mcp_export_snapshot_hides_out_of_scope_job_links(db_pool, enums):
    """Snapshot export should not include links to private jobs."""

    owner = await _make_agent(db_pool, enums, name="mcp-snapshot-job-owner", scopes=["public"])
    viewer = await _make_agent(db_pool, enums, name="mcp-snapshot-job-viewer", scopes=["public"])
    entity = await _make_entity(db_pool, enums, name="MCP Snapshot Job Entity")
    private_job = await _make_job(
        db_pool,
        enums,
        title="MCP Private Snapshot Job",
        agent_id=owner["id"],
        scopes=["private"],
    )
    rel = await _make_job_relationship(
        db_pool,
        enums,
        source_type="job",
        source_id=private_job["id"],
        target_type="entity",
        target_id=str(entity["id"]),
    )

    result = await export_data(
        ExportDataInput(resource="snapshot", format="json", params={"limit": 50}),
        _ctx(db_pool, enums, viewer),
    )
    ids = {str(item["id"]) for item in result["items"]["relationships"]}
    assert str(rel["id"]) not in ids
