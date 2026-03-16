"""Unit tests for schema contract helpers."""

from uuid import uuid4

import pytest

from nebula_mcp import schema

pytestmark = pytest.mark.unit


def test_stringify_ids_uses_default_and_custom_id_key() -> None:
    """_stringify_ids should convert IDs without mutating source rows."""

    row_id = uuid4()
    alt_id = uuid4()
    rows = [{"id": row_id, "name": "public"}, {"id": None, "name": "private"}]
    alt_rows = [{"scope_id": alt_id, "name": "admin"}]

    result = schema._stringify_ids(rows)
    alt_result = schema._stringify_ids(alt_rows, id_key="scope_id")

    assert result == [
        {"id": str(row_id), "name": "public"},
        {"id": None, "name": "private"},
    ]
    assert alt_result == [{"scope_id": str(alt_id), "name": "admin"}]
    # Source rows remain untouched.
    assert rows[0]["id"] == row_id
    assert alt_rows[0]["scope_id"] == alt_id


@pytest.mark.asyncio
async def test_load_schema_contract_builds_taxonomy_and_constraints(mock_pool) -> None:
    """load_schema_contract should shape taxonomy + constraints predictably."""

    scope_id = uuid4()
    entity_type_id = uuid4()
    relationship_type_id = uuid4()
    log_type_id = uuid4()
    status_id = uuid4()

    mock_pool.fetch.side_effect = [
        [{"id": scope_id, "name": "public"}],
        [{"id": entity_type_id, "name": "project"}],
        [{"id": relationship_type_id, "name": "related-to"}],
        [{"id": log_type_id, "name": "note"}],
        [{"id": status_id, "name": "active"}],
    ]

    contract = await schema.load_schema_contract(mock_pool)

    assert contract["taxonomy"]["scopes"] == [{"id": str(scope_id), "name": "public"}]
    assert contract["taxonomy"]["entity_types"] == [{"id": str(entity_type_id), "name": "project"}]
    assert contract["taxonomy"]["relationship_types"] == [
        {"id": str(relationship_type_id), "name": "related-to"}
    ]
    assert contract["taxonomy"]["log_types"] == [{"id": str(log_type_id), "name": "note"}]
    assert contract["statuses"] == [{"id": str(status_id), "name": "active"}]

    assert contract["constraints"]["jobs"]["priority"] == schema.JOB_PRIORITY_VALUES
    assert contract["constraints"]["approval_requests"]["status"] == (schema.APPROVAL_STATUS_VALUES)
    assert contract["constraints"]["relationships"]["node_types"] == (
        schema.RELATIONSHIP_NODE_TYPE_VALUES
    )
    assert contract["constraints"]["audit_log"]["action"] == schema.AUDIT_ACTION_VALUES
    assert contract["constraints"]["audit_log"]["actor_type"] == (schema.AUDIT_ACTOR_TYPE_VALUES)
    assert mock_pool.fetch.await_count == 5


def test_load_export_schema_contract_includes_expected_resources() -> None:
    """Export schema contract should lock supported resources and formats."""

    contract = schema.load_export_schema_contract()

    assert contract["$schema"] == "https://json-schema.org/draft/2020-12/schema"
    assert contract["version"] == "1.0.0"

    resources = contract["resources"]
    assert set(resources.keys()) == {
        "entities",
        "context",
        "relationships",
        "jobs",
        "snapshot",
    }
    assert resources["snapshot"]["formats"] == ["json"]
    assert "csv" in resources["entities"]["formats"]
    assert "csv" in resources["context"]["formats"]
    assert "csv" in resources["relationships"]["formats"]
    assert "csv" in resources["jobs"]["formats"]
