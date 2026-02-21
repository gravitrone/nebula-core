-- SQL query for server src queries taxonomy count_scope_usage
SELECT (
    (SELECT COUNT(*)::INT FROM entities WHERE $1::UUID = ANY(privacy_scope_ids)) +
    (SELECT COUNT(*)::INT FROM context_items WHERE $1::UUID = ANY(privacy_scope_ids)) +
    (SELECT COUNT(*)::INT FROM agents WHERE $1::UUID = ANY(scopes)) +
    (SELECT COUNT(*)::INT FROM semantic_search WHERE $1::UUID = ANY(scopes))
)::INT AS usage_count;
