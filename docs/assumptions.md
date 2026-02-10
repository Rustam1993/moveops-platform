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
