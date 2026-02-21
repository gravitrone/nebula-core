-- SQL query for server src queries taxonomy create_log_type
INSERT INTO log_types (
    name,
    description,
    value_schema,
    is_builtin,
    is_active
)
VALUES (
    $1,
    NULLIF($2, ''),
    COALESCE($3::jsonb, '{"type":"object"}'::jsonb),
    FALSE,
    TRUE
)
RETURNING
    id,
    name,
    description,
    value_schema,
    is_builtin,
    is_active,
    created_at,
    updated_at;
