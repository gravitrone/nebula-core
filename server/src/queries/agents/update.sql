-- Update agent fields (description, requires_approval, scopes)
UPDATE agents
SET
    description = COALESCE($2, description),
    requires_approval = COALESCE($3, requires_approval),
    scopes = COALESCE($4, scopes),
    updated_at = NOW()
WHERE id = $1::uuid
RETURNING
    id,
    name,
    description,
    scopes,
    capabilities,
    requires_approval,
    status_id,
    notes,
    created_at,
    updated_at;
