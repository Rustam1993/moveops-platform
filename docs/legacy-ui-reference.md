# Legacy UI Screenshot Reference (Sanitized)

Purpose: capture legacy workflow/layout patterns from screenshots for parity planning.
Constraint: use this for feature behavior and information architecture only, not for visual cloning or verbatim UI text.

## Privacy and handling
- Treat all screenshots as sensitive and non-public.
- Remove or blur personally identifiable information (PII) before reusing images in tickets, specs, demos, or PRs.
- This reference stores feature/layout observations only.

## Estimate-scoped workspace pattern
Observed tab structure tied to one estimate/job:
- Handbook
- Entry Form
- Inventory
- Items not Moving
- Printed Estimate
- Charges
- Tasks List
- Payments
- Operations

Implication for MoveOps:
- Keep estimate/job as a single workspace context.
- Prefer module tabs or segmented navigation inside estimate detail.
- Preserve canonical route ownership in our app (`/estimates/:id`, `/jobs/:id`) and avoid copycat wording.

## Screenshot legend (feature/layout only)

### `granot-new-estimate.jpg`
- New Estimate entry form with side-by-side `Moving From` and `Moving To`.
- Customer/contact/address fields plus move date/time and lead metadata.
- Actions indicate a flow from entry to inventory and charges.

### `2487b887-f43b-48ea-ab2b-d1de2e7ac1a7.png`
- Estimate-scoped tab bar showing the full lifecycle modules listed above.

### `8ee79cb3-2c6b-4a39-ba2f-acded0c62bba.png`
- Left category rail (Bedroom, Living Room, etc.).
- Center item table with volume and quantity controls (`-1`, `+1`, `+10` style interactions).
- Right utility panel with search, add-new-item controls, and total volume.

### `c665e955-750b-43de-a963-d4de173d4a3b.png`
- Long-distance charges page.
- Main pricing form (base price, surcharges, discounts, liability, packing materials, totals).
- Persistent right sidebar with job status/priority and booking actions.

### `348d81aa-41a1-43fc-a7e5-4fca3cb99382.png`
- Alternate long-distance state after booking.
- Sidebar switches to booked-state actions (e.g., release action).
- Primary form remains in-place; state changes are concentrated in sidebar controls.

### `56832cc9-511c-4a31-a9bf-ed5d4025649a.png`
- Close-up confirms detailed pricing rows and subtotal/total stacking pattern for charge breakdown.

### `990d46dd-122f-468b-b74f-f4e18db5199e.png`
- Local charges variant.
- Labor/travel inputs (`# vans`, `# workers`, `hours`, `rate`) while reusing the same sidebar pattern.

### `1b81754a-23d7-4491-bda0-129e5408353c.png`
- Priority picker modal with fixed options `Priority 1..8` and `General Priority Pool`.

### `c7046374-0f8f-4676-b356-2fa434c5d122.png`
- Email Center pattern.
- Template list with row-level options for inventory link, e-sign, and e-quote behavior.
- Emphasizes estimate communication as a first-class workflow.

### `d2e37a83-b6e1-422d-8926-18a0ca95c9a4.jpg`
- Estimate-type chooser.
- Department selection plus move type branches (Local, Long Distance, International, Auto Transport).

## Scope guidance (MoveOps)

### MVP (current)
- New Estimate entry form (origin/destination + required move metadata).
- Minimal pricing fields only.
- Calendar and Storage modules as already specified.

### Post-MVP candidates (from screenshots)
- Full inventory workspace with category-driven item catalog and quantity operators.
- Full local/long-distance tariff engine and charge breakdown rows.
- Estimate/job right sidebar state machine (follow-up, booked, hold, release, scheduling fields).
- Priority pool/level modal integrated with dispatch rules.
- Email Center templates with e-sign/e-quote workflows.
- Items-not-moving, payments, and operations submodules inside estimate/job workspace.
