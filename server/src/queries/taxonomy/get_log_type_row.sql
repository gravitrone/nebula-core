-- Fetch log type taxonomy row by ID
SELECT id, name, is_builtin
FROM log_types
WHERE id = $1::uuid;
