-- Retrieve relationship by ID
SELECT 
    r.id,
    r.source_type,
    r.source_id,
    r.target_type,
    r.target_id,
    rt.name AS relationship_type,
    s.name AS status,
    r.notes,
    r.created_at,
    r.updated_at
FROM relationships r
JOIN relationship_types rt ON r.type_id = rt.id
JOIN statuses s ON r.status_id = s.id
WHERE r.id = $1;
