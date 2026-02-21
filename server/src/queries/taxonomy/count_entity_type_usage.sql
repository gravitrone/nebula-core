-- SQL query for server src queries taxonomy count_entity_type_usage
SELECT COUNT(*)::INT AS usage_count
FROM entities
WHERE type_id = $1::UUID;
