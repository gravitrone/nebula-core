"""Coverage-focused tests for file routes."""

# Standard Library
import json

# Third-Party
import pytest


async def _insert_entity(db_pool, enums, name: str, scopes: list[str]) -> dict:
    """Insert an entity with explicit scopes."""

    status_id = enums.statuses.name_to_id["active"]
    type_id = enums.entity_types.name_to_id["person"]
    scope_ids = [enums.scopes.name_to_id[s] for s in scopes]
    row = await db_pool.fetchrow(
        """
        INSERT INTO entities (name, type_id, status_id, privacy_scope_ids, tags, metadata)
        VALUES ($1, $2, $3, $4, $5, $6::jsonb)
        RETURNING *
        """,
        name,
        type_id,
        status_id,
        scope_ids,
        ["test"],
        json.dumps({"kind": "entity"}),
    )
    return dict(row)


async def _insert_context(db_pool, enums, title: str, scopes: list[str]) -> dict:
    """Insert a context item with explicit scopes."""

    status_id = enums.statuses.name_to_id["active"]
    scope_ids = [enums.scopes.name_to_id[s] for s in scopes]
    row = await db_pool.fetchrow(
        """
        INSERT INTO context_items (
            title, url, source_type, content, privacy_scope_ids, status_id, tags, metadata
        )
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8::jsonb)
        RETURNING *
        """,
        title,
        None,
        "note",
        "ctx",
        scope_ids,
        status_id,
        ["test"],
        json.dumps({"kind": "context"}),
    )
    return dict(row)


async def _insert_job(db_pool, enums, title: str, scopes: list[str]) -> dict:
    """Insert a job with explicit scopes."""

    status_id = enums.statuses.name_to_id["planning"]
    scope_ids = [enums.scopes.name_to_id[s] for s in scopes]
    row = await db_pool.fetchrow(
        """
        INSERT INTO jobs (title, description, job_type, status_id, priority, metadata, privacy_scope_ids)
        VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7)
        RETURNING *
        """,
        title,
        "desc",
        "task",
        status_id,
        "medium",
        json.dumps({"kind": "job"}),
        scope_ids,
    )
    return dict(row)


async def _insert_file(db_pool, enums, filename: str) -> dict:
    """Insert a file record."""

    status_id = enums.statuses.name_to_id["active"]
    row = await db_pool.fetchrow(
        """
        INSERT INTO files (filename, uri, file_path, status_id, tags, metadata)
        VALUES ($1, $2, $3, $4, $5, $6::jsonb)
        RETURNING *
        """,
        filename,
        f"file:///{filename}",
        f"/vault/{filename}",
        status_id,
        ["test"],
        json.dumps({"kind": "file"}),
    )
    return dict(row)


async def _insert_relationship(
    db_pool,
    enums,
    source_type: str,
    source_id: str,
    target_type: str,
    target_id: str,
) -> None:
    """Insert a generic relationship row."""

    status_id = enums.statuses.name_to_id["active"]
    rel_type_id = enums.relationship_types.name_to_id["has-file"]
    await db_pool.execute(
        """
        INSERT INTO relationships (source_type, source_id, target_type, target_id, type_id, status_id, properties)
        VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb)
        """,
        source_type,
        source_id,
        target_type,
        target_id,
        rel_type_id,
        status_id,
        json.dumps({}),
    )


@pytest.mark.asyncio
async def test_files_create_and_get_roundtrip(api):
    """Create and fetch a file entry."""

    created = await api.post(
        "/api/files",
        json={
            "filename": "doc.md",
            "uri": "file:///doc.md",
            "status": "active",
            "tags": ["docs"],
            "metadata": {"owner": "alxx"},
        },
    )
    assert created.status_code == 200
    data = created.json()["data"]
    assert data["filename"] == "doc.md"

    fetched = await api.get(f"/api/files/{data['id']}")
    assert fetched.status_code == 200
    assert fetched.json()["data"]["id"] == data["id"]


@pytest.mark.asyncio
async def test_files_create_requires_uri_or_file_path(api):
    """Creating a file without uri/file_path should fail."""

    res = await api.post(
        "/api/files",
        json={"filename": "missing-path"},
    )
    assert res.status_code == 400
    assert res.json()["detail"]["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
async def test_files_create_and_update_validation_errors(api):
    """Status and id validation should return consistent 400 responses."""

    bad_status = await api.post(
        "/api/files",
        json={"filename": "x", "uri": "file:///x", "status": "invalid"},
    )
    assert bad_status.status_code == 400

    bad_get = await api.get("/api/files/not-a-uuid")
    assert bad_get.status_code == 400

    bad_patch_id = await api.patch("/api/files/not-a-uuid", json={"filename": "new"})
    assert bad_patch_id.status_code == 400

    not_found_patch = await api.patch(
        "/api/files/00000000-0000-0000-0000-000000000001",
        json={"filename": "new"},
    )
    assert not_found_patch.status_code == 404

    created = await api.post(
        "/api/files",
        json={"filename": "y", "uri": "file:///y"},
    )
    file_id = created.json()["data"]["id"]
    bad_patch_status = await api.patch(
        f"/api/files/{file_id}",
        json={"status": "nope"},
    )
    assert bad_patch_status.status_code == 400


@pytest.mark.asyncio
async def test_files_create_backfills_uri_from_file_path(api):
    """file_path-only payload should populate canonical uri."""

    res = await api.post(
        "/api/files",
        json={"filename": "legacy.txt", "file_path": "/vault/legacy.txt"},
    )
    assert res.status_code == 200
    data = res.json()["data"]
    assert data["uri"] == "/vault/legacy.txt"
    assert data["file_path"] == "/vault/legacy.txt"


@pytest.mark.asyncio
async def test_files_agent_scope_checks_entity_context_job(
    api_agent_auth, db_pool, enums
):
    """Agent should be blocked from files linked to inaccessible scoped nodes."""

    sensitive_entity = await _insert_entity(
        db_pool, enums, "SensitiveEntity", scopes=["sensitive"]
    )
    sensitive_context = await _insert_context(
        db_pool, enums, "SensitiveCtx", scopes=["sensitive"]
    )
    sensitive_job = await _insert_job(
        db_pool, enums, "SensitiveJob", scopes=["sensitive"]
    )

    file_entity = await _insert_file(db_pool, enums, "entity.bin")
    file_context = await _insert_file(db_pool, enums, "context.bin")
    file_job = await _insert_file(db_pool, enums, "job.bin")
    public_file = await _insert_file(db_pool, enums, "public.bin")

    await _insert_relationship(
        db_pool,
        enums,
        "entity",
        str(sensitive_entity["id"]),
        "file",
        str(file_entity["id"]),
    )
    await _insert_relationship(
        db_pool,
        enums,
        "context",
        str(sensitive_context["id"]),
        "file",
        str(file_context["id"]),
    )
    await _insert_relationship(
        db_pool,
        enums,
        "job",
        str(sensitive_job["id"]),
        "file",
        str(file_job["id"]),
    )

    # public linked file remains visible
    public_entity = await _insert_entity(
        db_pool, enums, "PublicEntity", scopes=["public"]
    )
    await _insert_relationship(
        db_pool,
        enums,
        "entity",
        str(public_entity["id"]),
        "file",
        str(public_file["id"]),
    )

    list_res = await api_agent_auth.get("/api/files")
    assert list_res.status_code == 200
    listed = {row["id"] for row in list_res.json()["data"]}
    assert str(public_file["id"]) in listed
    assert str(file_entity["id"]) not in listed
    assert str(file_context["id"]) not in listed
    assert str(file_job["id"]) not in listed

    for blocked in (file_entity, file_context, file_job):
        get_res = await api_agent_auth.get(f"/api/files/{blocked['id']}")
        assert get_res.status_code == 403
