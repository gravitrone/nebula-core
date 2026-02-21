-- Resolve log IDs to display labels for approval enrichment
SELECT l.id::text AS id, COALESCE(lt.name, 'log') AS label
FROM logs l
LEFT JOIN log_types lt ON l.type_id = lt.id
WHERE l.id = ANY($1::uuid[]);
