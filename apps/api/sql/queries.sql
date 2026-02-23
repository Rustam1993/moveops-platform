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
  total_volume_cf,
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
  e.total_volume_cf,
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

-- name: ListEstimates :many
SELECT
  e.id AS estimate_id,
  e.estimate_number,
  e.status,
  e.customer_name,
  e.primary_phone,
  e.email,
  e.move_date,
  j.id AS converted_job_id,
  e.created_at,
  e.updated_at,
  e.created_at AS sort_created_at,
  e.id AS sort_estimate_id
FROM estimates e
LEFT JOIN jobs j
  ON j.tenant_id = e.tenant_id
  AND j.estimate_id = e.id
WHERE e.tenant_id = sqlc.arg(tenant_id)
  AND (sqlc.narg(status)::text IS NULL OR e.status = sqlc.narg(status)::text)
  AND (
    sqlc.narg(search_q)::text IS NULL
    OR (
      e.estimate_number ILIKE '%' || sqlc.narg(search_q)::text || '%'
      OR e.customer_name ILIKE '%' || sqlc.narg(search_q)::text || '%'
      OR e.email ILIKE '%' || sqlc.narg(search_q)::text || '%'
      OR e.primary_phone ILIKE '%' || sqlc.narg(search_q)::text || '%'
    )
  )
  AND (
    sqlc.narg(cursor_created_at)::timestamptz IS NULL
    OR (e.created_at, e.id) < (sqlc.narg(cursor_created_at)::timestamptz, sqlc.narg(cursor_estimate_id)::uuid)
  )
ORDER BY e.created_at DESC, e.id DESC
LIMIT sqlc.arg(limit_rows);

-- name: CountOpenEstimates :one
SELECT COUNT(*)::bigint AS count
FROM estimates
WHERE tenant_id = sqlc.arg(tenant_id)
  AND status = 'draft';

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
  total_volume_cf,
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

-- name: GetEstimateInventoryItems :many
SELECT
  id,
  tenant_id,
  estimate_id,
  category,
  item_name,
  volume_cf,
  qty,
  is_custom,
  created_at,
  updated_at
FROM estimate_inventory_item
WHERE tenant_id = sqlc.arg(tenant_id)
  AND estimate_id = sqlc.arg(estimate_id)
ORDER BY lower(category), lower(item_name), id;

-- name: DeleteEstimateInventoryItems :exec
DELETE FROM estimate_inventory_item
WHERE tenant_id = sqlc.arg(tenant_id)
  AND estimate_id = sqlc.arg(estimate_id);

-- name: InsertEstimateInventoryItem :one
INSERT INTO estimate_inventory_item (
  tenant_id,
  estimate_id,
  category,
  item_name,
  volume_cf,
  qty,
  is_custom
) VALUES (
  sqlc.arg(tenant_id),
  sqlc.arg(estimate_id),
  sqlc.arg(category),
  sqlc.arg(item_name),
  sqlc.arg(volume_cf),
  sqlc.arg(qty),
  COALESCE(sqlc.narg(is_custom)::boolean, FALSE)
)
RETURNING *;

-- name: UpdateEstimateTotalVolumeCf :execrows
UPDATE estimates
SET
  total_volume_cf = sqlc.arg(total_volume_cf),
  updated_by = COALESCE(sqlc.narg(updated_by), updated_by),
  updated_at = NOW()
WHERE id = sqlc.arg(estimate_id)
  AND tenant_id = sqlc.arg(tenant_id);

-- name: CreateEstimateInventoryShareLink :one
INSERT INTO estimate_inventory_share_link (
  tenant_id,
  estimate_id,
  token_hash,
  recipient_email,
  delivery_mode,
  delivery_error,
  created_by,
  expires_at,
  last_accessed_at,
  last_updated_at,
  revoked_at
) VALUES (
  sqlc.arg(tenant_id),
  sqlc.arg(estimate_id),
  sqlc.arg(token_hash),
  sqlc.arg(recipient_email),
  sqlc.arg(delivery_mode),
  sqlc.narg(delivery_error),
  sqlc.narg(created_by),
  sqlc.arg(expires_at),
  sqlc.narg(last_accessed_at),
  sqlc.narg(last_updated_at),
  sqlc.narg(revoked_at)
)
RETURNING *;

-- name: GetEstimateInventoryShareLinkByTokenHash :one
SELECT
  id,
  tenant_id,
  estimate_id,
  token_hash,
  recipient_email,
  delivery_mode,
  delivery_error,
  created_by,
  expires_at,
  last_accessed_at,
  last_updated_at,
  revoked_at,
  created_at
FROM estimate_inventory_share_link
WHERE token_hash = sqlc.arg(token_hash)
  AND revoked_at IS NULL;

-- name: TouchEstimateInventoryShareLink :execrows
UPDATE estimate_inventory_share_link
SET
  last_accessed_at = COALESCE(sqlc.narg(last_accessed_at), last_accessed_at),
  last_updated_at = COALESCE(sqlc.narg(last_updated_at), last_updated_at)
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id);

-- name: ListNewEstimateCatalogCategories :many
SELECT
  id,
  tenant_id,
  name,
  sort_order,
  active,
  created_by,
  updated_by,
  created_at,
  updated_at
FROM new_estimate_catalog_category
WHERE tenant_id = sqlc.arg(tenant_id)
ORDER BY active DESC, sort_order ASC, lower(name), id;

-- name: GetNewEstimateCatalogCategoryByID :one
SELECT
  id,
  tenant_id,
  name,
  sort_order,
  active,
  created_by,
  updated_by,
  created_at,
  updated_at
FROM new_estimate_catalog_category
WHERE tenant_id = sqlc.arg(tenant_id)
  AND id = sqlc.arg(id);

-- name: CreateNewEstimateCatalogCategory :one
INSERT INTO new_estimate_catalog_category (
  tenant_id,
  name,
  sort_order,
  active,
  created_by,
  updated_by
) VALUES (
  sqlc.arg(tenant_id),
  sqlc.arg(name),
  sqlc.arg(sort_order),
  sqlc.arg(active),
  sqlc.narg(created_by),
  sqlc.narg(updated_by)
)
RETURNING *;

-- name: UpdateNewEstimateCatalogCategory :one
UPDATE new_estimate_catalog_category
SET
  name = COALESCE(sqlc.narg(name), name),
  sort_order = COALESCE(sqlc.narg(sort_order)::int, sort_order),
  active = COALESCE(sqlc.narg(active)::bool, active),
  updated_by = COALESCE(sqlc.narg(updated_by), updated_by),
  updated_at = NOW()
WHERE tenant_id = sqlc.arg(tenant_id)
  AND id = sqlc.arg(id)
RETURNING *;

-- name: DeleteNewEstimateCatalogCategory :execrows
DELETE FROM new_estimate_catalog_category
WHERE tenant_id = sqlc.arg(tenant_id)
  AND id = sqlc.arg(id);

-- name: ListNewEstimateCatalogItems :many
SELECT
  i.id,
  i.tenant_id,
  i.category_id,
  i.name,
  i.volume_cf,
  i.sort_order,
  i.active,
  i.created_by,
  i.updated_by,
  i.created_at,
  i.updated_at,
  c.name AS category_name,
  c.sort_order AS category_sort_order,
  c.active AS category_active
FROM new_estimate_catalog_item i
LEFT JOIN new_estimate_catalog_category c
  ON c.id = i.category_id
  AND c.tenant_id = i.tenant_id
WHERE i.tenant_id = sqlc.arg(tenant_id)
ORDER BY i.active DESC, COALESCE(c.sort_order, 9999), lower(COALESCE(c.name, '')), i.sort_order ASC, lower(i.name), i.id;

-- name: GetNewEstimateCatalogItemByID :one
SELECT
  i.id,
  i.tenant_id,
  i.category_id,
  i.name,
  i.volume_cf,
  i.sort_order,
  i.active,
  i.created_by,
  i.updated_by,
  i.created_at,
  i.updated_at,
  c.name AS category_name,
  c.sort_order AS category_sort_order,
  c.active AS category_active
FROM new_estimate_catalog_item i
LEFT JOIN new_estimate_catalog_category c
  ON c.id = i.category_id
  AND c.tenant_id = i.tenant_id
WHERE i.tenant_id = sqlc.arg(tenant_id)
  AND i.id = sqlc.arg(id);

-- name: CreateNewEstimateCatalogItem :one
INSERT INTO new_estimate_catalog_item (
  tenant_id,
  category_id,
  name,
  volume_cf,
  sort_order,
  active,
  created_by,
  updated_by
) VALUES (
  sqlc.arg(tenant_id),
  sqlc.narg(category_id),
  sqlc.arg(name),
  sqlc.arg(volume_cf),
  sqlc.arg(sort_order),
  sqlc.arg(active),
  sqlc.narg(created_by),
  sqlc.narg(updated_by)
)
RETURNING *;

-- name: UpdateNewEstimateCatalogItem :one
UPDATE new_estimate_catalog_item
SET
  category_id = COALESCE(sqlc.narg(category_id), category_id),
  name = COALESCE(sqlc.narg(name), name),
  volume_cf = COALESCE(sqlc.narg(volume_cf)::float8, volume_cf),
  sort_order = COALESCE(sqlc.narg(sort_order)::int, sort_order),
  active = COALESCE(sqlc.narg(active)::bool, active),
  updated_by = COALESCE(sqlc.narg(updated_by), updated_by),
  updated_at = NOW()
WHERE tenant_id = sqlc.arg(tenant_id)
  AND id = sqlc.arg(id)
RETURNING *;

-- name: DeleteNewEstimateCatalogItem :execrows
DELETE FROM new_estimate_catalog_item
WHERE tenant_id = sqlc.arg(tenant_id)
  AND id = sqlc.arg(id);

-- name: DeleteNewEstimateCatalogItemsByTenant :exec
DELETE FROM new_estimate_catalog_item
WHERE tenant_id = sqlc.arg(tenant_id);

-- name: DeleteNewEstimateCatalogCategoriesByTenant :exec
DELETE FROM new_estimate_catalog_category
WHERE tenant_id = sqlc.arg(tenant_id);

-- name: ListActiveNewEstimateCatalogItems :many
SELECT
  i.id,
  i.tenant_id,
  i.category_id,
  i.name,
  i.volume_cf,
  i.sort_order,
  i.active,
  i.created_by,
  i.updated_by,
  i.created_at,
  i.updated_at,
  c.name AS category_name,
  c.sort_order AS category_sort_order,
  c.active AS category_active
FROM new_estimate_catalog_item i
LEFT JOIN new_estimate_catalog_category c
  ON c.id = i.category_id
  AND c.tenant_id = i.tenant_id
WHERE i.tenant_id = sqlc.arg(tenant_id)
  AND i.active = TRUE
  AND (i.category_id IS NULL OR c.active = TRUE)
ORDER BY COALESCE(c.sort_order, 9999), lower(COALESCE(c.name, '')), i.sort_order ASC, lower(i.name), i.id;

-- name: GetTenantNewEstimateSettings :one
SELECT
  tenant_id,
  pricing_defaults_json,
  email_templates_json,
  document_branding_json,
  updated_by,
  created_at,
  updated_at
FROM tenant_new_estimate_settings
WHERE tenant_id = sqlc.arg(tenant_id);

-- name: UpsertTenantNewEstimateSettings :one
INSERT INTO tenant_new_estimate_settings (
  tenant_id,
  pricing_defaults_json,
  email_templates_json,
  document_branding_json,
  updated_by
) VALUES (
  sqlc.arg(tenant_id),
  COALESCE(sqlc.narg(pricing_defaults_json)::jsonb, '{}'::jsonb),
  COALESCE(sqlc.narg(email_templates_json)::jsonb, '{}'::jsonb),
  COALESCE(sqlc.narg(document_branding_json)::jsonb, '{}'::jsonb),
  sqlc.narg(updated_by)
)
ON CONFLICT (tenant_id) DO UPDATE
SET
  pricing_defaults_json = COALESCE(sqlc.narg(pricing_defaults_json)::jsonb, tenant_new_estimate_settings.pricing_defaults_json),
  email_templates_json = COALESCE(sqlc.narg(email_templates_json)::jsonb, tenant_new_estimate_settings.email_templates_json),
  document_branding_json = COALESCE(sqlc.narg(document_branding_json)::jsonb, tenant_new_estimate_settings.document_branding_json),
  updated_by = COALESCE(sqlc.narg(updated_by), tenant_new_estimate_settings.updated_by),
  updated_at = NOW()
RETURNING *;

-- name: InsertAnalyticsEvent :exec
INSERT INTO analytics_event (
  tenant_id,
  estimate_id,
  user_id,
  event_name,
  properties_json
) VALUES (
  sqlc.arg(tenant_id),
  sqlc.narg(estimate_id),
  sqlc.narg(user_id),
  sqlc.arg(event_name),
  COALESCE(sqlc.narg(properties_json)::jsonb, '{}'::jsonb)
);

-- name: GetAnalyticsMedianTimeToQuoteMinutes :one
WITH entry_events AS (
  SELECT ae.estimate_id, MIN(ae.created_at) AS started_at
  FROM analytics_event ae
  WHERE ae.tenant_id = sqlc.arg(tenant_id)
    AND ae.event_name = 'estimate.entry_started'
    AND ae.estimate_id IS NOT NULL
    AND (sqlc.narg(from_time)::timestamptz IS NULL OR ae.created_at >= sqlc.narg(from_time)::timestamptz)
    AND (sqlc.narg(to_time)::timestamptz IS NULL OR ae.created_at <= sqlc.narg(to_time)::timestamptz)
  GROUP BY ae.estimate_id
),
quote_events AS (
  SELECT ae.estimate_id, MIN(ae.created_at) AS quoted_at
  FROM analytics_event ae
  WHERE ae.tenant_id = sqlc.arg(tenant_id)
    AND ae.event_name = 'estimate.quote_sent'
    AND ae.estimate_id IS NOT NULL
    AND (sqlc.narg(from_time)::timestamptz IS NULL OR ae.created_at >= sqlc.narg(from_time)::timestamptz)
    AND (sqlc.narg(to_time)::timestamptz IS NULL OR ae.created_at <= sqlc.narg(to_time)::timestamptz)
  GROUP BY ae.estimate_id
),
pairs AS (
  SELECT EXTRACT(EPOCH FROM (q.quoted_at - e.started_at)) / 60.0 AS minutes
  FROM entry_events e
  JOIN quote_events q ON q.estimate_id = e.estimate_id
  WHERE q.quoted_at >= e.started_at
)
SELECT COALESCE(percentile_cont(0.5) WITHIN GROUP (ORDER BY minutes), 0)::float8 AS median_minutes
FROM pairs;

-- name: GetAnalyticsQuoteToSignCounts :one
WITH quote_events AS (
  SELECT ae.estimate_id, MIN(ae.created_at) AS quoted_at
  FROM analytics_event ae
  WHERE ae.tenant_id = sqlc.arg(tenant_id)
    AND ae.event_name = 'estimate.quote_sent'
    AND ae.estimate_id IS NOT NULL
    AND (sqlc.narg(from_time)::timestamptz IS NULL OR ae.created_at >= sqlc.narg(from_time)::timestamptz)
    AND (sqlc.narg(to_time)::timestamptz IS NULL OR ae.created_at <= sqlc.narg(to_time)::timestamptz)
  GROUP BY ae.estimate_id
),
sign_events AS (
  SELECT ae.estimate_id, MIN(ae.created_at) AS signed_at
  FROM analytics_event ae
  WHERE ae.tenant_id = sqlc.arg(tenant_id)
    AND ae.event_name = 'estimate.sign_completed'
    AND ae.estimate_id IS NOT NULL
    AND (sqlc.narg(from_time)::timestamptz IS NULL OR ae.created_at >= sqlc.narg(from_time)::timestamptz)
    AND (sqlc.narg(to_time)::timestamptz IS NULL OR ae.created_at <= sqlc.narg(to_time)::timestamptz)
  GROUP BY ae.estimate_id
)
SELECT
  COUNT(*) FILTER (WHERE s.estimate_id IS NOT NULL AND s.signed_at >= q.quoted_at)::bigint AS converted_count,
  COUNT(*)::bigint AS total_count
FROM quote_events q
LEFT JOIN sign_events s ON s.estimate_id = q.estimate_id;

-- name: GetAnalyticsSignToBookCounts :one
WITH sign_events AS (
  SELECT ae.estimate_id, MIN(ae.created_at) AS signed_at
  FROM analytics_event ae
  WHERE ae.tenant_id = sqlc.arg(tenant_id)
    AND ae.event_name = 'estimate.sign_completed'
    AND ae.estimate_id IS NOT NULL
    AND (sqlc.narg(from_time)::timestamptz IS NULL OR ae.created_at >= sqlc.narg(from_time)::timestamptz)
    AND (sqlc.narg(to_time)::timestamptz IS NULL OR ae.created_at <= sqlc.narg(to_time)::timestamptz)
  GROUP BY ae.estimate_id
),
book_events AS (
  SELECT ae.estimate_id, MIN(ae.created_at) AS booked_at
  FROM analytics_event ae
  WHERE ae.tenant_id = sqlc.arg(tenant_id)
    AND ae.event_name = 'estimate.booked'
    AND ae.estimate_id IS NOT NULL
    AND (sqlc.narg(from_time)::timestamptz IS NULL OR ae.created_at >= sqlc.narg(from_time)::timestamptz)
    AND (sqlc.narg(to_time)::timestamptz IS NULL OR ae.created_at <= sqlc.narg(to_time)::timestamptz)
  GROUP BY ae.estimate_id
)
SELECT
  COUNT(*) FILTER (WHERE b.estimate_id IS NOT NULL AND b.booked_at >= s.signed_at)::bigint AS converted_count,
  COUNT(*)::bigint AS total_count
FROM sign_events s
LEFT JOIN book_events b ON b.estimate_id = s.estimate_id;

-- name: GetAnalyticsInventoryCompletionCounts :one
WITH sent_events AS (
  SELECT ae.estimate_id, MIN(ae.created_at) AS sent_at
  FROM analytics_event ae
  WHERE ae.tenant_id = sqlc.arg(tenant_id)
    AND ae.event_name = 'estimate.inventory_link_sent'
    AND ae.estimate_id IS NOT NULL
    AND (sqlc.narg(from_time)::timestamptz IS NULL OR ae.created_at >= sqlc.narg(from_time)::timestamptz)
    AND (sqlc.narg(to_time)::timestamptz IS NULL OR ae.created_at <= sqlc.narg(to_time)::timestamptz)
  GROUP BY ae.estimate_id
),
updated_events AS (
  SELECT ae.estimate_id, MIN(ae.created_at) AS updated_at
  FROM analytics_event ae
  WHERE ae.tenant_id = sqlc.arg(tenant_id)
    AND ae.event_name = 'estimate.inventory_updated'
    AND ae.estimate_id IS NOT NULL
    AND (sqlc.narg(from_time)::timestamptz IS NULL OR ae.created_at >= sqlc.narg(from_time)::timestamptz)
    AND (sqlc.narg(to_time)::timestamptz IS NULL OR ae.created_at <= sqlc.narg(to_time)::timestamptz)
  GROUP BY ae.estimate_id
)
SELECT
  COUNT(*) FILTER (WHERE u.estimate_id IS NOT NULL AND u.updated_at >= s.sent_at)::bigint AS converted_count,
  COUNT(*)::bigint AS total_count
FROM sent_events s
LEFT JOIN updated_events u ON u.estimate_id = s.estimate_id;

-- name: CountStuckEstimates :one
SELECT COUNT(*)::bigint AS count
FROM estimates e
LEFT JOIN estimate_workflow w
  ON w.tenant_id = e.tenant_id
  AND w.estimate_id = e.id
WHERE e.tenant_id = sqlc.arg(tenant_id)
  AND COALESCE(w.status, 'draft') IN ('draft', 'open', 'follow_up', 'quoted')
  AND e.created_at < NOW() - make_interval(days => sqlc.arg(stuck_days)::int);

-- name: ListAuditLogsForTenant :many
SELECT
  id,
  tenant_id,
  user_id,
  action,
  entity_type,
  entity_id,
  request_id,
  metadata,
  created_at
FROM audit_log
WHERE tenant_id = sqlc.arg(tenant_id)
  AND (sqlc.narg(from_time)::timestamptz IS NULL OR created_at >= sqlc.narg(from_time)::timestamptz)
  AND (sqlc.narg(to_time)::timestamptz IS NULL OR created_at <= sqlc.narg(to_time)::timestamptz)
  AND (sqlc.narg(user_id)::uuid IS NULL OR user_id = sqlc.narg(user_id)::uuid)
  AND (sqlc.narg(action_like)::text IS NULL OR action ILIKE sqlc.narg(action_like)::text || '%')
  AND (sqlc.narg(entity_type)::text IS NULL OR entity_type = sqlc.narg(entity_type)::text)
  AND (sqlc.narg(entity_id)::uuid IS NULL OR entity_id = sqlc.narg(entity_id)::uuid)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(limit_rows)
OFFSET sqlc.arg(offset_rows);

-- name: CountAuditLogsForTenant :one
SELECT COUNT(*)::bigint AS count
FROM audit_log
WHERE tenant_id = sqlc.arg(tenant_id)
  AND (sqlc.narg(from_time)::timestamptz IS NULL OR created_at >= sqlc.narg(from_time)::timestamptz)
  AND (sqlc.narg(to_time)::timestamptz IS NULL OR created_at <= sqlc.narg(to_time)::timestamptz)
  AND (sqlc.narg(user_id)::uuid IS NULL OR user_id = sqlc.narg(user_id)::uuid)
  AND (sqlc.narg(action_like)::text IS NULL OR action ILIKE sqlc.narg(action_like)::text || '%')
  AND (sqlc.narg(entity_type)::text IS NULL OR entity_type = sqlc.narg(entity_type)::text)
  AND (sqlc.narg(entity_id)::uuid IS NULL OR entity_id = sqlc.narg(entity_id)::uuid);

-- name: GetEstimateChargesByEstimateID :one
SELECT
  id,
  tenant_id,
  estimate_id,
  mode,
  calculation_version,
  cf_lbs_ratio,
  fuel_surcharge_pct,
  ld_rate_per_cf,
  ld_fixed_base_amount_cents,
  local_trucks,
  local_workers,
  local_labor_hours,
  local_labor_rate_cents,
  local_travel_hours,
  local_travel_rate_cents,
  other_line_items_json,
  discount_coupon_pct,
  discount_coupon_amount_cents,
  discount_senior_pct,
  discount_senior_amount_cents,
  packing_packers,
  packing_hours,
  packing_rate_cents,
  liability_type,
  liability_valuation_charge_cents,
  tax_rate_pct,
  deposit_required_cents,
  amount_paid_cents,
  computed_total_cf,
  computed_total_lbs,
  computed_base_cents,
  computed_fuel_surcharge_cents,
  computed_other_items_cents,
  computed_packing_cents,
  computed_liability_cents,
  computed_subtotal_cents,
  computed_discounts_cents,
  computed_tax_cents,
  computed_total_cents,
  created_by,
  updated_by,
  created_at,
  updated_at
FROM estimate_charge
WHERE tenant_id = sqlc.arg(tenant_id)
  AND estimate_id = sqlc.arg(estimate_id);

-- name: UpsertEstimateCharges :one
INSERT INTO estimate_charge (
  tenant_id,
  estimate_id,
  mode,
  calculation_version,
  cf_lbs_ratio,
  fuel_surcharge_pct,
  ld_rate_per_cf,
  ld_fixed_base_amount_cents,
  local_trucks,
  local_workers,
  local_labor_hours,
  local_labor_rate_cents,
  local_travel_hours,
  local_travel_rate_cents,
  other_line_items_json,
  discount_coupon_pct,
  discount_coupon_amount_cents,
  discount_senior_pct,
  discount_senior_amount_cents,
  packing_packers,
  packing_hours,
  packing_rate_cents,
  liability_type,
  liability_valuation_charge_cents,
  tax_rate_pct,
  deposit_required_cents,
  amount_paid_cents,
  computed_total_cf,
  computed_total_lbs,
  computed_base_cents,
  computed_fuel_surcharge_cents,
  computed_other_items_cents,
  computed_packing_cents,
  computed_liability_cents,
  computed_subtotal_cents,
  computed_discounts_cents,
  computed_tax_cents,
  computed_total_cents,
  created_by,
  updated_by
) VALUES (
  sqlc.arg(tenant_id),
  sqlc.arg(estimate_id),
  sqlc.arg(mode),
  sqlc.arg(calculation_version),
  sqlc.arg(cf_lbs_ratio),
  sqlc.arg(fuel_surcharge_pct),
  sqlc.arg(ld_rate_per_cf),
  sqlc.narg(ld_fixed_base_amount_cents),
  sqlc.arg(local_trucks),
  sqlc.arg(local_workers),
  sqlc.arg(local_labor_hours),
  sqlc.arg(local_labor_rate_cents),
  sqlc.arg(local_travel_hours),
  sqlc.arg(local_travel_rate_cents),
  sqlc.arg(other_line_items_json),
  sqlc.arg(discount_coupon_pct),
  sqlc.arg(discount_coupon_amount_cents),
  sqlc.arg(discount_senior_pct),
  sqlc.arg(discount_senior_amount_cents),
  sqlc.arg(packing_packers),
  sqlc.arg(packing_hours),
  sqlc.arg(packing_rate_cents),
  sqlc.arg(liability_type),
  sqlc.arg(liability_valuation_charge_cents),
  sqlc.arg(tax_rate_pct),
  sqlc.narg(deposit_required_cents),
  sqlc.arg(amount_paid_cents),
  sqlc.arg(computed_total_cf),
  sqlc.arg(computed_total_lbs),
  sqlc.arg(computed_base_cents),
  sqlc.arg(computed_fuel_surcharge_cents),
  sqlc.arg(computed_other_items_cents),
  sqlc.arg(computed_packing_cents),
  sqlc.arg(computed_liability_cents),
  sqlc.arg(computed_subtotal_cents),
  sqlc.arg(computed_discounts_cents),
  sqlc.arg(computed_tax_cents),
  sqlc.arg(computed_total_cents),
  sqlc.narg(created_by),
  sqlc.narg(updated_by)
)
ON CONFLICT (tenant_id, estimate_id) DO UPDATE
SET
  mode = EXCLUDED.mode,
  calculation_version = EXCLUDED.calculation_version,
  cf_lbs_ratio = EXCLUDED.cf_lbs_ratio,
  fuel_surcharge_pct = EXCLUDED.fuel_surcharge_pct,
  ld_rate_per_cf = EXCLUDED.ld_rate_per_cf,
  ld_fixed_base_amount_cents = EXCLUDED.ld_fixed_base_amount_cents,
  local_trucks = EXCLUDED.local_trucks,
  local_workers = EXCLUDED.local_workers,
  local_labor_hours = EXCLUDED.local_labor_hours,
  local_labor_rate_cents = EXCLUDED.local_labor_rate_cents,
  local_travel_hours = EXCLUDED.local_travel_hours,
  local_travel_rate_cents = EXCLUDED.local_travel_rate_cents,
  other_line_items_json = EXCLUDED.other_line_items_json,
  discount_coupon_pct = EXCLUDED.discount_coupon_pct,
  discount_coupon_amount_cents = EXCLUDED.discount_coupon_amount_cents,
  discount_senior_pct = EXCLUDED.discount_senior_pct,
  discount_senior_amount_cents = EXCLUDED.discount_senior_amount_cents,
  packing_packers = EXCLUDED.packing_packers,
  packing_hours = EXCLUDED.packing_hours,
  packing_rate_cents = EXCLUDED.packing_rate_cents,
  liability_type = EXCLUDED.liability_type,
  liability_valuation_charge_cents = EXCLUDED.liability_valuation_charge_cents,
  tax_rate_pct = EXCLUDED.tax_rate_pct,
  deposit_required_cents = EXCLUDED.deposit_required_cents,
  amount_paid_cents = EXCLUDED.amount_paid_cents,
  computed_total_cf = EXCLUDED.computed_total_cf,
  computed_total_lbs = EXCLUDED.computed_total_lbs,
  computed_base_cents = EXCLUDED.computed_base_cents,
  computed_fuel_surcharge_cents = EXCLUDED.computed_fuel_surcharge_cents,
  computed_other_items_cents = EXCLUDED.computed_other_items_cents,
  computed_packing_cents = EXCLUDED.computed_packing_cents,
  computed_liability_cents = EXCLUDED.computed_liability_cents,
  computed_subtotal_cents = EXCLUDED.computed_subtotal_cents,
  computed_discounts_cents = EXCLUDED.computed_discounts_cents,
  computed_tax_cents = EXCLUDED.computed_tax_cents,
  computed_total_cents = EXCLUDED.computed_total_cents,
  updated_by = COALESCE(EXCLUDED.updated_by, estimate_charge.updated_by),
  updated_at = NOW()
RETURNING *;

-- name: UpdateEstimatePricingSummary :execrows
UPDATE estimates
SET
  estimated_total_cents = sqlc.narg(estimated_total_cents)::bigint,
  deposit_cents = sqlc.narg(deposit_cents)::bigint,
  location_type = COALESCE(sqlc.narg(location_type), location_type),
  updated_by = COALESCE(sqlc.narg(updated_by), updated_by),
  updated_at = NOW()
WHERE id = sqlc.arg(estimate_id)
  AND tenant_id = sqlc.arg(tenant_id);

-- name: GetEstimateWorkflowByEstimateID :one
SELECT
  id,
  tenant_id,
  estimate_id,
  status,
  priority_level,
  follow_up_at,
  follow_up_note,
  vip,
  booked_at,
  hold_reason,
  created_by,
  updated_by,
  created_at,
  updated_at
FROM estimate_workflow
WHERE tenant_id = sqlc.arg(tenant_id)
  AND estimate_id = sqlc.arg(estimate_id);

-- name: UpsertEstimateWorkflow :one
INSERT INTO estimate_workflow (
  tenant_id,
  estimate_id,
  status,
  priority_level,
  follow_up_at,
  follow_up_note,
  vip,
  booked_at,
  hold_reason,
  created_by,
  updated_by
) VALUES (
  sqlc.arg(tenant_id),
  sqlc.arg(estimate_id),
  sqlc.arg(status),
  sqlc.arg(priority_level),
  sqlc.narg(follow_up_at),
  sqlc.narg(follow_up_note),
  sqlc.arg(vip),
  sqlc.narg(booked_at),
  sqlc.narg(hold_reason),
  sqlc.narg(created_by),
  sqlc.narg(updated_by)
)
ON CONFLICT (tenant_id, estimate_id) DO UPDATE
SET
  status = EXCLUDED.status,
  priority_level = EXCLUDED.priority_level,
  follow_up_at = EXCLUDED.follow_up_at,
  follow_up_note = EXCLUDED.follow_up_note,
  vip = EXCLUDED.vip,
  booked_at = EXCLUDED.booked_at,
  hold_reason = EXCLUDED.hold_reason,
  updated_by = COALESCE(EXCLUDED.updated_by, estimate_workflow.updated_by),
  updated_at = NOW()
RETURNING *;

-- name: ListEstimateTasks :many
SELECT
  id,
  tenant_id,
  estimate_id,
  title,
  is_done,
  due_at,
  created_by,
  updated_by,
  created_at,
  updated_at,
  deleted_at
FROM estimate_task
WHERE tenant_id = sqlc.arg(tenant_id)
  AND estimate_id = sqlc.arg(estimate_id)
  AND deleted_at IS NULL
ORDER BY is_done ASC, due_at ASC NULLS LAST, created_at DESC, id DESC;

-- name: GetEstimateTaskByID :one
SELECT
  id,
  tenant_id,
  estimate_id,
  title,
  is_done,
  due_at,
  created_by,
  updated_by,
  created_at,
  updated_at,
  deleted_at
FROM estimate_task
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id)
  AND estimate_id = sqlc.arg(estimate_id)
  AND deleted_at IS NULL;

-- name: CreateEstimateTask :one
INSERT INTO estimate_task (
  tenant_id,
  estimate_id,
  title,
  is_done,
  due_at,
  created_by,
  updated_by
) VALUES (
  sqlc.arg(tenant_id),
  sqlc.arg(estimate_id),
  sqlc.arg(title),
  COALESCE(sqlc.narg(is_done)::boolean, FALSE),
  sqlc.narg(due_at),
  sqlc.narg(created_by),
  sqlc.narg(updated_by)
)
RETURNING *;

-- name: UpdateEstimateTask :one
UPDATE estimate_task
SET
  title = COALESCE(sqlc.narg(title), title),
  is_done = COALESCE(sqlc.narg(is_done)::boolean, is_done),
  due_at = CASE
    WHEN sqlc.arg(clear_due_at)::boolean THEN NULL
    WHEN sqlc.narg(due_at)::timestamptz IS NOT NULL THEN sqlc.narg(due_at)::timestamptz
    ELSE due_at
  END,
  updated_by = COALESCE(sqlc.narg(updated_by), updated_by),
  updated_at = NOW()
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id)
  AND estimate_id = sqlc.arg(estimate_id)
  AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteEstimateTask :execrows
UPDATE estimate_task
SET deleted_at = NOW()
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id)
  AND estimate_id = sqlc.arg(estimate_id)
  AND deleted_at IS NULL;

-- name: ListEstimatePayments :many
SELECT
  id,
  tenant_id,
  estimate_id,
  amount_cents,
  method,
  paid_at,
  notes,
  created_by,
  created_at,
  deleted_at
FROM estimate_payment
WHERE tenant_id = sqlc.arg(tenant_id)
  AND estimate_id = sqlc.arg(estimate_id)
  AND deleted_at IS NULL
ORDER BY paid_at DESC, created_at DESC, id DESC;

-- name: GetEstimatePaymentByID :one
SELECT
  id,
  tenant_id,
  estimate_id,
  amount_cents,
  method,
  paid_at,
  notes,
  created_by,
  created_at,
  deleted_at
FROM estimate_payment
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id)
  AND estimate_id = sqlc.arg(estimate_id)
  AND deleted_at IS NULL;

-- name: SumEstimatePayments :one
SELECT COALESCE(SUM(amount_cents), 0)::bigint AS amount_paid_cents
FROM estimate_payment
WHERE tenant_id = sqlc.arg(tenant_id)
  AND estimate_id = sqlc.arg(estimate_id)
  AND deleted_at IS NULL;

-- name: CreateEstimatePayment :one
INSERT INTO estimate_payment (
  tenant_id,
  estimate_id,
  amount_cents,
  method,
  paid_at,
  notes,
  created_by
) VALUES (
  sqlc.arg(tenant_id),
  sqlc.arg(estimate_id),
  sqlc.arg(amount_cents),
  sqlc.arg(method),
  sqlc.arg(paid_at),
  sqlc.narg(notes),
  sqlc.narg(created_by)
)
RETURNING *;

-- name: SoftDeleteEstimatePayment :execrows
UPDATE estimate_payment
SET deleted_at = NOW()
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id)
  AND estimate_id = sqlc.arg(estimate_id)
  AND deleted_at IS NULL;

-- name: CreateEstimateDocument :one
INSERT INTO estimate_document (
  tenant_id,
  estimate_id,
  document_type,
  file_name,
  mime_type,
  content_bytes,
  content_sha256,
  size_bytes,
  metadata_json,
  generated_by
) VALUES (
  sqlc.arg(tenant_id),
  sqlc.arg(estimate_id),
  sqlc.arg(document_type),
  sqlc.arg(file_name),
  sqlc.arg(mime_type),
  sqlc.arg(content_bytes),
  sqlc.arg(content_sha256),
  sqlc.arg(size_bytes),
  COALESCE(sqlc.narg(metadata_json)::jsonb, '{}'::jsonb),
  sqlc.narg(generated_by)
)
RETURNING *;

-- name: GetLatestEstimateDocumentByType :one
SELECT
  id,
  tenant_id,
  estimate_id,
  document_type,
  file_name,
  mime_type,
  content_bytes,
  content_sha256,
  size_bytes,
  metadata_json,
  generated_by,
  created_at
FROM estimate_document
WHERE tenant_id = sqlc.arg(tenant_id)
  AND estimate_id = sqlc.arg(estimate_id)
  AND document_type = sqlc.arg(document_type)
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: GetEstimateDocumentByID :one
SELECT
  id,
  tenant_id,
  estimate_id,
  document_type,
  file_name,
  mime_type,
  content_bytes,
  content_sha256,
  size_bytes,
  metadata_json,
  generated_by,
  created_at
FROM estimate_document
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id);

-- name: CreateEstimateEmailLog :one
INSERT INTO estimate_email_log (
  tenant_id,
  estimate_id,
  template_key,
  email_to,
  email_cc,
  email_from,
  subject,
  status,
  provider_message_id,
  delivery_mode,
  error_message,
  rendered_json,
  created_by
) VALUES (
  sqlc.arg(tenant_id),
  sqlc.arg(estimate_id),
  sqlc.arg(template_key),
  sqlc.arg(email_to),
  sqlc.narg(email_cc),
  sqlc.arg(email_from),
  sqlc.arg(subject),
  sqlc.arg(status),
  sqlc.narg(provider_message_id),
  sqlc.arg(delivery_mode),
  sqlc.narg(error_message),
  COALESCE(sqlc.narg(rendered_json)::jsonb, '{}'::jsonb),
  sqlc.narg(created_by)
)
RETURNING *;

-- name: ListEstimateEmailLogs :many
SELECT
  id,
  tenant_id,
  estimate_id,
  template_key,
  email_to,
  email_cc,
  email_from,
  subject,
  status,
  provider_message_id,
  delivery_mode,
  error_message,
  rendered_json,
  created_by,
  created_at
FROM estimate_email_log
WHERE tenant_id = sqlc.arg(tenant_id)
  AND estimate_id = sqlc.arg(estimate_id)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(limit_rows);

-- name: CreateEstimateQuoteShareLink :one
INSERT INTO estimate_quote_share_link (
  tenant_id,
  estimate_id,
  document_id,
  token_hash,
  recipient_email,
  expires_at,
  last_accessed_at,
  revoked_at,
  created_by
) VALUES (
  sqlc.arg(tenant_id),
  sqlc.arg(estimate_id),
  sqlc.narg(document_id),
  sqlc.arg(token_hash),
  sqlc.arg(recipient_email),
  sqlc.arg(expires_at),
  sqlc.narg(last_accessed_at),
  sqlc.narg(revoked_at),
  sqlc.narg(created_by)
)
RETURNING *;

-- name: GetEstimateQuoteShareLinkByTokenHash :one
SELECT
  id,
  tenant_id,
  estimate_id,
  document_id,
  token_hash,
  recipient_email,
  expires_at,
  last_accessed_at,
  revoked_at,
  created_by,
  created_at
FROM estimate_quote_share_link
WHERE token_hash = sqlc.arg(token_hash)
  AND revoked_at IS NULL;

-- name: TouchEstimateQuoteShareLink :execrows
UPDATE estimate_quote_share_link
SET
  last_accessed_at = COALESCE(sqlc.narg(last_accessed_at), last_accessed_at)
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id);

-- name: CreateEstimateSignatureRequest :one
INSERT INTO estimate_signature_request (
  tenant_id,
  estimate_id,
  document_id,
  token_hash,
  recipient_email,
  expires_at,
  used_at,
  last_accessed_at,
  revoked_at,
  created_by
) VALUES (
  sqlc.arg(tenant_id),
  sqlc.arg(estimate_id),
  sqlc.narg(document_id),
  sqlc.arg(token_hash),
  sqlc.arg(recipient_email),
  sqlc.arg(expires_at),
  sqlc.narg(used_at),
  sqlc.narg(last_accessed_at),
  sqlc.narg(revoked_at),
  sqlc.narg(created_by)
)
RETURNING *;

-- name: GetEstimateSignatureRequestByTokenHash :one
SELECT
  id,
  tenant_id,
  estimate_id,
  document_id,
  token_hash,
  recipient_email,
  expires_at,
  used_at,
  last_accessed_at,
  revoked_at,
  created_by,
  created_at
FROM estimate_signature_request
WHERE token_hash = sqlc.arg(token_hash)
  AND revoked_at IS NULL;

-- name: TouchEstimateSignatureRequest :execrows
UPDATE estimate_signature_request
SET
  last_accessed_at = COALESCE(sqlc.narg(last_accessed_at), last_accessed_at)
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id);

-- name: MarkEstimateSignatureRequestUsed :execrows
UPDATE estimate_signature_request
SET used_at = sqlc.arg(used_at)
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id)
  AND used_at IS NULL;

-- name: CreateEstimateSignature :one
INSERT INTO estimate_signature (
  tenant_id,
  estimate_id,
  signature_request_id,
  document_id,
  signer_name,
  signer_email,
  signature_type,
  signature_value,
  agreed_terms,
  ip_address,
  user_agent,
  signed_at
) VALUES (
  sqlc.arg(tenant_id),
  sqlc.arg(estimate_id),
  sqlc.arg(signature_request_id),
  sqlc.narg(document_id),
  sqlc.arg(signer_name),
  sqlc.arg(signer_email),
  sqlc.arg(signature_type),
  sqlc.arg(signature_value),
  sqlc.arg(agreed_terms),
  sqlc.narg(ip_address),
  sqlc.narg(user_agent),
  sqlc.arg(signed_at)
)
RETURNING *;

-- name: GetLatestEstimateSignatureByEstimateID :one
SELECT
  id,
  tenant_id,
  estimate_id,
  signature_request_id,
  document_id,
  signer_name,
  signer_email,
  signature_type,
  signature_value,
  agreed_terms,
  ip_address,
  user_agent,
  signed_at,
  created_at
FROM estimate_signature
WHERE tenant_id = sqlc.arg(tenant_id)
  AND estimate_id = sqlc.arg(estimate_id)
ORDER BY signed_at DESC, id DESC
LIMIT 1;

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

-- name: ListJobs :many
SELECT
  j.id AS job_id,
  j.job_number,
  j.status,
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
  EXISTS (
    SELECT 1 FROM storage_record sr
    WHERE sr.tenant_id = j.tenant_id
      AND sr.job_id = j.id
  ) AS has_storage,
  GREATEST(COALESCE(e.estimated_total_cents, 0) - COALESCE(e.deposit_cents, 0), 0)::bigint AS balance_due_cents,
  j.created_at,
  j.updated_at,
  j.created_at AS sort_created_at,
  j.id AS sort_job_id
FROM jobs j
JOIN customers c
  ON c.id = j.customer_id
  AND c.tenant_id = j.tenant_id
LEFT JOIN estimates e
  ON e.id = j.estimate_id
  AND e.tenant_id = j.tenant_id
WHERE j.tenant_id = sqlc.arg(tenant_id)
  AND (sqlc.narg(status)::text IS NULL OR j.status = sqlc.narg(status)::text)
  AND (
    sqlc.narg(scheduled)::bool IS NULL
    OR (sqlc.narg(scheduled)::bool = TRUE AND j.scheduled_date IS NOT NULL)
    OR (sqlc.narg(scheduled)::bool = FALSE AND j.scheduled_date IS NULL)
  )
  AND (sqlc.narg(scheduled_from)::date IS NULL OR j.scheduled_date >= sqlc.narg(scheduled_from)::date)
  AND (sqlc.narg(scheduled_to)::date IS NULL OR j.scheduled_date < sqlc.narg(scheduled_to)::date)
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
  AND (
    sqlc.narg(search_q)::text IS NULL
    OR (
      j.job_number ILIKE '%' || sqlc.narg(search_q)::text || '%'
      OR c.first_name ILIKE '%' || sqlc.narg(search_q)::text || '%'
      OR c.last_name ILIKE '%' || sqlc.narg(search_q)::text || '%'
      OR e.customer_name ILIKE '%' || sqlc.narg(search_q)::text || '%'
    )
  )
  AND (
    sqlc.narg(cursor_created_at)::timestamptz IS NULL
    OR (j.created_at, j.id) < (sqlc.narg(cursor_created_at)::timestamptz, sqlc.narg(cursor_job_id)::uuid)
  )
ORDER BY j.created_at DESC, j.id DESC
LIMIT sqlc.arg(limit_rows);

-- name: CountUpcomingJobs :one
SELECT COUNT(*)::bigint AS count
FROM jobs
WHERE tenant_id = sqlc.arg(tenant_id)
  AND status IN ('booked', 'scheduled');

-- name: CountStorageRecords :one
SELECT COUNT(*)::bigint AS count
FROM storage_record
WHERE tenant_id = sqlc.arg(tenant_id);

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

-- name: GetStorageRecordByID :one
SELECT
  id,
  tenant_id,
  job_id,
  facility,
  status,
  date_in,
  date_out,
  next_bill_date,
  lot_number,
  location_label,
  vaults,
  pads,
  items,
  oversize_items,
  volume,
  monthly_rate_cents,
  storage_balance_cents,
  move_balance_cents,
  last_payment_at,
  notes,
  created_at,
  updated_at
FROM storage_record
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id);

-- name: GetStorageRecordByJobID :one
SELECT
  id,
  tenant_id,
  job_id,
  facility,
  status,
  date_in,
  date_out,
  next_bill_date,
  lot_number,
  location_label,
  vaults,
  pads,
  items,
  oversize_items,
  volume,
  monthly_rate_cents,
  storage_balance_cents,
  move_balance_cents,
  last_payment_at,
  notes,
  created_at,
  updated_at
FROM storage_record
WHERE job_id = sqlc.arg(job_id)
  AND tenant_id = sqlc.arg(tenant_id);

-- name: CreateStorageRecord :one
INSERT INTO storage_record (
  tenant_id,
  job_id,
  facility,
  status,
  date_in,
  date_out,
  next_bill_date,
  lot_number,
  location_label,
  vaults,
  pads,
  items,
  oversize_items,
  volume,
  monthly_rate_cents,
  storage_balance_cents,
  move_balance_cents,
  last_payment_at,
  notes
) VALUES (
  sqlc.arg(tenant_id),
  sqlc.arg(job_id),
  sqlc.arg(facility),
  COALESCE(sqlc.narg(status)::text, 'in_storage'),
  sqlc.narg(date_in)::date,
  sqlc.narg(date_out)::date,
  sqlc.narg(next_bill_date)::date,
  sqlc.narg(lot_number),
  sqlc.narg(location_label),
  COALESCE(sqlc.narg(vaults)::int, 0),
  COALESCE(sqlc.narg(pads)::int, 0),
  COALESCE(sqlc.narg(items)::int, 0),
  COALESCE(sqlc.narg(oversize_items)::int, 0),
  COALESCE(sqlc.narg(volume)::int, 0),
  sqlc.narg(monthly_rate_cents)::bigint,
  COALESCE(sqlc.narg(storage_balance_cents)::bigint, 0),
  COALESCE(sqlc.narg(move_balance_cents)::bigint, 0),
  sqlc.narg(last_payment_at)::timestamptz,
  sqlc.narg(notes)
)
RETURNING *;

-- name: UpdateStorageRecordByID :one
UPDATE storage_record
SET
  facility = sqlc.arg(facility),
  status = sqlc.arg(status),
  date_in = sqlc.narg(date_in)::date,
  date_out = sqlc.narg(date_out)::date,
  next_bill_date = sqlc.narg(next_bill_date)::date,
  lot_number = sqlc.narg(lot_number),
  location_label = sqlc.narg(location_label),
  vaults = sqlc.arg(vaults),
  pads = sqlc.arg(pads),
  items = sqlc.arg(items),
  oversize_items = sqlc.arg(oversize_items),
  volume = sqlc.arg(volume),
  monthly_rate_cents = sqlc.narg(monthly_rate_cents)::bigint,
  storage_balance_cents = sqlc.arg(storage_balance_cents),
  move_balance_cents = sqlc.arg(move_balance_cents),
  last_payment_at = sqlc.narg(last_payment_at)::timestamptz,
  notes = sqlc.narg(notes),
  updated_at = NOW()
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id)
RETURNING *;

-- name: GetStorageRecordDetailByID :one
SELECT
  sr.id,
  sr.tenant_id,
  sr.job_id,
  j.job_number,
  COALESCE(NULLIF(TRIM(c.first_name || ' ' || c.last_name), ''), COALESCE(e.customer_name, j.job_number))::text AS customer_name,
  CASE
    WHEN e.id IS NULL THEN 'other'
    WHEN NULLIF(TRIM(COALESCE(e.origin_state, '')), '') IS NULL THEN 'other'
    WHEN NULLIF(TRIM(COALESCE(e.destination_state, '')), '') IS NULL THEN 'other'
    WHEN UPPER(e.origin_state) = UPPER(e.destination_state) THEN 'local'
    ELSE 'long_distance'
  END::text AS move_type,
  COALESCE(
    NULLIF(CONCAT_WS(', ', NULLIF(TRIM(e.origin_city), ''), NULLIF(TRIM(e.origin_state), '')), ''),
    'TBD'
  )::text AS from_short,
  COALESCE(
    NULLIF(CONCAT_WS(', ', NULLIF(TRIM(e.destination_city), ''), NULLIF(TRIM(e.destination_state), '')), ''),
    'TBD'
  )::text AS to_short,
  sr.facility,
  sr.status,
  sr.date_in,
  sr.date_out,
  sr.next_bill_date,
  sr.lot_number,
  sr.location_label,
  sr.vaults,
  sr.pads,
  sr.items,
  sr.oversize_items,
  sr.volume,
  sr.monthly_rate_cents,
  sr.storage_balance_cents,
  sr.move_balance_cents,
  sr.last_payment_at,
  sr.notes,
  sr.created_at,
  sr.updated_at
FROM storage_record sr
JOIN jobs j
  ON j.id = sr.job_id
  AND j.tenant_id = sr.tenant_id
JOIN customers c
  ON c.id = j.customer_id
  AND c.tenant_id = j.tenant_id
LEFT JOIN estimates e
  ON e.id = j.estimate_id
  AND e.tenant_id = j.tenant_id
WHERE sr.id = sqlc.arg(id)
  AND sr.tenant_id = sqlc.arg(tenant_id);

-- name: ListStorageRows :many
SELECT
  sr.id AS storage_record_id,
  j.id AS job_id,
  j.job_number,
  COALESCE(NULLIF(TRIM(c.first_name || ' ' || c.last_name), ''), COALESCE(e.customer_name, j.job_number))::text AS customer_name,
  CASE
    WHEN e.id IS NULL THEN 'other'
    WHEN NULLIF(TRIM(COALESCE(e.origin_state, '')), '') IS NULL THEN 'other'
    WHEN NULLIF(TRIM(COALESCE(e.destination_state, '')), '') IS NULL THEN 'other'
    WHEN UPPER(e.origin_state) = UPPER(e.destination_state) THEN 'local'
    ELSE 'long_distance'
  END::text AS move_type,
  COALESCE(
    NULLIF(CONCAT_WS(', ', NULLIF(TRIM(e.origin_city), ''), NULLIF(TRIM(e.origin_state), '')), ''),
    'TBD'
  )::text AS from_short,
  COALESCE(
    NULLIF(CONCAT_WS(', ', NULLIF(TRIM(e.destination_city), ''), NULLIF(TRIM(e.destination_state), '')), ''),
    'TBD'
  )::text AS to_short,
  sr.status,
  sr.date_in,
  sr.date_out,
  sr.next_bill_date,
  sr.lot_number,
  sr.location_label,
  COALESCE(sr.vaults, 0)::int AS vaults,
  COALESCE(sr.pads, 0)::int AS pads,
  COALESCE(sr.items, 0)::int AS items,
  COALESCE(sr.oversize_items, 0)::int AS oversize_items,
  COALESCE(sr.volume, 0)::int AS volume,
  sr.monthly_rate_cents,
  COALESCE(sr.storage_balance_cents, 0)::bigint AS storage_balance_cents,
  COALESCE(sr.move_balance_cents, 0)::bigint AS move_balance_cents,
  COALESCE(sr.facility, sqlc.arg(facility))::text AS facility,
  COALESCE(sr.updated_at, j.updated_at) AS sort_updated_at,
  j.id AS sort_job_id
FROM jobs j
JOIN customers c
  ON c.id = j.customer_id
  AND c.tenant_id = j.tenant_id
LEFT JOIN estimates e
  ON e.id = j.estimate_id
  AND e.tenant_id = j.tenant_id
LEFT JOIN storage_record sr
  ON sr.job_id = j.id
  AND sr.tenant_id = j.tenant_id
WHERE j.tenant_id = sqlc.arg(tenant_id)
  AND (sr.id IS NULL OR sr.facility = sqlc.arg(facility))
  AND (
    sqlc.narg(search_q)::text IS NULL
    OR j.job_number ILIKE '%' || sqlc.narg(search_q)::text || '%'
    OR COALESCE(NULLIF(TRIM(c.first_name || ' ' || c.last_name), ''), COALESCE(e.customer_name, '')) ILIKE '%' || sqlc.narg(search_q)::text || '%'
  )
  AND (sqlc.narg(status)::text IS NULL OR sr.status = sqlc.narg(status)::text)
  AND (
    sqlc.narg(has_date_out)::boolean IS NULL
    OR (sqlc.narg(has_date_out)::boolean = TRUE AND sr.date_out IS NOT NULL)
    OR (sqlc.narg(has_date_out)::boolean = FALSE AND sr.date_out IS NULL)
  )
  AND (
    sqlc.narg(balance_due)::boolean IS NULL
    OR (sqlc.narg(balance_due)::boolean = TRUE AND COALESCE(sr.storage_balance_cents, 0) > 0)
    OR (sqlc.narg(balance_due)::boolean = FALSE AND COALESCE(sr.storage_balance_cents, 0) <= 0)
  )
  AND (
    sqlc.narg(has_containers)::boolean IS NULL
    OR (
      sqlc.narg(has_containers)::boolean = TRUE
      AND (
        COALESCE(sr.vaults, 0) > 0
        OR COALESCE(sr.pads, 0) > 0
        OR COALESCE(sr.items, 0) > 0
        OR COALESCE(sr.oversize_items, 0) > 0
      )
    )
    OR (
      sqlc.narg(has_containers)::boolean = FALSE
      AND COALESCE(sr.vaults, 0) = 0
      AND COALESCE(sr.pads, 0) = 0
      AND COALESCE(sr.items, 0) = 0
      AND COALESCE(sr.oversize_items, 0) = 0
    )
  )
  AND (
    sqlc.narg(past_due_days)::int IS NULL
    OR (
      sr.last_payment_at IS NOT NULL
      AND sr.last_payment_at <= NOW() - make_interval(days => sqlc.narg(past_due_days)::int)
    )
  )
  AND (
    sqlc.narg(cursor_updated_at)::timestamptz IS NULL
    OR (
      COALESCE(sr.updated_at, j.updated_at) < sqlc.narg(cursor_updated_at)::timestamptz
      OR (
        COALESCE(sr.updated_at, j.updated_at) = sqlc.narg(cursor_updated_at)::timestamptz
        AND j.id < sqlc.narg(cursor_job_id)::uuid
      )
    )
  )
ORDER BY COALESCE(sr.updated_at, j.updated_at) DESC, j.id DESC
LIMIT sqlc.arg(limit_rows);

-- name: FindCustomerByEmail :one
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
WHERE tenant_id = sqlc.arg(tenant_id)
  AND lower(email) = lower(sqlc.arg(email));

-- name: FindCustomerByPhone :one
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
WHERE tenant_id = sqlc.arg(tenant_id)
  AND phone = sqlc.arg(phone);

-- name: GetJobByJobNumber :one
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
  AND job_number = sqlc.arg(job_number);

-- name: UpdateJobByJobNumber :one
UPDATE jobs
SET
  estimate_id = COALESCE(sqlc.narg(estimate_id)::uuid, estimate_id),
  customer_id = sqlc.arg(customer_id),
  status = COALESCE(sqlc.narg(status), status),
  scheduled_date = COALESCE(sqlc.narg(scheduled_date)::date, scheduled_date),
  pickup_time = COALESCE(sqlc.narg(pickup_time), pickup_time),
  updated_by = sqlc.narg(updated_by),
  updated_at = NOW()
WHERE tenant_id = sqlc.arg(tenant_id)
  AND job_number = sqlc.arg(job_number)
RETURNING *;

-- name: GetEstimateByNumber :one
SELECT *
FROM estimates
WHERE tenant_id = sqlc.arg(tenant_id)
  AND estimate_number = sqlc.arg(estimate_number);

-- name: UpdateEstimateByNumber :one
UPDATE estimates
SET
  customer_id = sqlc.arg(customer_id),
  status = COALESCE(sqlc.narg(status), status),
  customer_name = sqlc.arg(customer_name),
  primary_phone = sqlc.arg(primary_phone),
  secondary_phone = sqlc.narg(secondary_phone),
  email = sqlc.arg(email),
  origin_address_line1 = sqlc.arg(origin_address_line1),
  origin_city = sqlc.arg(origin_city),
  origin_state = sqlc.arg(origin_state),
  origin_postal_code = sqlc.arg(origin_postal_code),
  destination_address_line1 = sqlc.arg(destination_address_line1),
  destination_city = sqlc.arg(destination_city),
  destination_state = sqlc.arg(destination_state),
  destination_postal_code = sqlc.arg(destination_postal_code),
  move_date = sqlc.arg(move_date)::date,
  pickup_time = sqlc.narg(pickup_time),
  lead_source = sqlc.arg(lead_source),
  move_size = sqlc.narg(move_size),
  location_type = sqlc.narg(location_type),
  estimated_total_cents = sqlc.narg(estimated_total_cents)::bigint,
  deposit_cents = sqlc.narg(deposit_cents)::bigint,
  notes = sqlc.narg(notes),
  updated_by = sqlc.narg(updated_by),
  updated_at = NOW()
WHERE tenant_id = sqlc.arg(tenant_id)
  AND estimate_number = sqlc.arg(estimate_number)
RETURNING *;

-- name: CreateImportRun :one
INSERT INTO import_run (
  tenant_id,
  created_by_user_id,
  source,
  filename,
  file_sha256,
  mode,
  status,
  mapping_json,
  summary_json
) VALUES (
  sqlc.arg(tenant_id),
  sqlc.narg(created_by_user_id),
  sqlc.arg(source),
  sqlc.arg(filename),
  sqlc.arg(file_sha256),
  sqlc.arg(mode),
  sqlc.arg(status),
  sqlc.arg(mapping_json),
  sqlc.arg(summary_json)
)
RETURNING *;

-- name: CompleteImportRun :one
UPDATE import_run
SET
  status = sqlc.arg(status),
  summary_json = sqlc.arg(summary_json),
  completed_at = NOW()
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id)
RETURNING *;

-- name: GetImportRunByID :one
SELECT
  id,
  tenant_id,
  created_by_user_id,
  source,
  filename,
  file_sha256,
  mode,
  status,
  mapping_json,
  summary_json,
  created_at,
  completed_at
FROM import_run
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id);

-- name: UpsertImportRowResult :one
INSERT INTO import_row_result (
  tenant_id,
  import_run_id,
  row_number,
  severity,
  entity_type,
  idempotency_key,
  result,
  field,
  message,
  raw_value,
  target_entity_id
) VALUES (
  sqlc.arg(tenant_id),
  sqlc.arg(import_run_id),
  sqlc.arg(row_number),
  sqlc.arg(severity),
  sqlc.arg(entity_type),
  sqlc.arg(idempotency_key),
  sqlc.arg(result),
  sqlc.narg(field),
  sqlc.arg(message),
  sqlc.narg(raw_value),
  sqlc.narg(target_entity_id)
)
ON CONFLICT (tenant_id, import_run_id, entity_type, idempotency_key) DO UPDATE
SET
  row_number = EXCLUDED.row_number,
  severity = EXCLUDED.severity,
  result = EXCLUDED.result,
  field = EXCLUDED.field,
  message = EXCLUDED.message,
  raw_value = EXCLUDED.raw_value,
  target_entity_id = EXCLUDED.target_entity_id
RETURNING *;

-- name: ListImportRowResultsByRun :many
SELECT
  id,
  tenant_id,
  import_run_id,
  row_number,
  severity,
  entity_type,
  idempotency_key,
  result,
  field,
  message,
  raw_value,
  target_entity_id,
  created_at
FROM import_row_result
WHERE tenant_id = sqlc.arg(tenant_id)
  AND import_run_id = sqlc.arg(import_run_id)
ORDER BY row_number ASC, created_at ASC;

-- name: ListImportRowResultsByRunAndSeverity :many
SELECT
  id,
  tenant_id,
  import_run_id,
  row_number,
  severity,
  entity_type,
  idempotency_key,
  result,
  field,
  message,
  raw_value,
  target_entity_id,
  created_at
FROM import_row_result
WHERE tenant_id = sqlc.arg(tenant_id)
  AND import_run_id = sqlc.arg(import_run_id)
  AND severity = sqlc.arg(severity)
ORDER BY row_number ASC, created_at ASC
LIMIT sqlc.arg(limit_rows);

-- name: GetImportIdempotency :one
SELECT
  tenant_id,
  entity_type,
  idempotency_key,
  target_entity_id,
  first_seen_at,
  last_seen_at
FROM import_idempotency
WHERE tenant_id = sqlc.arg(tenant_id)
  AND entity_type = sqlc.arg(entity_type)
  AND idempotency_key = sqlc.arg(idempotency_key);

-- name: UpsertImportIdempotency :one
INSERT INTO import_idempotency (
  tenant_id,
  entity_type,
  idempotency_key,
  target_entity_id
) VALUES (
  sqlc.arg(tenant_id),
  sqlc.arg(entity_type),
  sqlc.arg(idempotency_key),
  sqlc.arg(target_entity_id)
)
ON CONFLICT (tenant_id, entity_type, idempotency_key) DO UPDATE
SET
  target_entity_id = EXCLUDED.target_entity_id,
  last_seen_at = NOW()
RETURNING *;

-- name: ExportCustomersRows :many
SELECT
  id,
  first_name,
  last_name,
  email,
  phone,
  created_at,
  updated_at
FROM customers
WHERE tenant_id = sqlc.arg(tenant_id)
ORDER BY created_at ASC;

-- name: ExportEstimatesRows :many
SELECT
  id,
  estimate_number,
  customer_name,
  email,
  primary_phone,
  secondary_phone,
  status,
  origin_city,
  origin_state,
  origin_postal_code,
  destination_city,
  destination_state,
  destination_postal_code,
  move_date,
  pickup_time,
  lead_source,
  estimated_total_cents,
  deposit_cents,
  notes,
  created_at,
  updated_at
FROM estimates
WHERE tenant_id = sqlc.arg(tenant_id)
ORDER BY created_at ASC;

-- name: ExportJobsRows :many
SELECT
  j.id,
  j.job_number,
  j.status,
  j.scheduled_date,
  j.pickup_time,
  c.first_name,
  c.last_name,
  c.email,
  c.phone,
  e.estimate_number,
  e.origin_city,
  e.origin_state,
  e.origin_postal_code,
  e.destination_city,
  e.destination_state,
  e.destination_postal_code,
  j.created_at,
  j.updated_at
FROM jobs j
JOIN customers c
  ON c.id = j.customer_id
  AND c.tenant_id = j.tenant_id
LEFT JOIN estimates e
  ON e.id = j.estimate_id
  AND e.tenant_id = j.tenant_id
WHERE j.tenant_id = sqlc.arg(tenant_id)
ORDER BY j.created_at ASC;

-- name: ExportStorageRows :many
SELECT
  sr.id,
  j.job_number,
  sr.facility,
  sr.status,
  sr.date_in,
  sr.date_out,
  sr.next_bill_date,
  sr.lot_number,
  sr.location_label,
  sr.vaults,
  sr.pads,
  sr.items,
  sr.oversize_items,
  sr.volume,
  sr.monthly_rate_cents,
  sr.storage_balance_cents,
  sr.move_balance_cents,
  sr.notes,
  sr.created_at,
  sr.updated_at
FROM storage_record sr
JOIN jobs j
  ON j.id = sr.job_id
  AND j.tenant_id = sr.tenant_id
WHERE sr.tenant_id = sqlc.arg(tenant_id)
ORDER BY sr.created_at ASC;

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
