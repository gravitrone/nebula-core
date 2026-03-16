-- Retrieve single entity by ID with join for type and status names
SELECT 
    e.id,
    e.name,
    et.name AS type,
    s.name AS status,
    e.privacy_scope_ids,
    e.tags,
    e.source_path,
    e.created_at,
    e.updated_at
FROM entities e
JOIN entity_types et ON e.type_id = et.id
JOIN statuses s ON e.status_id = s.id
WHERE e.id = $1::uuid;
