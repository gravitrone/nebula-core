-- Create new log entry
WITH inserted AS (
    INSERT INTO logs (
        log_type_id,
        timestamp,
        content,
        status_id,
        tags,
        notes
    )
    VALUES ($1, $2, $3, $4, $5, $6)
    RETURNING *
)
SELECT
    i.id,
    lt.name AS log_type,
    i.timestamp,
    i.content,
    s.name AS status,
    i.tags,
    i.notes,
    i.created_at,
    i.updated_at
FROM inserted i
JOIN log_types lt ON i.log_type_id = lt.id
JOIN statuses s ON i.status_id = s.id;
