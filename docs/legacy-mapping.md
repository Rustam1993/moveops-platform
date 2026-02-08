# Legacy Mapping (Screenshots → MVP)

Goal: map legacy screenshots to MVP screens and define field-by-field parity without copying competitor UI wording.
We capture legacy labels for migration/field parity, but MVP UI should use original wording.

---

## 1) Admin Menu → MVP Admin Dashboard

### Legacy navigation (observed)
- Quick Search
- New Estimate
- Estimates In Process
- Assign New Estimates
- Advanced Search
- Recurring Customers
- Operations Calendar
- Maintenance
- Leads Marketplace
- Operation
- Reports
- Storage
- Tasks to Complete
- Follow-Up Menu
- Recent Accessed Jobs
- Handbook & Workflow
- Newsletter

### MVP navigation (Phase MVP only)
- Dashboard (Admin Home)
- New Estimate
- Calendar
- Storage
- Import/Export (Admin only)

### MVP Dashboard requirements
- Quick search across: job number, estimate number, customer name, phone, city
- Recent items: recently viewed jobs/estimates (optional for MVP; can stub)

---

## 2) New Estimate Entry Form → MVP New Estimate

### Legacy layout
Two columns: “Moving From” and “Moving To”, plus move meta fields.

### Field-by-field mapping

#### A) Customer & contact (origin side)
Legacy label → MVP field → Entity column
- Customer* → Customer name → customer.full_name (required)
- Address → Origin street address → estimate.origin_address_line1 (+ line2 optional)
- City → Origin city → estimate.origin_city
- State → Origin state → estimate.origin_state
- Zip Code* → Origin zip → estimate.origin_postal_code (required)
- Phone 1* → Primary phone → customer.phone_primary (required)
- Phone 2 → Secondary phone → customer.phone_secondary (optional)
- Email* → Email → customer.email (required)
- Proxy → Proxy/alternate contact → estimate.origin_proxy_name (optional)
- Location / Level / Floor / APT # → Origin access details → estimate.origin_access_* (optional structured fields)

#### B) Destination side
Legacy label → MVP field → Entity column
- Customer → Destination contact name (optional) → estimate.destination_contact_name (optional)
- Address → Destination street address → estimate.dest_address_line1 (+ line2 optional)
- City → Destination city → estimate.dest_city
- State → Destination state → estimate.dest_state
- Zip Code* → Destination zip → estimate.dest_postal_code (required)
- Phone 3 / Mobile / Fax → Alternate phones → customer.phone_alt_* (optional)
- Proxy → Destination proxy → estimate.dest_proxy_name (optional)
- Location / Level / Floor / APT # → Destination access details → estimate.dest_access_* (optional)

#### C) Move details (below columns)
Legacy label → MVP field → Entity column
- Expected Move Date* → Planned pickup date → estimate.requested_pickup_date (required)
- Time → Planned pickup time window → estimate.requested_pickup_time (optional)
- Location Type (Residential) → Move context → estimate.location_type (enum; default “Residence”)
- How did you hear about us?* → Lead source → estimate.lead_source (required)
- Move Size (dropdown) → Estimated move size → estimate.move_size (optional)

### MVP actions
- Save Draft (creates/updates estimate)
- Convert to Job (creates job from estimate; idempotent)
- Schedule pickup date/time (job fields; minimal for MVP)

Note: legacy shows “Continue to Inventory / Continue to Charges”; MVP can keep a 2-step flow (Details → Pricing) but must use original wording.

---

## 3) Storage Process (filters) → MVP Storage

### Legacy “Storage Process” options observed
- Storage selector (dropdown)
- Search job in storage (text)
- Sort by (Job No)
- Filters/radios:
  - Jobs in Storage
  - Storage jobs in SIT
  - Storage jobs with delivery date out
  - Storage jobs past 30 days without payment
  - Jobs not assigned to storage (30 days past move date)
  - Jobs in storage with balance due
  - Customers need storage
  - Storage jobs with crates/vaults
  - Follow-up on storage customers
  - Run monthly invoices cycle
  - View previous invoices cycles

### MVP implementation plan
Working filters for MVP (the rest visible but stubbed):
- Storage site selector (required)
- Search by job number / customer name
- Status filter:
  - In Storage
  - SIT
  - Delivery scheduled (has date_out)
  - Past due (optional heuristic: last_payment_at older than N days)
  - Balance due (storage_balance > 0)
  - Has containers (vaults > 0 OR crates > 0)

Not in MVP:
- Invoice cycle runner + historical invoice cycles (show UI placeholder)

---

## 4) Jobs in Storage (table) → MVP Storage List

### Legacy columns observed
- No
- Job No (link)
- Type
- Customer
- From/To (combined origin/destination text)
- Date In
- Date Out
- Bill Date
- Sit
- Lot No
- Location
- Vaults
- Pads
- XL Items
- Volume
- Monthly
- Stg Bal
- Move Bal

### MVP table columns (original wording)
- Job #
- Move type
- Customer
- Origin → Destination (short)
- Storage in date
- Storage out date
- Next bill date
- Lot / Location
- Containers (vaults/pads/xl/items)
- Volume
- Monthly rate
- Storage balance
- Move balance (optional; can stub if not in MVP)

Row click opens Job/Storage drawer:
- Edit storage fields (site, dates, billing, container counts, notes)

---

## 5) Operations Calendar → MVP Calendar

### Legacy controls observed
- Month grid
- Quick navigation (today/this week/previous/next month)
- Filters:
  - Job phase (e.g., “Booked Jobs”)
  - Month/Year
  - Department (optional)
  - User (optional)
  - Job type
  - Submit button
- Special action link: “Block Pick-Up Days / Limit Booking” (not MVP)

### Legacy day-cell “job card” elements observed
Per job entry:
- Job number (often with prefix)
- Pickup label (“Pick Up”)
- Time
- Customer name
- Origin city/state
- Destination city/state
- Optional metrics (miles, volume/cf)
- Flags/badges:
  - Storage indicator (e.g., “STORAGE”)
  - Money indicator (balance due shown as $ signs)
  - “2nd Pick-Up” date line (optional)

### MVP calendar card content
- Job #
- Pickup date/time
- Customer name
- Origin short (City, ST) → Destination short (City, ST)
- Badges:
  - STORAGE (if storage_record exists or storage status true)
  - BALANCE DUE (if storage_balance > 0 or job_balance > 0; rules TBD)
Click opens Job detail drawer:
- Update status (Booked/Confirmed/etc; minimal enum)
- Update pickup time
- Toggle storage flag / open storage fields
