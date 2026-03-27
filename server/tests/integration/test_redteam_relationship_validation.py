"""Red team tests for relationship validation on non-entity nodes."""

# Standard Library
from uuid import uuid4

# Third-Party
import pytest

# Local
from nebula_mcp.models import CreateRelationshipInput
from nebula_mcp.server import create_relationship


@pytest.mark.asyncio
async def test_relationship_rejects_missing_job(db_pool, enums, mock_mcp_context):
    """Relationships should reject missing job nodes before approval."""

    status_id = enums.statuses.name_to_id["active"]
    type_id = enums.entity_types.name_to_id["person"]
    scope_ids = [enums.scopes.name_to_id["public"]]

    entity = await db_pool.fetchrow(
        """
        INSERT INTO entities (name, type_id, status_id, privacy_scope_ids, tags)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING *
        """,
        "Public Target",
        type_id,
        status_id,
        scope_ids,
        ["test"],
    )

    payload = CreateRelationshipInput(
        source_type="job",
        source_id=str(uuid4()),
        target_type="entity",
        target_id=str(entity["id"]),
        relationship_type="related-to",
        properties={"note": "missing job"},
    )

    with pytest.raises(ValueError):
        await create_relationship(payload, mock_mcp_context)


@pytest.mark.asyncio
async def test_relationship_rejects_invalid_uuid(db_pool, enums, mock_mcp_context):
    """Relationships should reject malformed UUIDs cleanly."""

    status_id = enums.statuses.name_to_id["active"]
    type_id = enums.entity_types.name_to_id["person"]
    scope_ids = [enums.scopes.name_to_id["public"]]

    entity = await db_pool.fetchrow(
        """
        INSERT INTO entities (name, type_id, status_id, privacy_scope_ids, tags)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING *
        """,
        "Valid Target",
        type_id,
        status_id,
        scope_ids,
        ["test"],
    )

    payload = CreateRelationshipInput(
        source_type="entity",
        source_id="not-a-uuid",
        target_type="entity",
        target_id=str(entity["id"]),
        relationship_type="related-to",
        properties={"note": "bad uuid"},
    )

    with pytest.raises(ValueError):
        await create_relationship(payload, mock_mcp_context)
