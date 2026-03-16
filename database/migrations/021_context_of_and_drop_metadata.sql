-- Add context-of relationship type and drop metadata columns for entities/jobs/context_items

INSERT INTO relationship_types (name, description, is_symmetric, is_builtin, is_active, metadata)
SELECT
    'context-of',
    'Context item used as scoped metadata for an owner',
    false,
    true,
    true,
    '{}'::jsonb
WHERE NOT EXISTS (
    SELECT 1 FROM relationship_types WHERE lower(name) = lower('context-of')
);

ALTER TABLE entities DROP CONSTRAINT IF EXISTS metadata_is_object;
ALTER TABLE entities DROP COLUMN IF EXISTS metadata;

ALTER TABLE jobs DROP CONSTRAINT IF EXISTS metadata_is_object;
ALTER TABLE jobs DROP COLUMN IF EXISTS metadata;

ALTER TABLE context_items DROP CONSTRAINT IF EXISTS metadata_is_object;
ALTER TABLE context_items DROP COLUMN IF EXISTS metadata;
