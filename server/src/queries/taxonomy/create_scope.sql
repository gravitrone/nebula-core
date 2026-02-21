-- SQL query for server src queries taxonomy create_scope
INSERT INTO privacy_scopes (
    name,
    description,
    metadata,
    is_builtin,
    is_active
)
VALUES (
    $1,
    NULLIF($2, ''),
    COALESCE($3::jsonb, '{}'::jsonb),
    FALSE,
    TRUE
)
RETURNING
    id,
    name,
    description,
    is_builtin,
    is_active,
    metadata,
    created_at,
    updated_at;
