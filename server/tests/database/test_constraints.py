"""Database tests for CHECK, FK, and UNIQUE constraints."""

# Standard Library
import uuid

import asyncpg
import pytest

pytestmark = pytest.mark.database


# --- Helpers ---


async def _make_entity(pool, enums, name):
    """Insert a minimal entity and return the row."""

    status_id = enums.statuses.name_to_id["active"]
    type_id = enums.entity_types.name_to_id["person"]
    scope_ids = [enums.scopes.name_to_id["public"]]

    row = await pool.fetchrow(
        """
        INSERT INTO entities (privacy_scope_ids, name, type_id, status_id)
        VALUES ($1, $2, $3, $4)
        RETURNING *
        """,
        scope_ids,
        name,
        type_id,
        status_id,
    )
    return row


# --- CHECK Constraints ---


@pytest.mark.asyncio
async def test_status_category_check_rejects_invalid(db_pool):
    """Statuses table rejects an invalid category value."""

    with pytest.raises(asyncpg.CheckViolationError):
        await db_pool.execute("""
            INSERT INTO statuses (name, description, category)
            VALUES ('bad-status', 'should fail', 'invalid_category')
            """)


@pytest.mark.asyncio
async def test_relationship_source_type_check_rejects_invalid(db_pool, enums):
    """Relationships table rejects an invalid source_type."""

    target = await _make_entity(db_pool, enums, "src-type-target")
    type_id = enums.relationship_types.name_to_id["depends-on"]
    status_id = enums.statuses.name_to_id["active"]

    with pytest.raises((asyncpg.CheckViolationError, asyncpg.RaiseError)):
        await db_pool.execute(
            """
            INSERT INTO relationships (source_type, source_id, target_type, target_id, type_id, status_id)
            VALUES ('invalid_type', $1, 'entity', $2, $3, $4)
            """,
            str(target["id"]),
            str(target["id"]),
            type_id,
            status_id,
        )


@pytest.mark.asyncio
async def test_relationship_target_type_check_rejects_invalid(db_pool, enums):
    """Relationships table rejects an invalid target_type."""

    source = await _make_entity(db_pool, enums, "tgt-type-source")
    type_id = enums.relationship_types.name_to_id["depends-on"]
    status_id = enums.statuses.name_to_id["active"]

    with pytest.raises((asyncpg.CheckViolationError, asyncpg.RaiseError)):
        await db_pool.execute(
            """
            INSERT INTO relationships (source_type, source_id, target_type, target_id, type_id, status_id)
            VALUES ('entity', $1, 'invalid_type', $2, $3, $4)
            """,
            str(source["id"]),
            str(source["id"]),
            type_id,
            status_id,
        )


@pytest.mark.asyncio
async def test_relationship_unique_rejects_duplicate(db_pool, enums):
    """Relationships UNIQUE constraint rejects duplicate source+target+type."""

    a = await _make_entity(db_pool, enums, "uniq-a")
    b = await _make_entity(db_pool, enums, "uniq-b")
    type_id = enums.relationship_types.name_to_id["depends-on"]
    status_id = enums.statuses.name_to_id["active"]

    await db_pool.execute(
        """
        INSERT INTO relationships (source_type, source_id, target_type, target_id, type_id, status_id)
        VALUES ('entity', $1, 'entity', $2, $3, $4)
        """,
        str(a["id"]),
        str(b["id"]),
        type_id,
        status_id,
    )

    with pytest.raises(asyncpg.UniqueViolationError):
        await db_pool.execute(
            """
            INSERT INTO relationships (source_type, source_id, target_type, target_id, type_id, status_id)
            VALUES ('entity', $1, 'entity', $2, $3, $4)
            """,
            str(a["id"]),
            str(b["id"]),
            type_id,
            status_id,
        )


@pytest.mark.asyncio
async def test_job_priority_check_rejects_invalid(db_pool, enums):
    """Jobs table rejects an invalid priority value."""

    status_id = enums.statuses.name_to_id["active"]

    with pytest.raises((asyncpg.CheckViolationError, asyncpg.RaiseError)):
        await db_pool.execute(
            """
            INSERT INTO jobs (title, status_id, priority)
            VALUES ('bad-priority-job', $1, 'urgent')
            """,
            status_id,
        )


@pytest.mark.asyncio
async def test_approval_status_check_rejects_invalid(db_pool, enums):
    """Approval requests table rejects an invalid status."""

    status_id = enums.statuses.name_to_id["active"]

    agent = await db_pool.fetchrow(
        """
        INSERT INTO agents (name, status_id, requires_approval)
        VALUES ('constraint-agent', $1, true)
        RETURNING *
        """,
        status_id,
    )

    with pytest.raises(asyncpg.CheckViolationError):
        await db_pool.execute(
            """
            INSERT INTO approval_requests (request_type, requested_by, status)
            VALUES ('create_entity', $1, 'invalid_status')
            """,
            agent["id"],
        )


@pytest.mark.asyncio
async def test_approval_approved_failed_status_allowed(db_pool, enums):
    """Approval requests table accepts the 'approved-failed' status."""

    status_id = enums.statuses.name_to_id["active"]

    agent = await db_pool.fetchrow(
        """
        INSERT INTO agents (name, status_id, requires_approval)
        VALUES ('approved-failed-agent', $1, true)
        RETURNING *
        """,
        status_id,
    )

    row = await db_pool.fetchrow(
        """
        INSERT INTO approval_requests (request_type, requested_by, status)
        VALUES ('create_entity', $1, 'approved-failed')
        RETURNING *
        """,
        agent["id"],
    )
    assert row is not None
    assert row["status"] == "approved-failed"


# --- FK Constraints ---


@pytest.mark.asyncio
async def test_entity_fk_status_id_rejects_fake(db_pool, enums):
    """Entity FK on status_id rejects a nonexistent UUID."""

    fake_status_id = uuid.uuid4()
    type_id = enums.entity_types.name_to_id["person"]
    scope_ids = [enums.scopes.name_to_id["public"]]

    with pytest.raises(asyncpg.ForeignKeyViolationError):
        await db_pool.execute(
            """
            INSERT INTO entities (privacy_scope_ids, name, type_id, status_id)
            VALUES ($1, $2, $3, $4)
            """,
            scope_ids,
            "fk-bad-status",
            type_id,
            fake_status_id,
        )


@pytest.mark.asyncio
async def test_relationship_fk_type_id_rejects_fake(db_pool, enums):
    """Relationship FK on type_id rejects a nonexistent UUID."""

    source = await _make_entity(db_pool, enums, "fk-type-src")
    fake_type_id = uuid.uuid4()
    status_id = enums.statuses.name_to_id["active"]

    with pytest.raises((asyncpg.ForeignKeyViolationError, asyncpg.RaiseError)):
        await db_pool.execute(
            """
            INSERT INTO relationships (source_type, source_id, target_type, target_id, type_id, status_id)
            VALUES ('entity', $1, 'entity', $2, $3, $4)
            """,
            str(source["id"]),
            str(source["id"]),
            fake_type_id,
            status_id,
        )


@pytest.mark.asyncio
async def test_job_fk_parent_job_id_rejects_nonexistent(db_pool, enums):
    """Job FK on parent_job_id rejects a nonexistent job ID."""

    status_id = enums.statuses.name_to_id["active"]

    with pytest.raises(asyncpg.ForeignKeyViolationError):
        await db_pool.execute(
            """
            INSERT INTO jobs (title, status_id, parent_job_id)
            VALUES ('orphan-subtask', $1, 'XXXX-FAKE')
            """,
            status_id,
        )
