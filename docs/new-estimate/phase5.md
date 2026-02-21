# New Estimate Phase 5

## Scope
Phase 5 completes estimate workflow controls and operational tabs inside the estimate workspace:
- Right sidebar workflow actions (status, priority, follow-up, VIP, book/release/hold)
- Tasks List tab CRUD
- Payments tab manual tracking (no processor)

Out of scope for this phase:
- Dispatch/operations calendar workflows
- Card processing/refunds

## Workflow states and transitions
Workflow status enum:
- `draft`
- `open`
- `follow_up`
- `quoted`
- `booked`
- `on_hold`
- `canceled`

Actions:
- `Book This Job` -> sets status to `booked` and `booked_at`
- `Release Book` -> sets status to `open` and clears `booked_at`
- `Move On Hold` -> sets status to `on_hold` with optional `hold_reason`

Priority model:
- `priority_level` supports `0..8`
- `0` means General Priority Pool
- `1..8` maps to Priority 1-8

## Endpoints
Internal workflow endpoints:
- `GET /estimates/{estimateId}/workflow`
- `PATCH /estimates/{estimateId}/workflow`
- `POST /estimates/{estimateId}/book`
- `POST /estimates/{estimateId}/release-book`
- `POST /estimates/{estimateId}/hold`

Tasks endpoints:
- `GET /estimates/{estimateId}/tasks`
- `POST /estimates/{estimateId}/tasks`
- `PATCH /estimates/{estimateId}/tasks/{taskId}`
- `DELETE /estimates/{estimateId}/tasks/{taskId}`

Payments endpoints:
- `GET /estimates/{estimateId}/payments`
- `POST /estimates/{estimateId}/payments`
- `DELETE /estimates/{estimateId}/payments/{paymentId}`

## Data model
Migration:
- `apps/api/migrations/00008_phase9_workflow_tasks_payments.sql`

Tables:
- `estimate_workflow`
  - `tenant_id`, `estimate_id` unique
  - `status`, `priority_level`, `follow_up_at`, `follow_up_note`, `vip`
  - `booked_at`, `hold_reason`, audit actor fields/timestamps
- `estimate_task`
  - `tenant_id`, `estimate_id`, `title`, `is_done`, optional `due_at`
  - soft delete via `deleted_at`
- `estimate_payment`
  - `tenant_id`, `estimate_id`, `amount_cents`, `method`, `paid_at`, optional `notes`
  - soft delete via `deleted_at`

Payment summary behavior:
- `amount_paid_cents` is the sum of non-deleted payments
- `deposit_required_cents` and `total_estimate_cents` are read from estimate pricing summary
- `remaining_balance_cents` is derived from total estimate minus amount paid when total exists

## UI behavior
Estimate workspace sidebar:
- Real-time workflow controls replace phase scaffold
- Priority picker dialog with Priority 1-8 and General Priority Pool
- Follow-up datetime and note saved through workflow patch endpoint
- Book/release/hold actions call dedicated endpoints

Tasks tab:
- Create, edit, complete, delete tasks scoped to estimate
- Done tasks remain visible and sorted after active tasks

Payments tab:
- Manual add/delete payment records
- Summary cards for deposit required, amount paid, remaining balance

## Security and audit
- All new internal endpoints require auth + RBAC (`estimates.read`/`estimates.write`)
- All queries are tenant-scoped (`tenant_id`) for isolation

Audit events:
- Workflow: `workflow.updated`, `booking.booked`, `booking.released`, `booking.held`
- Tasks: `task.created`, `task.updated`, `task.completed`, `task.deleted`
- Payments: `payment.added`, `payment.deleted`

## Tests
Backend integration coverage includes:
- Tenant isolation for workflow/tasks/payments endpoints
- Booking transition persistence (`book` -> `release` -> `hold`)

Frontend smoke coverage includes:
- Task creation and completion persistence after page reload
