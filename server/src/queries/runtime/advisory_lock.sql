-- Acquire transaction scoped advisory lock for a numeric key
SELECT pg_advisory_xact_lock($1);
