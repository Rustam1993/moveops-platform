# Phase 7: Tenant Admin Config + Metrics + Hardening

Phase 7 makes New Estimate operable at tenant scale by adding admin-managed settings, tenant metrics, and targeted production hardening.

## Admin settings (tenant-scoped)

All admin endpoints are protected by `admin.new_estimate` and tenant-scoped in queries.

### 1) Inventory catalog manager

Routes:
- Web: `/admin/new-estimate/catalog`
- API:
  - `GET /admin/new-estimate/catalog/categories`
  - `POST /admin/new-estimate/catalog/categories`
  - `PATCH /admin/new-estimate/catalog/categories/{categoryId}`
  - `DELETE /admin/new-estimate/catalog/categories/{categoryId}`
  - `GET /admin/new-estimate/catalog/items`
  - `POST /admin/new-estimate/catalog/items`
  - `PATCH /admin/new-estimate/catalog/items/{itemId}`
  - `DELETE /admin/new-estimate/catalog/items/{itemId}`
  - `POST /admin/new-estimate/catalog/import` (CSV)
  - `GET /admin/new-estimate/catalog/export` (CSV)

Behavior:
- Catalog is now tenant-managed for categories/items with `volume_cf`, `active`, and `sort_order`.
- Inventory screen consumes tenant catalog from `GET /estimates/{estimateId}/inventory/catalog`.
- If tenant catalog has no active items, app falls back to built-in defaults.

Audit events:
- `catalog.category.created|updated|deleted`
- `catalog.item.created|updated|deleted`
- `catalog.imported`

### 2) Pricing defaults

Routes:
- Web: `/admin/new-estimate/pricing`
- API:
  - `GET /admin/new-estimate/pricing`
  - `PUT /admin/new-estimate/pricing`

Fields include:
- Long distance defaults (rate per CF, fuel %, tax %)
- Local defaults (labor rate, travel rate, fuel %, tax %)
- Discount presets (senior %, coupon %)
- Liability defaults (type + valuation fee)

Integration:
- Charges loader applies defaults only when estimate charges are empty/initial.
- Existing estimate-level charges are never overwritten by admin defaults.

Audit event:
- `pricing_defaults.updated`

### 3) Email templates manager

Routes:
- Web: `/admin/new-estimate/email-templates`
- API:
  - `GET /admin/new-estimate/email-templates`
  - `PUT /admin/new-estimate/email-templates`
  - `POST /admin/new-estimate/email-templates/test-send`

Templates:
- `e_quote`
- `inventory_link`
- `e_sign`

Behavior:
- Email Center resolves tenant template first; falls back to system defaults.
- Template preview supports placeholder rendering with sample values.
- Test-send supports dev-friendly delivery mode.

Audit events:
- `email_templates.updated`
- `email_templates.test_sent`

### 4) Printed estimate branding

Routes:
- Web: `/admin/new-estimate/documents`
- API:
  - `GET /admin/new-estimate/documents`
  - `PUT /admin/new-estimate/documents`

Fields:
- Company display name
- Company phone/email
- Optional logo URL
- Terms snippet

Integration:
- Printed estimate PDF generation reads branding from tenant settings and embeds it in generated documents.

Audit event:
- `document_branding.updated`

## Measurement loop

### Analytics event sink

Endpoint:
- `POST /analytics/events` (authenticated, tenant-scoped)

Storage:
- `analytics_event` table with tenant, optional estimate/user, event name, JSON properties, timestamp.

Guardrails:
- Allow-list of supported event names.
- Property sanitization removes PII-like keys (`email`, `phone`, `address`, `notes`, `customer`).

### Metrics dashboard

Routes:
- Web: `/admin/new-estimate/metrics`
- API:
  - `GET /admin/new-estimate/metrics`

Displayed aggregates:
- Median time-to-quote (`entry_started` -> `quote_sent`)
- Quote -> Sign conversion
- Sign -> Book conversion
- Inventory completion rate (`inventory_link_sent` -> `inventory_updated`)
- Stuck estimates count (open/draft > 7 days)

Audit event:
- `metrics.viewed`

## Audit log viewer

Routes:
- Web: `/admin/audit-logs`
- API:
  - `GET /admin/audit-logs`

Features:
- Tenant-scoped, paginated list
- Filters: date range, actor user ID, action, entity type, entity ID

## Public token endpoint hardening

Added an additional per-IP token-attempt limiter layer on public routes:
- `/public/inventory/{token}`
- `/public/estimate/{token}`
- `/public/sign/{token}`

This is chained with existing public endpoint rate limiting middleware to reduce brute-force token attempts.

## Data model additions

Migration:
- `00009_phase10_admin_settings_metrics.sql`

Tables:
- `new_estimate_catalog_category`
- `new_estimate_catalog_item`
- `tenant_new_estimate_settings`
- `analytics_event`

All rows include tenant scoping.

## Test coverage added

Backend integration coverage:
- Admin endpoint tenant isolation and RBAC checks
- CSV import validation (malformed + valid)

Frontend/e2e coverage:
- Admin catalog item appears in Inventory UI
- Full happy path smoke: Entry -> Inventory -> Charges -> Quote -> Sign -> Book
