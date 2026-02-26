"""Schema artifact parity checks against migrations and live DB tables."""

# Standard Library
import os
from pathlib import Path
import re

# Third-Party
import pytest

# Local
from tests.conftest import MIGRATION_FILES, TEST_SCHEMA

pytestmark = pytest.mark.database

_ROOT = Path(__file__).resolve()
_env_artifact = os.getenv("NEBULA_SCHEMA_ARTIFACT")
ARTIFACT_SCHEMA_CANDIDATES = (
    ([Path(_env_artifact).expanduser()] if _env_artifact else [])
    + [
        _ROOT.parents[4] / "Artifacts" / "schema.sql",  # 00-The-Void/Nebula/Artifacts
        _ROOT.parents[5] / "Artifacts" / "schema.sql",  # 00-The-Void/Artifacts (legacy)
    ]
)

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


def _read_artifact_schema() -> str:
    """Load the schema artifact from the shared Artifacts directory."""

    for candidate in ARTIFACT_SCHEMA_CANDIDATES:
        if candidate and candidate.is_file():
            return candidate.read_text(encoding="utf-8")
    raise AssertionError(
        "schema artifact missing, looked at: "
        + ", ".join(str(path) for path in ARTIFACT_SCHEMA_CANDIDATES if str(path))
    )


def test_artifact_schema_lists_all_runtime_migrations():
    """Schema artifact header should enumerate all runtime migrations."""

    text = _read_artifact_schema()
    listed = set(re.findall(r"-\s+(\d+_[a-z0-9_]+\.sql)", text))
    assert set(MIGRATION_FILES).issubset(listed)


@pytest.mark.asyncio
async def test_artifact_schema_mentions_live_core_tables(db_pool):
    """Artifact should include CREATE TABLE entries for current core tables."""

    text = _read_artifact_schema()
    rows = await db_pool.fetch(
        """
        SELECT tablename
        FROM pg_catalog.pg_tables
        WHERE schemaname = $1
        """,
        TEST_SCHEMA,
    )
    live_tables = {str(row["tablename"]) for row in rows}

    assert CORE_TABLES.issubset(live_tables)

    missing = sorted(
        table
        for table in CORE_TABLES
        if f"CREATE TABLE public.{table}" not in text
        and f"CREATE TABLE {table}" not in text
    )
    assert not missing, f"schema artifact missing tables: {missing}"
