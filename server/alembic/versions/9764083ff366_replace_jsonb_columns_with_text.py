"""Replace 18 JSONB columns with TEXT for markdown-first storage.

Revision ID: 9764083ff366
Revises: 8752cbc5aa28
Create Date: 2026-03-27 18:28:54.373836

"""

from collections.abc import Sequence

from alembic import op

# Revision identifiers, used by Alembic.
revision: str = "9764083ff366"
down_revision: str | Sequence[str] | None = "8752cbc5aa28"
branch_labels: str | Sequence[str] | None = None
depends_on: str | Sequence[str] | None = None

# --- Standard metadata->notes conversions ---
# (table, old_column, new_column)
_METADATA_TO_NOTES = [
    ("agents", "metadata", "notes"),
    ("external_refs", "metadata", "notes"),
    ("files", "metadata", "notes"),
    ("protocols", "metadata", "notes"),
    ("semantic_search", "metadata", "notes"),
    ("entity_types", "metadata", "notes"),
    ("log_types", "metadata", "notes"),
    ("privacy_scopes", "metadata", "notes"),
    ("relationship_types", "metadata", "notes"),
    ("statuses", "metadata", "notes"),
]

# --- JSONB check constraints to drop before column removal ---
_JSONB_CONSTRAINTS = [
    ("external_refs", "external_refs_metadata_is_object"),
    ("privacy_scopes", "privacy_scopes_metadata_is_object"),
    ("entity_types", "entity_types_metadata_is_object"),
    ("relationship_types", "relationship_types_metadata_is_object"),
    ("agents", "agents_metadata_is_object"),
    ("files", "files_metadata_is_object"),
    ("protocols", "protocols_metadata_is_object"),
    ("logs", "logs_value_is_object"),
    ("approval_requests", "change_details_is_object"),
    ("approval_requests", "approval_requests_review_details_is_object"),
]


def _convert_jsonb_to_text(table: str, old_col: str, new_col: str) -> None:
    """Add TEXT column, copy JSONB data as text, drop old JSONB column."""

    op.execute(
        f"ALTER TABLE {table} ADD COLUMN {new_col} TEXT NOT NULL DEFAULT ''"
    )
    op.execute(
        f"UPDATE {table} SET {new_col} = {old_col}::text "
        f"WHERE {old_col} IS NOT NULL AND {old_col}::text != '{{}}'"
    )
    op.execute(f"ALTER TABLE {table} DROP COLUMN {old_col}")


def upgrade() -> None:
    """Replace JSONB columns with TEXT across all nebula tables."""

    # --- Drop JSONB check constraints ---
    for table, constraint in _JSONB_CONSTRAINTS:
        op.execute(f"ALTER TABLE {table} DROP CONSTRAINT IF EXISTS {constraint}")

    # --- Standard metadata -> notes conversions ---
    for table, old_col, new_col in _METADATA_TO_NOTES:
        _convert_jsonb_to_text(table, old_col, new_col)

    # --- logs: value (JSONB) -> content (TEXT), metadata -> notes ---
    _convert_jsonb_to_text("logs", "value", "content")
    _convert_jsonb_to_text("logs", "metadata", "notes")

    # --- relationships: properties (JSONB) -> notes (TEXT) ---
    _convert_jsonb_to_text("relationships", "properties", "notes")

    # --- audit_log: old_data -> old_values, new_data -> new_values, metadata -> notes ---
    _convert_jsonb_to_text("audit_log", "old_data", "old_values")
    _convert_jsonb_to_text("audit_log", "new_data", "new_values")
    _convert_jsonb_to_text("audit_log", "metadata", "notes")

    # --- approval_requests: change_details JSONB -> TEXT (keep name) ---
    op.execute(
        "ALTER TABLE approval_requests "
        "ALTER COLUMN change_details TYPE TEXT USING change_details::text"
    )
    op.execute(
        "ALTER TABLE approval_requests "
        "ALTER COLUMN change_details SET DEFAULT ''"
    )

    # --- approval_requests: merge review_details into review_notes, then drop ---
    op.execute(
        "UPDATE approval_requests "
        "SET review_notes = COALESCE(review_notes, '') || "
        "CASE WHEN review_details IS NOT NULL AND review_details::text != '{}' "
        "THEN review_details::text ELSE '' END"
    )
    op.execute("ALTER TABLE approval_requests DROP COLUMN review_details")

    # --- Update audit_trigger_function to use TEXT columns ---
    op.execute(r"""
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

    -- Convert old and new rows to JSONB for field comparison
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

    -- Insert audit record with TEXT columns
    INSERT INTO audit_log (
        table_name,
        record_id,
        action,
        changed_by_type,
        changed_by_id,
        old_values,
        new_values,
        changed_fields,
        changed_at
    ) VALUES (
        TG_TABLE_NAME,
        COALESCE(NEW.id::TEXT, OLD.id::TEXT),
        lower(TG_OP),
        changed_by_type,
        changed_by_id,
        to_jsonb(OLD)::text,
        to_jsonb(NEW)::text,
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

    # --- Create link_audit_to_approval helper function ---
    op.execute(r"""
CREATE OR REPLACE FUNCTION link_audit_to_approval(approval_uuid UUID)
RETURNS VOID AS $$
BEGIN
    UPDATE audit_log
    SET notes = COALESCE(notes, '') || 'approval_id: ' || approval_uuid::text
    WHERE id = (
        SELECT id FROM audit_log ORDER BY changed_at DESC LIMIT 1
    );
END;
$$ LANGUAGE plpgsql
""")


def downgrade() -> None:
    """Downgrade is not supported for this migration."""

    raise NotImplementedError(
        "Downgrade not supported: JSONB-to-TEXT migration is one-way. "
        "Restore from backup if rollback is needed."
    )
