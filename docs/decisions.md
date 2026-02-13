# Decisions

## Phase 0
- Treat OpenAPI as the source of truth for backend + generated TS client (Phase 1+).
- Migration tooling is an MVP feature (sales-critical), not “later”.
- Use original UI wording; legacy labels are used only for mapping and import parity.
- Multi-tenant + RBAC + audit logging are non-negotiable foundations (Phase 1).

## Phase 1
- Backend stack:
  - Go 1.22+
  - Router: chi
  - OpenAPI codegen: `oapi-codegen` (`v2.4.1`, pinned in Makefile)
  - OpenAPI request validation middleware: `oapi-codegen/nethttp-middleware`
  - DB: PostgreSQL
  - SQL access: sqlc (`v1.27.0`, pinned in Makefile) + pgx v5
  - Migrations: goose format files + goose runtime (`v3.22.1`, pinned in Makefile)
  - Password hashing: argon2id
- Frontend stack:
  - Next.js + TypeScript + Tailwind + minimal shadcn-style component setup.
- Client package:
  - TypeScript API types/client are generated from `apps/api/openapi.yaml` into `packages/client/src`.
- Tenant isolation status code rule:
  - Customer lookup across tenants returns `404 Not Found` (not `403`) to avoid disclosing record existence across tenant boundaries.
- Permissions model:
  - Use normalized `permissions`, `role_permissions`, and `user_roles` tables (instead of hardcoded permissions in application code) so permission assignment remains tenant-configurable from the data layer.
- Session model:
  - Session cookies carry an opaque token; DB stores only token hash + CSRF token + expiry/revocation timestamps.

## Phase 2
- Tenant-scoped numbering:
  - Use a `tenant_counters` table (`tenant_id`, `counter_type`, `next_value`) and an atomic `UPDATE ... RETURNING` path to issue `estimate_number` and `job_number` values without race conditions.
  - Formatted identifiers are generated in application code as `E-%06d` and `J-%06d`.
- Idempotency storage strategy:
  - Use nullable idempotency key columns directly on business tables:
    - `estimates.idempotency_key` with unique index `(tenant_id, idempotency_key)` when present.
    - `jobs.convert_idempotency_key` with unique index `(tenant_id, convert_idempotency_key)` when present.
  - Store `estimates.idempotency_payload_hash` to detect key reuse with different payloads and return `409 IDEMPOTENCY_KEY_REUSE`.

## Phase 3
- Permission model for calendar flows:
  - Introduce explicit `calendar.read` and `calendar.write` permissions.
  - `GET /calendar` requires `calendar.read`.
  - `PATCH /jobs/{jobId}` schedule/phase edits now require `calendar.write` (instead of `jobs.write`) so calendar access can be controlled independently.
  - Seed role mapping:
    - `admin`: all existing permissions plus `calendar.read` and `calendar.write`.
    - `ops`: `calendar.read`, `calendar.write`, `jobs.read`, `jobs.write`, and `estimates.read`.
    - `sales`: `calendar.read`, `jobs.read`, and existing estimate permissions.
- Canonical lifecycle field:
  - Keep `jobs.status` as canonical; no separate `phase` DB column was added.
  - Calendar filters expose `phase` as API-level naming, mapped directly to `jobs.status` values.

## Phase 4
- Storage endpoint style:
  - `POST /jobs/{jobId}/storage` creates the single storage record for a job (0/1 model).
  - `PUT /storage/{storageRecordId}` performs full editable-field updates for an existing record (no implicit upsert).
  - `GET /storage/{storageRecordId}` returns the canonical drawer payload.
- Storage list pagination cursor strategy:
  - Keyset pagination ordered by `COALESCE(storage_record.updated_at, jobs.updated_at) DESC, jobs.id DESC`.
  - Cursor is base64url-encoded `updatedAt|jobId` and returned as `nextCursor`.
- Storage permission mapping:
  - Added permissions: `storage.read`, `storage.write`.
  - Seeded role mapping:
    - `admin`: both `storage.read` and `storage.write`.
    - `ops`: both `storage.read` and `storage.write`.
    - `sales`: `storage.read` only.

## Phase 5
- Import endpoint style:
  - `POST /imports/dry-run` and `POST /imports/apply` both accept multipart file + options.
  - `GET /imports/{importRunId}` is the canonical status/summary read endpoint.
  - report artifacts are served as:
    - `GET /imports/{importRunId}/errors.csv`
    - `GET /imports/{importRunId}/report.json`
- Idempotency approach:
  - Natural keys are used first (email, phone, job_number, estimate_number, storage by job_id).
  - Cross-run mapping is persisted in `import_idempotency` to keep deterministic dedupe across repeated imports.
  - Row-level outcomes are persisted in `import_row_result` keyed by `(tenant_id, import_run_id, entity_type, idempotency_key)`.
- XLSX decision:
  - deferred for this phase; API rejects `.xlsx` with `XLSX_NOT_SUPPORTED`.
  - UI directs users to export legacy spreadsheets to CSV before upload.
- Permissions and access:
  - Added `imports.read`, `imports.write`, `exports.read`.
  - Seed mapping is admin-only for Phase 5:
    - `admin`: all three permissions.
    - `ops`: none by default.
    - `sales`: none by default.

## Phase 6
- CSRF mechanism:
  - Synchronizer token tied to session row (`sessions.csrf_token`).
  - `GET /auth/csrf` returns the active session token.
  - All mutating endpoints (`POST/PUT/PATCH/DELETE`) require `X-CSRF-Token`, except `POST /auth/login`.
  - Invalid or missing token returns `403` with code `CSRF_INVALID`.
- API security headers:
  - `X-Frame-Options: DENY`
  - `X-Content-Type-Options: nosniff`
  - `Referrer-Policy: strict-origin-when-cross-origin`
  - `Permissions-Policy: camera=(), microphone=(), geolocation=(), payment=(), usb=()`
  - API CSP: `default-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'none'`
  - HSTS enabled only when `APP_ENV=prod`: `max-age=31536000; includeSubDomains`.
- Web CSP decision:
  - Production CSP is strict baseline with `default-src 'self'`, `frame-ancestors 'none'`, and blocked plugin/object sources.
  - `style-src 'unsafe-inline'` is allowed for Tailwind runtime styles.
  - `script-src 'unsafe-inline'` remains as a minimal compatibility exception for current Next runtime bootstrap; no third-party script origins are allowed.
  - Development CSP allows `unsafe-eval` plus local HTTP/WS origins required by Next dev tooling.
- Rate limit strategy:
  - Keep endpoint-specific limiters (login/search/import/export) and add a global baseline API limiter.
  - Use bounded in-memory limiter state (`RATE_LIMIT_MAX_IPS`) with periodic cleanup + oldest-entry eviction to avoid unbounded growth.
  - Return `429` with `RATE_LIMITED` for limiter denials.
- Tenant isolation response policy:
  - Same-tenant permission failures return `403`.
  - Cross-tenant entity access returns `404` to avoid existence disclosure.
