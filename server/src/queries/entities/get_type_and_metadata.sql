-- Deprecated: entity metadata removed, keep legacy shape for callers.
SELECT
    type_id,
    NULL::jsonb AS metadata
FROM entities
WHERE id = $1::uuid;
