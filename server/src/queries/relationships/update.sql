-- Update relationship notes or status
UPDATE relationships
SET
    notes = COALESCE($2, notes),
    status_id = COALESCE($3::uuid, status_id)
WHERE id = $1::uuid
RETURNING
    id, source_type, source_id, target_type, target_id,
    type_id, status_id, notes, updated_at;
