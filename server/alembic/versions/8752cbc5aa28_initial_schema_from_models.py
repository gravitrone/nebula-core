"""initial schema from models

Revision ID: 8752cbc5aa28
Revises:
Create Date: 2026-03-16 02:18:23.049436

"""

from typing import Sequence, Union

import sqlalchemy as sa
from alembic import op
from sqlalchemy.dialects import postgresql

# Revision identifiers, used by Alembic.
revision: str = "8752cbc5aa28"
down_revision: Union[str, Sequence[str], None] = None
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    """Create all tables, functions, triggers, indexes, constraints, and seed data."""

    # =========================================================================
    # 1. Extensions
    # =========================================================================
    op.execute("CREATE EXTENSION IF NOT EXISTS vector")
    op.execute("CREATE EXTENSION IF NOT EXISTS pgcrypto")

    # =========================================================================
    # 2. Functions
    # =========================================================================

    # --- generate_job_id ---
    op.execute(r"""
CREATE OR REPLACE FUNCTION public.generate_job_id()
RETURNS TEXT
LANGUAGE plpgsql
AS $$
DECLARE
  alphabet TEXT := 'ABCDEFGHJKMNPQRSTUVWXYZ23456789';
  suffix TEXT := '';
  i INT;
  b INT;
  yr INT := EXTRACT(YEAR FROM NOW())::INT;
  q INT := ((EXTRACT(MONTH FROM NOW())::INT - 1) / 3 + 1)::INT;
  new_id TEXT;
  max_attempts INT := 10;
  attempt INT := 0;
BEGIN
  LOOP
    suffix := '';
    FOR i IN 1..4 LOOP
      b := get_byte(gen_random_bytes(1), 0);
      suffix := suffix || SUBSTR(alphabet, (b % LENGTH(alphabet)) + 1, 1);
    END LOOP;

    new_id := yr::TEXT || 'Q' || q::TEXT || '-' || suffix;

    -- Check if ID already exists
    IF NOT EXISTS (SELECT 1 FROM jobs WHERE id = new_id) THEN
      RETURN new_id;
    END IF;

    attempt := attempt + 1;
    IF attempt >= max_attempts THEN
      RAISE EXCEPTION 'Failed to generate unique job ID after % attempts', max_attempts;
    END IF;
  END LOOP;
END;
$$
""")

    # --- update_updated_at_column ---
    op.execute("""
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql
""")

    # --- validate_relationship_references (FINAL from 018 - uses context naming) ---
    op.execute("""
CREATE OR REPLACE FUNCTION validate_relationship_references()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.source_type = 'entity' THEN
        IF NOT EXISTS (SELECT 1 FROM entities WHERE id::text = NEW.source_id) THEN
            RAISE EXCEPTION 'source entity % does not exist', NEW.source_id;
        END IF;
    ELSIF NEW.source_type = 'context' THEN
        IF NOT EXISTS (SELECT 1 FROM context_items WHERE id::text = NEW.source_id) THEN
            RAISE EXCEPTION 'source context_item % does not exist', NEW.source_id;
        END IF;
    ELSIF NEW.source_type = 'log' THEN
        IF NOT EXISTS (SELECT 1 FROM logs WHERE id::text = NEW.source_id) THEN
            RAISE EXCEPTION 'source log % does not exist', NEW.source_id;
        END IF;
    ELSIF NEW.source_type = 'job' THEN
        IF NOT EXISTS (SELECT 1 FROM jobs WHERE id = NEW.source_id) THEN
            RAISE EXCEPTION 'source job % does not exist', NEW.source_id;
        END IF;
    ELSIF NEW.source_type = 'agent' THEN
        IF NOT EXISTS (SELECT 1 FROM agents WHERE id::text = NEW.source_id) THEN
            RAISE EXCEPTION 'source agent % does not exist', NEW.source_id;
        END IF;
    ELSIF NEW.source_type = 'file' THEN
        IF NOT EXISTS (SELECT 1 FROM files WHERE id::text = NEW.source_id) THEN
            RAISE EXCEPTION 'source file % does not exist', NEW.source_id;
        END IF;
    ELSIF NEW.source_type = 'protocol' THEN
        IF NOT EXISTS (SELECT 1 FROM protocols WHERE id::text = NEW.source_id) THEN
            RAISE EXCEPTION 'source protocol % does not exist', NEW.source_id;
        END IF;
    END IF;

    IF NEW.target_type = 'entity' THEN
        IF NOT EXISTS (SELECT 1 FROM entities WHERE id::text = NEW.target_id) THEN
            RAISE EXCEPTION 'target entity % does not exist', NEW.target_id;
        END IF;
    ELSIF NEW.target_type = 'context' THEN
        IF NOT EXISTS (SELECT 1 FROM context_items WHERE id::text = NEW.target_id) THEN
            RAISE EXCEPTION 'target context_item % does not exist', NEW.target_id;
        END IF;
    ELSIF NEW.target_type = 'log' THEN
        IF NOT EXISTS (SELECT 1 FROM logs WHERE id::text = NEW.target_id) THEN
            RAISE EXCEPTION 'target log % does not exist', NEW.target_id;
        END IF;
    ELSIF NEW.target_type = 'job' THEN
        IF NOT EXISTS (SELECT 1 FROM jobs WHERE id = NEW.target_id) THEN
            RAISE EXCEPTION 'target job % does not exist', NEW.target_id;
        END IF;
    ELSIF NEW.target_type = 'agent' THEN
        IF NOT EXISTS (SELECT 1 FROM agents WHERE id::text = NEW.target_id) THEN
            RAISE EXCEPTION 'target agent % does not exist', NEW.target_id;
        END IF;
    ELSIF NEW.target_type = 'file' THEN
        IF NOT EXISTS (SELECT 1 FROM files WHERE id::text = NEW.target_id) THEN
            RAISE EXCEPTION 'target file % does not exist', NEW.target_id;
        END IF;
    ELSIF NEW.target_type = 'protocol' THEN
        IF NOT EXISTS (SELECT 1 FROM protocols WHERE id::text = NEW.target_id) THEN
            RAISE EXCEPTION 'target protocol % does not exist', NEW.target_id;
        END IF;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql
""")

    # --- sync_symmetric_relationships (FINAL from 007 - with recursion guard) ---
    op.execute("""
CREATE OR REPLACE FUNCTION sync_symmetric_relationships()
RETURNS TRIGGER AS $$
DECLARE
    is_sym BOOLEAN;
BEGIN
    SELECT is_symmetric INTO is_sym
    FROM relationship_types
    WHERE id = COALESCE(NEW.type_id, OLD.type_id);

    IF is_sym THEN
        IF TG_OP = 'INSERT' THEN
            INSERT INTO relationships (
                source_type, source_id,
                target_type, target_id,
                type_id, properties
            )
            VALUES (
                NEW.target_type, NEW.target_id,
                NEW.source_type, NEW.source_id,
                NEW.type_id, NEW.properties
            )
            ON CONFLICT (source_type, source_id, target_type, target_id, type_id) DO NOTHING;

        ELSIF TG_OP = 'UPDATE' THEN
            IF current_setting('nebula.cascade_in_progress', true) = 'true' THEN
                RETURN NEW;
            END IF;

            UPDATE relationships
            SET properties = NEW.properties,
                updated_at = NOW()
            WHERE source_type = NEW.target_type
              AND source_id = NEW.target_id
              AND target_type = NEW.source_type
              AND target_id = NEW.source_id
              AND type_id = NEW.type_id;

        ELSIF TG_OP = 'DELETE' THEN
            DELETE FROM relationships
            WHERE source_type = OLD.target_type
              AND source_id = OLD.target_id
              AND target_type = OLD.source_type
              AND target_id = OLD.source_id
              AND type_id = OLD.type_id;
            RETURN OLD;
        END IF;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql
""")

    # --- cascade_status_to_relationships (FINAL from 018 - context naming + recursion guard) ---
    op.execute("""
CREATE OR REPLACE FUNCTION cascade_status_to_relationships()
RETURNS TRIGGER AS $$
DECLARE
    source_table TEXT;
    status_category TEXT;
    type_name TEXT;
BEGIN
    source_table := TG_TABLE_NAME;

    type_name := CASE
        WHEN source_table = 'entities' THEN 'entity'
        WHEN source_table = 'context_items' THEN 'context'
        WHEN source_table = 'logs' THEN 'log'
        WHEN source_table = 'jobs' THEN 'job'
        WHEN source_table = 'agents' THEN 'agent'
        WHEN source_table = 'files' THEN 'file'
        WHEN source_table = 'protocols' THEN 'protocol'
    END;

    SELECT category INTO status_category
    FROM statuses
    WHERE id = NEW.status_id;

    IF status_category = 'archived' THEN
        PERFORM set_config('nebula.cascade_in_progress', 'true', true);

        UPDATE relationships
        SET status_id = NEW.status_id,
            status_changed_at = NOW()
        WHERE (
            (source_type = type_name AND source_id = NEW.id::text)
            OR
            (target_type = type_name AND target_id = NEW.id::text)
        );

        PERFORM set_config('nebula.cascade_in_progress', 'false', true);
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql
""")

    # --- audit_trigger_function ---
    op.execute("""
CREATE OR REPLACE FUNCTION audit_trigger_function()
RETURNS TRIGGER AS $$
DECLARE
    changed_fields TEXT[];
    old_json JSONB;
    new_json JSONB;
    changed_by_type TEXT;
    changed_by_id UUID;
BEGIN
    -- Get current session variables (set by application)
    BEGIN
        changed_by_type := current_setting('app.changed_by_type', TRUE);
        changed_by_id := current_setting('app.changed_by_id', TRUE)::UUID;
    EXCEPTION
        WHEN OTHERS THEN
            changed_by_type := 'system';
            changed_by_id := NULL;
    END;

    -- Convert old and new rows to JSONB
    IF TG_OP = 'DELETE' THEN
        old_json := to_jsonb(OLD);
        new_json := NULL;
    ELSIF TG_OP = 'INSERT' THEN
        old_json := NULL;
        new_json := to_jsonb(NEW);
    ELSIF TG_OP = 'UPDATE' THEN
        old_json := to_jsonb(OLD);
        new_json := to_jsonb(NEW);

        -- Determine which fields changed
        SELECT array_agg(key)
        INTO changed_fields
        FROM jsonb_each(new_json)
        WHERE new_json->key IS DISTINCT FROM old_json->key;
    END IF;

    -- Insert audit record
    INSERT INTO audit_log (
        table_name,
        record_id,
        action,
        changed_by_type,
        changed_by_id,
        old_data,
        new_data,
        changed_fields,
        changed_at
    ) VALUES (
        TG_TABLE_NAME,
        COALESCE(NEW.id::TEXT, OLD.id::TEXT),
        lower(TG_OP),
        changed_by_type,
        changed_by_id,
        old_json,
        new_json,
        changed_fields,
        NOW()
    );

    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    ELSE
        RETURN NEW;
    END IF;
END;
$$ LANGUAGE plpgsql
""")

    # =========================================================================
    # 3. Tables
    # =========================================================================

    # --- Taxonomy tables (no FK dependencies) ---
    op.create_table(
        "entity_types",
        sa.Column("id", postgresql.UUID(as_uuid=True), server_default=sa.text("gen_random_uuid()"), nullable=False),
        sa.Column("name", sa.Text(), nullable=False),
        sa.Column("description", sa.Text(), nullable=True),
        sa.Column("is_builtin", sa.Boolean(), server_default=sa.text("false"), nullable=False),
        sa.Column("is_active", sa.Boolean(), server_default=sa.text("true"), nullable=False),
        sa.Column("metadata", postgresql.JSONB(), server_default=sa.text("'{}'"), nullable=True),
        sa.Column("value_schema", postgresql.JSONB(), nullable=True),
        sa.Column("created_at", sa.DateTime(timezone=True), server_default=sa.text("now()"), nullable=False),
        sa.Column("updated_at", sa.DateTime(timezone=True), server_default=sa.text("now()"), nullable=False),
        sa.PrimaryKeyConstraint("id"),
        sa.UniqueConstraint("name"),
    )

    op.create_table(
        "log_types",
        sa.Column("id", postgresql.UUID(as_uuid=True), server_default=sa.text("gen_random_uuid()"), nullable=False),
        sa.Column("name", sa.Text(), nullable=False),
        sa.Column("description", sa.Text(), nullable=True),
        sa.Column("is_builtin", sa.Boolean(), server_default=sa.text("false"), nullable=False),
        sa.Column("is_active", sa.Boolean(), server_default=sa.text("true"), nullable=False),
        sa.Column("metadata", postgresql.JSONB(), server_default=sa.text("'{}'"), nullable=True),
        sa.Column("value_schema", postgresql.JSONB(), nullable=True),
        sa.Column("created_at", sa.DateTime(timezone=True), server_default=sa.text("now()"), nullable=False),
        sa.Column("updated_at", sa.DateTime(timezone=True), server_default=sa.text("now()"), nullable=False),
        sa.PrimaryKeyConstraint("id"),
        sa.UniqueConstraint("name"),
    )

    op.create_table(
        "privacy_scopes",
        sa.Column("id", postgresql.UUID(as_uuid=True), server_default=sa.text("gen_random_uuid()"), nullable=False),
        sa.Column("name", sa.Text(), nullable=False),
        sa.Column("description", sa.Text(), nullable=True),
        sa.Column("is_builtin", sa.Boolean(), server_default=sa.text("false"), nullable=False),
        sa.Column("is_active", sa.Boolean(), server_default=sa.text("true"), nullable=False),
        sa.Column("metadata", postgresql.JSONB(), server_default=sa.text("'{}'"), nullable=True),
        sa.Column("created_at", sa.DateTime(timezone=True), server_default=sa.text("now()"), nullable=False),
        sa.Column("updated_at", sa.DateTime(timezone=True), server_default=sa.text("now()"), nullable=False),
        sa.PrimaryKeyConstraint("id"),
        sa.UniqueConstraint("name"),
    )

    op.create_table(
        "relationship_types",
        sa.Column("id", postgresql.UUID(as_uuid=True), server_default=sa.text("gen_random_uuid()"), nullable=False),
        sa.Column("name", sa.Text(), nullable=False),
        sa.Column("description", sa.Text(), nullable=True),
        sa.Column("is_symmetric", sa.Boolean(), server_default=sa.text("false"), nullable=False),
        sa.Column("is_builtin", sa.Boolean(), server_default=sa.text("false"), nullable=False),
        sa.Column("is_active", sa.Boolean(), server_default=sa.text("true"), nullable=False),
        sa.Column("metadata", postgresql.JSONB(), server_default=sa.text("'{}'"), nullable=True),
        sa.Column("created_at", sa.DateTime(timezone=True), server_default=sa.text("now()"), nullable=False),
        sa.Column("updated_at", sa.DateTime(timezone=True), server_default=sa.text("now()"), nullable=False),
        sa.PrimaryKeyConstraint("id"),
        sa.UniqueConstraint("name"),
    )

    op.create_table(
        "statuses",
        sa.Column("id", postgresql.UUID(as_uuid=True), server_default=sa.text("gen_random_uuid()"), nullable=False),
        sa.Column("name", sa.Text(), nullable=False),
        sa.Column("description", sa.Text(), nullable=True),
        sa.Column("category", sa.String(), server_default=sa.text("'active'"), nullable=False),
        sa.Column("is_builtin", sa.Boolean(), server_default=sa.text("false"), nullable=False),
        sa.Column("is_active", sa.Boolean(), server_default=sa.text("true"), nullable=False),
        sa.Column("metadata", postgresql.JSONB(), server_default=sa.text("'{}'"), nullable=True),
        sa.Column("created_at", sa.DateTime(timezone=True), server_default=sa.text("now()"), nullable=False),
        sa.Column("updated_at", sa.DateTime(timezone=True), server_default=sa.text("now()"), nullable=False),
        sa.PrimaryKeyConstraint("id"),
        sa.UniqueConstraint("name"),
    )

    # --- Core tables (depend on taxonomy) ---
    op.create_table(
        "agents",
        sa.Column("id", postgresql.UUID(as_uuid=True), server_default=sa.text("gen_random_uuid()"), nullable=False),
        sa.Column("name", sa.Text(), nullable=False),
        sa.Column("description", sa.Text(), nullable=True),
        sa.Column("system_prompt", sa.Text(), nullable=True),
        sa.Column("scopes", postgresql.ARRAY(postgresql.UUID(as_uuid=True)), server_default=sa.text("'{}'"), nullable=False),
        sa.Column("capabilities", postgresql.ARRAY(sa.Text()), server_default=sa.text("'{}'"), nullable=False),
        sa.Column("status_id", postgresql.UUID(as_uuid=True), sa.ForeignKey("statuses.id"), nullable=True),
        sa.Column("metadata", postgresql.JSONB(), server_default=sa.text("'{}'"), nullable=True),
        sa.Column("requires_approval", sa.Boolean(), server_default=sa.text("false"), nullable=False),
        sa.Column("created_at", sa.DateTime(timezone=True), server_default=sa.text("now()"), nullable=False),
        sa.Column("updated_at", sa.DateTime(timezone=True), server_default=sa.text("now()"), nullable=False),
        sa.PrimaryKeyConstraint("id"),
        sa.UniqueConstraint("name"),
    )

    op.create_table(
        "entities",
        sa.Column("id", postgresql.UUID(as_uuid=True), server_default=sa.text("gen_random_uuid()"), nullable=False),
        sa.Column("privacy_scope_ids", postgresql.ARRAY(postgresql.UUID(as_uuid=True)), server_default=sa.text("'{}'"), nullable=False),
        sa.Column("name", sa.Text(), nullable=False),
        sa.Column("type_id", postgresql.UUID(as_uuid=True), sa.ForeignKey("entity_types.id"), nullable=False),
        sa.Column("status_id", postgresql.UUID(as_uuid=True), sa.ForeignKey("statuses.id"), nullable=True),
        sa.Column("status_changed_at", sa.DateTime(timezone=True), nullable=True),
        sa.Column("status_reason", sa.Text(), nullable=True),
        sa.Column("tags", postgresql.ARRAY(sa.Text()), server_default=sa.text("'{}'"), nullable=False),
        sa.Column("source_path", sa.Text(), nullable=True),
        sa.Column("created_at", sa.DateTime(timezone=True), server_default=sa.text("now()"), nullable=False),
        sa.Column("updated_at", sa.DateTime(timezone=True), server_default=sa.text("now()"), nullable=False),
        sa.PrimaryKeyConstraint("id"),
    )

    op.create_table(
        "approval_requests",
        sa.Column("id", postgresql.UUID(as_uuid=True), server_default=sa.text("gen_random_uuid()"), nullable=False),
        sa.Column("job_id", sa.Text(), nullable=True),
        sa.Column("request_type", sa.Text(), nullable=False),
        sa.Column("requested_by", postgresql.UUID(as_uuid=True), nullable=True),
        sa.Column("change_details", postgresql.JSONB(), server_default=sa.text("'{}'"), nullable=True),
        sa.Column("status", sa.Text(), server_default=sa.text("'pending'"), nullable=True),
        sa.Column("reviewed_by", postgresql.UUID(as_uuid=True), nullable=True),
        sa.Column("reviewed_at", sa.DateTime(timezone=True), nullable=True),
        sa.Column("review_notes", sa.Text(), nullable=True),
        sa.Column("created_at", sa.DateTime(timezone=True), server_default=sa.text("now()"), nullable=False),
        sa.Column("execution_error", sa.Text(), nullable=True),
        sa.Column("review_details", postgresql.JSONB(), server_default=sa.text("'{}'"), nullable=False),
        sa.PrimaryKeyConstraint("id"),
    )

    op.create_table(
        "audit_log",
        sa.Column("id", postgresql.UUID(as_uuid=True), server_default=sa.text("gen_random_uuid()"), nullable=False),
        sa.Column("table_name", sa.Text(), nullable=False),
        sa.Column("record_id", sa.Text(), nullable=False),
        sa.Column("action", sa.Text(), nullable=False),
        sa.Column("changed_by_type", sa.Text(), nullable=True),
        sa.Column("changed_by_id", postgresql.UUID(as_uuid=True), nullable=True),
        sa.Column("old_data", postgresql.JSONB(), nullable=True),
        sa.Column("new_data", postgresql.JSONB(), nullable=True),
        sa.Column("changed_fields", postgresql.ARRAY(sa.Text()), nullable=True),
        sa.Column("change_reason", sa.Text(), nullable=True),
        sa.Column("changed_at", sa.DateTime(timezone=True), server_default=sa.text("now()"), nullable=False),
        sa.Column("metadata", postgresql.JSONB(), server_default=sa.text("'{}'"), nullable=True),
        sa.PrimaryKeyConstraint("id"),
    )

    op.create_table(
        "context_items",
        sa.Column("id", postgresql.UUID(as_uuid=True), server_default=sa.text("gen_random_uuid()"), nullable=False),
        sa.Column("privacy_scope_ids", postgresql.ARRAY(postgresql.UUID(as_uuid=True)), server_default=sa.text("'{}'"), nullable=False),
        sa.Column("title", sa.Text(), nullable=False),
        sa.Column("url", sa.Text(), nullable=True),
        sa.Column("source_type", sa.Text(), nullable=True),
        sa.Column("content", sa.Text(), nullable=True),
        sa.Column("status_id", postgresql.UUID(as_uuid=True), sa.ForeignKey("statuses.id"), nullable=True),
        sa.Column("status_changed_at", sa.DateTime(timezone=True), nullable=True),
        sa.Column("status_reason", sa.Text(), nullable=True),
        sa.Column("tags", postgresql.ARRAY(sa.Text()), server_default=sa.text("'{}'"), nullable=False),
        sa.Column("source_path", sa.Text(), nullable=True),
        sa.Column("created_at", sa.DateTime(timezone=True), server_default=sa.text("now()"), nullable=False),
        sa.Column("updated_at", sa.DateTime(timezone=True), server_default=sa.text("now()"), nullable=False),
        sa.PrimaryKeyConstraint("id"),
    )

    op.create_table(
        "external_refs",
        sa.Column("id", postgresql.UUID(as_uuid=True), server_default=sa.text("gen_random_uuid()"), nullable=False),
        sa.Column("node_type", sa.Text(), nullable=False),
        sa.Column("node_id", sa.Text(), nullable=False),
        sa.Column("system", sa.Text(), nullable=False),
        sa.Column("external_id", sa.Text(), nullable=False),
        sa.Column("url", sa.Text(), nullable=True),
        sa.Column("metadata", postgresql.JSONB(), server_default=sa.text("'{}'"), nullable=False),
        sa.Column("created_at", sa.DateTime(timezone=True), server_default=sa.text("now()"), nullable=False),
        sa.Column("updated_at", sa.DateTime(timezone=True), server_default=sa.text("now()"), nullable=False),
        sa.PrimaryKeyConstraint("id"),
    )

    op.create_table(
        "files",
        sa.Column("id", postgresql.UUID(as_uuid=True), server_default=sa.text("gen_random_uuid()"), nullable=False),
        sa.Column("filename", sa.Text(), nullable=False),
        sa.Column("uri", sa.Text(), nullable=True),
        sa.Column("file_path", sa.Text(), nullable=True),
        sa.Column("mime_type", sa.Text(), nullable=True),
        sa.Column("size_bytes", sa.BigInteger(), nullable=True),
        sa.Column("checksum", sa.Text(), nullable=True),
        sa.Column("privacy_scope_ids", postgresql.ARRAY(postgresql.UUID(as_uuid=True)), server_default=sa.text("'{}'"), nullable=False),
        sa.Column("status_id", postgresql.UUID(as_uuid=True), sa.ForeignKey("statuses.id"), nullable=True),
        sa.Column("tags", postgresql.ARRAY(sa.Text()), server_default=sa.text("'{}'"), nullable=False),
        sa.Column("metadata", postgresql.JSONB(), server_default=sa.text("'{}'"), nullable=True),
        sa.Column("source_path", sa.Text(), nullable=True),
        sa.Column("created_at", sa.DateTime(timezone=True), server_default=sa.text("now()"), nullable=False),
        sa.Column("updated_at", sa.DateTime(timezone=True), server_default=sa.text("now()"), nullable=False),
        sa.PrimaryKeyConstraint("id"),
    )

    op.create_table(
        "jobs",
        sa.Column("id", sa.Text(), server_default=sa.text("generate_job_id()"), nullable=False),
        sa.Column("title", sa.Text(), nullable=False),
        sa.Column("description", sa.Text(), nullable=True),
        sa.Column("assigned_to", postgresql.UUID(as_uuid=True), nullable=True),
        sa.Column("agent_id", postgresql.UUID(as_uuid=True), sa.ForeignKey("agents.id"), nullable=True),
        sa.Column("status_id", postgresql.UUID(as_uuid=True), sa.ForeignKey("statuses.id"), nullable=True),
        sa.Column("status_reason", sa.Text(), nullable=True),
        sa.Column("status_changed_at", sa.DateTime(timezone=True), nullable=True),
        sa.Column("priority", sa.String(), nullable=True),
        sa.Column("parent_job_id", sa.Text(), sa.ForeignKey("jobs.id"), nullable=True),
        sa.Column("due_at", sa.DateTime(timezone=True), nullable=True),
        sa.Column("completed_at", sa.DateTime(timezone=True), nullable=True),
        sa.Column("privacy_scope_ids", postgresql.ARRAY(postgresql.UUID(as_uuid=True)), server_default=sa.text("'{}'"), nullable=False),
        sa.Column("created_at", sa.DateTime(timezone=True), server_default=sa.text("now()"), nullable=False),
        sa.Column("updated_at", sa.DateTime(timezone=True), server_default=sa.text("now()"), nullable=False),
        sa.PrimaryKeyConstraint("id"),
    )

    op.create_table(
        "logs",
        sa.Column("id", postgresql.UUID(as_uuid=True), server_default=sa.text("gen_random_uuid()"), nullable=False),
        sa.Column("log_type_id", postgresql.UUID(as_uuid=True), sa.ForeignKey("log_types.id"), nullable=True),
        sa.Column("timestamp", sa.DateTime(timezone=True), server_default=sa.text("now()"), nullable=False),
        sa.Column("value", postgresql.JSONB(), server_default=sa.text("'{}'"), nullable=True),
        sa.Column("privacy_scope_ids", postgresql.ARRAY(postgresql.UUID(as_uuid=True)), server_default=sa.text("'{}'"), nullable=False),
        sa.Column("status_id", postgresql.UUID(as_uuid=True), sa.ForeignKey("statuses.id"), nullable=True),
        sa.Column("tags", postgresql.ARRAY(sa.Text()), server_default=sa.text("'{}'"), nullable=False),
        sa.Column("metadata", postgresql.JSONB(), server_default=sa.text("'{}'"), nullable=True),
        sa.Column("source_path", sa.Text(), nullable=True),
        sa.Column("created_at", sa.DateTime(timezone=True), server_default=sa.text("now()"), nullable=False),
        sa.Column("updated_at", sa.DateTime(timezone=True), server_default=sa.text("now()"), nullable=False),
        sa.PrimaryKeyConstraint("id"),
    )

    op.create_table(
        "protocols",
        sa.Column("id", postgresql.UUID(as_uuid=True), server_default=sa.text("gen_random_uuid()"), nullable=False),
        sa.Column("name", sa.Text(), nullable=False),
        sa.Column("title", sa.Text(), nullable=True),
        sa.Column("version", sa.Integer(), server_default=sa.text("1"), nullable=False),
        sa.Column("content", sa.Text(), nullable=True),
        sa.Column("protocol_type", sa.Text(), nullable=True),
        sa.Column("applies_to", postgresql.ARRAY(sa.Text()), server_default=sa.text("'{}'"), nullable=False),
        sa.Column("privacy_scope_ids", postgresql.ARRAY(postgresql.UUID(as_uuid=True)), server_default=sa.text("'{}'"), nullable=False),
        sa.Column("status_id", postgresql.UUID(as_uuid=True), sa.ForeignKey("statuses.id"), nullable=True),
        sa.Column("tags", postgresql.ARRAY(sa.Text()), server_default=sa.text("'{}'"), nullable=False),
        sa.Column("trusted", sa.Boolean(), server_default=sa.text("false"), nullable=False),
        sa.Column("metadata", postgresql.JSONB(), server_default=sa.text("'{}'"), nullable=True),
        sa.Column("source_path", sa.Text(), nullable=True),
        sa.Column("created_at", sa.DateTime(timezone=True), server_default=sa.text("now()"), nullable=False),
        sa.Column("updated_at", sa.DateTime(timezone=True), server_default=sa.text("now()"), nullable=False),
        sa.PrimaryKeyConstraint("id"),
    )

    op.create_table(
        "relationships",
        sa.Column("id", postgresql.UUID(as_uuid=True), server_default=sa.text("gen_random_uuid()"), nullable=False),
        sa.Column("source_type", sa.Text(), nullable=False),
        sa.Column("source_id", sa.Text(), nullable=False),
        sa.Column("target_type", sa.Text(), nullable=False),
        sa.Column("target_id", sa.Text(), nullable=False),
        sa.Column("type_id", postgresql.UUID(as_uuid=True), sa.ForeignKey("relationship_types.id"), nullable=False),
        sa.Column("status_id", postgresql.UUID(as_uuid=True), sa.ForeignKey("statuses.id"), nullable=True),
        sa.Column("status_changed_at", sa.DateTime(timezone=True), nullable=True),
        sa.Column("properties", postgresql.JSONB(), server_default=sa.text("'{}'"), nullable=True),
        sa.Column("created_at", sa.DateTime(timezone=True), server_default=sa.text("now()"), nullable=False),
        sa.Column("updated_at", sa.DateTime(timezone=True), server_default=sa.text("now()"), nullable=False),
        sa.PrimaryKeyConstraint("id"),
        sa.UniqueConstraint(
            "source_type", "source_id", "target_type", "target_id", "type_id",
            name="relationships_unique_edge",
        ),
    )

    op.create_table(
        "semantic_search",
        sa.Column("id", postgresql.UUID(as_uuid=True), server_default=sa.text("gen_random_uuid()"), nullable=False),
        sa.Column("source_type", sa.Text(), nullable=False),
        sa.Column("source_id", sa.Text(), nullable=False),
        sa.Column("segment_index", sa.Integer(), nullable=True),
        # NOTE: embedding column added via raw SQL below (vector type not in SA)
        sa.Column("scopes", postgresql.ARRAY(postgresql.UUID(as_uuid=True)), server_default=sa.text("'{}'"), nullable=False),
        sa.Column("metadata", postgresql.JSONB(), server_default=sa.text("'{}'"), nullable=True),
        sa.Column("created_at", sa.DateTime(timezone=True), server_default=sa.text("now()"), nullable=False),
        sa.PrimaryKeyConstraint("id"),
    )

    # --- Tables with FK to both agents and approval_requests ---
    op.create_table(
        "api_keys",
        sa.Column("id", postgresql.UUID(as_uuid=True), server_default=sa.text("gen_random_uuid()"), nullable=False),
        sa.Column("entity_id", postgresql.UUID(as_uuid=True), sa.ForeignKey("entities.id"), nullable=True),
        sa.Column("agent_id", postgresql.UUID(as_uuid=True), sa.ForeignKey("agents.id"), nullable=True),
        sa.Column("key_hash", sa.Text(), nullable=False),
        sa.Column("key_prefix", sa.Text(), nullable=False),
        sa.Column("name", sa.Text(), nullable=False),
        sa.Column("scopes", postgresql.ARRAY(postgresql.UUID(as_uuid=True)), server_default=sa.text("'{}'"), nullable=False),
        sa.Column("last_used_at", sa.DateTime(timezone=True), nullable=True),
        sa.Column("expires_at", sa.DateTime(timezone=True), nullable=True),
        sa.Column("revoked_at", sa.DateTime(timezone=True), nullable=True),
        sa.Column("created_at", sa.DateTime(timezone=True), server_default=sa.text("now()"), nullable=False),
        sa.PrimaryKeyConstraint("id"),
    )

    op.create_table(
        "agent_enrollment_sessions",
        sa.Column("id", postgresql.UUID(as_uuid=True), server_default=sa.text("gen_random_uuid()"), nullable=False),
        sa.Column("agent_id", postgresql.UUID(as_uuid=True), sa.ForeignKey("agents.id"), nullable=False),
        sa.Column("approval_request_id", postgresql.UUID(as_uuid=True), sa.ForeignKey("approval_requests.id"), nullable=False),
        sa.Column("status", sa.Text(), server_default=sa.text("'pending_approval'"), nullable=False),
        sa.Column("enrollment_token_hash", sa.Text(), nullable=False),
        sa.Column("requested_scope_ids", postgresql.ARRAY(postgresql.UUID(as_uuid=True)), server_default=sa.text("'{}'"), nullable=False),
        sa.Column("granted_scope_ids", postgresql.ARRAY(postgresql.UUID(as_uuid=True)), server_default=sa.text("'{}'"), nullable=True),
        sa.Column("requested_requires_approval", sa.Boolean(), server_default=sa.text("false"), nullable=False),
        sa.Column("granted_requires_approval", sa.Boolean(), nullable=True),
        sa.Column("rejected_reason", sa.Text(), nullable=True),
        sa.Column("approved_by", postgresql.UUID(as_uuid=True), nullable=True),
        sa.Column("approved_at", sa.DateTime(timezone=True), nullable=True),
        sa.Column("redeemed_at", sa.DateTime(timezone=True), nullable=True),
        sa.Column("expires_at", sa.DateTime(timezone=True), nullable=False),
        sa.Column("created_at", sa.DateTime(timezone=True), server_default=sa.text("now()"), nullable=False),
        sa.Column("updated_at", sa.DateTime(timezone=True), server_default=sa.text("now()"), nullable=False),
        sa.PrimaryKeyConstraint("id"),
    )

    # =========================================================================
    # 4. Embedding column (vector type via raw SQL)
    # =========================================================================
    op.execute("ALTER TABLE semantic_search ADD COLUMN embedding vector(1536) NOT NULL")

    # =========================================================================
    # 5. Triggers
    # =========================================================================

    # --- updated_at triggers (15 tables) ---
    op.execute("""
CREATE TRIGGER update_entities_updated_at BEFORE UPDATE ON entities
FOR EACH ROW EXECUTE FUNCTION update_updated_at_column()
""")
    op.execute("""
CREATE TRIGGER update_context_items_updated_at BEFORE UPDATE ON context_items
FOR EACH ROW EXECUTE FUNCTION update_updated_at_column()
""")
    op.execute("""
CREATE TRIGGER update_logs_updated_at BEFORE UPDATE ON logs
FOR EACH ROW EXECUTE FUNCTION update_updated_at_column()
""")
    op.execute("""
CREATE TRIGGER update_statuses_updated_at BEFORE UPDATE ON statuses
FOR EACH ROW EXECUTE FUNCTION update_updated_at_column()
""")
    op.execute("""
CREATE TRIGGER update_privacy_scopes_updated_at BEFORE UPDATE ON privacy_scopes
FOR EACH ROW EXECUTE FUNCTION update_updated_at_column()
""")
    op.execute("""
CREATE TRIGGER update_relationship_types_updated_at BEFORE UPDATE ON relationship_types
FOR EACH ROW EXECUTE FUNCTION update_updated_at_column()
""")
    op.execute("""
CREATE TRIGGER update_relationships_updated_at BEFORE UPDATE ON relationships
FOR EACH ROW EXECUTE FUNCTION update_updated_at_column()
""")
    op.execute("""
CREATE TRIGGER update_agents_updated_at BEFORE UPDATE ON agents
FOR EACH ROW EXECUTE FUNCTION update_updated_at_column()
""")
    op.execute("""
CREATE TRIGGER update_jobs_updated_at BEFORE UPDATE ON jobs
FOR EACH ROW EXECUTE FUNCTION update_updated_at_column()
""")
    op.execute("""
CREATE TRIGGER update_files_updated_at BEFORE UPDATE ON files
FOR EACH ROW EXECUTE FUNCTION update_updated_at_column()
""")
    op.execute("""
CREATE TRIGGER update_protocols_updated_at BEFORE UPDATE ON protocols
FOR EACH ROW EXECUTE FUNCTION update_updated_at_column()
""")
    op.execute("""
CREATE TRIGGER update_entity_types_updated_at BEFORE UPDATE ON entity_types
FOR EACH ROW EXECUTE FUNCTION update_updated_at_column()
""")
    op.execute("""
CREATE TRIGGER update_log_types_updated_at BEFORE UPDATE ON log_types
FOR EACH ROW EXECUTE FUNCTION update_updated_at_column()
""")
    op.execute("""
CREATE TRIGGER update_external_refs_updated_at BEFORE UPDATE ON external_refs
FOR EACH ROW EXECUTE FUNCTION update_updated_at_column()
""")
    op.execute("""
CREATE TRIGGER update_agent_enrollment_sessions_updated_at BEFORE UPDATE ON agent_enrollment_sessions
FOR EACH ROW EXECUTE FUNCTION update_updated_at_column()
""")

    # --- Validation trigger (1) ---
    op.execute("""
CREATE TRIGGER validate_relationships_trigger BEFORE INSERT OR UPDATE ON relationships
FOR EACH ROW EXECUTE FUNCTION validate_relationship_references()
""")

    # --- Symmetric sync trigger (1) ---
    op.execute("""
CREATE TRIGGER sync_symmetric_relationships_trigger AFTER INSERT OR UPDATE OR DELETE ON relationships
FOR EACH ROW EXECUTE FUNCTION sync_symmetric_relationships()
""")

    # --- Cascade status triggers (7 tables) ---
    op.execute("""
CREATE TRIGGER cascade_entity_status_trigger AFTER UPDATE OF status_id ON entities
FOR EACH ROW EXECUTE FUNCTION cascade_status_to_relationships()
""")
    op.execute("""
CREATE TRIGGER cascade_context_status_trigger AFTER UPDATE OF status_id ON context_items
FOR EACH ROW EXECUTE FUNCTION cascade_status_to_relationships()
""")
    op.execute("""
CREATE TRIGGER cascade_log_status_trigger AFTER UPDATE OF status_id ON logs
FOR EACH ROW EXECUTE FUNCTION cascade_status_to_relationships()
""")
    op.execute("""
CREATE TRIGGER cascade_job_status_trigger AFTER UPDATE OF status_id ON jobs
FOR EACH ROW EXECUTE FUNCTION cascade_status_to_relationships()
""")
    op.execute("""
CREATE TRIGGER cascade_agent_status_trigger AFTER UPDATE OF status_id ON agents
FOR EACH ROW EXECUTE FUNCTION cascade_status_to_relationships()
""")
    op.execute("""
CREATE TRIGGER cascade_file_status_trigger AFTER UPDATE OF status_id ON files
FOR EACH ROW EXECUTE FUNCTION cascade_status_to_relationships()
""")
    op.execute("""
CREATE TRIGGER cascade_protocol_status_trigger AFTER UPDATE OF status_id ON protocols
FOR EACH ROW EXECUTE FUNCTION cascade_status_to_relationships()
""")

    # --- Audit triggers (8 tables) ---
    op.execute("""
CREATE TRIGGER audit_entities_trigger AFTER INSERT OR UPDATE OR DELETE ON entities
FOR EACH ROW EXECUTE FUNCTION audit_trigger_function()
""")
    op.execute("""
CREATE TRIGGER audit_context_items_trigger AFTER INSERT OR UPDATE OR DELETE ON context_items
FOR EACH ROW EXECUTE FUNCTION audit_trigger_function()
""")
    op.execute("""
CREATE TRIGGER audit_relationships_trigger AFTER INSERT OR UPDATE OR DELETE ON relationships
FOR EACH ROW EXECUTE FUNCTION audit_trigger_function()
""")
    op.execute("""
CREATE TRIGGER audit_jobs_trigger AFTER INSERT OR UPDATE OR DELETE ON jobs
FOR EACH ROW EXECUTE FUNCTION audit_trigger_function()
""")
    op.execute("""
CREATE TRIGGER audit_agents_trigger AFTER INSERT OR UPDATE OR DELETE ON agents
FOR EACH ROW EXECUTE FUNCTION audit_trigger_function()
""")
    op.execute("""
CREATE TRIGGER audit_approval_requests_trigger AFTER INSERT OR UPDATE OR DELETE ON approval_requests
FOR EACH ROW EXECUTE FUNCTION audit_trigger_function()
""")
    op.execute("""
CREATE TRIGGER audit_protocols_trigger AFTER INSERT OR UPDATE OR DELETE ON protocols
FOR EACH ROW EXECUTE FUNCTION audit_trigger_function()
""")
    op.execute("""
CREATE TRIGGER audit_agent_enrollment_sessions_trigger AFTER INSERT OR UPDATE OR DELETE ON agent_enrollment_sessions
FOR EACH ROW EXECUTE FUNCTION audit_trigger_function()
""")

    # =========================================================================
    # 6. Indexes
    # =========================================================================

    # --- Entities ---
    op.execute("CREATE INDEX idx_entities_type_id ON entities(type_id)")
    op.execute("CREATE INDEX idx_entities_status ON entities(status_id)")
    op.execute("CREATE INDEX idx_entities_privacy ON entities USING gin(privacy_scope_ids)")
    op.execute("CREATE INDEX idx_entities_tags ON entities USING gin(tags)")
    op.execute(
        "CREATE INDEX idx_entities_search ON entities "
        "USING gin(to_tsvector('english', name))"
    )

    # --- Context Items ---
    op.execute("CREATE INDEX idx_context_status ON context_items(status_id)")
    op.execute("CREATE INDEX idx_context_privacy ON context_items USING gin(privacy_scope_ids)")
    op.execute("CREATE INDEX idx_context_tags ON context_items USING gin(tags)")
    op.execute("CREATE INDEX idx_context_source_type ON context_items(source_type)")
    op.execute(
        "CREATE INDEX idx_context_search ON context_items "
        "USING gin(to_tsvector('english', title || ' ' || COALESCE(content, '')))"
    )

    # --- Logs ---
    op.execute("CREATE INDEX idx_logs_type_id ON logs(log_type_id)")
    op.execute("CREATE INDEX idx_logs_timestamp ON logs(timestamp DESC)")
    op.execute("CREATE INDEX idx_logs_status ON logs(status_id)")
    op.execute("CREATE INDEX idx_logs_tags ON logs USING gin(tags)")
    op.execute("CREATE INDEX idx_logs_type_id_timestamp ON logs(log_type_id, timestamp DESC)")

    # --- Relationships ---
    op.execute("CREATE INDEX idx_relationships_source ON relationships(source_type, source_id)")
    op.execute("CREATE INDEX idx_relationships_target ON relationships(target_type, target_id)")
    op.execute("CREATE INDEX idx_relationships_type ON relationships(type_id)")
    op.execute("CREATE INDEX idx_relationships_status ON relationships(status_id)")
    op.execute(
        "CREATE INDEX idx_relationships_full ON relationships"
        "(source_type, source_id, target_type, target_id, type_id)"
    )

    # --- Semantic Search ---
    op.execute(
        "CREATE INDEX idx_semantic_search_vector ON semantic_search "
        "USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100)"
    )
    op.execute("CREATE INDEX idx_semantic_search_scopes ON semantic_search USING gin(scopes)")
    op.execute("CREATE INDEX idx_semantic_search_source ON semantic_search(source_type, source_id)")

    # --- Agents ---
    op.execute("CREATE INDEX idx_agents_status ON agents(status_id)")
    op.execute("CREATE INDEX idx_agents_scopes ON agents USING gin(scopes)")
    op.execute("CREATE INDEX idx_agents_capabilities ON agents USING gin(capabilities)")
    op.execute(
        "CREATE INDEX idx_agents_requires_approval ON agents(requires_approval) "
        "WHERE requires_approval = true"
    )

    # --- Jobs ---
    op.execute("CREATE INDEX idx_jobs_status ON jobs(status_id)")
    op.execute("CREATE INDEX idx_jobs_assigned_to ON jobs(assigned_to)")
    op.execute("CREATE INDEX idx_jobs_agent ON jobs(agent_id)")
    op.execute("CREATE INDEX idx_jobs_parent ON jobs(parent_job_id)")
    op.execute("CREATE INDEX idx_jobs_due_at ON jobs(due_at)")
    op.execute("CREATE INDEX idx_jobs_priority ON jobs(priority)")
    op.execute("CREATE INDEX idx_jobs_privacy ON jobs USING gin(privacy_scope_ids)")

    # --- Approval Requests ---
    op.execute("CREATE INDEX idx_approval_job ON approval_requests(job_id)")
    op.execute("CREATE INDEX idx_approval_status ON approval_requests(status)")
    op.execute("CREATE INDEX idx_approval_requested_by ON approval_requests(requested_by)")
    op.execute("CREATE INDEX idx_approval_reviewed_by ON approval_requests(reviewed_by)")

    # --- Files ---
    op.execute("CREATE INDEX idx_files_status ON files(status_id)")
    op.execute("CREATE INDEX idx_files_tags ON files USING gin(tags)")
    op.execute("CREATE INDEX idx_files_mime_type ON files(mime_type)")
    op.execute("CREATE INDEX idx_files_checksum ON files(checksum)")
    op.execute("CREATE INDEX idx_files_uri ON files(uri)")

    # --- Protocols ---
    op.execute("CREATE INDEX idx_protocols_type ON protocols(protocol_type)")
    op.execute("CREATE INDEX idx_protocols_status ON protocols(status_id)")
    op.execute("CREATE INDEX idx_protocols_tags ON protocols USING gin(tags)")
    op.execute("CREATE INDEX idx_protocols_applies_to ON protocols USING gin(applies_to)")
    op.execute("CREATE INDEX idx_protocols_name ON protocols(name)")
    op.execute(
        "CREATE INDEX idx_protocols_search ON protocols "
        "USING gin(to_tsvector('english', COALESCE(title, '') || ' ' || COALESCE(content, '')))"
    )

    # --- Audit Log ---
    op.execute("CREATE INDEX idx_audit_table_record ON audit_log(table_name, record_id)")
    op.execute("CREATE INDEX idx_audit_changed_at ON audit_log(changed_at DESC)")
    op.execute("CREATE INDEX idx_audit_changed_by ON audit_log(changed_by_type, changed_by_id)")
    op.execute("CREATE INDEX idx_audit_action ON audit_log(action)")
    op.execute("CREATE INDEX idx_audit_table_name ON audit_log(table_name)")
    op.execute("CREATE INDEX idx_audit_log_metadata_approval ON audit_log USING GIN (metadata)")

    # --- API Keys ---
    op.execute("CREATE INDEX idx_api_keys_prefix ON api_keys(key_prefix)")
    op.execute("CREATE INDEX idx_api_keys_entity ON api_keys(entity_id)")
    op.execute("CREATE INDEX idx_api_keys_agent ON api_keys(agent_id)")

    # --- Agent Enrollment Sessions ---
    op.execute("CREATE INDEX idx_agent_enroll_status ON agent_enrollment_sessions(status)")
    op.execute("CREATE INDEX idx_agent_enroll_expires_at ON agent_enrollment_sessions(expires_at)")
    op.execute(
        "CREATE UNIQUE INDEX idx_agent_enroll_pending_agent ON agent_enrollment_sessions(agent_id) "
        "WHERE status = 'pending_approval'"
    )
    op.execute(
        "CREATE UNIQUE INDEX idx_agent_enroll_approval_request "
        "ON agent_enrollment_sessions(approval_request_id)"
    )

    # --- Taxonomy tables ---
    op.execute("CREATE INDEX idx_entity_types_name ON entity_types(name)")
    op.execute("CREATE INDEX idx_log_types_name ON log_types(name)")
    op.execute("CREATE INDEX idx_privacy_scopes_active_name ON privacy_scopes(is_active, name)")
    op.execute("CREATE INDEX idx_entity_types_active_name ON entity_types(is_active, name)")
    op.execute("CREATE INDEX idx_relationship_types_active_name ON relationship_types(is_active, name)")
    op.execute("CREATE INDEX idx_log_types_active_name ON log_types(is_active, name)")

    # --- Case-insensitive unique indexes for taxonomy ---
    op.execute("CREATE UNIQUE INDEX uq_privacy_scopes_name_ci ON privacy_scopes (LOWER(name))")
    op.execute("CREATE UNIQUE INDEX uq_entity_types_name_ci ON entity_types (LOWER(name))")
    op.execute("CREATE UNIQUE INDEX uq_relationship_types_name_ci ON relationship_types (LOWER(name))")
    op.execute("CREATE UNIQUE INDEX uq_log_types_name_ci ON log_types (LOWER(name))")

    # --- External Refs ---
    op.execute(
        "CREATE UNIQUE INDEX uq_external_refs_system_external_id "
        "ON external_refs (system, external_id)"
    )
    op.execute("CREATE INDEX idx_external_refs_node ON external_refs (node_type, node_id)")
    op.execute("CREATE INDEX idx_external_refs_system ON external_refs (system)")

    # =========================================================================
    # 7. Check Constraints
    # =========================================================================

    # --- statuses.category ---
    op.execute(
        "ALTER TABLE statuses ADD CONSTRAINT statuses_category_check "
        "CHECK (category IN ('active', 'archived'))"
    )

    # --- approval_requests.status (includes approved-failed from 004) ---
    op.execute(
        "ALTER TABLE approval_requests ADD CONSTRAINT approval_requests_status_check "
        "CHECK (status IN ('pending', 'approved', 'rejected', 'approved-failed'))"
    )

    # --- api_keys owner XOR (entity_id or agent_id, not both, from 009) ---
    op.execute(
        "ALTER TABLE api_keys ADD CONSTRAINT api_keys_owner_check "
        "CHECK ((entity_id IS NOT NULL AND agent_id IS NULL) "
        "OR (entity_id IS NULL AND agent_id IS NOT NULL))"
    )

    # --- relationships source/target type checks (context naming from 018) ---
    op.execute(
        "ALTER TABLE relationships ADD CONSTRAINT relationships_source_type_check "
        "CHECK (source_type IN ('entity', 'context', 'log', 'job', 'agent', 'file', 'protocol'))"
    )
    op.execute(
        "ALTER TABLE relationships ADD CONSTRAINT relationships_target_type_check "
        "CHECK (target_type IN ('entity', 'context', 'log', 'job', 'agent', 'file', 'protocol'))"
    )

    # --- semantic_search source_type (context naming from 018) ---
    op.execute(
        "ALTER TABLE semantic_search ADD CONSTRAINT semantic_search_source_type_check "
        "CHECK (source_type IN ('entity', 'context', 'log', 'job', 'agent', 'file', 'protocol'))"
    )

    # --- external_refs node_type (from 019) ---
    op.execute(
        "ALTER TABLE external_refs ADD CONSTRAINT external_refs_node_type_check "
        "CHECK (node_type IN ('entity', 'context', 'log', 'job', 'agent', 'file', 'protocol'))"
    )
    op.execute(
        "ALTER TABLE external_refs ADD CONSTRAINT external_refs_metadata_is_object "
        "CHECK (jsonb_typeof(metadata) = 'object')"
    )

    # --- enrollment sessions status + token (from 014) ---
    op.execute(
        "ALTER TABLE agent_enrollment_sessions "
        "ADD CONSTRAINT agent_enrollment_sessions_status_check "
        "CHECK (status IN ('pending_approval', 'approved', 'rejected', 'redeemed', 'expired'))"
    )
    op.execute(
        "ALTER TABLE agent_enrollment_sessions "
        "ADD CONSTRAINT agent_enrollment_sessions_token_nonempty "
        "CHECK (char_length(enrollment_token_hash) > 0)"
    )

    # --- Tag limits (from 011) ---
    op.execute(
        "ALTER TABLE entities ADD CONSTRAINT entities_tags_limit "
        "CHECK (COALESCE(array_length(tags, 1), 0) <= 50)"
    )
    op.execute(
        "ALTER TABLE context_items ADD CONSTRAINT context_items_tags_limit "
        "CHECK (COALESCE(array_length(tags, 1), 0) <= 50)"
    )
    op.execute(
        "ALTER TABLE logs ADD CONSTRAINT logs_tags_limit "
        "CHECK (COALESCE(array_length(tags, 1), 0) <= 50)"
    )
    op.execute(
        "ALTER TABLE files ADD CONSTRAINT files_tags_limit "
        "CHECK (COALESCE(array_length(tags, 1), 0) <= 50)"
    )
    op.execute(
        "ALTER TABLE protocols ADD CONSTRAINT protocols_tags_limit "
        "CHECK (COALESCE(array_length(tags, 1), 0) <= 50)"
    )

    # --- metadata_is_object constraints (taxonomy from 012, others from 000) ---
    op.execute(
        "ALTER TABLE privacy_scopes ADD CONSTRAINT privacy_scopes_metadata_is_object "
        "CHECK (jsonb_typeof(metadata) = 'object')"
    )
    op.execute(
        "ALTER TABLE entity_types ADD CONSTRAINT entity_types_metadata_is_object "
        "CHECK (jsonb_typeof(metadata) = 'object')"
    )
    op.execute(
        "ALTER TABLE relationship_types ADD CONSTRAINT relationship_types_metadata_is_object "
        "CHECK (jsonb_typeof(metadata) = 'object')"
    )
    op.execute(
        "ALTER TABLE agents ADD CONSTRAINT agents_metadata_is_object "
        "CHECK (jsonb_typeof(metadata) = 'object')"
    )
    op.execute(
        "ALTER TABLE files ADD CONSTRAINT files_metadata_is_object "
        "CHECK (jsonb_typeof(metadata) = 'object')"
    )
    op.execute(
        "ALTER TABLE protocols ADD CONSTRAINT protocols_metadata_is_object "
        "CHECK (jsonb_typeof(metadata) = 'object')"
    )
    op.execute(
        "ALTER TABLE logs ADD CONSTRAINT logs_value_is_object "
        "CHECK (jsonb_typeof(value) = 'object')"
    )
    op.execute(
        "ALTER TABLE approval_requests ADD CONSTRAINT change_details_is_object "
        "CHECK (jsonb_typeof(change_details) = 'object')"
    )
    op.execute(
        "ALTER TABLE approval_requests ADD CONSTRAINT approval_requests_review_details_is_object "
        "CHECK (jsonb_typeof(review_details) = 'object')"
    )

    # --- jobs priority check ---
    op.execute(
        "ALTER TABLE jobs ADD CONSTRAINT jobs_priority_check "
        "CHECK (priority IN ('low', 'medium', 'high', 'critical'))"
    )

    # --- audit_log checks ---
    op.execute(
        "ALTER TABLE audit_log ADD CONSTRAINT audit_log_action_check "
        "CHECK (action IN ('insert', 'update', 'delete'))"
    )
    op.execute(
        "ALTER TABLE audit_log ADD CONSTRAINT audit_log_changed_by_type_check "
        "CHECK (changed_by_type IN ('agent', 'entity', 'system'))"
    )

    # =========================================================================
    # 8. Seed Data
    # =========================================================================

    # --- Statuses (9) ---
    op.execute("""
INSERT INTO statuses (name, description, category, is_builtin, is_active) VALUES
    ('active', 'Currently active and in use', 'active', true, true),
    ('in-progress', 'Actively being worked on', 'active', true, true),
    ('planning', 'In ideation/planning phase', 'active', true, true),
    ('on-hold', 'Paused temporarily, will resume', 'active', true, true),
    ('completed', 'Successfully finished', 'archived', true, true),
    ('abandoned', 'Gave up, will not finish', 'archived', true, true),
    ('replaced', 'Superseded by something better', 'archived', true, true),
    ('deleted', 'Soft delete, can be restored', 'archived', true, true),
    ('inactive', 'Not using, undecided if will return', 'archived', true, true)
ON CONFLICT (name) DO NOTHING
""")

    # --- Privacy Scopes (4 builtin) ---
    op.execute("""
INSERT INTO privacy_scopes (name, description, is_builtin, is_active) VALUES
    ('public', 'Accessible to all agents', true, true),
    ('private', 'Private data visible only to explicitly permitted actors', true, true),
    ('sensitive', 'High-risk data requiring stricter controls', true, true),
    ('admin', 'Administrative and governance operations', true, true)
ON CONFLICT (name) DO NOTHING
""")

    # --- Entity Types (5 builtin) ---
    op.execute("""
INSERT INTO entity_types (name, description, is_builtin, is_active) VALUES
    ('person', 'A human individual', true, true),
    ('organization', 'A company, team, or institution', true, true),
    ('project', 'A product or initiative', true, true),
    ('tool', 'Software, model, or utility', true, true),
    ('document', 'A document, note, or specification', true, true)
ON CONFLICT (name) DO NOTHING
""")

    # --- Relationship Types (11 builtin, including context-of from 021) ---
    op.execute("""
INSERT INTO relationship_types (name, description, is_symmetric, is_builtin, is_active) VALUES
    ('related-to', 'General relationship between records', true, true, true),
    ('depends-on', 'Source depends on target', false, true, true),
    ('references', 'Source references target', false, true, true),
    ('blocks', 'Source blocks target', false, true, true),
    ('assigned-to', 'Source is assigned to target', false, true, true),
    ('owns', 'Source owns target', false, true, true),
    ('about', 'Source is about target', false, true, true),
    ('mentions', 'Source mentions target', false, true, true),
    ('created-by', 'Source created by target', false, true, true),
    ('has-file', 'Source has file attachment target', false, true, true),
    ('context-of', 'Context item used as scoped metadata for an owner', false, true, true)
ON CONFLICT (name) DO NOTHING
""")

    # --- Log Types (3 builtin) ---
    op.execute("""
INSERT INTO log_types (name, description, value_schema, is_builtin, is_active) VALUES
    ('event', 'Generic event log', '{"type":"object"}'::jsonb, true, true),
    ('note', 'Generic textual note log', '{"type":"object"}'::jsonb, true, true),
    ('metric', 'Generic metric/value log', '{"type":"object"}'::jsonb, true, true)
ON CONFLICT (name) DO NOTHING
""")


def downgrade() -> None:
    """Drop everything in reverse order."""

    # --- Drop triggers ---
    # Audit triggers
    op.execute("DROP TRIGGER IF EXISTS audit_agent_enrollment_sessions_trigger ON agent_enrollment_sessions")
    op.execute("DROP TRIGGER IF EXISTS audit_protocols_trigger ON protocols")
    op.execute("DROP TRIGGER IF EXISTS audit_approval_requests_trigger ON approval_requests")
    op.execute("DROP TRIGGER IF EXISTS audit_agents_trigger ON agents")
    op.execute("DROP TRIGGER IF EXISTS audit_jobs_trigger ON jobs")
    op.execute("DROP TRIGGER IF EXISTS audit_relationships_trigger ON relationships")
    op.execute("DROP TRIGGER IF EXISTS audit_context_items_trigger ON context_items")
    op.execute("DROP TRIGGER IF EXISTS audit_entities_trigger ON entities")

    # Cascade status triggers
    op.execute("DROP TRIGGER IF EXISTS cascade_protocol_status_trigger ON protocols")
    op.execute("DROP TRIGGER IF EXISTS cascade_file_status_trigger ON files")
    op.execute("DROP TRIGGER IF EXISTS cascade_agent_status_trigger ON agents")
    op.execute("DROP TRIGGER IF EXISTS cascade_job_status_trigger ON jobs")
    op.execute("DROP TRIGGER IF EXISTS cascade_log_status_trigger ON logs")
    op.execute("DROP TRIGGER IF EXISTS cascade_context_status_trigger ON context_items")
    op.execute("DROP TRIGGER IF EXISTS cascade_entity_status_trigger ON entities")

    # Symmetric sync trigger
    op.execute("DROP TRIGGER IF EXISTS sync_symmetric_relationships_trigger ON relationships")

    # Validation trigger
    op.execute("DROP TRIGGER IF EXISTS validate_relationships_trigger ON relationships")

    # updated_at triggers
    op.execute("DROP TRIGGER IF EXISTS update_agent_enrollment_sessions_updated_at ON agent_enrollment_sessions")
    op.execute("DROP TRIGGER IF EXISTS update_external_refs_updated_at ON external_refs")
    op.execute("DROP TRIGGER IF EXISTS update_log_types_updated_at ON log_types")
    op.execute("DROP TRIGGER IF EXISTS update_entity_types_updated_at ON entity_types")
    op.execute("DROP TRIGGER IF EXISTS update_protocols_updated_at ON protocols")
    op.execute("DROP TRIGGER IF EXISTS update_files_updated_at ON files")
    op.execute("DROP TRIGGER IF EXISTS update_jobs_updated_at ON jobs")
    op.execute("DROP TRIGGER IF EXISTS update_agents_updated_at ON agents")
    op.execute("DROP TRIGGER IF EXISTS update_relationships_updated_at ON relationships")
    op.execute("DROP TRIGGER IF EXISTS update_relationship_types_updated_at ON relationship_types")
    op.execute("DROP TRIGGER IF EXISTS update_privacy_scopes_updated_at ON privacy_scopes")
    op.execute("DROP TRIGGER IF EXISTS update_statuses_updated_at ON statuses")
    op.execute("DROP TRIGGER IF EXISTS update_logs_updated_at ON logs")
    op.execute("DROP TRIGGER IF EXISTS update_context_items_updated_at ON context_items")
    op.execute("DROP TRIGGER IF EXISTS update_entities_updated_at ON entities")

    # --- Drop tables in reverse dependency order ---
    op.drop_table("agent_enrollment_sessions")
    op.drop_table("api_keys")
    op.drop_table("semantic_search")
    op.drop_table("relationships")
    op.drop_table("protocols")
    op.drop_table("logs")
    op.drop_table("jobs")
    op.drop_table("files")
    op.drop_table("external_refs")
    op.drop_table("context_items")
    op.drop_table("audit_log")
    op.drop_table("approval_requests")
    op.drop_table("entities")
    op.drop_table("agents")
    op.drop_table("statuses")
    op.drop_table("relationship_types")
    op.drop_table("privacy_scopes")
    op.drop_table("log_types")
    op.drop_table("entity_types")

    # --- Drop functions ---
    op.execute("DROP FUNCTION IF EXISTS audit_trigger_function() CASCADE")
    op.execute("DROP FUNCTION IF EXISTS cascade_status_to_relationships() CASCADE")
    op.execute("DROP FUNCTION IF EXISTS sync_symmetric_relationships() CASCADE")
    op.execute("DROP FUNCTION IF EXISTS validate_relationship_references() CASCADE")
    op.execute("DROP FUNCTION IF EXISTS update_updated_at_column() CASCADE")
    op.execute("DROP FUNCTION IF EXISTS generate_job_id() CASCADE")

    # --- Drop extensions ---
    op.execute("DROP EXTENSION IF EXISTS pgcrypto")
    op.execute("DROP EXTENSION IF EXISTS vector")
