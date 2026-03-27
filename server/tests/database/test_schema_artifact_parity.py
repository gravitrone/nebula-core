"""Schema artifact parity checks against live DB tables."""

from pathlib import Path

import pytest

from tests.conftest import TEST_SCHEMA

pytestmark = pytest.mark.database

ALEMBIC_VERSIONS_DIR = Path(__file__).resolve().parents[2] / "alembic" / "versions"

CORE_TABLES = {
    "agent_enrollment_sessions",
    "agents",
    "api_keys",
    "approval_requests",
    "audit_log",
    "context_items",
    "entities",
    "entity_types",
    "external_refs",
    "files",
    "jobs",
    "log_types",
    "logs",
    "privacy_scopes",
    "protocols",
    "relationship_types",
    "relationships",
    "semantic_search",
    "statuses",
}


def test_alembic_migrations_exist():
    """Alembic versions directory should have migration files."""

    migrations = list(ALEMBIC_VERSIONS_DIR.glob("*.py"))
    assert len(migrations) >= 2, "Expected at least initial + JSONB->TEXT migrations"


@pytest.mark.asyncio
async def test_live_schema_has_core_tables(db_pool):
    """Live test schema should contain all core tables after migration."""

    rows = await db_pool.fetch(
        """
        SELECT tablename
        FROM pg_catalog.pg_tables
        WHERE schemaname = $1
        """,
        TEST_SCHEMA,
    )
    live_tables = {str(row["tablename"]) for row in rows}

    missing = sorted(CORE_TABLES - live_tables)
    assert not missing, f"Missing core tables in test schema: {missing}"
