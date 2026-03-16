"""Database tests for seed data verification."""

# Third-Party
import pytest

pytestmark = pytest.mark.database


# --- Statuses ---


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "status_name",
    [
        "active",
        "in-progress",
        "planning",
        "on-hold",
        "completed",
        "abandoned",
        "replaced",
        "deleted",
        "inactive",
    ],
)
async def test_status_exists(db_pool, status_name):
    """Each expected status exists in the statuses table."""

    row = await db_pool.fetchrow("SELECT * FROM statuses WHERE name = $1", status_name)
    assert row is not None, f"Status '{status_name}' not found"


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "status_name",
    ["active", "in-progress", "planning", "on-hold"],
)
async def test_active_statuses_have_active_category(db_pool, status_name):
    """Active statuses have category = 'active'."""

    row = await db_pool.fetchrow("SELECT category FROM statuses WHERE name = $1", status_name)
    assert row["category"] == "active"


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "status_name",
    ["completed", "abandoned", "replaced", "deleted", "inactive"],
)
async def test_archived_statuses_have_archived_category(db_pool, status_name):
    """Archived statuses have category = 'archived'."""

    row = await db_pool.fetchrow("SELECT category FROM statuses WHERE name = $1", status_name)
    assert row["category"] == "archived"


# --- Privacy Scopes ---


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "scope_name",
    [
        "public",
        "private",
        "sensitive",
        "admin",
    ],
)
async def test_enterprise_scope_exists_and_active(db_pool, scope_name):
    """Each enterprise scope exists and is active."""

    row = await db_pool.fetchrow(
        "SELECT is_active, is_builtin FROM privacy_scopes WHERE name = $1",
        scope_name,
    )
    assert row is not None, f"Scope '{scope_name}' not found"
    assert row["is_active"] is True
    assert row["is_builtin"] is True


@pytest.mark.asyncio
async def test_active_scopes_are_minimal_enterprise_set(db_pool):
    """Only the enterprise scope allowlist is active by default."""

    rows = await db_pool.fetch(
        "SELECT name FROM privacy_scopes WHERE is_active = TRUE AND is_builtin = TRUE"
    )
    names = {r["name"] for r in rows}
    assert names == {"public", "private", "sensitive", "admin"}


# --- Entity Types ---


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "type_name",
    [
        "person",
        "organization",
        "project",
        "tool",
        "document",
    ],
)
async def test_enterprise_entity_type_exists_and_active(db_pool, type_name):
    """Each enterprise entity type exists and is active."""

    row = await db_pool.fetchrow(
        "SELECT is_active, is_builtin FROM entity_types WHERE name = $1", type_name
    )
    assert row is not None, f"Entity type '{type_name}' not found"
    assert row["is_active"] is True
    assert row["is_builtin"] is True


@pytest.mark.asyncio
async def test_active_entity_types_are_minimal_enterprise_set(db_pool):
    """Only the enterprise entity type allowlist is active by default."""

    rows = await db_pool.fetch(
        "SELECT name FROM entity_types WHERE is_active = TRUE AND is_builtin = TRUE"
    )
    names = {r["name"] for r in rows}
    assert names == {"person", "organization", "project", "tool", "document"}


# --- Log Types ---


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "log_type_name",
    [
        "event",
        "note",
        "metric",
    ],
)
async def test_enterprise_log_type_exists_and_active(db_pool, log_type_name):
    """Each enterprise log type exists and is active."""

    row = await db_pool.fetchrow(
        "SELECT is_active, is_builtin FROM log_types WHERE name = $1", log_type_name
    )
    assert row is not None, f"Log type '{log_type_name}' not found"
    assert row["is_active"] is True
    assert row["is_builtin"] is True


@pytest.mark.asyncio
async def test_active_log_types_are_minimal_enterprise_set(db_pool):
    """Only the enterprise log type allowlist is active by default."""

    rows = await db_pool.fetch(
        "SELECT name FROM log_types WHERE is_active = TRUE AND is_builtin = TRUE"
    )
    names = {r["name"] for r in rows}
    assert names == {"event", "note", "metric"}


# --- Relationship Types ---


@pytest.mark.asyncio
async def test_relationship_type_related_to_is_symmetric(db_pool):
    """The related-to relationship type is symmetric."""

    row = await db_pool.fetchrow(
        "SELECT is_symmetric FROM relationship_types WHERE name = $1",
        "related-to",
    )
    assert row is not None, "Relationship type 'related-to' not found"
    assert row["is_symmetric"] is True


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "rel_type_name",
    [
        "depends-on",
        "references",
        "blocks",
        "assigned-to",
        "owns",
        "about",
        "mentions",
        "created-by",
        "has-file",
    ],
)
async def test_enterprise_relationship_types_not_symmetric(db_pool, rel_type_name):
    """Enterprise relationship types are non-symmetric (except related-to)."""

    row = await db_pool.fetchrow(
        "SELECT is_symmetric FROM relationship_types WHERE name = $1", rel_type_name
    )
    assert row is not None
    assert row["is_symmetric"] is False


@pytest.mark.asyncio
async def test_active_relationship_types_are_minimal_enterprise_set(db_pool):
    """Only the enterprise relationship type allowlist is active by default."""

    rows = await db_pool.fetch(
        "SELECT name FROM relationship_types WHERE is_active = TRUE AND is_builtin = TRUE"
    )
    names = {r["name"] for r in rows}
    assert names == {
        "related-to",
        "depends-on",
        "references",
        "blocks",
        "assigned-to",
        "owns",
        "about",
        "mentions",
        "created-by",
        "has-file",
        "context-of",
    }
