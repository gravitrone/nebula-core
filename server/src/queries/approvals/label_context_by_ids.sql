-- Resolve context IDs to display labels for approval enrichment
SELECT id::text AS id, title AS label
FROM context_items
WHERE id = ANY($1::uuid[]);
