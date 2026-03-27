-- Query logs with filters
SELECT
    l.id,
    lt.name AS log_type,
    l.timestamp,
    l.content,
    s.name AS status,
    l.tags,
    l.notes,
    l.created_at,
    l.updated_at
FROM logs l
JOIN log_types lt ON l.log_type_id = lt.id
JOIN statuses s ON l.status_id = s.id
WHERE
    ($1::uuid IS NULL OR l.log_type_id = $1)
    AND ($2::text[] IS NULL OR l.tags && $2)
    AND s.category = $3
ORDER BY l.timestamp DESC
LIMIT $4 OFFSET $5;
