-- SQL query for server src queries taxonomy list_log_types
SELECT
    id,
    name,
    description,
    value_schema,
    is_builtin,
    is_active,
    created_at,
    updated_at
FROM log_types
WHERE ($1::BOOLEAN OR is_active = TRUE)
  AND (
      COALESCE(NULLIF(TRIM($2::TEXT), ''), NULL) IS NULL
      OR name ILIKE '%' || TRIM($2::TEXT) || '%'
  )
ORDER BY name
LIMIT COALESCE(NULLIF($3::INT, 0), 200)
OFFSET GREATEST($4::INT, 0);
