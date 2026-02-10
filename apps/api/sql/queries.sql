-- name: ListUsersByEmail :many
SELECT
  u.id,
  u.tenant_id,
  u.email,
  u.full_name,
  u.password_hash,
  u.is_active,
  t.slug AS tenant_slug,
  t.name AS tenant_name
FROM users u
JOIN tenants t ON t.id = u.tenant_id
WHERE lower(u.email) = lower(sqlc.arg(email));

-- name: CreateSession :one
INSERT INTO sessions (
  tenant_id,
  user_id,
  token_hash,
  csrf_token,
  expires_at
) VALUES (
  sqlc.arg(tenant_id),
  sqlc.arg(user_id),
  sqlc.arg(token_hash),
  sqlc.arg(csrf_token),
  sqlc.arg(expires_at)
)
RETURNING *;

-- name: GetSessionPrincipalByTokenHash :one
SELECT
  s.id AS session_id,
  s.tenant_id,
  s.user_id,
  s.csrf_token,
  s.expires_at,
  u.email,
  u.full_name,
  t.slug AS tenant_slug,
  t.name AS tenant_name
FROM sessions s
JOIN users u ON u.id = s.user_id
JOIN tenants t ON t.id = s.tenant_id
WHERE s.token_hash = sqlc.arg(token_hash)
  AND s.revoked_at IS NULL
  AND s.expires_at > NOW()
  AND u.is_active = TRUE;

-- name: TouchSession :exec
UPDATE sessions
SET last_seen_at = NOW()
WHERE id = sqlc.arg(id);

-- name: RevokeSessionByID :execrows
UPDATE sessions
SET revoked_at = NOW()
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id)
  AND revoked_at IS NULL;

-- name: RevokeSessionByTokenHash :execrows
UPDATE sessions
SET revoked_at = NOW()
WHERE token_hash = sqlc.arg(token_hash)
  AND revoked_at IS NULL;

-- name: UserHasPermission :one
SELECT EXISTS (
  SELECT 1
  FROM user_roles ur
  JOIN roles r ON r.id = ur.role_id
  JOIN role_permissions rp ON rp.role_id = r.id
  JOIN permissions p ON p.id = rp.permission_id
  WHERE ur.user_id = sqlc.arg(user_id)
    AND ur.tenant_id = sqlc.arg(tenant_id)
    AND r.tenant_id = sqlc.arg(tenant_id)
    AND p.name = sqlc.arg(permission)
) AS has_permission;

-- name: CreateCustomer :one
INSERT INTO customers (
  tenant_id,
  first_name,
  last_name,
  email,
  phone,
  created_by,
  updated_by
) VALUES (
  sqlc.arg(tenant_id),
  sqlc.arg(first_name),
  sqlc.arg(last_name),
  sqlc.narg(email),
  sqlc.narg(phone),
  sqlc.arg(created_by),
  sqlc.arg(updated_by)
)
RETURNING *;

-- name: GetCustomerByID :one
SELECT
  id,
  tenant_id,
  first_name,
  last_name,
  email,
  phone,
  created_by,
  updated_by,
  created_at,
  updated_at
FROM customers
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id);

-- name: InsertAuditLog :exec
INSERT INTO audit_log (
  tenant_id,
  user_id,
  action,
  entity_type,
  entity_id,
  request_id,
  metadata
) VALUES (
  sqlc.arg(tenant_id),
  sqlc.narg(user_id),
  sqlc.arg(action),
  sqlc.arg(entity_type),
  sqlc.narg(entity_id),
  sqlc.narg(request_id),
  sqlc.arg(metadata)
);
