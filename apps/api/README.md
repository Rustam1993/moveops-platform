# API (Backend)

Go service that exposes the MoveOps HTTP API under `/api`.

## Tech stack
- Go 1.22+
- Router: chi
- OpenAPI validation: `oapi-codegen/nethttp-middleware`
- DB access: pgx + sqlc-generated queries
- Migrations: goose format
- Auth: Postgres-backed sessions + httpOnly cookie

## How it is built
- Entrypoints:
  - `cmd/server` (API server)
  - `cmd/migrate` (apply migrations)
  - `cmd/seed` (seed local tenant/admin and RBAC)
- Runtime requires `openapi.yaml` in working directory.
- Docker image builds all 3 binaries and starts with:
  - migrate
  - seed
  - api

## Required environment variables
Minimum:
- `DATABASE_URL` (required)

Common optional:
- `API_ADDR` default `:8080`
- `APP_ENV` default `dev`
- `SESSION_COOKIE_NAME` default `mo_sess`
- `SESSION_TTL_HOURS` default `12`
- `COOKIE_SECURE` default `false` (forced true when `APP_ENV=prod`)
- `CSRF_ENFORCE` default `true`

Seed-specific:
- `SEED_TENANT_SLUG` default `local-dev`
- `SEED_TENANT_NAME` default `Local Dev Tenant`
- `SEED_ADMIN_EMAIL` default `admin@local.moveops`
- `SEED_ADMIN_PASSWORD` default `Admin12345!`
- `SEED_ADMIN_NAME` default `Local Admin`

## Start locally (without Docker for API process)
Run from `apps/api`.

1. Set env (or copy `.env.example` to `.env`).
2. Ensure Postgres is reachable via `DATABASE_URL`.
3. Apply migrations:
```bash
go run ./cmd/migrate -dir ./migrations
```
4. Seed dev data:
```bash
go run ./cmd/seed
```
5. Start server:
```bash
go run ./cmd/server
```

Health check:
```bash
curl http://localhost:8080/api/health
```

## Start via Docker Compose
From repo root:
```bash
docker compose up --build
```

## Test
From `apps/api`:
```bash
go test ./...
```

Notes:
- Integration tests in `internal/app/app_integration_test.go` need `TEST_DATABASE_URL` for full execution.
- Without `TEST_DATABASE_URL`, integration tests are skipped.

## How it works (high level)
- Middleware stack: request-id, security headers, structured logging, OpenAPI request validation.
- Auth flow:
  - `POST /api/auth/login` validates credentials and creates DB session.
  - `GET /api/auth/me` loads actor from session cookie.
  - `GET /api/auth/csrf` returns CSRF token from session.
  - `POST /api/auth/logout` revokes session.
- Tenancy/RBAC:
  - Session includes tenant context.
  - Protected routes require permission checks.
  - Customer queries are tenant-scoped.
- Audit:
  - Writes events for auth login/logout and customer create.
