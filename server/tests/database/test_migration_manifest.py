"""Alembic migration manifest contract tests."""

from pathlib import Path

import pytest

pytestmark = pytest.mark.database

ALEMBIC_VERSIONS_DIR = Path(__file__).resolve().parents[2] / "alembic" / "versions"


def test_alembic_versions_directory_exists():
    """Alembic versions directory should exist with migration files."""

    assert ALEMBIC_VERSIONS_DIR.is_dir()
    migrations = list(ALEMBIC_VERSIONS_DIR.glob("*.py"))
    assert len(migrations) >= 2, "Expected at least initial + JSONB->TEXT migrations"


def test_alembic_initial_migration_exists():
    """Initial schema migration should be present."""

    files = [f.name for f in ALEMBIC_VERSIONS_DIR.glob("*.py")]
    initial = [f for f in files if "initial_schema" in f]
    assert len(initial) == 1, f"Expected exactly one initial migration, found: {initial}"


def test_alembic_jsonb_to_text_migration_exists():
    """JSONB to TEXT migration should be present."""

    files = [f.name for f in ALEMBIC_VERSIONS_DIR.glob("*.py")]
    jsonb_text = [f for f in files if "jsonb" in f.lower() or "replace" in f.lower()]
    assert len(jsonb_text) >= 1, f"Expected JSONB->TEXT migration, found: {jsonb_text}"
