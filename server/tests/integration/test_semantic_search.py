"""Integration tests for MCP semantic search tool."""

# Standard Library
import json

# Third-Party
import pytest
from pydantic import ValidationError

from nebula_mcp.models import SemanticSearchInput
from nebula_mcp.server import semantic_search

pytestmark = pytest.mark.integration


async def _insert_entity(db_pool, enums, *, name: str, scopes: list[str], summary: str):
    """Handle insert entity.

    Args:
        db_pool: Input parameter for _insert_entity.
        enums: Input parameter for _insert_entity.
        name: Input parameter for _insert_entity.
        scopes: Input parameter for _insert_entity.
        summary: Input parameter for _insert_entity.

    Returns:
        Result value from the operation.
    """

    status_id = enums.statuses.name_to_id["active"]
    type_id = enums.entity_types.name_to_id["project"]
    scope_ids = [enums.scopes.name_to_id[s] for s in scopes]
    row = await db_pool.fetchrow(
        """
        INSERT INTO entities (name, type_id, status_id, privacy_scope_ids, tags)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING id
        """,
        name,
        type_id,
        status_id,
        scope_ids,
        ["semantic"],
    )
    return str(row["id"])


async def _insert_context(
    db_pool, enums, *, title: str, scopes: list[str], content: str
):
    """Handle insert context.

    Args:
        db_pool: Input parameter for _insert_context.
        enums: Input parameter for _insert_context.
        title: Input parameter for _insert_context.
        scopes: Input parameter for _insert_context.
        content: Input parameter for _insert_context.

    Returns:
        Result value from the operation.
    """

    status_id = enums.statuses.name_to_id["active"]
    scope_ids = [enums.scopes.name_to_id[s] for s in scopes]
    row = await db_pool.fetchrow(
        """
        INSERT INTO context_items (title, source_type, content, status_id, privacy_scope_ids, tags)
        VALUES ($1, $2, $3, $4, $5, $6)
        RETURNING id
        """,
        title,
        "note",
        content,
        status_id,
        scope_ids,
        ["semantic"],
    )
    return str(row["id"])


async def test_semantic_search_tool_happy_path(mock_mcp_context, db_pool, enums):
    """MCP semantic search should return ranked entity and context matches."""

    entity_id = await _insert_entity(
        db_pool,
        enums,
        name="Agent Context Mesh",
        scopes=["public"],
        summary="Shared memory system for agents",
    )
    context_id = await _insert_context(
        db_pool,
        enums,
        title="Memory Retrieval Guide",
        scopes=["public"],
        content="Semantic retrieval improves prompt context quality.",
    )

    rows = await semantic_search(
        SemanticSearchInput(query="agent memory retrieval"),
        mock_mcp_context,
    )
    ids = {row["id"] for row in rows}
    assert entity_id in ids
    assert context_id in ids


async def test_semantic_search_tool_scope_enforced(
    untrusted_mcp_context, db_pool, enums
):
    """MCP semantic search should not return nodes outside agent scopes."""

    public_id = await _insert_entity(
        db_pool,
        enums,
        name="Public Context Graph",
        scopes=["public"],
        summary="Public graph data",
    )
    await _insert_entity(
        db_pool,
        enums,
        name="Sensitive Context Graph",
        scopes=["sensitive"],
        summary="Sensitive graph data",
    )

    rows = await semantic_search(
        SemanticSearchInput(query="context graph", kinds=["entity"]),
        untrusted_mcp_context,
    )
    ids = {row["id"] for row in rows}
    assert ids == {public_id}


def test_semantic_search_tool_rejects_invalid_query():
    """SemanticSearchInput should reject blank query values."""

    with pytest.raises(ValidationError):
        SemanticSearchInput(query=" ")
