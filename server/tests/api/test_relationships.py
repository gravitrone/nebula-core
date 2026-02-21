"""Relationship route tests."""

# Third-Party
import pytest


async def _make_entity(api, name="RelEntity"):
    """Make entity."""

    r = await api.post(
        "/api/entities",
        json={
            "name": name,
            "type": "person",
            "scopes": ["public"],
        },
    )
    return r.json()["data"]


@pytest.mark.asyncio
async def test_create_relationship(api):
    """Test create relationship."""

    e1 = await _make_entity(api, "Source")
    e2 = await _make_entity(api, "Target")

    r = await api.post(
        "/api/relationships",
        json={
            "source_type": "entity",
            "source_id": str(e1["id"]),
            "target_type": "entity",
            "target_id": str(e2["id"]),
            "relationship_type": "related-to",
        },
    )
    assert r.status_code == 200
    assert "id" in r.json()["data"]


@pytest.mark.asyncio
async def test_get_relationships(api):
    """Test get relationships."""

    e1 = await _make_entity(api, "GetSrc")
    e2 = await _make_entity(api, "GetTgt")

    await api.post(
        "/api/relationships",
        json={
            "source_type": "entity",
            "source_id": str(e1["id"]),
            "target_type": "entity",
            "target_id": str(e2["id"]),
            "relationship_type": "related-to",
        },
    )

    r = await api.get(f"/api/relationships/entity/{e1['id']}")
    assert r.status_code == 200
    assert len(r.json()["data"]) >= 1


@pytest.mark.asyncio
async def test_query_relationships(api):
    """Test query relationships."""

    e1 = await _make_entity(api, "QSrc")
    e2 = await _make_entity(api, "QTgt")

    await api.post(
        "/api/relationships",
        json={
            "source_type": "entity",
            "source_id": str(e1["id"]),
            "target_type": "entity",
            "target_id": str(e2["id"]),
            "relationship_type": "related-to",
        },
    )

    r = await api.get("/api/relationships", params={"source_type": "entity"})
    assert r.status_code == 200
    assert len(r.json()["data"]) >= 1


@pytest.mark.asyncio
async def test_update_relationship(api):
    """Test update relationship."""

    e1 = await _make_entity(api, "UpdSrc")
    e2 = await _make_entity(api, "UpdTgt")

    # use asymmetric type to avoid symmetric trigger recursion bug
    cr = await api.post(
        "/api/relationships",
        json={
            "source_type": "entity",
            "source_id": str(e1["id"]),
            "target_type": "entity",
            "target_id": str(e2["id"]),
            "relationship_type": "depends-on",
        },
    )
    rel_id = cr.json()["data"]["id"]

    r = await api.patch(
        f"/api/relationships/{rel_id}",
        json={
            "properties": {"note": "updated"},
        },
    )
    assert r.status_code == 200


@pytest.mark.asyncio
async def test_get_relationships_direction_filter(api):
    """Test get relationships direction filter."""

    e1 = await _make_entity(api, "DirSrc")
    e2 = await _make_entity(api, "DirTgt")

    await api.post(
        "/api/relationships",
        json={
            "source_type": "entity",
            "source_id": str(e1["id"]),
            "target_type": "entity",
            "target_id": str(e2["id"]),
            "relationship_type": "related-to",
        },
    )

    r = await api.get(
        f"/api/relationships/entity/{e1['id']}", params={"direction": "outgoing"}
    )
    assert r.status_code == 200


@pytest.mark.asyncio
async def test_get_relationships_rejects_invalid_source_type(api, test_entity):
    """Route should reject unknown relationship node types."""

    r = await api.get(f"/api/relationships/not-a-type/{test_entity['id']}")
    assert r.status_code == 400
    body = r.json()
    assert body["detail"]["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
async def test_get_relationships_rejects_invalid_job_id_shape(api):
    """Job relationship lookups should reject malformed canonical ids."""

    r = await api.get("/api/relationships/job/2026Q5-ABCDE")
    assert r.status_code == 400
    body = r.json()
    assert body["detail"]["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
async def test_get_relationships_normalizes_job_id_case(api):
    """Lowercase canonical job ids should be accepted and normalized."""

    r = await api.get("/api/relationships/job/2026q1-abcd")
    assert r.status_code == 200
    assert isinstance(r.json()["data"], list)


@pytest.mark.asyncio
async def test_create_relationship_rejects_unknown_relationship_type(api):
    """Relationship type validation should happen before approval queueing."""

    e1 = await _make_entity(api, "InvalidTypeSrc")
    e2 = await _make_entity(api, "InvalidTypeTgt")

    r = await api.post(
        "/api/relationships",
        json={
            "source_type": "entity",
            "source_id": str(e1["id"]),
            "target_type": "entity",
            "target_id": str(e2["id"]),
            "relationship_type": "not-a-real-type",
        },
    )
    assert r.status_code == 400
    body = r.json()
    assert body["detail"]["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
async def test_update_relationship_rejects_unknown_status(api):
    """Status validation should reject unknown names before writes."""

    e1 = await _make_entity(api, "UpdInvalidStatusSrc")
    e2 = await _make_entity(api, "UpdInvalidStatusTgt")
    cr = await api.post(
        "/api/relationships",
        json={
            "source_type": "entity",
            "source_id": str(e1["id"]),
            "target_type": "entity",
            "target_id": str(e2["id"]),
            "relationship_type": "depends-on",
        },
    )
    rel_id = cr.json()["data"]["id"]

    r = await api.patch(
        f"/api/relationships/{rel_id}",
        json={"status": "todo"},
    )
    assert r.status_code == 400
    body = r.json()
    assert body["detail"]["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
async def test_update_relationship_accepts_archived_alias(api, db_pool):
    """Archived alias should map to a valid archived-category relationship status."""

    e1 = await _make_entity(api, "UpdArchivedAliasSrc")
    e2 = await _make_entity(api, "UpdArchivedAliasTgt")
    created = await api.post(
        "/api/relationships",
        json={
            "source_type": "entity",
            "source_id": str(e1["id"]),
            "target_type": "entity",
            "target_id": str(e2["id"]),
            "relationship_type": "depends-on",
        },
    )
    rel_id = created.json()["data"]["id"]

    archived_names = {
        row["name"]
        for row in await db_pool.fetch(
            "SELECT name FROM statuses WHERE category = 'archived'"
        )
    }

    resp = await api.patch(
        f"/api/relationships/{rel_id}",
        json={"status": "archived"},
    )
    assert resp.status_code == 200
    row = await db_pool.fetchrow(
        """
        SELECT s.name, s.category
        FROM relationships r
        JOIN statuses s ON s.id = r.status_id
        WHERE r.id = $1::uuid
        """,
        rel_id,
    )
    assert row["category"] == "archived"
    assert row["name"] in archived_names
