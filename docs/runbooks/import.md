# Runbook: Import and Export

## Access
- Requires authenticated admin session with:
  - `imports.write` for dry-run/apply
  - `imports.read` for run/report retrieval
  - `exports.read` for CSV exports

## Environment limits
- `IMPORT_MAX_FILE_MB` (default `15`)
- `IMPORT_MAX_ROWS` (default `5000`)

## Local run (dev)
1. Start stack:
```bash
docker compose up --build
```
2. Login as admin in web app.
3. Open `/import`.
4. Upload CSV and map fields.
5. Run dry-run and inspect summary.
6. Download `errors.csv` and `report.json` if needed.
7. Apply import.
8. Verify data in `/calendar` and `/storage`.

## API flow (manual)
1. `POST /imports/dry-run` multipart:
  - `file`: CSV
  - `options`: JSON string (`source`, `hasHeader`, `mapping`)
2. `GET /imports/{importRunId}`
3. `GET /imports/{importRunId}/errors.csv`
4. `POST /imports/apply` with same payload
5. Optional tenant exports:
  - `GET /exports/customers.csv`
  - `GET /exports/estimates.csv`
  - `GET /exports/jobs.csv`
  - `GET /exports/storage.csv`

## Troubleshooting
- `XLSX_NOT_SUPPORTED`:
  - Export spreadsheet to CSV and re-upload.
- `invalid_mapping`:
  - Verify mapping values match CSV header names exactly (or valid indexes).
- `row_limit_exceeded`:
  - Split file into smaller batches or increase `IMPORT_MAX_ROWS`.
- `rate_limited`:
  - Retry after the limiter window.
- `forbidden`:
  - Verify role permissions include `imports.*` / `exports.read`.

## Audit and safety checks
- Confirm audit events are written:
  - `import.dry_run_started`
  - `import.dry_run_completed`
  - `import.apply_started`
  - `import.apply_completed`
  - `export.download`
- Review that tenant scoping is enforced on:
  - import run retrieval
  - report downloads
  - export CSV queries
