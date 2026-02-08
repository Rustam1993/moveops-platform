# Migration Plan (Phase 0)

Goal: onboard customers from legacy exports (Granot-style) with a low-friction, safe, idempotent importer.

## Supported input formats (MVP)
1) CSV (primary)
- Required for MVP import path.
- Recommended: UTF-8, comma-delimited, header row.

2) Excel (secondary)
- Support only if parsing is stable and safe (e.g., .xlsx via a maintained library).
- If Excel parsing is deferred: user must export to CSV first (documented in UI).

3) Manual mapping fallback (MVP)
- If headers don’t match, user maps columns to fields in UI.
- Template download: we provide a “canonical CSV” template per entity.

## Import modes
- Dry-run (no writes):
  - parse + validate + map + dedupe simulation
  - outputs a report (JSON + downloadable CSV error report)
- Import (writes):
  - upserts customers
  - upserts estimates
  - optionally creates jobs + storage records when enough fields exist

## Entities supported
- Customer
- Estimate
- Job (created from legacy job rows OR via “convert estimate” mode)
- StorageRecord (attached to job)

## Column mapping strategy
- UI shows detected headers and suggested mappings.
- Canonical fields are grouped by entity:
  - Customer: name, email, phones
  - Estimate: origin/dest addresses, requested date/time, lead source, move size, minimal pricing
  - Job: job number, status, pickup date/time, job type
  - Storage: site, dates, lot/location, counts, volume, monthly, balances

## Validation rules (MVP)
General:
- Enforce tenant scoping.
- File size limit (e.g., 10–25MB) and row limit (configurable).
- Required canonical fields:
  - customer.full_name OR (email/phone) must exist
  - estimate: origin zip + destination zip + requested pickup date (if importing estimates)
  - storage: job_number (or job_id via mapping) required to attach record

Normalization:
- Emails lowercased + trimmed.
- Phone normalization:
  - strip non-digits; attempt E.164 formatting if country known
  - keep raw input if normalization fails (but warn)
- Dates:
  - accept ISO (YYYY-MM-DD) and common US formats; warn on ambiguity

## Dedupe & upsert strategy (idempotent)
We dedupe per-tenant using a priority match:
1) Job number (exact match) for jobs/storage rows
2) Customer email (exact)
3) Customer primary phone (normalized)
4) Customer full name + origin zip + move date (fallback heuristic; warn)

Upsert behavior:
- Customers: upsert by (tenant_id, email) when present else (tenant_id, phone_primary) else create new.
- Jobs: upsert by (tenant_id, job_number) if provided.
- Estimates: upsert by (tenant_id, legacy_estimate_key) if provided; else by (tenant_id, estimate_number) if provided; else create.
- StorageRecord: upsert by (tenant_id, job_id) (unique).

## Dry-run report output format (MVP)
- Summary:
  - rows_total
  - rows_valid
  - rows_error
  - customers_create/update counts
  - estimates_create/update counts
  - jobs_create/update counts
  - storage_create/update counts
- Row-level results:
  - row_number
  - severity: error|warn|info
  - entity_type
  - idempotency_key (computed)
  - message
  - field (optional)
  - raw_value (optional)

Downloads:
- errors.csv (row_number, severity, entity_type, field, message, raw_value)
- report.json (full structured report)

## Incremental import (multiple runs)
Key requirement: re-importing the same file or overlapping files must not duplicate.

Mechanisms:
- import_file table:
  - tenant_id, id (uuid), filename, sha256, uploaded_by, created_at
  - unique (tenant_id, sha256)
- import_row table:
  - tenant_id, import_file_id, row_number
  - entity_type, idempotency_key
  - result_status (created/updated/skipped/error)
  - target_entity_id (uuid nullable)
  - unique (tenant_id, entity_type, idempotency_key)

Idempotency key examples:
- customer: sha256(lower(email)) OR sha256(phone_primary) OR sha256(name + phone + tenant)
- job: sha256(job_number)
- estimate: sha256(legacy_estimate_id) OR sha256(customer_key + move_date + origin_zip + dest_zip)
- storage: sha256(job_number + storage_site)

## Security & privacy (import)
- Admin-only permission required.
- Reject formulas/macros (Excel) if supported later.
- Never log full rows containing PII; log row_number + field + error code only.
- Rate-limit import endpoints.
- Audit log:
  - import started/completed
  - counts, file hash, actor

## Export (trust pitch)
Provide CSV exports from our system:
- customers.csv
- estimates.csv
- jobs.csv
- storage_records.csv

Exports must be tenant-scoped and permission-guarded.
