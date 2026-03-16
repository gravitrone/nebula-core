-- Create new entity
INSERT INTO entities (
  privacy_scope_ids,
  name,
  type_id,
  status_id,
  tags,
  source_path
)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;
