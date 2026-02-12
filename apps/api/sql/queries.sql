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

-- name: CreateCustomerForEstimate :one
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
  sqlc.arg(email),
  sqlc.arg(phone),
  sqlc.arg(created_by),
  sqlc.arg(updated_by)
)
RETURNING *;

-- name: UpdateCustomerForEstimate :one
UPDATE customers
SET
  first_name = COALESCE(sqlc.narg(first_name), first_name),
  last_name = COALESCE(sqlc.narg(last_name), last_name),
  email = COALESCE(sqlc.narg(email), email),
  phone = COALESCE(sqlc.narg(phone), phone),
  updated_by = sqlc.arg(updated_by),
  updated_at = NOW()
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id)
RETURNING *;

-- name: IncrementTenantCounter :one
INSERT INTO tenant_counters (tenant_id, counter_type, next_value)
VALUES (sqlc.arg(tenant_id), sqlc.arg(counter_type), 2)
ON CONFLICT (tenant_id, counter_type) DO UPDATE
SET
  next_value = tenant_counters.next_value + 1,
  updated_at = NOW()
RETURNING (next_value - 1)::bigint AS value;

-- name: CreateEstimate :one
INSERT INTO estimates (
  tenant_id,
  estimate_number,
  customer_id,
  status,
  customer_name,
  primary_phone,
  secondary_phone,
  email,
  origin_address_line1,
  origin_city,
  origin_state,
  origin_postal_code,
  destination_address_line1,
  destination_city,
  destination_state,
  destination_postal_code,
  move_date,
  pickup_time,
  lead_source,
  move_size,
  location_type,
  estimated_total_cents,
  deposit_cents,
  notes,
  idempotency_key,
  idempotency_payload_hash,
  created_by,
  updated_by
) VALUES (
  sqlc.arg(tenant_id),
  sqlc.arg(estimate_number),
  sqlc.arg(customer_id),
  sqlc.arg(status),
  sqlc.arg(customer_name),
  sqlc.arg(primary_phone),
  sqlc.narg(secondary_phone),
  sqlc.arg(email),
  sqlc.arg(origin_address_line1),
  sqlc.arg(origin_city),
  sqlc.arg(origin_state),
  sqlc.arg(origin_postal_code),
  sqlc.arg(destination_address_line1),
  sqlc.arg(destination_city),
  sqlc.arg(destination_state),
  sqlc.arg(destination_postal_code),
  sqlc.arg(move_date),
  sqlc.narg(pickup_time),
  sqlc.arg(lead_source),
  sqlc.narg(move_size),
  sqlc.narg(location_type),
  sqlc.narg(estimated_total_cents),
  sqlc.narg(deposit_cents),
  sqlc.narg(notes),
  sqlc.arg(idempotency_key),
  sqlc.arg(idempotency_payload_hash),
  sqlc.arg(created_by),
  sqlc.arg(updated_by)
)
RETURNING *;

-- name: GetEstimateByID :one
SELECT
  id,
  tenant_id,
  estimate_number,
  customer_id,
  status,
  customer_name,
  primary_phone,
  secondary_phone,
  email,
  origin_address_line1,
  origin_city,
  origin_state,
  origin_postal_code,
  destination_address_line1,
  destination_city,
  destination_state,
  destination_postal_code,
  move_date,
  pickup_time,
  lead_source,
  move_size,
  location_type,
  estimated_total_cents,
  deposit_cents,
  notes,
  idempotency_key,
  idempotency_payload_hash,
  created_by,
  updated_by,
  created_at,
  updated_at
FROM estimates
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id);

-- name: GetEstimateDetailByID :one
SELECT
  e.id,
  e.tenant_id,
  e.estimate_number,
  e.customer_id,
  e.status,
  e.customer_name,
  e.primary_phone,
  e.secondary_phone,
  e.email,
  e.origin_address_line1,
  e.origin_city,
  e.origin_state,
  e.origin_postal_code,
  e.destination_address_line1,
  e.destination_city,
  e.destination_state,
  e.destination_postal_code,
  e.move_date,
  e.pickup_time,
  e.lead_source,
  e.move_size,
  e.location_type,
  e.estimated_total_cents,
  e.deposit_cents,
  e.notes,
  e.idempotency_key,
  e.idempotency_payload_hash,
  e.created_by,
  e.updated_by,
  e.created_at,
  e.updated_at,
  j.id AS converted_job_id
FROM estimates e
LEFT JOIN jobs j
  ON j.tenant_id = e.tenant_id
  AND j.estimate_id = e.id
WHERE e.id = sqlc.arg(id)
  AND e.tenant_id = sqlc.arg(tenant_id);

-- name: GetEstimateByIdempotencyKey :one
SELECT
  id,
  tenant_id,
  estimate_number,
  customer_id,
  status,
  customer_name,
  primary_phone,
  secondary_phone,
  email,
  origin_address_line1,
  origin_city,
  origin_state,
  origin_postal_code,
  destination_address_line1,
  destination_city,
  destination_state,
  destination_postal_code,
  move_date,
  pickup_time,
  lead_source,
  move_size,
  location_type,
  estimated_total_cents,
  deposit_cents,
  notes,
  idempotency_key,
  idempotency_payload_hash,
  created_by,
  updated_by,
  created_at,
  updated_at
FROM estimates
WHERE tenant_id = sqlc.arg(tenant_id)
  AND idempotency_key = sqlc.arg(idempotency_key);

-- name: UpdateEstimate :one
UPDATE estimates
SET
  customer_name = COALESCE(sqlc.narg(customer_name), customer_name),
  primary_phone = COALESCE(sqlc.narg(primary_phone), primary_phone),
  secondary_phone = COALESCE(sqlc.narg(secondary_phone), secondary_phone),
  email = COALESCE(sqlc.narg(email), email),
  origin_address_line1 = COALESCE(sqlc.narg(origin_address_line1), origin_address_line1),
  origin_city = COALESCE(sqlc.narg(origin_city), origin_city),
  origin_state = COALESCE(sqlc.narg(origin_state), origin_state),
  origin_postal_code = COALESCE(sqlc.narg(origin_postal_code), origin_postal_code),
  destination_address_line1 = COALESCE(sqlc.narg(destination_address_line1), destination_address_line1),
  destination_city = COALESCE(sqlc.narg(destination_city), destination_city),
  destination_state = COALESCE(sqlc.narg(destination_state), destination_state),
  destination_postal_code = COALESCE(sqlc.narg(destination_postal_code), destination_postal_code),
  move_date = COALESCE(sqlc.narg(move_date)::date, move_date),
  pickup_time = COALESCE(sqlc.narg(pickup_time), pickup_time),
  lead_source = COALESCE(sqlc.narg(lead_source), lead_source),
  move_size = COALESCE(sqlc.narg(move_size), move_size),
  location_type = COALESCE(sqlc.narg(location_type), location_type),
  estimated_total_cents = COALESCE(sqlc.narg(estimated_total_cents)::bigint, estimated_total_cents),
  deposit_cents = COALESCE(sqlc.narg(deposit_cents)::bigint, deposit_cents),
  notes = COALESCE(sqlc.narg(notes), notes),
  updated_by = sqlc.arg(updated_by),
  updated_at = NOW()
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id)
RETURNING *;

-- name: MarkEstimateConverted :execrows
UPDATE estimates
SET
  status = 'converted',
  updated_by = sqlc.arg(updated_by),
  updated_at = NOW()
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id)
  AND status <> 'converted';

-- name: CreateJob :one
INSERT INTO jobs (
  tenant_id,
  job_number,
  estimate_id,
  customer_id,
  status,
  scheduled_date,
  pickup_time,
  convert_idempotency_key,
  created_by,
  updated_by
) VALUES (
  sqlc.arg(tenant_id),
  sqlc.arg(job_number),
  sqlc.narg(estimate_id),
  sqlc.arg(customer_id),
  sqlc.arg(status),
  sqlc.narg(scheduled_date),
  sqlc.narg(pickup_time),
  sqlc.narg(convert_idempotency_key),
  sqlc.arg(created_by),
  sqlc.arg(updated_by)
)
RETURNING *;

-- name: GetJobByID :one
SELECT
  id,
  tenant_id,
  job_number,
  estimate_id,
  customer_id,
  status,
  scheduled_date,
  pickup_time,
  convert_idempotency_key,
  created_by,
  updated_by,
  created_at,
  updated_at
FROM jobs
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id);

-- name: GetJobDetailByID :one
SELECT
  j.id,
  j.tenant_id,
  j.job_number,
  j.estimate_id,
  j.customer_id,
  j.status,
  j.scheduled_date,
  j.pickup_time,
  j.convert_idempotency_key,
  j.created_by,
  j.updated_by,
  j.created_at,
  j.updated_at,
  c.first_name,
  c.last_name,
  COALESCE(c.phone, e.primary_phone, '') AS phone,
  COALESCE(c.email, e.email, 'no-reply@moveops.local') AS email
FROM jobs j
JOIN customers c
  ON c.id = j.customer_id
  AND c.tenant_id = j.tenant_id
LEFT JOIN estimates e
  ON e.id = j.estimate_id
  AND e.tenant_id = j.tenant_id
WHERE j.id = sqlc.arg(id)
  AND j.tenant_id = sqlc.arg(tenant_id);

-- name: ListCalendarJobs :many
SELECT
  j.id AS job_id,
  j.job_number,
  j.scheduled_date,
  j.pickup_time,
  COALESCE(NULLIF(TRIM(c.first_name || ' ' || c.last_name), ''), j.job_number) AS customer_name,
  COALESCE(
    NULLIF(CONCAT_WS(', ', NULLIF(TRIM(e.origin_city), ''), NULLIF(TRIM(e.origin_state), '')), ''),
    'TBD'
  )::text AS origin_short,
  COALESCE(
    NULLIF(CONCAT_WS(', ', NULLIF(TRIM(e.destination_city), ''), NULLIF(TRIM(e.destination_state), '')), ''),
    'TBD'
  )::text AS destination_short,
  j.status,
  FALSE AS has_storage,
  GREATEST(COALESCE(e.estimated_total_cents, 0) - COALESCE(e.deposit_cents, 0), 0)::bigint AS balance_due_cents
FROM jobs j
JOIN customers c
  ON c.id = j.customer_id
  AND c.tenant_id = j.tenant_id
LEFT JOIN estimates e
  ON e.id = j.estimate_id
  AND e.tenant_id = j.tenant_id
WHERE j.tenant_id = sqlc.arg(tenant_id)
  AND j.scheduled_date IS NOT NULL
  AND j.scheduled_date >= sqlc.arg(from_date)::date
  AND j.scheduled_date < sqlc.arg(to_date)::date
  AND (sqlc.narg(phase)::text IS NULL OR j.status = sqlc.narg(phase)::text)
  AND (
    sqlc.narg(job_type)::text IS NULL
    OR (
      CASE
        WHEN e.id IS NULL THEN 'other'
        WHEN NULLIF(TRIM(COALESCE(e.origin_state, '')), '') IS NULL THEN 'other'
        WHEN NULLIF(TRIM(COALESCE(e.destination_state, '')), '') IS NULL THEN 'other'
        WHEN UPPER(e.origin_state) = UPPER(e.destination_state) THEN 'local'
        ELSE 'long_distance'
      END
    ) = sqlc.narg(job_type)::text
  )
ORDER BY j.scheduled_date ASC, COALESCE(j.pickup_time, ''), j.job_number ASC;

-- name: GetJobByEstimateID :one
SELECT
  id,
  tenant_id,
  job_number,
  estimate_id,
  customer_id,
  status,
  scheduled_date,
  pickup_time,
  convert_idempotency_key,
  created_by,
  updated_by,
  created_at,
  updated_at
FROM jobs
WHERE tenant_id = sqlc.arg(tenant_id)
  AND estimate_id = sqlc.arg(estimate_id);

-- name: GetJobByConvertIdempotencyKey :one
SELECT
  id,
  tenant_id,
  job_number,
  estimate_id,
  customer_id,
  status,
  scheduled_date,
  pickup_time,
  convert_idempotency_key,
  created_by,
  updated_by,
  created_at,
  updated_at
FROM jobs
WHERE tenant_id = sqlc.arg(tenant_id)
  AND convert_idempotency_key = sqlc.arg(convert_idempotency_key);

-- name: UpdateJobScheduleStatus :one
UPDATE jobs
SET
  scheduled_date = COALESCE(sqlc.narg(scheduled_date)::date, scheduled_date),
  pickup_time = COALESCE(sqlc.narg(pickup_time), pickup_time),
  status = COALESCE(sqlc.narg(status), status),
  updated_by = sqlc.arg(updated_by),
  updated_at = NOW()
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id)
RETURNING *;

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
