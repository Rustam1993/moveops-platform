# Assumptions

## Phase 0
- MVP supports one tenant per customer company; tenant_id scopes all data.
- One job is created from one estimate in MVP (1:1).
- Storage record is optional and at most one per job in MVP (0/1).
- CSV is the primary migration format; Excel import is secondary and can be deferred if riskier.
- Pricing is minimal manual entry for MVP; calculator/tariff engine is post-MVP.
- Calendar is monthly view only for MVP.

## Phase 1
- Tenant is inferred from the authenticated user record at login (no tenant picker yet).
- Cookie sessions are server-side in Postgres and are the single auth mechanism for web/API in local MVP.
- CSRF tokens are tied to session rows and exposed via `GET /auth/csrf`; state-changing routes enforce `X-CSRF-Token` when `CSRF_ENFORCE=true` (default).
- Local seed data is development-only and includes one default tenant and one admin credential pair.
- Integration tests requiring a live Postgres instance use `TEST_DATABASE_URL`; when it is absent, those tests are skipped.

## Phase 2
- `POST /estimates` requires `Idempotency-Key` in the request header; body-level idempotency keys are not used.
- New estimate required fields: `customerName`, `primaryPhone`, `email`, origin address/city/state/postal code, destination address/city/state/postal code, `moveDate`, and `leadSource`.
- Optional estimate fields (`pickupTime`, `secondaryPhone`, `moveSize`, `locationType`, pricing fields, and notes) are stored when provided and omitted otherwise.

## Phase 3
- Calendar defaults:
  - UI defaults to `phase=booked` and `jobType=all`.
  - Month navigation drives query range as the first day of the month to the first day of the next month.
- Calendar date range semantics:
  - `GET /calendar` uses `[from, to)` (inclusive `from`, exclusive `to`).
- Phase/status naming:
  - The jobs table canonical lifecycle field remains `status` (`booked`, `scheduled`, `completed`, `cancelled`).
  - Calendar query accepts `phase` as a filter alias mapped to the same canonical `status` values.

## Phase 4
- Facility selection is required before querying storage. `GET /storage` requires `facility` and the UI keeps Storage list empty until a facility is selected.
- Storage filters in this phase:
  - Working: facility, search (`q`), status, delivery scheduled (`hasDateOut`), balance due (`balanceDue`), and has containers (`hasContainers`).
  - Stubbed in UI: “Past 30 days without payment” and “Monthly invoice cycle actions”.

## Phase 5
- Upload constraints:
  - `IMPORT_MAX_FILE_MB` defaults to `25` MB.
  - `IMPORT_MAX_ROWS` defaults to `5000` rows.
- Import file support:
  - CSV is required for this phase.
  - XLSX uploads are explicitly rejected with `XLSX_NOT_SUPPORTED`.
- Required import semantics:
  - customer row requires `customer_name` or a contact signal (`email` / `phone_primary`).
  - estimate upsert requires `origin_zip`, `destination_zip`, and `requested_pickup_date` when estimate fields are present.
- `job_number` stance:
  - strongly recommended for deterministic job idempotency.
  - if missing, system generates a deterministic import job number and emits a warning.

## Phase 6
- CSRF is mandatory for all state-changing API routes except `POST /auth/login`.
- API body limits:
  - default API requests: `API_MAX_BODY_MB` (default `2` MB).
  - import uploads: `IMPORT_MAX_FILE_MB` (default `25` MB).
- CSP differences:
  - production uses strict baseline CSP.
  - development keeps required Next.js allowances (`unsafe-eval` for dev tooling).
- Authorization response policy:
  - missing permission on same-tenant endpoint returns `403`.
  - cross-tenant resource access returns `404` to avoid resource existence leaks.
