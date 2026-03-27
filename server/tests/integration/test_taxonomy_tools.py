"""Integration tests for MCP taxonomy tools."""

# Standard Library

# Third-Party
import pytest

from nebula_mcp.models import (
    CreateTaxonomyInput,
    ListTaxonomyInput,
    ToggleTaxonomyInput,
    UpdateTaxonomyInput,
)
from nebula_mcp.server import (
    activate_taxonomy,
    archive_taxonomy,
    create_taxonomy,
    list_taxonomy,
    update_taxonomy,
)

pytestmark = pytest.mark.integration


async def test_list_taxonomy_requires_admin(untrusted_mcp_context):
    """Non-admin agents cannot list taxonomy rows."""

    with pytest.raises(ValueError, match="Admin scope required"):
        await list_taxonomy(ListTaxonomyInput(kind="scopes"), untrusted_mcp_context)


async def test_scope_taxonomy_lifecycle_roundtrip(mock_mcp_context):
    """Admin should create, update, archive, and activate scope taxonomy rows."""

    created = await create_taxonomy(
        CreateTaxonomyInput(
            kind="scopes",
            name="team-alpha",
            description="Team private scope",
            metadata={"owner": "qa"},
        ),
        mock_mcp_context,
    )
    assert created["name"] == "team-alpha"
    assert created["is_active"] is True

    rows = await list_taxonomy(
        ListTaxonomyInput(kind="scopes", search="team-alpha"),
        mock_mcp_context,
    )
    assert any(r["id"] == created["id"] for r in rows)

    updated = await update_taxonomy(
        UpdateTaxonomyInput(
            kind="scopes",
            item_id=str(created["id"]),
            name="team-alpha-v2",
            description="Updated scope",
            metadata={"owner": "ops"},
        ),
        mock_mcp_context,
    )
    assert updated["name"] == "team-alpha-v2"

    archived = await archive_taxonomy(
        ToggleTaxonomyInput(kind="scopes", item_id=str(created["id"])),
        mock_mcp_context,
    )
    assert archived["is_active"] is False

    activated = await activate_taxonomy(
        ToggleTaxonomyInput(kind="scopes", item_id=str(created["id"])),
        mock_mcp_context,
    )
    assert activated["is_active"] is True


async def test_archive_taxonomy_conflict_when_in_use(mock_mcp_context, db_pool, enums):
    """Archiving an in-use taxonomy row should fail with a conflict message."""

    created = await create_taxonomy(
        CreateTaxonomyInput(
            kind="entity-types",
            name="dataset",
            description="Temporary dataset type",
            metadata={},
        ),
        mock_mcp_context,
    )

    status_id = enums.statuses.name_to_id["active"]
    scope_ids = [enums.scopes.name_to_id["public"]]
    await db_pool.fetchrow(
        """
        INSERT INTO entities (name, type_id, status_id, privacy_scope_ids, tags)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING id
        """,
        "Taxonomy Usage Entity",
        created["id"],
        status_id,
        scope_ids,
        ["test"],
    )

    with pytest.raises(ValueError, match="referenced"):
        await archive_taxonomy(
            ToggleTaxonomyInput(kind="entity-types", item_id=str(created["id"])),
            mock_mcp_context,
        )
