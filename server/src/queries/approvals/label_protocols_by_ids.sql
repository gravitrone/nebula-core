-- Resolve protocol IDs to display labels for approval enrichment
SELECT id::text AS id, COALESCE(title, name, 'protocol') AS label
FROM protocols
WHERE id = ANY($1::uuid[]);
