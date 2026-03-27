-- SQL query for server src queries taxonomy set_scope_active
UPDATE privacy_scopes
SET is_active = $2
WHERE id = $1
  AND (NOT is_builtin OR $2 = TRUE)
RETURNING
    id,
    name,
    description,
    is_builtin,
    is_active,
    notes,
    created_at,
    updated_at;
