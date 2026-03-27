-- List files with filters
SELECT
    f.id,
    f.filename,
    f.uri,
    f.file_path,
    f.mime_type,
    f.size_bytes,
    f.checksum,
    s.name AS status,
    f.tags,
    f.notes,
    f.created_at,
    f.updated_at
FROM files f
JOIN statuses s ON f.status_id = s.id
WHERE
    ($1::text[] IS NULL OR f.tags && $1)
    AND ($2::text IS NULL OR f.mime_type ILIKE '%' || $2 || '%')
    AND s.category = $3
ORDER BY f.created_at DESC
LIMIT $4 OFFSET $5;
