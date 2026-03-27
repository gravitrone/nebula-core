-- Retrieve protocol by name
SELECT 
    p.id,
    p.name,
    p.title,
    p.version,
    p.content,
    p.protocol_type,
    p.applies_to,
    s.name AS status,
    p.tags,
    p.trusted,
    p.notes,
    p.source_path,
    p.created_at,
    p.updated_at
FROM protocols p
JOIN statuses s ON p.status_id = s.id
WHERE p.name = $1;
