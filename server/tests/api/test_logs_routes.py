"""Coverage-focused tests for log routes."""

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
        INSERT INTO entities (name, type_id, status_id, privacy_scope_ids, tags)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING *
        """,
        name,
        type_id,
        status_id,
        scope_ids,
        ["test"],
    )
    return dict(row)


async def _insert_context(db_pool, enums, title: str, scopes: list[str]) -> dict:
    """Insert a context item with explicit scopes."""

    status_id = enums.statuses.name_to_id["active"]
    scope_ids = [enums.scopes.name_to_id[s] for s in scopes]
    row = await db_pool.fetchrow(
        """
        INSERT INTO context_items (
            title, url, source_type, content, privacy_scope_ids, status_id, tags
        )
        VALUES ($1, $2, $3, $4, $5, $6, $7)
        RETURNING *
        """,
        title,
        None,
        "note",
        "ctx",
        scope_ids,
        status_id,
        ["test"],
    )
    return dict(row)


async def _insert_job(db_pool, enums, title: str, scopes: list[str]) -> dict:
    """Insert a job with explicit scopes."""

    status_id = enums.statuses.name_to_id["planning"]
    scope_ids = [enums.scopes.name_to_id[s] for s in scopes]
    row = await db_pool.fetchrow(
        """
        INSERT INTO jobs (title, description, job_type, status_id, priority, privacy_scope_ids)
        VALUES ($1, $2, $3, $4, $5, $6)
        RETURNING *
        """,
        title,
        "desc",
        "task",
        status_id,
        "medium",
        scope_ids,
    )
    return dict(row)


async def _insert_log(db_pool, enums, log_type: str = "event") -> dict:
    """Insert a log entry."""

    status_id = enums.statuses.name_to_id["active"]
    log_type_id = enums.log_types.name_to_id[log_type]
    row = await db_pool.fetchrow(
        """
        INSERT INTO logs (log_type_id, timestamp, value, status_id, tags, metadata)
        VALUES ($1, now(), $2::jsonb, $3, $4, $5::jsonb)
        RETURNING *
        """,
        log_type_id,
        json.dumps({"v": 1}),
        status_id,
        ["test"],
        json.dumps({"kind": "log"}),
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
    rel_type_id = enums.relationship_types.name_to_id["related-to"]
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
async def test_logs_create_get_query_and_update_roundtrip(api):
    """Log route should support basic create/get/query/update flow."""

    created = await api.post(
        "/api/logs",
        json={"log_type": "event"},
    )
    assert created.status_code == 200
    log_row = created.json()["data"]
    assert isinstance(log_row["value"], dict)
    assert log_row["value"] == {}
    assert isinstance(log_row["metadata"], dict)
    assert log_row["metadata"] == {}

    fetched = await api.get(f"/api/logs/{log_row['id']}")
    assert fetched.status_code == 200
    assert isinstance(fetched.json()["data"]["value"], dict)
    assert isinstance(fetched.json()["data"]["metadata"], dict)

    queried = await api.get("/api/logs", params={"log_type": "event"})
    assert queried.status_code == 200
    assert queried.json()["data"]
    assert isinstance(queried.json()["data"][0]["value"], dict)
    assert isinstance(queried.json()["data"][0]["metadata"], dict)

    updated = await api.patch(
        f"/api/logs/{log_row['id']}",
        json={"status": "in-progress", "metadata": {"note": "x"}},
    )
    assert updated.status_code == 200
    updated_meta = updated.json()["data"]["metadata"]
    if isinstance(updated_meta, str):
        updated_meta = json.loads(updated_meta)
    assert updated_meta["note"] == "x"


@pytest.mark.asyncio
async def test_logs_validation_errors(api):
    """Invalid ids, status, and log types should return 400."""

    bad_create = await api.post(
        "/api/logs",
        json={"log_type": "nope"},
    )
    assert bad_create.status_code == 400

    bad_get = await api.get("/api/logs/not-a-uuid")
    assert bad_get.status_code == 400

    not_found_get = await api.get("/api/logs/00000000-0000-0000-0000-000000000001")
    assert not_found_get.status_code == 404

    bad_query = await api.get("/api/logs", params={"log_type": "missing"})
    assert bad_query.status_code == 400

    bad_patch_id = await api.patch("/api/logs/not-a-uuid", json={"status": "active"})
    assert bad_patch_id.status_code == 400

    created = await api.post("/api/logs", json={"log_type": "event"})
    log_id = created.json()["data"]["id"]
    bad_patch_status = await api.patch(
        f"/api/logs/{log_id}",
        json={"status": "not-real"},
    )
    assert bad_patch_status.status_code == 400

    bad_patch_type = await api.patch(
        f"/api/logs/{log_id}",
        json={"log_type": "not-real"},
    )
    assert bad_patch_type.status_code == 400

    not_found_patch = await api.patch(
        "/api/logs/00000000-0000-0000-0000-000000000001",
        json={"status": "active"},
    )
    assert not_found_patch.status_code == 404


@pytest.mark.asyncio
async def test_logs_agent_scope_checks_entity_context_job(api_agent_auth, db_pool, enums):
    """Agent should be blocked from logs linked to inaccessible scoped nodes."""

    sensitive_entity = await _insert_entity(db_pool, enums, "SensitiveEntity", scopes=["sensitive"])
    sensitive_context = await _insert_context(db_pool, enums, "SensitiveCtx", scopes=["sensitive"])
    sensitive_job = await _insert_job(db_pool, enums, "SensitiveJob", scopes=["sensitive"])

    log_entity = await _insert_log(db_pool, enums, "event")
    log_context = await _insert_log(db_pool, enums, "event")
    log_job = await _insert_log(db_pool, enums, "event")
    public_log = await _insert_log(db_pool, enums, "event")

    await _insert_relationship(
        db_pool,
        enums,
        "entity",
        str(sensitive_entity["id"]),
        "log",
        str(log_entity["id"]),
    )
    await _insert_relationship(
        db_pool,
        enums,
        "context",
        str(sensitive_context["id"]),
        "log",
        str(log_context["id"]),
    )
    await _insert_relationship(
        db_pool,
        enums,
        "job",
        str(sensitive_job["id"]),
        "log",
        str(log_job["id"]),
    )

    public_entity = await _insert_entity(db_pool, enums, "PublicEntity", scopes=["public"])
    await _insert_relationship(
        db_pool,
        enums,
        "entity",
        str(public_entity["id"]),
        "log",
        str(public_log["id"]),
    )

    list_res = await api_agent_auth.get("/api/logs", params={"log_type": "event"})
    assert list_res.status_code == 200
    listed = {row["id"] for row in list_res.json()["data"]}
    assert str(public_log["id"]) in listed
    assert str(log_entity["id"]) not in listed
    assert str(log_context["id"]) not in listed
    assert str(log_job["id"]) not in listed

    for blocked in (log_entity, log_context, log_job):
        get_res = await api_agent_auth.get(f"/api/logs/{blocked['id']}")
        assert get_res.status_code == 403


@pytest.mark.asyncio
async def test_logs_get_and_query_preserve_object_payloads(api, db_pool, enums):
    """Directly inserted object payloads should stay object-shaped via API responses."""

    status_id = enums.statuses.name_to_id["active"]
    log_type_id = enums.log_types.name_to_id["note"]
    row = await db_pool.fetchrow(
        """
        INSERT INTO logs (log_type_id, timestamp, value, status_id, tags, metadata)
        VALUES ($1, now(), $2::jsonb, $3, $4, $5::jsonb)
        RETURNING id
        """,
        log_type_id,
        json.dumps({"text": "legacy-value"}),
        status_id,
        ["legacy"],
        json.dumps({"source": "legacy"}),
    )
    assert row is not None
    log_id = str(row["id"])

    get_res = await api.get(f"/api/logs/{log_id}")
    assert get_res.status_code == 200, get_res.text
    get_payload = get_res.json()["data"]
    assert get_payload["value"] == {"text": "legacy-value"}
    assert isinstance(get_payload["value"], dict)
    assert get_payload["metadata"] == {"source": "legacy"}
    assert isinstance(get_payload["metadata"], dict)

    list_res = await api.get("/api/logs", params={"log_type": "note"})
    assert list_res.status_code == 200, list_res.text
    listed = next(
        (item for item in list_res.json()["data"] if str(item.get("id")) == log_id),
        None,
    )
    assert listed is not None
    assert listed["value"] == {"text": "legacy-value"}
    assert listed["metadata"] == {"source": "legacy"}
