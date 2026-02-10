# Data Model (Conceptual + MVP Columns)

All tables include:
- tenant_id (UUID) NOT NULL
- created_at, updated_at (timestamptz)
- created_by (user_id) for write events where applicable

## Core entities

### tenant
- id (UUID PK)
- name
- slug (unique)

### user
- id (UUID PK)
- tenant_id (FK)
- email (unique per tenant)
- password_hash
- name
- is_active

### role
- id (UUID PK)
- tenant_id (FK)
- name (e.g., Admin, Ops)
- description

### user_role (many-to-many)
- tenant_id
- user_id
- role_id
Unique: (tenant_id, user_id, role_id)

### permission (static list in code or DB)
- key (string; e.g., estimates.write)
- description

### role_permission
- tenant_id
- role_id
- permission_key
Unique: (tenant_id, role_id, permission_key)

---

## Business entities

### customer
- id (UUID PK)
- tenant_id
- full_name
- email (normalized lowercase)
- phone_primary (E.164 normalized where possible)
- phone_secondary, phone_mobile (optional)
- notes (optional)

Indexes:
- (tenant_id, lower(email))
- (tenant_id, phone_primary)
- trigram index for name search (optional later)

### estimate
- id (UUID PK)
- tenant_id
- estimate_number (string; unique per tenant; generated)
- customer_id (FK)
- status (Draft/Converted/Archived)

Origin:
- origin_address_line1, origin_address_line2
- origin_city, origin_state, origin_postal_code
- origin_access_location_type, origin_access_level, origin_access_floor, origin_access_apt

Destination:
- dest_address_line1, dest_address_line2
- dest_city, dest_state, dest_postal_code
- dest_access_location_type, dest_access_level, dest_access_floor, dest_access_apt

Move metadata:
- requested_pickup_date (date)
- requested_pickup_time (time or string window)
- location_type (enum)
- lead_source (string/enum)
- move_size (string/enum)

Pricing (MVP-minimal):
- estimated_total_cents
- deposit_cents
- pricing_notes

Idempotency:
- idempotency_key (string, nullable) unique per tenant when present

### job
- id (UUID PK)
- tenant_id
- job_number (string; unique per tenant; generated)
- estimate_id (FK unique per tenant; one job per estimate for MVP)
- customer_id (FK)
- status (Booked/Confirmed/Completed/Cancelled; minimal)
- pickup_date (date)
- pickup_time (time/window)
- job_type (Local/LongDistance/Other)

Balances (optional for MVP; can be stubbed):
- move_balance_cents

Idempotency:
- convert_idempotency_key (string, nullable) unique per tenant when present

### storage_record (0/1 per job for MVP)
- id (UUID PK)
- tenant_id
- job_id (FK unique per tenant)
- storage_site (string; “ARIZONA”, etc)
- status (InStorage/SIT/Out)
- date_in (date)
- date_out (date, nullable)
- next_bill_date (date, nullable)

Location:
- lot_number (string/int, nullable)
- location_code (string, nullable)

Counts:
- vaults (int)
- pads (int)
- xl_items (int)
- items (int, optional)
- volume_cuft (int, optional)

Financial:
- monthly_rate_cents
- storage_balance_cents

Notes:
- notes (text)

### audit_log
- id (UUID PK)
- tenant_id
- actor_user_id (FK)
- entity_type (string: customer/estimate/job/storage_record)
- entity_id (UUID)
- action (create/update/delete/status_change/import)
- diff_json (jsonb; redacted; no secrets)
- ip (inet, optional)
- user_agent (text, optional)
- request_id (string)

Indexes:
- (tenant_id, entity_type, entity_id, created_at desc)
- (tenant_id, actor_user_id, created_at desc)

---

## Multi-tenant guarantees (must enforce)
- Every query scopes by tenant_id.
- Unique constraints are per-tenant (composite unique keys).
- Cross-tenant access attempts must fail with 404 or 403 (policy-defined), and must be covered by tests.
