-- SQL query for server src queries taxonomy update_relationship_type
UPDATE relationship_types
SET
    name = COALESCE(NULLIF($2, ''), name),
    description = COALESCE($3, description),
    is_symmetric = COALESCE($4, is_symmetric),
    notes = COALESCE($5, notes)
WHERE id = $1
RETURNING
    id,
    name,
    description,
    is_symmetric,
    is_builtin,
    is_active,
    notes,
    created_at,
    updated_at;
