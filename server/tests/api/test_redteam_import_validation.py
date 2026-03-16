"""Red team tests for import validation error handling."""

# Third-Party
import pytest


@pytest.mark.asyncio
async def test_import_entities_rejects_invalid_format(api):
    """Invalid format should not crash the import endpoint."""

    payload = {
        "format": "xml",
        "items": [
            {
                "name": "BadFormat",
                "type": "person",
                "status": "active",
                "scopes": ["public"],
                "tags": [],
            }
        ],
    }
    resp = await api.post("/api/import/entities", json=payload)
    assert resp.status_code == 400
    body = resp.json()
    assert body["detail"]["error"]["code"] == "VALIDATION_ERROR"


@pytest.mark.asyncio
async def test_import_entities_rejects_empty_csv(api):
    """Missing CSV data should not crash the import endpoint."""

    payload = {
        "format": "csv",
        "data": "",
    }
    resp = await api.post("/api/import/entities", json=payload)
    assert resp.status_code == 400
    body = resp.json()
    assert body["detail"]["error"]["code"] == "VALIDATION_ERROR"
