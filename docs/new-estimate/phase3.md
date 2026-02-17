# New Estimate Phase 3

## Scope
Phase 3 delivers estimate-scoped Charges/Pricing for Local and Long Distance moves, with deterministic calculations and estimate summary persistence.

Implemented:
- Charges tab UI at `/estimates/:id/charges` in the estimate workspace shell
- Local/Long Distance pricing mode switch with live totals
- Server-side charges model + upsert/get endpoints
- Inventory-driven pricing input (`total_cf`) integration from Phase 2
- Server + client calculation alignment (client for live preview, server as source of truth)
- Sidebar updates for service type and total estimate

Not implemented in Phase 3:
- Printed estimate generation
- E-sign, email center workflows, payments processing
- Operations/trips workflow behavior

## Routes
- Internal workspace route: `/estimates/:id/charges`
- API:
  - `GET /estimates/{estimateId}/charges`
  - `PUT /estimates/{estimateId}/charges`

## Charges model

### Persistence table
- `estimate_charge` (tenant-scoped, one row per estimate)
  - identity: `tenant_id`, `estimate_id`, unique index
  - mode: `local | long_distance`
  - input fields:
    - long distance: `ld_rate_per_cf`, `ld_fixed_base_amount_cents`
    - local: `local_trucks`, `local_workers`, `local_labor_hours`, `local_labor_rate_cents`, `local_travel_hours`, `local_travel_rate_cents`
    - shared: `fuel_surcharge_pct`, `cf_lbs_ratio`, discounts, packing, liability, `other_line_items_json`, tax, deposit, paid
  - computed snapshot fields:
    - `computed_base_cents`, `computed_fuel_surcharge_cents`, `computed_other_items_cents`, `computed_packing_cents`, `computed_liability_cents`
    - `computed_subtotal_cents`, `computed_discounts_cents`, `computed_tax_cents`, `computed_total_cents`
    - `computed_total_cf`, `computed_total_lbs`
  - `calculation_version` (currently `v1`)

### Estimate summary sync
On every charges upsert, estimate summary is updated transactionally:
- `estimates.estimated_total_cents = computed_total_cents`
- `estimates.deposit_cents = deposit_required_cents`
- `estimates.location_type` set from mode (`Local` / `Long Distance`)

This exposes the final estimate total through existing `GET /estimates/{estimateId}` payloads.

## Calculation rules (v1)
Server-side is authoritative; client mirrors rules for live preview.

Inputs:
- `total_cf` from estimate inventory aggregation (Phase 2)
- mode-specific fields (local or long-distance)
- shared adjustments (fuel, discounts, other line items, packing, liability, tax)

Rules:
- Long Distance base:
  - if fixed base amount is provided: use it
  - else `base = total_cf * rate_per_cf`
- Local base:
  - `base = labor_hours * labor_rate * workers_multiplier + travel_hours * travel_rate`
  - `workers_multiplier = workers` when `workers > 0`, else `1`
- Fuel surcharge:
  - applied to base (`fuel = base * fuel_surcharge_pct`)
- Subtotal:
  - `subtotal = base + fuel + other_items + packing + liability`
- Discounts:
  - coupon % and senior % applied to non-negative subtotal
  - fixed coupon/senior amounts added
  - capped to non-negative subtotal
- Tax:
  - `tax = (subtotal - discounts) * tax_rate_pct` on non-negative taxable amount
- Total:
  - `total = max(0, subtotal - discounts + tax)`
- Derived weight:
  - `total_lbs = total_cf * cf_lbs_ratio`

## UX behavior
- Charges editor is grouped into:
  - mode + inventory totals
  - core mode-specific pricing
  - advanced adjustments (line items, discounts, packing, liability, tax/deposit)
  - totals summary
- Save model:
  - debounced autosave for valid edits
  - explicit Save button
  - visible save states (`Saving...`, `Saved`, `Save failed`)
- Validation:
  - zod schema-based
  - inline field errors on blur and on manual save

## Security and tenancy
- Internal endpoints remain authenticated and RBAC-protected:
  - read: `estimates.read`
  - write: `estimates.write`
- All charges and estimate updates are tenant-scoped by `tenant_id`.
- Cross-tenant access returns not found behavior for tenant isolation.

## Audit events
- `charges.updated` on each successful write
- `charges.mode_switched` when mode changes between persisted writes

## Tests
Backend integration tests added:
- tenant isolation for charges `GET`/`PUT`
- deterministic calculation coverage for:
  - one long-distance scenario
  - one local scenario
  - estimate summary total propagation

Frontend smoke test updated:
- create estimate
- open charges
- edit local pricing inputs
- save and verify total persists after reload
