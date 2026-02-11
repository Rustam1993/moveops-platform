# Infrastructure (Local)

Local infrastructure orchestration for MoveOps Phase 1.

## Files
- `infra/docker-compose.yml` (canonical compose definition)
- `docker-compose.yml` (root convenience copy)

## Services
- `db`:
  - Postgres 16
  - Port `5432`
  - Named volume `moveops_pgdata`
- `api`:
  - Built from `apps/api/Dockerfile`
  - Port `8080`
  - Runs migrate + seed + server on startup
- `web`:
  - Built from `apps/web/Dockerfile`
  - Port `3000`
  - Uses `NEXT_PUBLIC_API_URL=http://localhost:8080/api`

## Start and stop
From repo root:
```bash
docker compose up --build
```

Stop and remove volumes:
```bash
docker compose down -v
```

## Operational notes
- `api` waits for `db` healthcheck before startup.
- DB credentials and seed defaults are currently embedded in compose for local dev.
- For non-local environments, move secrets to environment/secret manager and avoid default credentials.
