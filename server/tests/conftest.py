"""Root test configuration: session DB setup, pool, enums, per-test cleanup."""

# Standard Library
import os
import sys
from pathlib import Path

# Third-Party
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
    "017_enterprise_defaults.sql",
    "018_context_core_rename.sql",
    "019_source_refs_and_files_uri.sql",
    "020_requires_approval_defaults.sql",
    "021_context_of_and_drop_metadata.sql",
]

TEST_DB = os.getenv("NEBULA_TEST_DB", "postgres")
TEST_SCHEMA = os.getenv("NEBULA_TEST_SCHEMA", "nebula_test")
ADMIN_SERVER_SETTINGS = {
    # Keep setup bounded without flaking on slower local schema bootstraps.
    "statement_timeout": "30000",  # ms
    "lock_timeout": "5000",  # ms
}

MUTABLE_TABLES = [
    "api_keys",
    "approval_requests",
    "audit_log",
    "semantic_search",
    "relationships",
    "jobs",
    "context_items",
    "logs",
    "files",
    "protocols",
    "entities",
    "agents",
    "external_refs",
]


def _admin_dsn(port: str | None = None) -> str:
    """DSN to connect to the default 'postgres' database for admin ops."""

    host = os.getenv("POSTGRES_HOST", "localhost")
    port = port or os.getenv("POSTGRES_PORT", "6432")
    user = os.getenv("POSTGRES_USER", "nebula")
    password = os.getenv("POSTGRES_PASSWORD", "nebula-agent-database-pass")
    return f"postgresql://{user}:{password}@{host}:{port}/postgres"


def _test_dsn(port: str | None = None) -> str:
    """DSN to connect to the test database."""

    host = os.getenv("POSTGRES_HOST", "localhost")
    port = port or os.getenv("POSTGRES_PORT", "6432")
    user = os.getenv("POSTGRES_USER", "nebula")
    password = os.getenv("POSTGRES_PASSWORD", "nebula-agent-database-pass")
    return f"postgresql://{user}:{password}@{host}:{port}/{TEST_DB}"


async def _connect_with_port_fallback(
    dsn_builder: callable,
    *,
    server_settings: dict[str, str] | None = None,
) -> tuple[asyncpg.Connection, str]:
    """Connect to Postgres, falling back to localhost:5432 when needed.

    Docker Desktop can go into recovery mode or crash while we are developing.
    This keeps local test runs usable by falling back to a local Postgres.
    """

    primary_port = os.getenv("POSTGRES_PORT", "6432")
    fallback_port = os.getenv("POSTGRES_FALLBACK_PORT", "5432")
    last_err: Exception | None = None

    for port in (primary_port, fallback_port):
        try:
            dsn = dsn_builder(port)
            conn = await asyncpg.connect(dsn, server_settings=server_settings)
            return conn, dsn
        except (asyncpg.CannotConnectNowError, OSError) as exc:
            last_err = exc

    if last_err:
        raise last_err
    raise RuntimeError("Failed to connect to Postgres")


@pytest.fixture(scope="session")
async def test_db_dsn():
    """Create a fresh test schema and run all migrations.

    Notes:
        Dropping databases can hang under Docker Desktop / bind mounts. For
        reliability, tests run in an isolated schema within a configurable DB.
    """

    conn, dsn = await _connect_with_port_fallback(
        _test_dsn, server_settings=ADMIN_SERVER_SETTINGS
    )
    try:
        # Fresh schema per run.
        await conn.execute(f"DROP SCHEMA IF EXISTS {TEST_SCHEMA} CASCADE")
        await conn.execute(f"CREATE SCHEMA {TEST_SCHEMA}")
        await conn.execute(f"SET search_path TO {TEST_SCHEMA}, public")

        for migration_file in MIGRATION_FILES:
            sql = (MIGRATIONS_DIR / migration_file).read_text(encoding="utf-8")
            await conn.execute(sql)
    finally:
        await conn.close()

    yield dsn

    conn = await asyncpg.connect(dsn, server_settings=ADMIN_SERVER_SETTINGS)
    try:
        await conn.execute(f"DROP SCHEMA IF EXISTS {TEST_SCHEMA} CASCADE")
    finally:
        await conn.close()


@pytest.fixture(scope="session")
async def db_pool(test_db_dsn):
    """Session-scoped asyncpg pool connected to the test DB."""

    pool = await asyncpg.create_pool(
        test_db_dsn,
        min_size=2,
        max_size=5,
        server_settings={"search_path": f"{TEST_SCHEMA}, public"},
    )
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
