-- Update job fields
WITH updated AS (
    UPDATE jobs
    SET
        title = COALESCE($2, title),
        description = COALESCE($3, description),
        status_id = COALESCE($4, status_id),
        priority = COALESCE($5, priority),
        assigned_to = COALESCE($6::uuid, assigned_to),
        due_at = CASE
            WHEN $8::boolean THEN $7::timestamptz
            ELSE due_at
        END
    WHERE id = $1
    RETURNING *
)
SELECT
    u.id,
    u.title,
    u.description,
    u.job_type,
    u.assigned_to,
    u.agent_id,
    s.name AS status,
    u.status_reason,
    u.priority,
    u.parent_job_id,
    u.due_at,
    u.completed_at,
    u.privacy_scope_ids,
    u.created_at,
    u.updated_at
FROM updated u
JOIN statuses s ON u.status_id = s.id;
