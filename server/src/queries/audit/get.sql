-- Get audit log entry by id
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
WHERE id = $1;
