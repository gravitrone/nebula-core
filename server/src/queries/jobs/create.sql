-- Create new job with auto-generated ID from database function
INSERT INTO jobs (
    title,
    description,
    job_type,
    assigned_to,
    agent_id,
    status_id,
    priority,
    parent_job_id,
    due_at,
    privacy_scope_ids
)
VALUES ($1, $2, $3, $4::uuid, $5::uuid, $6, $7, $8, $9::timestamptz, $10::uuid[])
RETURNING 
    id, title, description, job_type, assigned_to, agent_id,
    status_id, priority, parent_job_id, due_at, privacy_scope_ids, created_at;
