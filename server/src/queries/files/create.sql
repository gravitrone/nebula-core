-- Create file record
INSERT INTO files (
    filename,
    uri,
    file_path,
    mime_type,
    size_bytes,
    checksum,
    status_id,
    tags,
    notes
)
VALUES (
    $1,
    COALESCE($2, $3),
    COALESCE($3, $2),
    $4,
    $5,
    $6,
    $7,
    $8,
    $9
)
RETURNING *;
