-- Resolve agent IDs to display labels for approval enrichment
SELECT id::text AS id, name AS label
FROM agents
WHERE id = ANY($1::uuid[]);
