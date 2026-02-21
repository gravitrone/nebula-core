-- SQL query for server src queries taxonomy update_entity_type
UPDATE entity_types
SET
    name = COALESCE(NULLIF($2, ''), name),
    description = COALESCE($3, description),
    metadata = COALESCE($4::jsonb, metadata)
WHERE id = $1
RETURNING
    id,
    name,
    description,
    is_builtin,
    is_active,
    metadata,
    created_at,
    updated_at;
