-- Search jobs with multiple filters
SELECT 
    j.id,
    j.title,
    j.description,
    j.job_type,
    j.assigned_to,
    j.agent_id,
    s.name AS status,
    j.priority,
    j.parent_job_id,
    j.due_at,
    j.completed_at,
    j.privacy_scope_ids,
    j.created_at
FROM jobs j
JOIN statuses s ON j.status_id = s.id
WHERE 
    ($1::text[] IS NULL OR s.name = ANY($1))
    AND ($2::uuid IS NULL OR j.assigned_to = $2)
    AND ($3::uuid IS NULL OR j.agent_id = $3)
    AND ($4::text IS NULL OR j.priority = $4)
    AND ($5::timestamptz IS NULL OR j.due_at < $5)
    AND ($6::timestamptz IS NULL OR j.due_at > $6)
    AND (NOT $7 OR (j.due_at < NOW() AND s.name != 'completed'))
    AND ($8::text IS NULL OR j.parent_job_id = $8)
    AND (
        $9::uuid[] IS NULL
        OR cardinality(j.privacy_scope_ids) = 0
        OR j.privacy_scope_ids && $9
    )
ORDER BY j.created_at DESC
LIMIT $10;
