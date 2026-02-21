-- SQL query for server src queries taxonomy update_log_type
UPDATE log_types
SET
    name = COALESCE(NULLIF($2, ''), name),
    description = COALESCE($3, description),
    value_schema = COALESCE($4::jsonb, value_schema)
WHERE id = $1
RETURNING
    id,
    name,
    description,
    value_schema,
    is_builtin,
    is_active,
    created_at,
    updated_at;
