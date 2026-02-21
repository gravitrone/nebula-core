-- Fetch scope taxonomy row by ID
SELECT id, name, is_builtin
FROM privacy_scopes
WHERE id = $1::uuid;
