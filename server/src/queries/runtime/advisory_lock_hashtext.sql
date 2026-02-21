-- Acquire transaction scoped advisory lock from hashed text key
SELECT pg_advisory_xact_lock(hashtext($1));
