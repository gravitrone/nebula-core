-- Update file record
WITH updated AS (
    UPDATE files
    SET
        filename = COALESCE($2, filename),
        uri = COALESCE($3, uri),
        file_path = COALESCE($4, file_path),
        mime_type = COALESCE($5, mime_type),
        size_bytes = COALESCE($6, size_bytes),
        checksum = COALESCE($7, checksum),
        status_id = COALESCE($8, status_id),
        tags = COALESCE($9, tags),
        notes = COALESCE($10, notes)
    WHERE id = $1
    RETURNING *
)
SELECT
    u.id,
    u.filename,
    u.uri,
    u.file_path,
    u.mime_type,
    u.size_bytes,
    u.checksum,
    s.name AS status,
    u.tags,
    u.notes,
    u.created_at,
    u.updated_at
FROM updated u
JOIN statuses s ON u.status_id = s.id;
