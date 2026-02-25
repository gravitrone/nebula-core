-- Align trust defaults with direct-write UX.
-- Existing rows are preserved; only defaults for new records are changed.

ALTER TABLE agents
ALTER COLUMN requires_approval SET DEFAULT false;

ALTER TABLE agent_enrollment_sessions
ALTER COLUMN requested_requires_approval SET DEFAULT false;
