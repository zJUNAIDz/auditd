-- name: GetTenantByAPIKey :one
SELECT id, name, hmac_secret, active
FROM tenants
WHERE api_key = $1 AND active = TRUE;

-- name: CreateTenant :one
INSERT INTO tenants (name, api_key, hmac_secret)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetLastEventHash :one
SELECT hash
FROM audit_events
WHERE tenant_id = $1
ORDER BY created_at DESC
LIMIT 1;

-- name: InsertEvent :one
INSERT INTO audit_events (
    id, tenant_id, actor_id, actor_type, action,
    resource_type, resource_id, metadata,
    timestamp, created_at, prev_hash, hash
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8,
    $9, $10, $11, $12
)
RETURNING id;

-- name: ListEvents :many
SELECT *
FROM audit_events
WHERE tenant_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: GetEventByID :one
SELECT *
FROM audit_events
WHERE id = $1 AND tenant_id = $2;