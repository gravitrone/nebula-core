-- Search context items with filters
SELECT 
    k.id,
    k.title,
    k.url,
    k.source_type,
    k.content,
    k.privacy_scope_ids,
    s.name AS status,
    k.tags,
    k.source_path,
    k.created_at,
    k.updated_at
FROM context_items k
JOIN statuses s ON k.status_id = s.id
WHERE 
    ($1::text IS NULL OR k.source_type = $1)
    AND ($2::text[] IS NULL OR k.tags && $2)
    AND ($3::text IS NULL OR to_tsvector('english', k.title || ' ' || COALESCE(k.content, '')) @@ plainto_tsquery('english', $3))
    AND ($4::uuid[] IS NULL OR k.privacy_scope_ids && $4)
    AND s.category = 'active'
ORDER BY k.created_at DESC
LIMIT $5 OFFSET $6;
