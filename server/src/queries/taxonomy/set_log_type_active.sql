-- SQL query for server src queries taxonomy set_log_type_active
UPDATE log_types
SET is_active = $2
WHERE id = $1
  AND (NOT is_builtin OR $2 = TRUE)
RETURNING
    id,
    name,
    description,
    value_schema,
    is_builtin,
    is_active,
    created_at,
    updated_at;
