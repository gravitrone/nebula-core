-- Approve approval request
UPDATE approval_requests
SET
  status = 'approved',
  reviewed_by = $2,
  reviewed_at = NOW(),
  review_notes = COALESCE($3, review_notes)
WHERE id = $1
  AND status = 'pending'
RETURNING *;
