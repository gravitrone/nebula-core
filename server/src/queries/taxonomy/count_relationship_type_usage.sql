-- SQL query for server src queries taxonomy count_relationship_type_usage
SELECT COUNT(*)::INT AS usage_count
FROM relationships
WHERE type_id = $1::UUID;
