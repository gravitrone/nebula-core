"""Red team API tests for bulk import edge cases and error contracts."""

# Third-Party
from httpx import ASGITransport, AsyncClient
import pytest

# Local
from nebula_api.app import app
from nebula_api.auth import require_auth


def _untrusted_auth_override(agent_row: dict, enums: object, scopes: list[str]):
    """Override require_auth to simulate an untrusted agent caller."""

    scope_ids = [enums.scopes.name_to_id[s] for s in scopes]
    auth_dict = {
        "key_id": None,
        "caller_type": "agent",
        "entity_id": None,
        "entity": None,
        "agent_id": agent_row["id"],
        "agent": {**agent_row, "requires_approval": True},
        "scopes": scope_ids,
    }

    async def mock_auth():
        """Mock agent auth for bulk import tests."""

        return auth_dict

    return mock_auth


@pytest.mark.asyncio
async def test_import_entities_rejects_malformed_json(api):
    """Malformed JSON should not crash import endpoint."""

    resp = await api.post(
        "/api/import/entities",
        content=b"{bad json",
        headers={"content-type": "application/json"},
    )
    assert resp.status_code < 500


@pytest.mark.asyncio
async def test_import_entities_partial_failure_reports_rows(api):
    """Mixed valid/invalid items should return created+failed counts with row errors."""

    payload = {
        "format": "json",
        "items": [
            {
                "name": "Good Entity",
                "type": "person",
                "status": "active",
                "scopes": ["public"],
                "tags": ["ok"],
            },
            {
                "name": "Bad Entity Missing Type",
                "status": "active",
                "scopes": ["public"],
                "tags": [],
            },
        ],
    }
    resp = await api.post("/api/import/entities", json=payload)
    assert resp.status_code == 200

    data = resp.json()["data"]
    assert data["created"] == 1
    assert data["failed"] == 1
    assert len(data["items"]) == 1
    assert data["errors"][0]["row"] == 2


@pytest.mark.asyncio
async def test_import_entities_csv_missing_type_reports_error(api):
    """CSV rows missing required fields should surface row errors, not 500."""

    csv_data = "name,type,scopes\nAlpha,,public\n"
    payload = {"format": "csv", "data": csv_data}
    resp = await api.post("/api/import/entities", json=payload)
    assert resp.status_code == 200

    data = resp.json()["data"]
    assert data["created"] == 0
    assert data["failed"] == 1
    assert data["errors"][0]["row"] == 1


@pytest.mark.asyncio
async def test_import_relationships_referential_validation_reports_error(api, test_entity):
    """Relationships to missing nodes should be reported as errors (no dangling refs)."""

    payload = {
        "format": "json",
        "items": [
            {
                "source_type": "entity",
                "source_id": str(test_entity["id"]),
                "target_type": "entity",
                "target_id": "00000000-0000-0000-0000-000000000001",
                "relationship_type": "related-to",
                "properties": {},
            }
        ],
    }
    resp = await api.post("/api/import/relationships", json=payload)
    assert resp.status_code == 200

    data = resp.json()["data"]
    assert data["created"] == 0
    assert data["failed"] == 1
    assert data["errors"][0]["row"] == 1


@pytest.mark.asyncio
async def test_import_entities_untrusted_agent_returns_approval_required(
    db_pool, enums, untrusted_agent_row
):
    """Untrusted agent bulk imports should queue per-item approvals and report errors."""

    app.state.pool = db_pool
    app.state.enums = enums
    app.dependency_overrides[require_auth] = _untrusted_auth_override(
        untrusted_agent_row, enums, ["public"]
    )

    payload = {
        "format": "json",
        "items": [
            {
                "name": "Queued Good Entity",
                "type": "person",
                "status": "active",
                "scopes": ["public"],
                "tags": [],
            },
            {
                "name": "Queued Bad Missing Type",
                "status": "active",
                "scopes": ["public"],
                "tags": [],
            },
        ],
    }

    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.post("/api/import/entities", json=payload)

    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 202
    body = resp.json()
    assert body["status"] == "approval_required"
    assert len(body["approvals"]) == 1
    assert len(body["errors"]) == 1
    assert body["errors"][0]["row"] == 2
