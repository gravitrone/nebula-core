-- SQL query for server src queries taxonomy update_entity_type
UPDATE entity_types
SET
    name = COALESCE(NULLIF($2, ''), name),
    description = COALESCE($3, description),
    notes = COALESCE($4, notes)
WHERE id = $1
RETURNING
    id,
    name,
    description,
    is_builtin,
    is_active,
    notes,
    created_at,
    updated_at;
