-- Resolve job IDs to display labels for approval enrichment
SELECT id::text AS id, title AS label
FROM jobs
WHERE id = ANY($1::text[]);
