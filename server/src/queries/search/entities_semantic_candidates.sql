-- Candidate entities for semantic ranking.
SELECT
    e.id,
    e.name,
    et.name AS type,
    e.tags,
    e.privacy_scope_ids,
    e.created_at,
    e.updated_at
FROM entities e
JOIN entity_types et ON et.id = e.type_id
JOIN statuses s ON s.id = e.status_id
WHERE
    s.category = 'active'
    AND ($1::uuid[] IS NULL OR e.privacy_scope_ids && $1)
ORDER BY e.updated_at DESC
LIMIT $2;
