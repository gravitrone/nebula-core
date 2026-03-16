-- Update entity tags or status
UPDATE entities
SET 
    tags = COALESCE($2::text[], tags),
    status_id = COALESCE($3::uuid, status_id),
    status_reason = COALESCE($4::text, status_reason),
    status_changed_at = CASE WHEN $3::uuid IS NOT NULL THEN NOW() ELSE status_changed_at END
WHERE id = $1::uuid
RETURNING 
    id, name, type_id, status_id, privacy_scope_ids, 
    tags, status_reason, updated_at;
