-- SQL query for server src queries taxonomy list_entity_types
SELECT
    id,
    name,
    description,
    is_builtin,
    is_active,
    notes,
    created_at,
    updated_at
FROM entity_types
WHERE ($1::BOOLEAN OR is_active = TRUE)
  AND (
      COALESCE(NULLIF(TRIM($2::TEXT), ''), NULL) IS NULL
      OR name ILIKE '%' || TRIM($2::TEXT) || '%'
  )
ORDER BY name
LIMIT COALESCE(NULLIF($3::INT, 0), 200)
OFFSET GREATEST($4::INT, 0);
