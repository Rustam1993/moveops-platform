# Migration Plan (Phase 5)

Goal: onboard legacy data safely with an admin-only import flow that supports dry-run validation, idempotent apply, and tenant-scoped CSV exports.

## Implemented scope
- Import UI at `/import` with four steps:
  - upload
  - column mapping
  - dry-run summary + downloadable reports
  - apply import + completion summary
- Backend import endpoints:
  - `POST /imports/dry-run`
  - `POST /imports/apply`
  - `GET /imports/{importRunId}`
  - `GET /imports/{importRunId}/errors.csv`
  - `GET /imports/{importRunId}/report.json`
  - `GET /imports/templates/{template}.csv`
- Backend export endpoints:
  - `GET /exports/customers.csv`
  - `GET /exports/estimates.csv`
  - `GET /exports/jobs.csv`
  - `GET /exports/storage.csv`

## Input formats
- CSV: supported.
- XLSX: not supported in this phase; API returns `XLSX_NOT_SUPPORTED` and UI instructs CSV export first.

## Limits and safety
- Upload size limit: `IMPORT_MAX_FILE_MB` (default `15`).
- Row limit per file: `IMPORT_MAX_ROWS` (default `5000`).
- Import endpoints are multipart-only and CSV content-type validated.
- Import/export endpoints are IP rate-limited.
- Import/export endpoints require authenticated session and RBAC permissions.

## Idempotency and dedupe (implemented)
- Customers:
  - priority: email, then primary phone, then fallback deterministic key.
  - stored cross-run mapping in `import_idempotency`.
- Jobs:
  - primary key: `job_number` when present.
  - if missing, deterministic import job number is generated with warning.
- Estimates:
  - `estimate_number` when present.
  - otherwise deterministic fallback key derived from customer/date/route signals.
- Storage:
  - upsert by `job_id` (one storage record per job).
- Cross-run idempotency:
  - `import_idempotency` table maps `(tenant_id, entity_type, idempotency_key)` to target entity id.
  - re-importing the same CSV updates/skips existing entities instead of duplicating rows.

## Reporting
- Every import creates `import_run`.
- Row outcomes are written to `import_row_result` with:
  - row number
  - severity
  - entity type
  - result
  - field/message
  - optional raw value (truncated)
- Dry-run and apply both return summary counts and report download URLs.

## Security posture for Phase 5
- Admin-only by seeded role mapping:
  - `imports.read`
  - `imports.write`
  - `exports.read`
- Audit events:
  - `import.dry_run_started`
  - `import.dry_run_completed`
  - `import.apply_started`
  - `import.apply_completed`
  - `export.download`
- No raw CSV rows are logged in server logs.
