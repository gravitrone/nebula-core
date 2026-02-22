-- Get entity type_id and metadata by entity id
SELECT
    type_id,
    metadata
FROM entities
WHERE id = $1::uuid;
