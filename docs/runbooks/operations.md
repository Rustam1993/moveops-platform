# Runbook: Operations

## Logs
- Web/API logs are forwarded to Log Analytics via Container Apps Environment configuration.

## Scaling knobs
- Web defaults to `minReplicas=0` for cost.
  - Tradeoff: cold starts on first request.
- API defaults to `minReplicas=1` to keep auth/session endpoints responsive.

## Database backups
- Postgres Flexible Server defaults:
  - Burstable SKU (minimal)
  - storage 32GB
  - backup retention 7 days

## Postgres networking
- MVP default allows Azure services (0.0.0.0 firewall rule) and optionally explicit IP ranges.
- Wide-open firewall is intentionally not the default.

## Security posture summary
- Web is public ingress only.
- API is internal ingress only.
- CSRF enforced on mutating endpoints.
- CSP + security headers enabled.
- Rate limiting enabled on API.
- Migrations are executed via a one-shot job (not API startup).
