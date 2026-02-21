-- SQL query for server src queries taxonomy count_log_type_usage
SELECT COUNT(*)::INT AS usage_count
FROM logs
WHERE log_type_id = $1::UUID;
