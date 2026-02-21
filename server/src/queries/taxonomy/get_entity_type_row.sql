-- Fetch entity type taxonomy row by ID
SELECT id, name, is_builtin
FROM entity_types
WHERE id = $1::uuid;
