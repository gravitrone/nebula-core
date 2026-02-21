-- Resolve entity IDs to display labels for approval enrichment
SELECT id::text AS id, name AS label
FROM entities
WHERE id = ANY($1::uuid[]);
