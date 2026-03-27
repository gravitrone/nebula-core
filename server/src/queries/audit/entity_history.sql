-- List audit log entries for a single entity
SELECT
  id,
  table_name,
  record_id,
  action,
  changed_by_type,
  changed_by_id,
  old_values,
  new_values,
  changed_fields,
  changed_at
FROM audit_log
WHERE table_name = 'entities'
  AND record_id = $1
ORDER BY changed_at DESC
LIMIT $2
OFFSET $3;
