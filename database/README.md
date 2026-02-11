# Database

PostgreSQL schema and migration/seeding flow used by the API.

## Where schema lives
- Migration files: `apps/api/migrations`
- SQL schema reference for sqlc/test setup: `apps/api/sql/schema.sql`
- Query definitions for generated access layer: `apps/api/sql/queries.sql`

## Main entities
- `tenants`
- `users`
- `roles`
- `permissions`
- `role_permissions`
- `user_roles`
- `customers`
- `sessions`
- `audit_log`

## Multi-tenant and RBAC model
- Tenant-scoped data includes `tenant_id` and uses tenant-aware queries.
- Users are assigned roles via `user_roles`.
- Roles map to permissions via `role_permissions`.

## Migrations
Apply migrations from `apps/api`:
```bash
go run ./cmd/migrate -dir ./migrations
```

Or with goose binary:
```bash
goose -dir ./migrations postgres "$DATABASE_URL" up
```

## Seed data (local/dev)
From `apps/api`:
```bash
go run ./cmd/seed
```

This creates/updates:
- default tenant
- admin user
- admin role
- `customers.read` and `customers.write` permissions

## Local DB start options
Option 1: Compose stack (recommended):
```bash
docker compose up --build
```

Option 2: DB only:
```bash
docker compose up db
```
Then run migrate/seed/server manually from `apps/api`.

## Testing
- API integration tests can target a dedicated DB with:
  - `TEST_DATABASE_URL=postgres://...`
- Tests reset schema and apply `apps/api/sql/schema.sql`.
