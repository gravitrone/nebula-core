-- List audit log entries with optional filters
SELECT
  audit_log.id,
  audit_log.table_name,
  audit_log.record_id,
  audit_log.action,
  audit_log.changed_by_type,
  audit_log.changed_by_id,
  audit_log.old_values,
  audit_log.new_values,
  audit_log.changed_fields,
  audit_log.changed_at,
  audit_log.change_reason,
  audit_log.notes,
  COALESCE(entities.name, agents.name) AS actor_name
FROM audit_log
LEFT JOIN entities
  ON audit_log.changed_by_type = 'entity'
  AND audit_log.changed_by_id = entities.id
LEFT JOIN agents
  ON audit_log.changed_by_type = 'agent'
  AND audit_log.changed_by_id = agents.id
LEFT JOIN entities AS scoped_entities
  ON audit_log.table_name = 'entities'
  AND audit_log.record_id = scoped_entities.id::text
LEFT JOIN context_items AS scoped_context
  ON audit_log.table_name = 'context_items'
  AND audit_log.record_id = scoped_context.id::text
WHERE ($1::text IS NULL OR audit_log.table_name = $1)
  AND ($2::text IS NULL OR audit_log.action = $2)
  AND ($3::text IS NULL OR audit_log.changed_by_type = $3)
  AND ($4::uuid IS NULL OR audit_log.changed_by_id = $4)
  AND ($5::text IS NULL OR audit_log.record_id = $5)
  AND (
    $6::uuid IS NULL
    OR (
      scoped_entities.privacy_scope_ids && ARRAY[$6]
      OR scoped_context.privacy_scope_ids && ARRAY[$6]
    )
  )
ORDER BY audit_log.changed_at DESC
LIMIT $7
OFFSET $8;
