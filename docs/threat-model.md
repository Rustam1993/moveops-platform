# Threat Model (Phase 6)

## Scope
- API (`apps/api`) using cookie-based sessions + CSRF.
- Web app (`apps/web`) using Next.js + React.
- Tenant-scoped business entities (customers, estimates, jobs, storage, imports/exports).

## Primary threats and mitigations

### XSS
- Mitigations:
  - React output encoding defaults for dynamic content.
  - Web CSP headers configured in `next.config.ts`.
  - `frame-ancestors 'none'`, `object-src 'none'`, `base-uri 'self'` in CSP.
- Residual risk:
  - Next runtime currently requires inline bootstrap scripts, so production `script-src` includes a minimal `'unsafe-inline'` exception.

### CSRF
- Threat:
  - cookie-based session auth enables CSRF risk for mutating requests.
- Mitigations:
  - synchronizer token (`csrf_token`) stored server-side in session rows.
  - `GET /auth/csrf` exposes token for active session.
  - all mutating endpoints require `X-CSRF-Token`.
  - invalid/missing token returns `403` with code `CSRF_INVALID`.
  - `POST /auth/login` is explicitly exempt.

### IDOR / tenant data leakage
- Threat:
  - cross-tenant reads/updates on entity IDs.
- Mitigations:
  - all entity queries scoped by `tenant_id`.
  - integration tests assert cross-tenant access is denied.
  - policy:
    - existing resource in other tenant -> `404`
    - missing permission on same-tenant route -> `403`.

### Injection
- Mitigations:
  - sqlc-generated parameterized SQL.
  - OpenAPI request validation middleware.
  - endpoint-level input validation and type parsing.

### Auth/session risks
- Mitigations:
  - opaque session token in cookie; DB stores hashed token.
  - session revocation on logout.
  - CSRF token bound to session.
  - secure cookies forced in production (`COOKIE_SECURE=true` when `APP_ENV=prod`).

### Import pipeline risks
- Threats:
  - malicious/malformed uploads
  - abuse via huge files / high request rates
  - accidental PII logging
- Mitigations:
  - multipart validation, CSV-only support, explicit XLSX rejection.
  - body-size and row-count limits.
  - import/export rate limiting.
  - import row diagnostics store row number/field/message and truncated raw values only.

### Rate limiting / abuse
- Mitigations:
  - endpoint-specific and global API limiters.
  - bounded in-memory limiter (max tracked IPs + cleanup) to avoid unbounded growth.
  - `429` responses use `RATE_LIMITED`.

### Logging and PII
- Mitigations:
  - audit log captures action metadata and request IDs.
  - no raw full import rows in audit or app logs.
  - import row `raw_value` is truncated.
