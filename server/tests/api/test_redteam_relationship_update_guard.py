"""Red team API tests for relationship update approval gating and mutation safety."""

# Standard Library
import json

# Third-Party
from httpx import ASGITransport, AsyncClient
import pytest

# Local
from nebula_api.app import app
from nebula_api.auth import require_auth


async def _make_agent(db_pool, enums, name, scopes, requires_approval):
    """Insert an agent row for relationship update guard tests."""

    status_id = enums.statuses.name_to_id["active"]
    scope_ids = [enums.scopes.name_to_id[s] for s in scopes]
    row = await db_pool.fetchrow(
        """
        INSERT INTO agents (name, description, scopes, requires_approval, status_id)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING *
        """,
        name,
        "redteam agent",
        scope_ids,
        requires_approval,
        status_id,
    )
    return dict(row)


async def _make_entity(db_pool, enums, name, scopes):
    """Insert an entity row for relationship update guard tests."""

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
        json.dumps({"note": "seed"}),
    )
    return dict(row)


async def _make_relationship(db_pool, enums, source_id, target_id, properties):
    """Insert an entity->entity relationship with properties."""

    status_id = enums.statuses.name_to_id["active"]
    # Use an asymmetric type to avoid known symmetric trigger recursion behavior.
    rel_type_id = enums.relationship_types.name_to_id["depends-on"]
    row = await db_pool.fetchrow(
        """
        INSERT INTO relationships (source_type, source_id, target_type, target_id, type_id, status_id, properties)
        VALUES ('entity', $1, 'entity', $2, $3, $4, $5::jsonb)
        RETURNING *
        """,
        str(source_id),
        str(target_id),
        rel_type_id,
        status_id,
        json.dumps(properties),
    )
    return dict(row)


def _auth_override(agent_row: dict, enums: object):
    """Override require_auth to simulate an agent caller."""

    scope_ids = [enums.scopes.name_to_id["public"]]
    auth_dict = {
        "key_id": None,
        "caller_type": "agent",
        "entity_id": None,
        "entity": None,
        "agent_id": agent_row["id"],
        "agent": agent_row,
        "scopes": scope_ids,
    }

    async def mock_auth():
        """Mock auth for agent caller."""

        return auth_dict

    return mock_auth


@pytest.mark.asyncio
async def test_untrusted_update_queues_approval_without_mutating(db_pool, enums):
    """Untrusted updates should not mutate the relationship row directly."""

    a = await _make_entity(db_pool, enums, "A", ["public"])
    b = await _make_entity(db_pool, enums, "B", ["public"])
    relationship = await _make_relationship(
        db_pool, enums, a["id"], b["id"], {"note": "original"}
    )
    untrusted = await _make_agent(
        db_pool, enums, "rel-untrusted-guard", ["public"], True
    )

    before = await db_pool.fetchval(
        "SELECT properties FROM relationships WHERE id = $1", relationship["id"]
    )

    app.dependency_overrides[require_auth] = _auth_override(untrusted, enums)
    app.state.pool = db_pool
    app.state.enums = enums
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.patch(
            f"/api/relationships/{relationship['id']}",
            json={"properties": {"note": "should-not-apply"}},
        )
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 202
    assert resp.json()["status"] == "approval_required"

    after = await db_pool.fetchval(
        "SELECT properties FROM relationships WHERE id = $1", relationship["id"]
    )
    assert before == after


@pytest.mark.asyncio
async def test_trusted_update_mutates_immediately(db_pool, enums):
    """Trusted agents should be able to update allowed relationships directly."""

    a = await _make_entity(db_pool, enums, "A", ["public"])
    b = await _make_entity(db_pool, enums, "B", ["public"])
    relationship = await _make_relationship(
        db_pool, enums, a["id"], b["id"], {"note": "original"}
    )
    trusted = await _make_agent(db_pool, enums, "rel-trusted-guard", ["public"], False)

    app.dependency_overrides[require_auth] = _auth_override(trusted, enums)
    app.state.pool = db_pool
    app.state.enums = enums
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.patch(
            f"/api/relationships/{relationship['id']}",
            json={"properties": {"note": "updated"}},
        )
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 200
    props = await db_pool.fetchval(
        "SELECT properties FROM relationships WHERE id = $1", relationship["id"]
    )
    if isinstance(props, str):
        props = json.loads(props)
    assert props["note"] == "updated"
