-- Get relationships for an item with direction filter
SELECT 
    r.id,
    r.source_type,
    r.source_id,
    r.target_type,
    r.target_id,
    rt.name AS relationship_type,
    s.name AS status,
    r.properties,
    r.created_at,
    COALESCE(es.name, ks.title, js.title) AS source_name,
    COALESCE(et.name, kt.title, jt.title) AS target_name
FROM relationships r
JOIN relationship_types rt ON r.type_id = rt.id
JOIN statuses s ON r.status_id = s.id
LEFT JOIN entities es ON r.source_type = 'entity' AND es.id::text = r.source_id
LEFT JOIN context_items ks ON r.source_type = 'context' AND ks.id::text = r.source_id
LEFT JOIN jobs js ON r.source_type = 'job' AND js.id = r.source_id
LEFT JOIN entities et ON r.target_type = 'entity' AND et.id::text = r.target_id
LEFT JOIN context_items kt ON r.target_type = 'context' AND kt.id::text = r.target_id
LEFT JOIN jobs jt ON r.target_type = 'job' AND jt.id = r.target_id
WHERE 
    CASE 
        WHEN $3 = 'outgoing' THEN r.source_type = $1 AND r.source_id = $2
        WHEN $3 = 'incoming' THEN r.target_type = $1 AND r.target_id = $2
        ELSE (r.source_type = $1 AND r.source_id = $2)
             OR (r.target_type = $1 AND r.target_id = $2)
    END
    AND ($4::text IS NULL OR rt.name = $4)
    AND (
        $5::uuid[] IS NULL
        OR (
            (
                r.source_type NOT IN ('entity', 'context')
                OR (r.source_type = 'entity' AND es.privacy_scope_ids && $5)
                OR (r.source_type = 'context' AND ks.privacy_scope_ids && $5)
            )
            AND (
                r.target_type NOT IN ('entity', 'context')
                OR (r.target_type = 'entity' AND et.privacy_scope_ids && $5)
                OR (r.target_type = 'context' AND kt.privacy_scope_ids && $5)
            )
        )
    )
    AND s.category = 'active'
ORDER BY r.created_at DESC;
