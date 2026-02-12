# Import Format (Phase 5)

## Supported file type
- `.csv` only.
- `.xlsx` is rejected with `XLSX_NOT_SUPPORTED` in this phase.

## Canonical combined template
Use `GET /imports/templates/combined.csv` or `docs/examples/sample-import.csv`.

Recommended columns for combined import:
- `job_number`
- `estimate_number`
- `customer_name`
- `email`
- `phone_primary`
- `phone_secondary`
- `origin_zip`
- `destination_zip`
- `origin_city`
- `destination_city`
- `origin_state`
- `destination_state`
- `requested_pickup_date`
- `requested_pickup_time`
- `scheduled_date`
- `pickup_time`
- `status`
- `job_type`
- `lead_source`
- `estimated_total`
- `deposit`
- `pricing_notes`
- `facility`
- `storage_status`
- `date_in`
- `date_out`
- `next_bill_date`
- `lot_number`
- `location_label`
- `vaults`
- `pads`
- `items`
- `oversize_items`
- `volume`
- `monthly_rate`
- `storage_balance`
- `move_balance`

## Required semantics
- Customer identity:
  - `customer_name` or (`email` / `phone_primary`) must be present.
- Estimate creation/update:
  - if estimate fields are provided, require `origin_zip`, `destination_zip`, `requested_pickup_date`.
- Job idempotency:
  - `job_number` is strongly recommended.
  - if missing, importer generates deterministic job number and records a warning.

## Value formats
- Dates: `YYYY-MM-DD` preferred.
- Times: `HH:MM` preferred.
- Money fields: either integer cents (`32900`) or decimal currency (`329.00`) are accepted.
- Numeric count/volume fields are clamped to `>= 0`.

## Mapping payload
`options.mapping` is canonical field -> source column name (or index).

Example:
```json
{
  "source": "generic",
  "hasHeader": true,
  "mapping": {
    "job_number": "Job Number",
    "customer_name": "Customer",
    "email": "Email",
    "origin_zip": "From Zip",
    "destination_zip": "To Zip",
    "requested_pickup_date": "Move Date",
    "facility": "Facility"
  }
}
```
