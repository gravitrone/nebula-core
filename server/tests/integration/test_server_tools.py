"""Integration tests for MCP server tool functions."""

# Standard Library

import pytest
from pydantic import ValidationError

from nebula_mcp.models import (
    AttachFileInput,
    BulkImportInput,
    CreateEntityInput,
    CreateFileInput,
    CreateJobInput,
    CreateContextInput,
    CreateRelationshipInput,
    CreateSubtaskInput,
    GetAgentInfoInput,
    GetEntityInput,
    GetFileInput,
    GetJobInput,
    GetRelationshipsInput,
    GraphNeighborsInput,
    GraphShortestPathInput,
    LinkContextInput,
    ListAgentsInput,
    QueryEntitiesInput,
    QueryFilesInput,
    QueryJobsInput,
    QueryContextInput,
    QueryRelationshipsInput,
    SearchEntitiesByMetadataInput,
    UpdateEntityInput,
    UpdateJobInput,
    UpdateJobStatusInput,
)
from nebula_mcp.server import (
    attach_file_to_entity,
    attach_file_to_job,
    bulk_import_entities,
    create_entity,
    create_file,
    create_job,
    create_context,
    create_relationship,
    create_subtask,
    get_agent_info,
    get_entity,
    get_file,
    get_job,
    get_pending_approvals,
    get_relationships,
    graph_neighbors,
    graph_shortest_path,
    link_context_to_entity,
    list_active_protocols,
    list_agents,
    list_files,
    query_entities,
    query_jobs,
    query_context,
    query_relationships,
    reload_enums,
    search_entities_by_metadata,
    update_entity,
    update_job,
    update_job_status,
)

pytestmark = pytest.mark.integration


# --- Entity Tools ---


async def test_create_entity_trusted(mock_mcp_context, test_agent):
    """A trusted agent should create an entity directly."""

    payload = CreateEntityInput(
        name="Server Tool Entity",
        type="project",
        status="active",
        scopes=["public"],
        tags=["test"],
        metadata={"met_at": "gym"},
    )

    result = await create_entity(payload, mock_mcp_context)
    assert "id" in result
    assert isinstance(result["metadata"], dict)
    assert result["metadata"]["met_at"] == "gym"


async def test_create_entity_untrusted_returns_approval(
    untrusted_mcp_context, untrusted_agent
):
    """An untrusted agent should receive an approval_required response."""

    payload = CreateEntityInput(
        name="Untrusted Entity",
        type="project",
        status="active",
        scopes=["public"],
    )

    result = await create_entity(payload, untrusted_mcp_context)
    assert result["status"] == "approval_required"


async def test_create_entity_respects_runtime_trust_toggle(
    db_pool, untrusted_mcp_context, untrusted_agent
):
    """Runtime trust toggles should apply without restarting MCP server."""

    await db_pool.execute(
        """
        UPDATE agents
        SET requires_approval = FALSE, updated_at = NOW()
        WHERE id = $1::uuid
        """,
        str(untrusted_agent["id"]),
    )

    payload = CreateEntityInput(
        name="Now Trusted Entity",
        type="project",
        status="active",
        scopes=["public"],
    )

    result = await create_entity(payload, untrusted_mcp_context)
    assert "id" in result
    assert result.get("status") != "approval_required"


async def test_create_entity_respects_runtime_trust_toggle_to_untrusted(
    db_pool, mock_mcp_context, test_agent
):
    """Runtime trust toggles to untrusted should queue writes immediately."""

    await db_pool.execute(
        """
        UPDATE agents
        SET requires_approval = TRUE, updated_at = NOW()
        WHERE id = $1::uuid
        """,
        str(test_agent["id"]),
    )

    payload = CreateEntityInput(
        name="Now Untrusted Entity",
        type="project",
        status="active",
        scopes=["public"],
    )

    result = await create_entity(payload, mock_mcp_context)
    assert result["status"] == "approval_required"


async def test_get_entity_success(mock_mcp_context, test_agent, test_entity):
    """Getting an existing entity by ID should return it."""

    payload = GetEntityInput(
        entity_id=str(test_entity["id"]),
    )

    result = await get_entity(payload, mock_mcp_context)
    assert result["name"] == "Test Person"


async def test_get_entity_not_found(mock_mcp_context, test_agent):
    """Getting a nonexistent entity should raise ValueError."""

    payload = GetEntityInput(
        entity_id="00000000-0000-0000-0000-000000000000",
    )

    with pytest.raises(ValueError, match="not found"):
        await get_entity(payload, mock_mcp_context)


async def test_get_entity_access_denied(db_pool, enums):
    """An agent with only public scope should not access a private entity."""

    from unittest.mock import MagicMock

    # Create a public-only agent
    status_id = enums.statuses.name_to_id["active"]
    public_scope_id = enums.scopes.name_to_id["public"]

    health_agent = await db_pool.fetchrow(
        """
        INSERT INTO agents (name, description, scopes, requires_approval, status_id)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING *
        """,
        "public-agent",
        "Public-only agent",
        [public_scope_id],
        False,
        status_id,
    )

    # Create a private-scoped entity
    type_id = enums.entity_types.name_to_id["project"]
    private_scope_id = enums.scopes.name_to_id["private"]

    entity = await db_pool.fetchrow(
        """
        INSERT INTO entities (name, type_id, status_id, privacy_scope_ids, tags, metadata)
        VALUES ($1, $2, $3, $4, $5, $6::jsonb)
        RETURNING *
        """,
        "Private Project",
        type_id,
        status_id,
        [private_scope_id],
        ["private"],
        "{}",
    )

    # build context with the public-only agent
    ctx = MagicMock()
    ctx.request_context.lifespan_context = {
        "pool": db_pool,
        "enums": enums,
        "agent": dict(health_agent),
    }

    payload = GetEntityInput(
        entity_id=str(entity["id"]),
    )

    with pytest.raises(ValueError, match="Access denied"):
        await get_entity(payload, ctx)


async def test_query_entities(mock_mcp_context, test_agent, test_entity):
    """Querying entities should return a list containing the test entity."""

    payload = QueryEntitiesInput()

    result = await query_entities(payload, mock_mcp_context)
    assert isinstance(result, list)
    assert len(result) >= 1


async def test_update_entity(mock_mcp_context, test_agent, test_entity):
    """Updating an entity status via the server tool should succeed."""

    payload = UpdateEntityInput(
        entity_id=str(test_entity["id"]),
        status="on-hold",
        status_reason="Test pause",
    )

    result = await update_entity(payload, mock_mcp_context)
    assert "id" in result


async def test_search_entities_by_metadata(mock_mcp_context, test_agent, test_entity):
    """Searching by metadata containment should return matching entities."""

    payload = SearchEntitiesByMetadataInput(
        metadata_query={"first_name": "Test"},
    )

    result = await search_entities_by_metadata(payload, mock_mcp_context)
    assert isinstance(result, list)


async def test_bulk_import_entities(mock_mcp_context, test_agent):
    """Bulk import entities should create items."""

    payload = BulkImportInput(
        format="json",
        items=[
            {
                "name": "Bulk Import MCP",
                "type": "project",
                "status": "active",
                "scopes": ["public"],
            }
        ],
    )

    result = await bulk_import_entities(payload, mock_mcp_context)
    assert result["created"] == 1


# --- Context Tools ---


async def test_create_context(mock_mcp_context, test_agent):
    """Creating a context item via the server tool should succeed."""

    payload = CreateContextInput(
        title="Server Context",
        source_type="article",
        scopes=["public"],
    )

    result = await create_context(payload, mock_mcp_context)
    assert "id" in result


async def test_query_context(mock_mcp_context, test_agent):
    """Querying context items should return a list."""

    payload = QueryContextInput()

    result = await query_context(payload, mock_mcp_context)
    assert isinstance(result, list)


async def test_link_context_to_entity(
    mock_mcp_context, test_agent, test_entity, enums, db_pool
):
    """Linking context to an entity should create a relationship."""

    # Create a context item first
    ki_payload = CreateContextInput(
        title="Linkable Context",
        source_type="note",
        scopes=["public"],
    )
    ki = await create_context(ki_payload, mock_mcp_context)

    payload = LinkContextInput(
        context_id=str(ki["id"]),
        entity_id=str(test_entity["id"]),
        relationship_type="about",
    )

    result = await link_context_to_entity(payload, mock_mcp_context)
    assert "id" in result


# --- Relationship Tools ---


async def test_create_relationship(
    mock_mcp_context, test_agent, test_entity, db_pool, enums
):
    """Creating a relationship between two entities via the server tool should succeed."""

    # Create a second entity
    status_id = enums.statuses.name_to_id["active"]
    type_id = enums.entity_types.name_to_id["project"]
    scope_ids = [enums.scopes.name_to_id["public"]]

    target = await db_pool.fetchrow(
        """
        INSERT INTO entities (name, type_id, status_id, privacy_scope_ids, tags, metadata)
        VALUES ($1, $2, $3, $4, $5, $6::jsonb)
        RETURNING *
        """,
        "Rel Target",
        type_id,
        status_id,
        scope_ids,
        ["test"],
        "{}",
    )

    payload = CreateRelationshipInput(
        source_type="entity",
        source_id=str(test_entity["id"]),
        target_type="entity",
        target_id=str(target["id"]),
        relationship_type="depends-on",
    )

    result = await create_relationship(payload, mock_mcp_context)
    assert "id" in result


async def test_create_relationship_accepts_job_target_id(
    mock_mcp_context, test_agent, test_entity
):
    """Relationship creation should accept canonical job IDs on job nodes."""

    job = await create_job(
        CreateJobInput(
            title="Relationship Job Target",
            priority="medium",
        ),
        mock_mcp_context,
    )

    result = await create_relationship(
        CreateRelationshipInput(
            source_type="entity",
            source_id=str(test_entity["id"]),
            target_type="job",
            target_id=job["id"],
            relationship_type="related-to",
        ),
        mock_mcp_context,
    )
    assert result["target_type"] == "job"
    assert result["target_id"] == job["id"]


async def test_get_relationships(mock_mcp_context, test_agent, test_entity):
    """Getting relationships for an entity should return a list."""

    payload = GetRelationshipsInput(
        source_type="entity",
        source_id=str(test_entity["id"]),
    )

    result = await get_relationships(payload, mock_mcp_context)
    assert isinstance(result, list)


async def test_query_relationships(mock_mcp_context, test_agent):
    """Querying relationships with no filters should return a list."""

    payload = QueryRelationshipsInput()

    result = await query_relationships(payload, mock_mcp_context)
    assert isinstance(result, list)


# --- Job Tools ---


async def test_create_job(mock_mcp_context, test_agent):
    """Creating a job via the server tool should succeed."""

    payload = CreateJobInput(
        title="Server Job",
        priority="medium",
    )

    result = await create_job(payload, mock_mcp_context)
    assert "id" in result


async def test_create_job_accepts_iso_due_at(mock_mcp_context, test_agent):
    """MCP create_job should accept ISO due_at strings."""

    payload = CreateJobInput(
        title="Server Timed Job",
        priority="medium",
        due_at="2026-02-18T18:00:00Z",
    )

    result = await create_job(payload, mock_mcp_context)
    assert "id" in result
    assert result["due_at"] is not None


def test_create_job_rejects_unknown_status_field():
    """Unsupported create_job status input should be rejected at validation."""

    with pytest.raises(ValidationError):
        CreateJobInput.model_validate(
            {
                "title": "Unknown Status Probe",
                "priority": "medium",
                "status": "todo",
            }
        )


async def test_get_job(mock_mcp_context, test_agent):
    """Getting a job by ID should return the job row."""

    job_payload = CreateJobInput(
        title="Fetchable Job",
        priority="low",
    )
    job = await create_job(job_payload, mock_mcp_context)

    payload = GetJobInput(
        job_id=job["id"],
    )

    result = await get_job(payload, mock_mcp_context)
    assert result["title"] == "Fetchable Job"


async def test_query_jobs(mock_mcp_context, test_agent):
    """Querying jobs should return a list."""

    payload = QueryJobsInput()

    result = await query_jobs(payload, mock_mcp_context)
    assert isinstance(result, list)


async def test_query_jobs_accepts_iso_due_filters(mock_mcp_context, test_agent):
    """MCP query_jobs should parse ISO due filters."""

    payload = QueryJobsInput(due_before="2026-12-31T00:00:00Z")
    result = await query_jobs(payload, mock_mcp_context)
    assert isinstance(result, list)


async def test_update_job_status(mock_mcp_context, test_agent, enums):
    """Updating job status should return the updated row."""

    job_payload = CreateJobInput(
        title="Status Update Job",
        priority="high",
    )
    job = await create_job(job_payload, mock_mcp_context)

    payload = UpdateJobStatusInput(
        job_id=job["id"],
        status="completed",
    )

    result = await update_job_status(payload, mock_mcp_context)
    assert "id" in result


async def test_update_job_status_accepts_iso_completed_at(
    mock_mcp_context, test_agent, enums
):
    """MCP update_job_status should accept ISO completed_at values."""

    job_payload = CreateJobInput(
        title="Status Date Job",
        priority="high",
    )
    job = await create_job(job_payload, mock_mcp_context)

    payload = UpdateJobStatusInput(
        job_id=job["id"],
        status="completed",
        completed_at="2026-02-18T18:00:00Z",
    )

    result = await update_job_status(payload, mock_mcp_context)
    assert "id" in result
    assert result["completed_at"] is not None


async def test_update_job_due_at_omitted_preserves_existing_value(
    mock_mcp_context, test_agent
):
    """MCP update_job should not clear due_at when due_at is omitted."""

    job = await create_job(
        CreateJobInput(
            title="Due Preserve MCP",
            priority="medium",
            due_at="2026-02-18T18:00:00Z",
        ),
        mock_mcp_context,
    )
    assert job["due_at"] is not None

    updated = await update_job(
        UpdateJobInput(
            job_id=job["id"],
            title="Due Preserve MCP Patched",
        ),
        mock_mcp_context,
    )
    assert updated["due_at"] is not None


async def test_update_job_due_at_null_clears_existing_value(
    mock_mcp_context, test_agent
):
    """MCP update_job should clear due_at when explicitly set to null."""

    job = await create_job(
        CreateJobInput(
            title="Due Clear MCP",
            priority="medium",
            due_at="2026-02-18T18:00:00Z",
        ),
        mock_mcp_context,
    )
    assert job["due_at"] is not None

    updated = await update_job(
        UpdateJobInput(
            job_id=job["id"],
            due_at=None,
        ),
        mock_mcp_context,
    )
    assert updated["due_at"] is None


async def test_create_subtask(mock_mcp_context, test_agent):
    """Creating a subtask under a parent job should succeed."""

    parent_payload = CreateJobInput(
        title="Parent Job",
        priority="medium",
    )
    parent = await create_job(parent_payload, mock_mcp_context)

    payload = CreateSubtaskInput(
        parent_job_id=parent["id"],
        title="Child Subtask",
    )

    result = await create_subtask(payload, mock_mcp_context)
    assert "id" in result


# --- File Tools ---


async def test_create_get_list_file(mock_mcp_context, test_agent):
    """Creating and listing files should return the file row."""

    payload = CreateFileInput(
        filename="spec.md",
        file_path="/vault/00-The-Void/spec.md",
        mime_type="text/markdown",
        size_bytes=2048,
        checksum="sha256:test",
        tags=["docs", "spec"],
        metadata={"source": "test"},
    )

    created = await create_file(payload, mock_mcp_context)
    assert "id" in created

    get_payload = GetFileInput(file_id=str(created["id"]))
    fetched = await get_file(get_payload, mock_mcp_context)
    assert fetched["filename"] == "spec.md"

    list_payload = QueryFilesInput(tags=["docs"])
    listed = await list_files(list_payload, mock_mcp_context)
    assert any(row["id"] == created["id"] for row in listed)


async def test_attach_file_to_entity(mock_mcp_context, test_agent):
    """Attaching a file to an entity should create a relationship."""

    entity = await create_entity(
        CreateEntityInput(
            name="File Target Entity",
            type="project",
            status="active",
            scopes=["public"],
        ),
        mock_mcp_context,
    )

    file_row = await create_file(
        CreateFileInput(
            filename="attachment.txt",
            file_path="/vault/attachments/attachment.txt",
            mime_type="text/plain",
            tags=["attachment"],
        ),
        mock_mcp_context,
    )

    rel = await attach_file_to_entity(
        AttachFileInput(
            file_id=str(file_row["id"]),
            target_id=str(entity["id"]),
            relationship_type="has-file",
        ),
        mock_mcp_context,
    )

    assert rel["source_id"] == str(file_row["id"])
    assert rel["target_id"] == str(entity["id"])


async def test_attach_file_to_job_accepts_job_target_id(mock_mcp_context, test_agent):
    """File attachment should accept canonical job IDs for job targets."""

    job = await create_job(
        CreateJobInput(
            title="File Attachment Job",
            priority="medium",
        ),
        mock_mcp_context,
    )
    file_row = await create_file(
        CreateFileInput(
            filename="job-attachment.txt",
            file_path="/vault/attachments/job-attachment.txt",
            mime_type="text/plain",
        ),
        mock_mcp_context,
    )

    rel = await attach_file_to_job(
        AttachFileInput(
            file_id=str(file_row["id"]),
            target_id=job["id"],
            relationship_type="has-file",
        ),
        mock_mcp_context,
    )
    assert rel["target_type"] == "job"
    assert rel["target_id"] == job["id"]


# --- Graph Tools ---


async def test_graph_neighbors_and_shortest_path(mock_mcp_context, test_agent):
    """Graph traversal should return neighbors and shortest path."""

    node_a = await create_entity(
        CreateEntityInput(
            name="Graph Node A",
            type="project",
            status="active",
            scopes=["public"],
        ),
        mock_mcp_context,
    )
    node_b = await create_entity(
        CreateEntityInput(
            name="Graph Node B",
            type="project",
            status="active",
            scopes=["public"],
        ),
        mock_mcp_context,
    )
    node_c = await create_entity(
        CreateEntityInput(
            name="Graph Node C",
            type="project",
            status="active",
            scopes=["public"],
        ),
        mock_mcp_context,
    )

    await create_relationship(
        CreateRelationshipInput(
            source_type="entity",
            source_id=str(node_a["id"]),
            target_type="entity",
            target_id=str(node_b["id"]),
            relationship_type="related-to",
        ),
        mock_mcp_context,
    )
    await create_relationship(
        CreateRelationshipInput(
            source_type="entity",
            source_id=str(node_b["id"]),
            target_type="entity",
            target_id=str(node_c["id"]),
            relationship_type="related-to",
        ),
        mock_mcp_context,
    )

    neighbors_one = await graph_neighbors(
        GraphNeighborsInput(
            source_type="entity",
            source_id=str(node_a["id"]),
            max_hops=1,
            limit=10,
        ),
        mock_mcp_context,
    )
    neighbor_ids_one = {row["node_id"] for row in neighbors_one}
    assert str(node_b["id"]) in neighbor_ids_one
    assert str(node_c["id"]) not in neighbor_ids_one

    neighbors_two = await graph_neighbors(
        GraphNeighborsInput(
            source_type="entity",
            source_id=str(node_a["id"]),
            max_hops=2,
            limit=10,
        ),
        mock_mcp_context,
    )
    neighbor_ids_two = {row["node_id"] for row in neighbors_two}
    assert str(node_c["id"]) in neighbor_ids_two

    path = await graph_shortest_path(
        GraphShortestPathInput(
            source_type="entity",
            source_id=str(node_a["id"]),
            target_type="entity",
            target_id=str(node_c["id"]),
            max_hops=4,
        ),
        mock_mcp_context,
    )
    assert path["depth"] == 2
    assert path["path"][0]["id"] == str(node_a["id"])
    assert path["path"][-1]["id"] == str(node_c["id"])


async def test_graph_tools_accept_job_node_ids(mock_mcp_context, test_agent):
    """Graph tools should accept canonical job IDs as source/target IDs."""

    job = await create_job(
        CreateJobInput(
            title="Graph Job Node",
            priority="medium",
        ),
        mock_mcp_context,
    )
    entity = await create_entity(
        CreateEntityInput(
            name="Graph Job Entity",
            type="project",
            status="active",
            scopes=["public"],
        ),
        mock_mcp_context,
    )
    await create_relationship(
        CreateRelationshipInput(
            source_type="job",
            source_id=job["id"],
            target_type="entity",
            target_id=str(entity["id"]),
            relationship_type="related-to",
        ),
        mock_mcp_context,
    )

    neighbors = await graph_neighbors(
        GraphNeighborsInput(
            source_type="job",
            source_id=job["id"],
            max_hops=2,
            limit=10,
        ),
        mock_mcp_context,
    )
    assert any(row["node_id"] == str(entity["id"]) for row in neighbors)

    path = await graph_shortest_path(
        GraphShortestPathInput(
            source_type="entity",
            source_id=str(entity["id"]),
            target_type="job",
            target_id=job["id"],
            max_hops=3,
        ),
        mock_mcp_context,
    )
    assert path["path"][-1]["id"] == job["id"]


# --- Protocol Tools ---


async def test_list_active_protocols(mock_mcp_context, test_agent):
    """Listing active protocols should return a list."""

    result = await list_active_protocols(mock_mcp_context)
    assert isinstance(result, list)


# --- Agent Tools ---


async def test_get_agent_info(mock_mcp_context, test_agent):
    """Getting agent info by name should return the agent row."""

    payload = GetAgentInfoInput(
        name="test-agent",
    )

    result = await get_agent_info(payload, mock_mcp_context)
    assert result["name"] == "test-agent"


async def test_list_agents(mock_mcp_context, test_agent):
    """Listing agents should return a list containing the test agent."""

    payload = ListAgentsInput()

    result = await list_agents(payload, mock_mcp_context)
    assert isinstance(result, list)
    assert len(result) >= 1


# --- Admin Tools ---


async def test_reload_enums(mock_mcp_context, test_agent):
    """Reloading enums should return a success message."""

    result = await reload_enums(mock_mcp_context)
    assert result == "Enums reloaded."


async def test_get_pending_approvals(mock_mcp_context, test_agent):
    """Getting pending approvals for an agent should return a list."""

    result = await get_pending_approvals(mock_mcp_context)
    assert isinstance(result, list)
