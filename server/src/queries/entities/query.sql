-- Search entities with filters and full-text search
SELECT 
    e.id,
    e.name,
    et.name AS type,
    s.name AS status,
    e.privacy_scope_ids,
    e.tags,
    e.created_at,
    e.updated_at
FROM entities e
JOIN entity_types et ON e.type_id = et.id
JOIN statuses s ON e.status_id = s.id
WHERE 
    ($1::uuid IS NULL OR e.type_id = $1)
    AND ($2::text[] IS NULL OR e.tags && $2)
    AND (
        $3::text IS NULL
        OR to_tsvector('english', e.name) @@ plainto_tsquery('english', $3)
        OR e.name ILIKE '%' || $3 || '%'
    )
    AND s.category = $4
    AND ($5::uuid[] IS NULL OR e.privacy_scope_ids && $5)
ORDER BY e.created_at DESC
LIMIT $6 OFFSET $7;
