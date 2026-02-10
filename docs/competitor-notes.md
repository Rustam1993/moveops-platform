# Competitor Notes (Public-Facing Only)

This document summarizes publicly available competitor positioning to understand expected workflows and migration expectations.
Do not copy proprietary code or replicate competitor UI text verbatim; use original UI wording and implementation.

## Sources reviewed (public web pages)
- Granot “OMS Legacy”
- Granot “Software Pricing”
- Granot “Terms of Service”
- Granot “Privacy Policy”

## High-level modules mentioned publicly

From the OMS Legacy and Pricing pages, the competitor positions a “sales → ops → storage” workflow and highlights (names paraphrased):
- Lead intake/lead management
- Estimate management (inventory-based pricing, accessorials, supplies/materials, liability options)
- Operations/dispatch management (bill of lading, operations updates)
- Operations calendar / scheduling
- Storage management (billing cycles, invoices, automation)
- Claims management
- Payroll/salary
- Affiliate/subcontractor management
- Missions/trips scheduling
- Truck/fleet management
- Reporting, checklists, document storage, payments

### MVP overlap (what we implement now)
Our MVP scope maps to these areas:
- New Estimate (draft estimate creation + minimal pricing fields)
- Calendar (monthly operations calendar + filters)
- Storage (storage record list + editable storage fields)
- Migration import/export (CSV first; Excel fallback; idempotent)

### Not in MVP (defer)
- Lead marketplace / lead routing
- Inventory UI (room-by-room) and tariff engine
- eSign workflows and document templates
- Full dispatch/crew staffing and payroll
- Claims/damages, affiliates/subcontractor balances
- Missions/trip scheduling
- Payments processing and automated storage charging
- Full reporting suite and checklists

## Data ownership / export / backup expectations (high-level)

Public Terms/Privacy materials mention optional integrations that support:
- Exporting reports/grids to spreadsheets (Google Sheets) for analytics/backup
- Creating storage folders in Google Drive for backups/reports
- Creating dedicated calendars/events in Google Calendar for redundancy and mobile access

Implication for us (product & sales):
- Migration tooling and exports should be first-class for trust (import from legacy exports; export out of our system)
- Keep cloud portability; avoid requiring a specific vendor integration for core backup/export

## What we should match later (post-MVP)
- Inventory-driven estimate calculator with configurable tariffs/accessorials
- Storage billing automation (cycles, invoices, auto-charging)
- Document templates (estimate PDFs, bill of lading) + eSign
- Broader calendar views and dispatch tooling (crew/trucks)
- Role-based department/user filtering across ops
- Deeper reporting and audit/traceability features
