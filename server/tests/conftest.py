"""Root test configuration: session DB setup, pool, enums, per-test cleanup."""

# Standard Library
import os

# Third-Party
import sys
from pathlib import Path

import asyncpg
import pytest

SRC_DIR = Path(__file__).resolve().parents[1] / "src"
sys.path.insert(0, str(SRC_DIR))

PROJECT_ROOT = Path(__file__).resolve().parents[2]
MIGRATIONS_DIR = PROJECT_ROOT / "database" / "migrations"

MIGRATION_FILES = [
    "006_pgcrypto.sql",
    "000_init.sql",
    "001_seed.sql",
    "002_entity_types.sql",
    "003_add_requires_approval.sql",
    "004_approval_execution.sql",
    "005_log_types.sql",
    "007_schema_fixes.sql",
    "008_api_keys.sql",
    "009_agent_api_keys.sql",
    "010_add_work_scope.sql",
    "011_security_hardening.sql",
    "012_taxonomy_lifecycle.sql",
    "013_taxonomy_generalization.sql",
    "014_mcp_agent_enrollment.sql",
    "015_add_default_log_types.sql",
    "016_jobs_privacy_scopes.sql",
]

TEST_DB = "nebula_test"

MUTABLE_TABLES = [
    "api_keys",
    "approval_requests",
    "audit_log",
    "semantic_search",
    "relationships",
    "jobs",
    "knowledge_items",
    "logs",
    "files",
    "protocols",
    "entities",
    "agents",
]


def _admin_dsn() -> str:
    """DSN to connect to the default 'postgres' database for admin ops."""

    host = os.getenv("POSTGRES_HOST", "localhost")
    port = os.getenv("POSTGRES_PORT", "6432")
    user = os.getenv("POSTGRES_USER", "nebula")
    password = os.getenv("POSTGRES_PASSWORD", "nebula-agent-database-pass")
    return f"postgresql://{user}:{password}@{host}:{port}/postgres"


def _test_dsn() -> str:
    """DSN to connect to the test database."""

    host = os.getenv("POSTGRES_HOST", "localhost")
    port = os.getenv("POSTGRES_PORT", "6432")
    user = os.getenv("POSTGRES_USER", "nebula")
    password = os.getenv("POSTGRES_PASSWORD", "nebula-agent-database-pass")
    return f"postgresql://{user}:{password}@{host}:{port}/{TEST_DB}"


@pytest.fixture(scope="session")
async def test_db_dsn():
    """Create a fresh test database and run all migrations."""

    admin_conn = await asyncpg.connect(_admin_dsn())
    try:
        # Drop if leftover from a previous failed run
        await admin_conn.execute(f"DROP DATABASE IF EXISTS {TEST_DB}")
        await admin_conn.execute(f"CREATE DATABASE {TEST_DB}")
    finally:
        await admin_conn.close()

    # Run migrations against the test database
    dsn = _test_dsn()
    conn = await asyncpg.connect(dsn)
    try:
        for migration_file in MIGRATION_FILES:
            sql = (MIGRATIONS_DIR / migration_file).read_text(encoding="utf-8")
            await conn.execute(sql)
    finally:
        await conn.close()

    yield dsn

    # Teardown: drop test database
    admin_conn = await asyncpg.connect(_admin_dsn())
    try:
        # Terminate active connections before drop
        await admin_conn.execute(f"""
            SELECT pg_terminate_backend(pid)
            FROM pg_stat_activity
            WHERE datname = '{TEST_DB}' AND pid <> pg_backend_pid()
        """)
        await admin_conn.execute(f"DROP DATABASE IF EXISTS {TEST_DB}")
    finally:
        await admin_conn.close()


@pytest.fixture(scope="session")
async def db_pool(test_db_dsn):
    """Session-scoped asyncpg pool connected to the test DB."""

    pool = await asyncpg.create_pool(test_db_dsn, min_size=2, max_size=5)
    yield pool
    await pool.close()


@pytest.fixture(scope="session")
async def enums(db_pool):
    """Session-scoped EnumRegistry loaded from the test DB."""

    from nebula_mcp.enums import load_enums

    return await load_enums(db_pool)


@pytest.fixture(autouse=True)
async def clean_test_data(request, db_pool):
    """Truncate mutable tables after each test (skip for unit tests)."""

    yield

    # Skip cleanup for unit tests (they don't use the DB)
    if "unit" in str(request.fspath):
        return

    async with db_pool.acquire() as conn:
        for table in MUTABLE_TABLES:
            await conn.execute(f"TRUNCATE {table} CASCADE")
