-- Query protocols with optional filters
SELECT
    p.id,
    p.name,
    p.title,
    p.version,
    p.protocol_type,
    p.applies_to,
    s.name AS status,
    p.tags,
    p.trusted,
    p.source_path,
    p.created_at,
    p.updated_at
FROM protocols p
JOIN statuses s ON p.status_id = s.id
WHERE ($1::text IS NULL OR s.category = $1)
  AND ($2::text IS NULL OR p.protocol_type = $2)
  AND ($3::text IS NULL OR p.name ILIKE '%' || $3 || '%' OR p.title ILIKE '%' || $3 || '%')
  AND ($5::boolean OR p.trusted IS NOT TRUE)
ORDER BY p.name
LIMIT $4;
