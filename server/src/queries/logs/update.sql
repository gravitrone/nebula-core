-- Update log entry
WITH updated AS (
    UPDATE logs
    SET
        log_type_id = COALESCE($2, log_type_id),
        timestamp = COALESCE($3, timestamp),
        content = COALESCE($4, content),
        status_id = COALESCE($5, status_id),
        tags = COALESCE($6, tags),
        notes = COALESCE($7, notes)
    WHERE id = $1
    RETURNING *
)
SELECT
    u.id,
    lt.name AS log_type,
    u.timestamp,
    u.content,
    s.name AS status,
    u.tags,
    u.notes,
    u.created_at,
    u.updated_at
FROM updated u
JOIN log_types lt ON u.log_type_id = lt.id
JOIN statuses s ON u.status_id = s.id;
