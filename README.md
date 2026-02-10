# MoveOps Platform

Phase 1 foundation monorepo for MoveOps: API auth/session + tenancy + RBAC + audit log baseline, plus minimal web shell and generated API client package.

## Repository Layout

```
apps/
  api/                 # Go API (OpenAPI, auth, tenancy, RBAC, audit, migrations)
  web/                 # Next.js app (login + dashboard placeholder)
packages/
  client/              # Generated TypeScript API types/client
infra/
  docker-compose.yml   # Local stack definition
docs/
  assumptions.md
  decisions.md
```

## Quickstart (Local)

1. Copy env examples if you want local overrides:

```bash
cp .env.example .env
cp apps/api/.env.example apps/api/.env
cp apps/web/.env.example apps/web/.env
```

2. Start everything:

```bash
docker compose up --build
```

3. Open:
- Web: `http://localhost:3000`
- API: `http://localhost:8080/api`
- Health: `http://localhost:8080/api/health`

Seeded local admin (dev-only):
- Email: `admin@local.moveops`
- Password: `Admin12345!`

## Dev Commands

```bash
make gen      # openapi/sqlc/client generation (pinned tool versions)
make test     # go test ./... (apps/api)
make lint     # gofmt
```

## Notes

- OpenAPI source of truth: `apps/api/openapi.yaml`.
- Tenant isolation is enforced in SQL and middleware; cross-tenant customer reads return 404 by decision.
- Security hardening beyond baseline (strict CSP, advanced rate limits, session anomaly detection, etc.) is deferred to later phases.
