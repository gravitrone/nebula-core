-- Deprecated: entity metadata removed, keep query to avoid hard failures.
SELECT
    e.id,
    e.name,
    et.name AS type,
    s.name AS status,
    e.privacy_scope_ids,
    e.tags,
    NULL::jsonb AS metadata,
    e.created_at
FROM entities e
JOIN entity_types et ON e.type_id = et.id
JOIN statuses s ON e.status_id = s.id
WHERE false
LIMIT 0;
