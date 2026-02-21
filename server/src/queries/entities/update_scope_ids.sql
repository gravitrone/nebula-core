-- Update entity scope IDs and return refreshed row
UPDATE entities
SET privacy_scope_ids = $2::uuid[], updated_at = NOW()
WHERE id = $1::uuid
RETURNING *;
