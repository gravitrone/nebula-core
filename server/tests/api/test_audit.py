"""Audit route tests."""

# Third-Party
import pytest


@pytest.mark.asyncio
async def test_list_audit_scopes(api, auth_override, enums):
    """List audit scopes."""

    auth_override["scopes"] = [enums.scopes.name_to_id["admin"]]

    r = await api.get("/api/audit/scopes")
    assert r.status_code == 200
    data = r.json()["data"]
    assert any(row["name"] == "public" for row in data)


@pytest.mark.asyncio
async def test_list_audit_actors(api, test_entity, auth_override, enums):
    """List audit actors."""

    auth_override["scopes"] = [enums.scopes.name_to_id["admin"]]

    r = await api.get("/api/audit/actors")
    assert r.status_code == 200
    data = r.json()["data"]
    assert len(data) >= 1
    for row in data:
        assert row.get("changed_by_type") not in {"", "unknown", "none", "null"}
        actor_name = row.get("actor_name")
        if actor_name is not None:
            assert str(actor_name).strip().lower() not in {
                "",
                "unknown",
                "none",
                "null",
            }


@pytest.mark.asyncio
async def test_list_audit_scope_filter(api, enums, test_entity, auth_override):
    """Filter audit log by scope."""

    auth_override["scopes"] = [enums.scopes.name_to_id["admin"]]

    scope_id = enums.scopes.name_to_id["public"]
    r = await api.get("/api/audit", params={"scope_id": str(scope_id)})
    assert r.status_code == 200
    data = r.json()["data"]
    assert isinstance(data, list)
