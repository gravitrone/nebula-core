-- Check if active entity exists with same name, type, and scopes.
-- Archived entities should not block re-creation.
SELECT e.id, e.name
FROM entities e
JOIN statuses s ON s.id = e.status_id
WHERE e.name = $1
  AND e.type_id = $2
  AND e.privacy_scope_ids = $3
  AND s.category = 'active';
