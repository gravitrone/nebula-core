-- Create protocol
INSERT INTO protocols (
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
    source_path
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
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
