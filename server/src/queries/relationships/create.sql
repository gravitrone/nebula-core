-- Create new relationship (polymorphic)
INSERT INTO relationships (
    source_type,
    source_id,
    target_type,
    target_id,
    type_id,
    status_id,
    notes
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING 
    id, source_type, source_id, target_type, target_id,
    type_id, status_id, notes, created_at;
