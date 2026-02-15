-- ---
-- ADD JOB PRIVACY SCOPES
-- ---
-- Purpose:
-- - Make job visibility scoped (like entities/knowledge), instead of per-agent isolation.
-- - Default existing jobs to public so multiple agents can collaborate on the same board.

ALTER TABLE jobs
ADD COLUMN IF NOT EXISTS privacy_scope_ids UUID[] NOT NULL DEFAULT '{}'::uuid[];

-- Backfill: ensure all existing jobs are at least public-visible.
UPDATE jobs
SET privacy_scope_ids = ARRAY[(SELECT id FROM privacy_scopes WHERE name = 'public')]
WHERE privacy_scope_ids = '{}'::uuid[];

CREATE INDEX IF NOT EXISTS idx_jobs_privacy ON jobs USING gin(privacy_scope_ids);

