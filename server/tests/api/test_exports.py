"""Export route tests."""

# Third-Party
from fastapi import HTTPException
import pytest

# Local
from nebula_api.routes import exports as exports_routes


@pytest.mark.asyncio
async def test_export_entities_json(api, test_entity):
    """Export entities in json format."""

    r = await api.get("/api/export/entities")
    assert r.status_code == 200
    data = r.json()["data"]
    assert data["format"] == "json"
    assert len(data["items"]) >= 1


@pytest.mark.asyncio
async def test_export_entities_csv(api, test_entity):
    """Export entities in csv format."""

    r = await api.get("/api/export/entities", params={"format": "csv"})
    assert r.status_code == 200
    data = r.json()["data"]
    assert data["format"] == "csv"
    assert "name" in data["content"]


@pytest.mark.asyncio
async def test_export_context_json(api):
    """Export context in json format."""

    await api.post(
        "/api/context",
        json={"title": "Export Context", "source_type": "note", "scopes": ["public"]},
    )
    r = await api.get("/api/export/context")
    assert r.status_code == 200
    data = r.json()["data"]
    assert data["format"] == "json"
    assert len(data["items"]) >= 1


@pytest.mark.asyncio
async def test_export_relationships_json(api):
    """Export relationships in json format."""

    r1 = await api.post(
        "/api/entities",
        json={"name": "ExportSource", "type": "person", "scopes": ["public"]},
    )
    r2 = await api.post(
        "/api/entities",
        json={"name": "ExportTarget", "type": "person", "scopes": ["public"]},
    )
    await api.post(
        "/api/relationships",
        json={
            "source_type": "entity",
            "source_id": r1.json()["data"]["id"],
            "target_type": "entity",
            "target_id": r2.json()["data"]["id"],
            "relationship_type": "related-to",
        },
    )
    r = await api.get("/api/export/relationships")
    assert r.status_code == 200
    data = r.json()["data"]
    assert data["format"] == "json"
    assert len(data["items"]) >= 1


@pytest.mark.asyncio
async def test_export_snapshot_json(api):
    """Export full snapshot in json format."""

    r = await api.get("/api/export/snapshot")
    assert r.status_code == 200
    data = r.json()["data"]
    assert data["format"] == "json"
    assert "entities" in data
    assert "context" in data


def test_export_helpers_handle_none_and_empty_rows():
    """Export helpers should normalize none values and empty csv payloads."""

    assert exports_routes._flatten_value(None) == ""
    assert exports_routes._to_csv([]) == ""


def test_export_response_rejects_invalid_format():
    """Export response helper should validate output format."""

    with pytest.raises(HTTPException):
        exports_routes._export_response([], "yaml")


@pytest.mark.asyncio
async def test_export_schema_returns_contract(api):
    """Export schema endpoint should return a schema contract payload."""

    r = await api.get("/api/export/schema")
    assert r.status_code == 200
    assert isinstance(r.json()["data"], dict)


@pytest.mark.asyncio
async def test_export_snapshot_rejects_non_json_format(api):
    """Snapshot endpoint should reject non-json formats."""

    r = await api.get("/api/export/snapshot", params={"format": "csv"})
    assert r.status_code == 400
    assert r.json()["detail"]["error"]["code"] == "VALIDATION_ERROR"
