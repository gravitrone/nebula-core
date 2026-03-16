"""Red team tests for approval queue validation gaps."""

# Standard Library
from uuid import uuid4

# Third-Party
from pydantic import ValidationError
import pytest

# Local
import nebula_mcp.helpers as helper_mod
from nebula_mcp.models import (
    BulkImportInput,
    CreateEntityInput,
    CreateJobInput,
    CreateContextInput,
    CreateRelationshipInput,
    RevertEntityInput,
)
from nebula_mcp.server import (
    bulk_import_entities,
    create_entity,
    create_context,
    create_relationship,
    revert_entity,
)


def test_create_job_rejects_unknown_status_field_before_queue():
    """create_job payload should reject unsupported status before queueing."""

    with pytest.raises(ValidationError):
        CreateJobInput.model_validate(
            {
                "title": "Queue Status Probe",
                "priority": "medium",
                "status": "todo",
            }
        )


@pytest.mark.asyncio
async def test_revert_entity_rejects_invalid_audit_id(test_entity, untrusted_mcp_context):
    """Nonexistent audit ids should be rejected before approval queue."""

    payload = RevertEntityInput(
        entity_id=str(test_entity["id"]),
        audit_id=str(uuid4()),
    )

    with pytest.raises(ValueError):
        await revert_entity(payload, untrusted_mcp_context)


@pytest.mark.asyncio
async def test_revert_entity_rejects_invalid_audit_format(test_entity, untrusted_mcp_context):
    """Invalid audit id formats should be rejected before DB access."""

    payload = RevertEntityInput(
        entity_id=str(test_entity["id"]),
        audit_id="fake-audit-id-12345",
    )

    with pytest.raises(ValueError):
        await revert_entity(payload, untrusted_mcp_context)


@pytest.mark.asyncio
async def test_create_relationship_rejects_missing_nodes(enums, test_entity, untrusted_mcp_context):
    """Relationships to missing nodes should be rejected before approval."""

    payload = CreateRelationshipInput(
        source_type="entity",
        source_id=str(test_entity["id"]),
        target_type="entity",
        target_id="00000000-0000-0000-0000-000000000001",
        relationship_type="related-to",
        properties={"note": "bad target"},
    )

    with pytest.raises(ValueError):
        await create_relationship(payload, untrusted_mcp_context)


@pytest.mark.asyncio
async def test_create_entity_allows_neutral_source_path(mock_mcp_context):
    """Entities accept generic source_path strings in neutral mode."""

    payload = CreateEntityInput(
        name="Path Traversal",
        type="person",
        status="active",
        scopes=["public"],
        tags=["test"],
        source_path="../../../../etc/passwd",
    )
    assert payload.source_path == "../../../../etc/passwd"


@pytest.mark.asyncio
async def test_create_context_rejects_javascript_url(mock_mcp_context):
    """Context URLs should be restricted to http and https."""

    with pytest.raises(ValidationError):
        CreateContextInput(
            title="Bad URL",
            url="javascript:alert('xss')",
            source_type="article",
            content="x",
            status="active",
            scopes=["public"],
            tags=["test"],
        )


@pytest.mark.asyncio
async def test_create_entity_rejects_proto_pollution(mock_mcp_context):
    """Entities should reject unexpected metadata payloads."""

    with pytest.raises(ValidationError):
        CreateEntityInput(
            name="Proto",
            type="person",
            status="active",
            scopes=["public"],
            tags=["test"],
            metadata={"__proto__": {"isAdmin": True}},
        )


@pytest.mark.asyncio
async def test_bulk_import_requires_per_item_approval(db_pool, untrusted_mcp_context):
    """Bulk imports should not collapse into a single approval."""

    payload = BulkImportInput(
        items=[
            {
                "name": "Alpha",
                "type": "person",
                "status": "active",
                "scopes": ["public"],
                "tags": ["test"],
            },
            {
                "name": "Beta",
                "type": "person",
                "status": "active",
                "scopes": ["public"],
                "tags": ["test"],
            },
        ]
    )

    await bulk_import_entities(payload, untrusted_mcp_context)

    count = await db_pool.fetchval("SELECT COUNT(*) FROM approval_requests")
    assert count >= 2


@pytest.mark.asyncio
async def test_approval_queue_rate_limit(db_pool, untrusted_mcp_context, monkeypatch):
    """Approval queue should cap pending approvals per agent."""

    monkeypatch.setattr(helper_mod, "MAX_PENDING_APPROVALS", 10)
    start_count = await db_pool.fetchval("SELECT COUNT(*) FROM approval_requests")
    rejected = False
    for i in range(20):
        try:
            payload = CreateEntityInput(
                name=f"Rate Limit Probe {i}",
                type="person",
                status="active",
                scopes=["public"],
                tags=["redteam"],
            )
            await create_entity(payload, untrusted_mcp_context)
        except ValueError as exc:
            assert "Approval queue limit reached" in str(exc)
            rejected = True
            break

    end_count = await db_pool.fetchval("SELECT COUNT(*) FROM approval_requests")
    assert rejected is True
    assert end_count - start_count <= 10
