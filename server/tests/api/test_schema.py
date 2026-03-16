"""Schema contract endpoint tests."""

# Third-Party
import pytest


@pytest.mark.asyncio
async def test_get_schema_includes_enterprise_taxonomy(api):
    """Schema endpoint should return active enterprise taxonomy + constraints."""

    r = await api.get("/api/schema/")
    assert r.status_code == 200

    body = r.json()["data"]
    assert "taxonomy" in body
    assert "constraints" in body
    assert "statuses" in body

    taxonomy = body["taxonomy"]
    assert set(taxonomy.keys()) == {
        "scopes",
        "entity_types",
        "relationship_types",
        "log_types",
    }

    builtin_scopes = {
        row["name"] for row in taxonomy["scopes"] if row.get("is_builtin") is True
    }
    assert builtin_scopes == {"admin", "private", "public", "sensitive"}

    builtin_entity_types = {
        row["name"] for row in taxonomy["entity_types"] if row.get("is_builtin") is True
    }
    assert builtin_entity_types == {
        "document",
        "organization",
        "person",
        "project",
        "tool",
    }

    builtin_relationship_types = {
        row["name"]
        for row in taxonomy["relationship_types"]
        if row.get("is_builtin") is True
    }
    assert builtin_relationship_types == {
        "about",
        "assigned-to",
        "blocks",
        "context-of",
        "created-by",
        "depends-on",
        "has-file",
        "mentions",
        "owns",
        "references",
        "related-to",
    }

    builtin_log_types = {
        row["name"] for row in taxonomy["log_types"] if row.get("is_builtin") is True
    }
    assert builtin_log_types == {"event", "metric", "note"}

    assert body["constraints"]["jobs"]["priority"] == [
        "low",
        "medium",
        "high",
        "critical",
    ]
    assert "approved-failed" in body["constraints"]["approval_requests"]["status"]
