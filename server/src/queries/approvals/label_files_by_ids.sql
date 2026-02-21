-- Resolve file IDs to display labels for approval enrichment
SELECT id::text AS id, filename AS label
FROM files
WHERE id = ANY($1::uuid[]);
