-- Retrieve single job by ID with status name
SELECT 
    j.id,
    j.title,
    j.description,
    j.job_type,
    j.assigned_to,
    j.agent_id,
    s.name AS status,
    j.status_reason,
    j.priority,
    j.parent_job_id,
    j.due_at,
    j.completed_at,
    j.privacy_scope_ids,
    j.created_at,
    j.updated_at
FROM jobs j
JOIN statuses s ON j.status_id = s.id
WHERE j.id = $1;
