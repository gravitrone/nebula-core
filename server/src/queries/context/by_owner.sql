-- List context items linked to an owner via context-of relationship
SELECT
    c.id,
    c.title,
    c.url,
    c.source_type,
    c.content,
    c.privacy_scope_ids,
    s.name AS status,
    c.tags,
    c.source_path,
    c.created_at,
    c.updated_at
FROM relationships r
JOIN context_items c ON c.id::text = r.target_id
JOIN statuses s ON c.status_id = s.id
WHERE r.source_type = $1
  AND r.source_id = $2
  AND r.target_type = 'context'
  AND r.type_id = $3::uuid
  AND ($4::uuid[] IS NULL OR c.privacy_scope_ids && $4)
ORDER BY c.created_at ASC
LIMIT $5
OFFSET $6;
