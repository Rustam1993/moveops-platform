# New Estimate Phase 1

## Scope
Phase 1 delivers an estimate-scoped workspace with Entry Form implemented end-to-end.  
Inventory, charges, tasks, payments, operations, and printed estimate tabs are scaffolded as placeholders.

## Routes
- `/estimates/new`
- `/estimates/:id/entry`
- `/estimates/:id/inventory`
- `/estimates/:id/items-not-moving`
- `/estimates/:id/printed-estimate`
- `/estimates/:id/charges`
- `/estimates/:id/tasks`
- `/estimates/:id/payments`
- `/estimates/:id/operations`

Compatibility redirect:
- `/estimates/:id` redirects to `/estimates/:id/entry`.

## Workspace behavior
- Estimate-scoped tab bar is rendered inside the estimate workspace shell.
- On `/estimates/new`, only `Entry Form` is enabled until first save creates an estimate id.
- Header scaffolds estimate identity, customer, status, priority control, and disabled comms actions.
- Right sidebar is scaffolded with status/priority/follow-up/VIP/booked placeholders plus reserved `Total CF` and `Total LBS`.

## Entry Form UX
- Modern grouped layout:
  - Customer
  - Moving From / Moving To side-by-side
  - Move Basics
  - Advanced section (referral source)
- Fields:
  - First name, last name, email, phone
  - Moving from and moving to address sets (street/city/state/zip)
  - Move date
  - Preferred time window (optional)
  - Service type (`Local`, `Long Distance`)
  - Internal notes
- Validation:
  - `zod` schema-based
  - field-level validation on blur
  - full-form validation on Save/Next
- Save behavior:
  - explicit `Save`
  - explicit `Next: Inventory`
  - autosave (debounced) when editing an existing estimate
  - visible save state text (`Saving...`, `Saved`, `Save failed`)

## API endpoints used
- `POST /estimates` for create
- `GET /estimates/{estimateId}` for workspace load
- `PATCH /estimates/{estimateId}` for update

No backend API schema changes were required in Phase 1.

## Field mapping notes
- Entry form `firstName` + `lastName` map to API `customerName`.
- Entry form `serviceType` maps to API `locationType` (`Local` / `Long Distance`) for Phase 1 compatibility.
- Referral source maps to API `leadSource` (required by current API contract).

## Testing
- Playwright smoke test updated to cover:
  - login
  - create estimate via `/estimates/new`
  - navigate to inventory placeholder tab
  - return to entry tab, edit, save, reload, verify persistence
