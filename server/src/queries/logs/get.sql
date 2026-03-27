-- Retrieve log by id
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
WHERE l.id = $1;
