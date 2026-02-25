-- Get the current trust mode for an agent regardless of status.
SELECT requires_approval
FROM agents
WHERE id = $1::uuid;
