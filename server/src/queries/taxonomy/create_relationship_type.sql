-- SQL query for server src queries taxonomy create_relationship_type
INSERT INTO relationship_types (
    name,
    description,
    is_symmetric,
    metadata,
    is_builtin,
    is_active
)
VALUES (
    $1,
    NULLIF($2, ''),
    COALESCE($3, FALSE),
    COALESCE($4::jsonb, '{}'::jsonb),
    FALSE,
    TRUE
)
RETURNING
    id,
    name,
    description,
    is_symmetric,
    is_builtin,
    is_active,
    metadata,
    created_at,
    updated_at;
