# New Estimate Phase 2

## Scope
Phase 2 delivers estimate-scoped Inventory for internal users and customer self-serve inventory via tokenized public links.

Implemented:
- Internal Inventory tab UX/UI and persistence
- Inventory data model + API endpoints
- Server-side Total CF derivation and estimate exposure
- Public inventory page (token-based, no login)
- "Send Inventory Link" action with email delivery mode support

Not implemented in Phase 2:
- Charges/pricing workflows
- Printed estimate/e-sign/payments/operations feature logic
- Full "Items not Moving" workflow (tab remains scaffold)

## Routes
Internal (authenticated workspace):
- `/estimates/:id/inventory`

Public (no login):
- `/public/inventory/[token]`

## UX behavior

### Internal inventory page
- Uses estimate workspace shell and estimate-scoped tabs.
- Three-column layout:
  - Categories list (includes screenshot-familiar set, including `Boxes`)
  - Data-dense item grid with columns: Item, Volume (cf), Qty, Controls
  - Right tools panel with search, custom item form, and total volume card
- Qty controls:
  - `-1`, `+1`, `+10`, and `Remove`
- Supports custom items with:
  - item name
  - category
  - volume_cf
  - qty
- Persistence:
  - debounced autosave for edits
  - explicit `Save` button
  - visible save state (`Unsaved changes`, `Saving...`, `Saved`, `Save failed`)
- "Send Inventory Link" action:
  - creates share token
  - attempts email send
  - falls back to log mode when SMTP is not configured/available

### Public inventory page
- Token-only access (no session/cookie auth required for authorization).
- Simplified version of inventory editing:
  - categories
  - item qty controls
  - custom item add
  - total volume display
  - explicit submit button
- Handles invalid/expired tokens with clear error states.

## Inventory model and derived totals

### Tables
- `estimate_inventory_item`
  - `tenant_id`, `estimate_id`, `category`, `item_name`, `volume_cf`, `qty`, `is_custom`
- `estimate_inventory_share_link`
  - `tenant_id`, `estimate_id`, `token_hash`, `recipient_email`, `expires_at`, delivery metadata, access timestamps, revocation

### Estimate column
- `estimates.total_volume_cf DOUBLE PRECISION NOT NULL DEFAULT 0`

### Derivation
- `total_volume_cf = sum(volume_cf * qty)` for inventory items with `qty > 0`
- Recomputed transactionally on internal and public inventory updates.
- Exposed on `GET /estimates/{estimateId}` as `estimate.totalVolumeCf`.

## API endpoints used

Internal (RBAC + tenant scoped):
- `GET /estimates/{estimateId}/inventory`
- `PUT /estimates/{estimateId}/inventory`
- `POST /estimates/{estimateId}/inventory-share-links`

Public (token scoped):
- `GET /public/inventory/{token}`
- `PUT /public/inventory/{token}`

## Token security
- Share token generation uses high-entropy random tokens.
- Only `token_hash` (SHA-256 based app helper) is stored in DB.
- Share link row binds token to `tenant_id` and `estimate_id`.
- `expires_at` enforced on every public request.
- Public endpoints are rate-limited.
- Invalid token returns `inventory_share_not_found`.
- Expired token returns `inventory_share_expired`.

## Authorization, tenancy, audit
- Internal inventory endpoints enforce RBAC (`estimates.read`/`estimates.write`).
- All internal estimate/inventory queries are tenant-scoped.
- Audit events:
  - `inventory.updated` (internal)
  - `inventory.updated` with `{via:"public_link"}` (public)
  - `inventory.share_link.created`

## Email behavior
- Config-driven email mode:
  - `EMAIL_MODE=log` (default): link is logged server-side for dev
  - `EMAIL_MODE=smtp`: sends via SMTP settings
- If SMTP mode is configured but unavailable/fails, the handler falls back to log mode and stores delivery error metadata.

## Tests
Backend integration tests added:
- internal inventory tenant isolation (`GET`/`PUT` cross-tenant denied)
- public token flow (`valid read/update`, `invalid rejected`, `expired rejected`)

Frontend smoke test updated:
- create estimate
- open inventory tab
- add inventory item
- save and verify total volume persists after reload
