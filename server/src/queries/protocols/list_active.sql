-- List all active protocols
SELECT 
    p.id,
    p.name,
    p.title,
    p.version,
    p.protocol_type,
    p.applies_to,
    p.tags,
    p.trusted,
    p.created_at
FROM protocols p
JOIN statuses s ON p.status_id = s.id
WHERE s.category = 'active'
ORDER BY p.name;
