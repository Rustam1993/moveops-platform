# New Estimate Phase 6 Measurement Plan

## Goal
Prove that UX improvements in the estimate workspace reduce time-to-quote, reduce avoidable errors, and improve conversion through sign/book outcomes.

## Success metrics

### 1) Time-to-quote
Definition: `entry_started_at -> email_sent(template=moving_estimate)`.
Target direction: down.
Reporting grain: tenant-level median and P90.

### 2) Validation error rate
Definition: count of validation failures per estimate session.
Target direction: down.
Reporting grain: per form section and per field.

### 3) Inventory completion rate
Definition: share links sent -> inventory updated via public link within token validity window.
Target direction: up.
Reporting grain: tenant-level and campaign window.

### 4) Quote-to-sign conversion
Definition: moving estimate email sent -> signature completed.
Target direction: up.
Reporting grain: tenant-level funnel stage conversion.

### 5) Booking conversion
Definition: signature completed -> workflow status transitions to booked.
Target direction: up.
Reporting grain: tenant-level funnel conversion and time delta.

## Event instrumentation plan

Event payload rules:
- No PII in analytics payloads (no raw names/emails/phones/addresses).
- Include identifiers safe for analytics joins:
  - `tenant_id`
  - `estimate_id`
  - `workspace_tab`
  - `request_id` when emitted from server context

### Frontend events (workspace interaction)

| Event name | When emitted | Required properties |
|---|---|---|
| `estimate_workspace.opened` | user opens `/estimates/:id/*` | `tenant_id`, `estimate_id`, `tab` |
| `estimate_readiness.viewed` | readiness panel rendered | `tenant_id`, `estimate_id`, `complete_count`, `remaining_count` |
| `estimate_readiness.link_clicked` | user clicks checklist deep link | `tenant_id`, `estimate_id`, `target_tab`, `item_key` |
| `inventory.quick_add_used` | user clicks box quick-add preset | `tenant_id`, `estimate_id`, `preset_id`, `items_added_count` |
| `inventory.search_shortcut_used` | `/` shortcut focuses search | `tenant_id`, `estimate_id` |
| `charges.guardrail_shown` | LD zero-CF warning visible | `tenant_id`, `estimate_id`, `mode` |
| `email.preview_viewed` | email preview visible | `tenant_id`, `estimate_id`, `template_key` |
| `email.quick_action_clicked` | quick CTA button used | `tenant_id`, `estimate_id`, `template_key` |

### Server events (source-of-truth outcomes)

| Event name | Where emitted | Required properties |
|---|---|---|
| `estimate.entry_saved` | estimate create/update handlers | `tenant_id`, `estimate_id`, `mode` |
| `estimate.inventory_saved` | inventory put/public put handlers | `tenant_id`, `estimate_id`, `via` (`internal`/`public_link`), `total_cf` |
| `estimate.charges_saved` | charges put handler | `tenant_id`, `estimate_id`, `mode`, `total_cents` |
| `estimate.email_sent` | email send handler success | `tenant_id`, `estimate_id`, `template_key`, `delivery_mode` |
| `estimate.email_failed` | email send handler failure | `tenant_id`, `estimate_id`, `template_key`, `failure_code` |
| `estimate.signature_completed` | public sign completion handler | `tenant_id`, `estimate_id`, `signature_type` |
| `estimate.workflow_booked` | booking endpoint | `tenant_id`, `estimate_id`, `from_status`, `to_status` |

## Where to emit events

Frontend emission points:
- `estimate-workspace-shell.tsx` for workspace/tab open
- `estimate-readiness-checklist.tsx` for readiness view and link clicks
- `estimate-inventory-editor.tsx` for quick-add and search shortcut usage
- `estimate-charges-editor.tsx` for guardrail visibility
- `estimate-email-center.tsx` for preview/quick-action interactions

Server emission points:
- Existing estimate/inventory/charges/email/sign/workflow handlers
- Reuse current audit/event logger wiring where possible

## Dev vs prod behavior

### Development
- Log events as structured JSON to server logs for quick inspection.
- Optionally mirror frontend events to console in non-production mode.

### Production
- Write outcome events server-side (authoritative funnel events).
- Sample high-volume UI telemetry (e.g., `estimate_workspace.opened`) at 20-30%.
- Do not sample low-volume conversion events (`email_sent`, `signature_completed`, `workflow_booked`).

## Validation plan

1. Add dashboard slices for each metric at tenant and global views.
2. Run 2-week baseline before broad rollout where possible.
3. Compare pre/post Phase 6 medians for:
- time-to-quote
- validation errors per estimate
- inventory link completion
- quote-to-sign and sign-to-book conversions

## Notes
- Keep analytics separate from audit trails; audit remains compliance-focused while analytics is product-improvement focused.
- Ensure retention policy and access control are tenant-safe and consistent with existing privacy controls.
