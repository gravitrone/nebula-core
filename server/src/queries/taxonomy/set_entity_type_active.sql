-- SQL query for server src queries taxonomy set_entity_type_active
UPDATE entity_types
SET is_active = $2
WHERE id = $1
  AND (NOT is_builtin OR $2 = TRUE)
RETURNING
    id,
    name,
    description,
    is_builtin,
    is_active,
    metadata,
    created_at,
    updated_at;
