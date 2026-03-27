-- Update protocol
UPDATE protocols
SET
    title = COALESCE($2, title),
    version = COALESCE($3, version),
    content = COALESCE($4, content),
    protocol_type = COALESCE($5, protocol_type),
    applies_to = COALESCE($6, applies_to),
    status_id = COALESCE($7, status_id),
    tags = COALESCE($8, tags),
    trusted = COALESCE($9, trusted),
    notes = COALESCE($10, notes),
    source_path = COALESCE($11, source_path)
WHERE name = $1
RETURNING
    id,
    name,
    title,
    version,
    content,
    protocol_type,
    applies_to,
    status_id,
    tags,
    trusted,
    notes,
    source_path,
    created_at,
    updated_at;
