-- Resolve relationship IDs to display edge context for approval enrichment
SELECT
    r.id::text AS id,
    rt.name AS relationship_type,
    r.source_type,
    r.source_id::text AS source_id,
    r.target_type,
    r.target_id::text AS target_id
FROM relationships r
LEFT JOIN relationship_types rt ON r.type_id = rt.id
WHERE r.id = ANY($1::uuid[]);
