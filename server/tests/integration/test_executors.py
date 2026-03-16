"""Integration tests for executor functions against a real Postgres database."""

# Standard Library
import json
import re

import pytest

from nebula_mcp.executors import (
    execute_create_entity,
    execute_create_job,
    execute_create_context,
    execute_create_relationship,
    execute_update_context,
    execute_update_entity,
    execute_update_job,
    execute_update_job_status,
)

pytestmark = pytest.mark.integration


# --- TestCreateEntity ---


class TestCreateEntity:
    """Tests for execute_create_entity."""

    async def test_success(self, db_pool, enums):
        """Creating a valid entity should return a row with an id."""

        result = await execute_create_entity(
            db_pool,
            enums,
            {
                "name": "Alpha Project",
                "type": "project",
                "status": "active",
                "scopes": ["public"],
                "tags": ["test"],
            },
        )

        assert "id" in result
        assert result["name"] == "Alpha Project"

    async def test_invalid_status_raises(self, db_pool, enums):
        """An unknown status name should raise ValueError."""

        with pytest.raises(ValueError, match="Unknown status"):
            await execute_create_entity(
                db_pool,
                enums,
                {
                    "name": "Bad Status",
                    "type": "project",
                    "status": "INVALID_STATUS",
                    "scopes": ["public"],
                },
            )

    async def test_invalid_type_raises(self, db_pool, enums):
        """An unknown entity type should raise ValueError."""

        with pytest.raises(ValueError, match="Unknown entity type"):
            await execute_create_entity(
                db_pool,
                enums,
                {
                    "name": "Bad Type",
                    "type": "INVALID_TYPE",
                    "status": "active",
                    "scopes": ["public"],
                },
            )

    async def test_invalid_scopes_raises(self, db_pool, enums):
        """An unknown scope name should raise ValueError."""

        with pytest.raises(ValueError, match="Unknown scope"):
            await execute_create_entity(
                db_pool,
                enums,
                {
                    "name": "Bad Scope",
                    "type": "project",
                    "status": "active",
                    "scopes": ["INVALID_SCOPE"],
                },
            )

    async def test_source_path_allows_duplicates(self, db_pool, enums):
        """Entities may share source_path values in neutral mode."""

        await execute_create_entity(
            db_pool,
            enums,
            {
                "name": "First",
                "type": "project",
                "status": "active",
                "scopes": ["public"],
                "source_path": "00-Vault/unique-path.md",
            },
        )

        second = await execute_create_entity(
            db_pool,
            enums,
            {
                "name": "Second",
                "type": "project",
                "status": "active",
                "scopes": ["public"],
                "source_path": "00-Vault/unique-path.md",
            },
        )
        assert second["id"] != ""

    async def test_name_type_scope_dedup_raises(self, db_pool, enums):
        """Inserting two entities with the same name+type+scopes should raise."""

        await execute_create_entity(
            db_pool,
            enums,
            {
                "name": "Duplicate Test",
                "type": "tool",
                "status": "active",
                "scopes": ["public"],
            },
        )

        with pytest.raises(ValueError, match="already exists"):
            await execute_create_entity(
                db_pool,
                enums,
                {
                    "name": "Duplicate Test",
                    "type": "tool",
                    "status": "active",
                    "scopes": ["public"],
                },
            )

    async def test_name_type_scope_can_be_reused_after_archive(self, db_pool, enums):
        """Archived entities should not block same name+type+scope re-creation."""

        first = await execute_create_entity(
            db_pool,
            enums,
            {
                "name": "Reusable Name",
                "type": "project",
                "status": "active",
                "scopes": ["public"],
            },
        )

        archived = await execute_update_entity(
            db_pool,
            enums,
            {
                "entity_id": str(first["id"]),
                "status": "archived",
                "status_reason": "archive for reuse regression test",
            },
        )
        assert archived["status_id"] != first["status_id"]

        recreated = await execute_create_entity(
            db_pool,
            enums,
            {
                "name": "Reusable Name",
                "type": "project",
                "status": "active",
                "scopes": ["public"],
            },
        )
        assert recreated["id"] != first["id"]

    async def test_json_string_change_details(self, db_pool, enums):
        """Passing change_details as a JSON string should work."""

        payload = json.dumps(
            {
                "name": "JSON String Entity",
                "type": "tool",
                "status": "active",
                "scopes": ["public"],
            }
        )

        result = await execute_create_entity(db_pool, enums, payload)
        assert "id" in result


# --- TestCreateContext ---


class TestCreateContext:
    """Tests for execute_create_context."""

    async def test_success(self, db_pool, enums):
        """Creating a valid context item should return a row with an id."""

        result = await execute_create_context(
            db_pool,
            enums,
            {
                "title": "Test Article",
                "source_type": "article",
                "scopes": ["public"],
                "tags": ["test"],
            },
        )

        assert "id" in result

    async def test_url_dedup_raises(self, db_pool, enums):
        """Inserting two context items with the same URL should raise."""

        await execute_create_context(
            db_pool,
            enums,
            {
                "title": "First Article",
                "url": "https://example.com/unique",
                "source_type": "article",
                "scopes": ["public"],
            },
        )

        with pytest.raises(ValueError, match="Context item already exists for URL"):
            await execute_create_context(
                db_pool,
                enums,
                {
                    "title": "Second Article",
                    "url": "https://example.com/unique",
                    "source_type": "article",
                    "scopes": ["public"],
                },
            )

    async def test_no_url_no_dedup(self, db_pool, enums):
        """Two context items with the same title but no URL should both succeed."""

        r1 = await execute_create_context(
            db_pool,
            enums,
            {
                "title": "Same Title",
                "source_type": "note",
                "scopes": ["public"],
            },
        )

        r2 = await execute_create_context(
            db_pool,
            enums,
            {
                "title": "Same Title",
                "source_type": "note",
                "scopes": ["public"],
            },
        )

        assert r1["id"] != r2["id"]


# --- TestCreateRelationship ---


class TestCreateRelationship:
    """Tests for execute_create_relationship."""

    async def test_success(self, db_pool, enums, test_entity):
        """Creating a relationship between two entities should succeed."""

        # Create a second entity for the target
        status_id = enums.statuses.name_to_id["active"]
        type_id = enums.entity_types.name_to_id["project"]
        scope_ids = [enums.scopes.name_to_id["public"]]

        target = await db_pool.fetchrow(
            """
            INSERT INTO entities (name, type_id, status_id, privacy_scope_ids, tags)
            VALUES ($1, $2, $3, $4, $5)
            RETURNING *
            """,
            "Target Project",
            type_id,
            status_id,
            scope_ids,
            ["test"],
        )

        result = await execute_create_relationship(
            db_pool,
            enums,
            {
                "source_type": "entity",
                "source_id": str(test_entity["id"]),
                "target_type": "entity",
                "target_id": str(target["id"]),
                "relationship_type": "depends-on",
            },
        )

        assert "id" in result

    async def test_invalid_type_raises(self, db_pool, enums, test_entity):
        """An unknown relationship type should raise ValueError."""

        with pytest.raises(ValueError, match="Unknown relationship type"):
            await execute_create_relationship(
                db_pool,
                enums,
                {
                    "source_type": "entity",
                    "source_id": str(test_entity["id"]),
                    "target_type": "entity",
                    "target_id": str(test_entity["id"]),
                    "relationship_type": "INVALID_REL_TYPE",
                },
            )


# --- TestCreateJob ---


class TestCreateJob:
    """Tests for execute_create_job."""

    async def test_success(self, db_pool, enums):
        """Creating a valid job should return a row with an id."""

        result = await execute_create_job(
            db_pool,
            enums,
            {
                "title": "Test Job",
                "description": "A test job",
                "priority": "medium",
            },
        )

        assert "id" in result

    async def test_id_format(self, db_pool, enums):
        """Job ID should match the YYYYQ#-XXXX format."""

        result = await execute_create_job(
            db_pool,
            enums,
            {
                "title": "Format Check Job",
                "priority": "high",
            },
        )

        assert re.match(r"^\d{4}Q[1-4]-[A-Z0-9]{4}$", result["id"])

    async def test_due_at_iso_string_is_parsed(self, db_pool, enums):
        """ISO due_at strings from approval payloads should execute cleanly."""

        result = await execute_create_job(
            db_pool,
            enums,
            {
                "title": "Due At Parse Job",
                "priority": "high",
                "due_at": "2026-02-18T18:00:00Z",
            },
        )

        assert result["due_at"] is not None


class TestUpdateJobStatus:
    """Tests for execute_update_job_status."""

    async def test_completed_at_iso_string_is_parsed(self, db_pool, enums):
        """ISO completed_at strings should execute cleanly."""

        created = await execute_create_job(
            db_pool,
            enums,
            {
                "title": "Status Date Parse Job",
                "priority": "medium",
            },
        )

        updated = await execute_update_job_status(
            db_pool,
            enums,
            {
                "job_id": created["id"],
                "status": "completed",
                "completed_at": "2026-02-18T18:00:00Z",
            },
        )

        assert updated["completed_at"] is not None


class TestUpdateJob:
    """Tests for execute_update_job."""

    async def test_due_at_null_clears_existing_value(self, db_pool, enums):
        """Explicit due_at null in approval payload should clear due_at."""

        created = await execute_create_job(
            db_pool,
            enums,
            {
                "title": "Executor Due Clear",
                "priority": "medium",
                "due_at": "2026-02-18T18:00:00Z",
            },
        )
        assert created["due_at"] is not None

        updated = await execute_update_job(
            db_pool,
            enums,
            {
                "job_id": created["id"],
                "due_at": None,
            },
        )
        assert updated["due_at"] is None


# --- TestUpdateEntity ---


class TestUpdateEntity:
    """Tests for execute_update_entity."""

    async def test_status_change(self, db_pool, enums, test_entity):
        """Updating an entity status should return the updated row."""

        result = await execute_update_entity(
            db_pool,
            enums,
            {
                "entity_id": str(test_entity["id"]),
                "status": "on-hold",
                "status_reason": "Integration test pause",
            },
        )

        assert result["status_id"] == enums.statuses.name_to_id["on-hold"]

    async def test_nonexistent_raises(self, db_pool, enums):
        """Updating a nonexistent entity should raise ValueError."""

        with pytest.raises(ValueError, match="not found"):
            await execute_update_entity(
                db_pool,
                enums,
                {
                    "entity_id": "00000000-0000-0000-0000-000000000000",
                },
            )


# --- TestUpdateContext ---


class TestUpdateContext:
    """Tests for execute_update_context."""

    async def test_nonexistent_context_raises(self, db_pool, enums):
        """Updating a missing context row should raise ValueError."""

        with pytest.raises(ValueError, match="Context not found"):
            await execute_update_context(
                db_pool,
                enums,
                {
                    "context_id": "00000000-0000-0000-0000-000000000000",
                },
            )
