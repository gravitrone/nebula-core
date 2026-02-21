"""Red team tests for export relationships."""

# Standard Library
import json

# Third-Party
import pytest

# Local
from nebula_api.app import app
from nebula_api.auth import require_auth


async def _make_entity(db_pool, enums, name: str) -> dict:
    """Create and return a public entity row."""

    row = await db_pool.fetchrow(
        """
        INSERT INTO entities (name, type_id, status_id, privacy_scope_ids, tags, metadata)
        VALUES ($1, $2, $3, $4, $5, $6::jsonb)
        RETURNING *
        """,
        name,
        enums.entity_types.name_to_id["person"],
        enums.statuses.name_to_id["active"],
        [enums.scopes.name_to_id["public"]],
        [],
        "{}",
    )
    return dict(row)


async def _make_agent(db_pool, enums, name: str) -> dict:
    """Create and return an active public-scope agent row."""

    row = await db_pool.fetchrow(
        """
        INSERT INTO agents (name, description, scopes, requires_approval, status_id)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING *
        """,
        name,
        "export relationships agent",
        [enums.scopes.name_to_id["public"]],
        False,
        enums.statuses.name_to_id["active"],
    )
    return dict(row)


async def _make_job(
    db_pool, enums, *, title: str, agent_id: str, scopes: list[str]
) -> dict:
    """Create and return a job row with explicit scopes."""

    row = await db_pool.fetchrow(
        """
        INSERT INTO jobs (title, status_id, agent_id, privacy_scope_ids, metadata)
        VALUES ($1, $2, $3, $4, $5::jsonb)
        RETURNING *
        """,
        title,
        enums.statuses.name_to_id["active"],
        agent_id,
        [enums.scopes.name_to_id[s] for s in scopes],
        json.dumps({"note": "job-export-api"}),
    )
    return dict(row)


async def _make_relationship_with_segments(
    db_pool, enums, source_id: str, target_id: str
):
    """Create and return a relationship with mixed-scope context segments."""

    props = {
        "context_segments": [
            {"text": "public edge context", "scopes": ["public"]},
            {"text": "sensitive edge context", "scopes": ["sensitive"]},
        ],
        "note": "mixed-scope-export-api",
    }
    row = await db_pool.fetchrow(
        """
        INSERT INTO relationships (source_type, source_id, target_type, target_id, type_id, status_id, properties)
        VALUES ('entity', $1, 'entity', $2, $3, $4, $5::jsonb)
        RETURNING *
        """,
        source_id,
        target_id,
        enums.relationship_types.name_to_id["related-to"],
        enums.statuses.name_to_id["active"],
        json.dumps(props),
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
    """Create and return a relationship row linking job and non-job nodes."""

    row = await db_pool.fetchrow(
        """
        INSERT INTO relationships (source_type, source_id, target_type, target_id, type_id, status_id, properties)
        VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb)
        RETURNING *
        """,
        source_type,
        source_id,
        target_type,
        target_id,
        enums.relationship_types.name_to_id["related-to"],
        enums.statuses.name_to_id["active"],
        json.dumps({"note": "job-rel-api"}),
    )
    return dict(row)


def _public_auth(entity_row, enums):
    """Build auth payload constrained to public scope."""

    return {
        "key_id": None,
        "caller_type": "user",
        "entity_id": entity_row["id"],
        "entity": entity_row,
        "agent_id": None,
        "agent": None,
        "scopes": [enums.scopes.name_to_id["public"]],
    }


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
async def test_export_relationships_default(api):
    """Export relationships should return a response."""

    resp = await api.get("/api/export/relationships")
    assert resp.status_code == 200


@pytest.mark.asyncio
async def test_export_relationships_filters_properties_context_segments(
    api_no_auth, db_pool, enums
):
    """Relationships export should hide sensitive property segments for public callers."""

    source = await _make_entity(db_pool, enums, "Export API Src")
    target = await _make_entity(db_pool, enums, "Export API Dst")
    rel = await _make_relationship_with_segments(
        db_pool, enums, str(source["id"]), str(target["id"])
    )

    async def mock_auth():
        """Return public-only auth context."""

        return _public_auth(source, enums)

    app.dependency_overrides[require_auth] = mock_auth
    try:
        resp = await api_no_auth.get("/api/export/relationships")
    finally:
        app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 200
    rows = resp.json()["data"]["items"]
    row = next((item for item in rows if item["id"] == str(rel["id"])), None)
    assert row is not None
    segments = _as_dict(row.get("properties")).get("context_segments", [])
    texts = {seg.get("text") for seg in segments if isinstance(seg, dict)}
    assert "public edge context" in texts
    assert "sensitive edge context" not in texts


@pytest.mark.asyncio
async def test_export_relationships_properties_payload_is_object(
    api_no_auth, db_pool, enums
):
    """Relationships export should return properties as structured object."""

    source = await _make_entity(db_pool, enums, "Export API Type Src")
    target = await _make_entity(db_pool, enums, "Export API Type Dst")
    rel = await _make_relationship_with_segments(
        db_pool, enums, str(source["id"]), str(target["id"])
    )

    async def mock_auth():
        """Return public-only auth context."""

        return _public_auth(source, enums)

    app.dependency_overrides[require_auth] = mock_auth
    try:
        resp = await api_no_auth.get("/api/export/relationships")
    finally:
        app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 200
    row = next(
        (item for item in resp.json()["data"]["items"] if item["id"] == str(rel["id"])),
        None,
    )
    assert row is not None
    assert isinstance(row.get("properties"), dict)


@pytest.mark.asyncio
async def test_export_snapshot_filters_relationship_properties_context_segments(
    api_no_auth, db_pool, enums
):
    """Snapshot export should hide sensitive relationship segments for public callers."""

    source = await _make_entity(db_pool, enums, "Snapshot API Src")
    target = await _make_entity(db_pool, enums, "Snapshot API Dst")
    rel = await _make_relationship_with_segments(
        db_pool, enums, str(source["id"]), str(target["id"])
    )

    async def mock_auth():
        """Return public-only auth context."""

        return _public_auth(source, enums)

    app.dependency_overrides[require_auth] = mock_auth
    try:
        resp = await api_no_auth.get("/api/export/snapshot")
    finally:
        app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 200
    rows = resp.json()["data"]["relationships"]
    row = next((item for item in rows if item["id"] == str(rel["id"])), None)
    assert row is not None
    segments = _as_dict(row.get("properties")).get("context_segments", [])
    texts = {seg.get("text") for seg in segments if isinstance(seg, dict)}
    assert "public edge context" in texts
    assert "sensitive edge context" not in texts


@pytest.mark.asyncio
async def test_export_snapshot_relationship_properties_payload_is_object(
    api_no_auth, db_pool, enums
):
    """Snapshot export should return relationship properties as structured object."""

    source = await _make_entity(db_pool, enums, "Snapshot API Type Src")
    target = await _make_entity(db_pool, enums, "Snapshot API Type Dst")
    rel = await _make_relationship_with_segments(
        db_pool, enums, str(source["id"]), str(target["id"])
    )

    async def mock_auth():
        """Return public-only auth context."""

        return _public_auth(source, enums)

    app.dependency_overrides[require_auth] = mock_auth
    try:
        resp = await api_no_auth.get("/api/export/snapshot")
    finally:
        app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 200
    row = next(
        (
            item
            for item in resp.json()["data"]["relationships"]
            if item["id"] == str(rel["id"])
        ),
        None,
    )
    assert row is not None
    assert isinstance(row.get("properties"), dict)


@pytest.mark.asyncio
async def test_export_relationships_hides_out_of_scope_job_links_for_user(
    api_no_auth, db_pool, enums
):
    """Relationships export should hide private job links for user callers."""

    source = await _make_entity(db_pool, enums, "Export API Job Src")
    owner = await _make_agent(db_pool, enums, "export-job-owner")
    private_job = await _make_job(
        db_pool,
        enums,
        title="Export API Private Job",
        agent_id=owner["id"],
        scopes=["private"],
    )
    rel = await _make_job_relationship(
        db_pool,
        enums,
        source_type="entity",
        source_id=str(source["id"]),
        target_type="job",
        target_id=private_job["id"],
    )

    async def mock_auth():
        """Return public-only user auth context."""

        return _public_auth(source, enums)

    app.dependency_overrides[require_auth] = mock_auth
    try:
        resp = await api_no_auth.get("/api/export/relationships")
    finally:
        app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 200
    ids = {row["id"] for row in resp.json()["data"]["items"]}
    assert str(rel["id"]) not in ids


@pytest.mark.asyncio
async def test_export_snapshot_hides_out_of_scope_job_links_for_user(
    api_no_auth, db_pool, enums
):
    """Snapshot export should hide private job links for user callers."""

    source = await _make_entity(db_pool, enums, "Snapshot API Job Src")
    owner = await _make_agent(db_pool, enums, "snapshot-job-owner")
    private_job = await _make_job(
        db_pool,
        enums,
        title="Snapshot API Private Job",
        agent_id=owner["id"],
        scopes=["private"],
    )
    rel = await _make_job_relationship(
        db_pool,
        enums,
        source_type="job",
        source_id=private_job["id"],
        target_type="entity",
        target_id=str(source["id"]),
    )

    async def mock_auth():
        """Return public-only user auth context."""

        return _public_auth(source, enums)

    app.dependency_overrides[require_auth] = mock_auth
    try:
        resp = await api_no_auth.get("/api/export/snapshot")
    finally:
        app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 200
    ids = {row["id"] for row in resp.json()["data"]["relationships"]}
    assert str(rel["id"]) not in ids
