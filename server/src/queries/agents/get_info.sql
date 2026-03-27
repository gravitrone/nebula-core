-- Retrieve agent configuration including system_prompt
SELECT 
    a.id,
    a.name,
    a.description,
    a.system_prompt,
    a.scopes,
    a.capabilities,
    s.name AS status,
    a.requires_approval,
    a.notes,
    a.created_at,
    a.updated_at
FROM agents a
JOIN statuses s ON a.status_id = s.id
WHERE a.name = $1;
