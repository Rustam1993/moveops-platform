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
