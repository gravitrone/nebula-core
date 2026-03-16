-- Create new context item
INSERT INTO context_items (
    title,
    url,
    source_type,
    content,
    privacy_scope_ids,
    status_id,
    tags
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING 
    id, title, url, source_type, content,
    privacy_scope_ids, status_id, tags, created_at;
