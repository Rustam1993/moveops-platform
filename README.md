# **MoveOps Platform**

Modern moving-company operations SaaS (MVP): New Estimate, Storage, Calendar, plus migration/import from legacy exports.
Stack: Go + Next.js + Postgres, Azure-ready, security-first.

Repo layout
```
apps/
  api/                 # Go backend (REST API)
  web/                 # Next.js frontend
packages/
  client/              # Generated TypeScript API client/types from OpenAPI
infra/
  docker/              # docker-compose for local dev
  terraform/           # Azure IaC (modular; AWS later)
docs/
  architecture.md
  assumptions.md
  decisions.md
  threat-model.md
  migration-plan.md
  runbook-azure.md

```

# **Getting Started**

## *Prereqs*:
- Docker Desktop
- Go 1.22+
- Node.js (LTS)

## Configure environment
```
cp apps/api/.env.example apps/api/.env
cp apps/web/.env.example apps/web/.env
```

## Start Services
```
docker compose up --build
```

## Defaults:
- Web: http://localhost:3000
- API: http://localhost:8080
- Postgres: localhost:5432

## Run DB migrations + seed
Exact command depends on the migration tool used in apps/api.
```
cd apps/api

# Example (goose):
# goose -dir ./migrations postgres "$DATABASE_URL" up
# go run ./cmd/seed

```

## API contract (OpenAPI)

Source of truth:
apps/api/openapi.yaml
Used for:
- Generating packages/client for the web app
- Request/response validation
- Interactive docs (Swagger/Redoc) if enabled

## MVP scope
**Included**:
- New Estimate: create/save estimate, minimal pricing fields, convert estimate → job
- Calendar: monthly view of scheduled jobs with basic filters
- Storage: storage list + filters + editable storage fields
- Migration/Import: admin import (CSV first; Excel optional) with dry-run + idempotency
- Export: CSV export for Customers / Estimates / Jobs / Storage

**Not included (yet)**:
- Inventory module
- Automated invoicing cycles
- Payments gateway integration
- SMS/email automations
  
  **Migration / Import**
- Admin-only import screen
- Dry-run shows: counts, duplicates, validation errors
- Import is idempotent (re-importing the same file should not duplicate records)
Details: docs/migration-plan.md

## Security baseline
- Multi-tenant isolation (tenant_id everywhere) + automated cross-tenant tests
- RBAC enforced server-side
- Secure sessions + CSRF protection (cookie auth)
- Rate limiting (auth/search)
- Input validation (OpenAPI + business rules)
- Audit log for sensitive changes
- CI: static analysis + dependency scanning
Details: docs/threat-model.md


## Tests
**Backend unit tests**
```
cd apps/api
go test ./...
```
**E2E smoke test (Playwright)**
```
cd apps/web
npm ci
npm run test:e2e
```
**Smoke flow:**
login → create estimate → convert/schedule → appears on calendar → update storage → appears in storage list

## Deployment (Azure)
**Target**:
- Azure Container Apps (web + api)
- Azure Database for PostgreSQL Flexible Server
- Terraform/OpenTofu IaC
Runbook: docs/runbook-azure.md

**Contributing (internal)**
- Keep changes small and phase-aligned
- Update docs/decisions.md for meaningful architectural changes
- Add tests for anything security-sensitive (RBAC/tenant isolation/import)

## License
Proprietary (update when ready).
