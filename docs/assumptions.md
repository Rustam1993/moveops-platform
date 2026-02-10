# Assumptions

## Phase 0
- MVP supports one tenant per customer company; tenant_id scopes all data.
- One job is created from one estimate in MVP (1:1).
- Storage record is optional and at most one per job in MVP (0/1).
- CSV is the primary migration format; Excel import is secondary and can be deferred if riskier.
- Pricing is minimal manual entry for MVP; calculator/tariff engine is post-MVP.
- Calendar is monthly view only for MVP.
