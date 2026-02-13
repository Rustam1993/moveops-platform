# Runbook: Security Operations

## Required env vars (production)

### API
- `APP_ENV=prod`
- `COOKIE_SECURE=true` (forced automatically when `APP_ENV=prod`)
- `CSRF_ENFORCE=true`
- `CORS_ALLOWED_ORIGINS=<trusted web origin(s)>`
- `API_MAX_BODY_MB=2`
- `IMPORT_MAX_FILE_MB=25`
- `IMPORT_MAX_ROWS=5000` (adjust as needed)
- `API_READ_HEADER_TIMEOUT_SEC=5`
- `API_READ_TIMEOUT_SEC=15`
- `API_WRITE_TIMEOUT_SEC=30`
- `API_IDLE_TIMEOUT_SEC=60`
- `RATE_LIMIT_MAX_IPS=10000` (or lower based on expected traffic)

### Web
- `NEXT_PUBLIC_API_URL=<public API origin>/api`
- deploy behind TLS so HSTS and secure cookies are meaningful end-to-end.

## Header verification checklist

### API
Run:
```bash
curl -I https://<api-host>/api/health
```
Expected:
- `X-Content-Type-Options: nosniff`
- `Referrer-Policy: strict-origin-when-cross-origin`
- `X-Frame-Options: DENY`
- `Permissions-Policy: ...`
- `Content-Security-Policy: ...`
- `Strict-Transport-Security: ...` (prod only)

### Web
Run:
```bash
curl -I https://<web-host>/login
```
Expected:
- CSP header present (prod strict policy)
- frame/content/referrer/permissions headers present.

## CSRF verification checklist
1. Login and obtain session cookie.
2. Call mutating endpoint without `X-CSRF-Token` -> expect `403` + `CSRF_INVALID`.
3. Call `GET /auth/csrf`, then retry same mutating endpoint with header -> expect success.

## Tenant isolation verification
- Create tenant A + tenant B data.
- From tenant B, request tenant A resource ID.
- Expect `404` (not `403`) for existing cross-tenant resources.

## Incident response basics

### Suspected session compromise
1. Rotate affected user sessions:
  - revoke session rows for impacted users.
  - for broad compromise, revoke all active sessions (global logout).
2. Rotate deployment secrets used by app/runtime (and redeploy), including database credentials and any shared environment secrets.
3. Reset account credentials if needed.
4. Review audit logs around:
  - `auth.login`, `auth.logout`
  - state-changing business actions
  - import/export events.

### Elevated abuse/rate-limit pressure
1. Lower limiter thresholds temporarily.
2. Restrict CORS origins to known domains only.
3. Monitor 429 volume and source IP distribution.

### Import misuse
1. Disable `imports.write` assignment at role level if necessary.
2. Review `import_run` + `import_row_result` metadata.
3. Confirm no PII-rich logs were emitted outside import result tables.
