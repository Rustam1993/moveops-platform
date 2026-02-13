# Runbook: Environment Variables

## API (apps/api)
Secrets (Container Apps secrets):
- `DATABASE_URL` (TLS required: `sslmode=require`)
- `SESSION_SECRET`
- `CSRF_SECRET`

Non-secrets:
- `APP_ENV=production`
- `API_ADDR=:8080`
- `SESSION_COOKIE_NAME=mo_sess`
- `SESSION_TTL_HOURS=12`
- `COOKIE_SECURE=true`
- `CSRF_ENFORCE=true`
- `CORS_ALLOWED_ORIGINS=<web public url>`
- `MIGRATE_ON_START=false` (prod)
- `SEED_ON_START=false` (prod)

Dev/local notes:
- `docker-compose.yml` sets `MIGRATE_ON_START=true` and `SEED_ON_START=true` for convenience.

## Web (apps/web)
Non-secrets:
- `NODE_ENV=production`
- `PUBLIC_BASE_URL=<web public url>`

API routing:
- `NEXT_PUBLIC_API_URL=/api` (prod)
  - Browser calls go to the web app.
  - Web proxies requests to the API using `API_INTERNAL_URL`.
- `API_INTERNAL_URL=http://<api internal fqdn>/api` (prod)

Dev/local notes:
- `docker-compose.yml` uses `NEXT_PUBLIC_API_URL=http://localhost:8080/api` (direct API from browser).
