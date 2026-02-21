-- Count pending approvals requested by a single agent
SELECT COUNT(*)
FROM approval_requests
WHERE status = 'pending'
  AND requested_by = $1;
