-- Revert entity fields from an audit snapshot
UPDATE entities
SET
  privacy_scope_ids = $2::uuid[],
  name = $3::text,
  type_id = $4::uuid,
  status_id = $5::uuid,
  status_changed_at = $6::timestamptz,
  status_reason = $7::text,
  tags = COALESCE($8::text[], '{}'),
  source_path = $9::text
WHERE id = $1::uuid
RETURNING
  id,
  name,
  type_id,
  status_id,
  privacy_scope_ids,
  tags,
  status_reason,
  updated_at;
