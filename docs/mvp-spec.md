# MVP Spec (Phase 0)

Repo: moveops-platform  
Scope: Admin Dashboard + New Estimate + Calendar + Storage + Migration Import/Export

## Product principles
- Multi-tenant by default (tenant_id everywhere)
- Defensive-by-default security posture
- Original UI wording; do not replicate competitor copy
- CSV import/export is a sales trust feature

---

## Screen 1: Admin Dashboard
### Must-have
- Navigation: New Estimate, Calendar, Storage, Import/Export (admin only)
- Quick search:
  - job number
  - estimate number
  - customer name
  - phone
  - city
- Search results show both estimates and jobs with type badges.

### Nice-to-have (can stub)
- Recent items (recently opened jobs/estimates)

---

## Screen 2: New Estimate
### Layout
Two-column address/contact: “Origin” and “Destination”.

### Required fields (from legacy semantics)
- Customer name
- Primary phone
- Email
- Origin zip
- Destination zip
- Requested pickup date
- Lead source

### Optional fields
- Addresses, city/state
- Apt/floor/access details
- Additional phones
- Location type enum (Residence/Commercial/Other)
- Move size enum (Studio/1BR/2BR/3BR/Other)

### Actions
- Save draft
- Convert to Job (idempotent)
- Minimal pricing section:
  - Estimated total
  - Deposit
  - Notes

---

## Screen 3: Calendar (Monthly Ops View)
### Must-have
- Month grid with next/prev navigation
- Filters:
  - Status/phase (default “Booked”)
  - Month/year
  - Job type (Local/Long Distance/Other; minimal)
  - Department/user (visible but can stub if no data yet)
- Day cell shows job cards (compact)
- Click job → details drawer:
  - Update status
  - Update pickup time
  - Link to storage record / indicator

### Badge rules (MVP)
- STORAGE badge when job has storage_record OR storage_flag true
- BALANCE DUE badge when job_balance > 0 OR storage_balance > 0 (exact rule can be adjusted)

---

## Screen 4: Storage
### Must-have
- Storage site selector
- Search box (job # or customer name)
- Working filters:
  - In Storage
  - SIT
  - Delivery scheduled (date_out set)
  - Balance due (storage_balance > 0)
  - Has containers (vaults/pads/items > 0)
- Table with columns close to legacy semantics
- Click row → storage drawer:
  - Edit: dates in/out, next bill date, lot/location, counts, volume, monthly rate, balances, notes

### Not in MVP (visible placeholders only)
- Invoice cycle runner
- Previous invoice cycles
- Follow-up automation

---

## Migration import/export (Admin-only)
### Import
- Upload CSV (Excel supported only if we can parse safely; otherwise require CSV export)
- Map columns to canonical fields (minimal UI-assisted mapping)
- Dry-run output:
  - Created/updated counts
  - Warnings + errors (downloadable)
- Import execution:
  - Upsert customers
  - Upsert estimates
  - Optionally create jobs + storage records if present
- Idempotent re-import: same file should not duplicate

### Export
- CSV export endpoints for:
  - Customers
  - Estimates
  - Jobs
  - Storage records
