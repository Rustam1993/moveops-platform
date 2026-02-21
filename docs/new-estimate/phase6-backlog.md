# New Estimate Phase 6 Differentiation Backlog

Ranked backlog to exceed GRANOT parity while staying in current estimate workspace scope.

## P0 (highest priority)

### 1. [P0] Readiness checklist and send gate
Problem: Reps can move to quote/send without noticing missing inventory or pricing.
Proposed solution: Estimate readiness panel with section-level completeness checks and deep links.
Why it is better than GRANOT: Explicit completion guidance reduces hidden-state handoff errors.
Effort: S
Risk: Low
Dependencies: Entry, inventory, charges data already available in workspace context.
Success metric: Drop in quotes sent with missing inventory or zero total.

### 2. [P0] Inventory box quick-add presets
Problem: Common box-heavy moves require repetitive qty clicks.
Proposed solution: One-click preset bundles (starter pack, small/medium/large increments).
Why it is better than GRANOT: Same capability plus lower click count for common workflows.
Effort: S
Risk: Low
Dependencies: Inventory catalog item keys.
Success metric: Lower median inventory interactions per estimate.

### 3. [P0] Charges zero-CF guardrail for long distance
Problem: LD rate-per-CF entries can underquote when inventory total CF is zero.
Proposed solution: Inline warning when LD mode relies on CF but total CF is zero.
Why it is better than GRANOT: Prevents silent misquote conditions earlier.
Effort: S
Risk: Low
Dependencies: Inventory total_cf in estimate context.
Success metric: Reduction in LD quotes with zero CF and non-zero rate.

### 4. [P0] Sticky charges totals summary
Problem: Users scroll away from totals while editing dense charge sections.
Proposed solution: Sticky summary card showing subtotal/discounts/tax/total.
Why it is better than GRANOT: Constant feedback loop on pricing edits.
Effort: S
Risk: Low
Dependencies: Existing live calculation output.
Success metric: Fewer price rework edits after initial save.

### 5. [P0] Email template preview before send
Problem: Reps cannot quickly validate message content and link intent prior to sending.
Proposed solution: Subject/body preview with token placeholders and recipient visibility.
Why it is better than GRANOT: Reduces accidental wrong-template sends.
Effort: S
Risk: Low
Dependencies: Existing template keys.
Success metric: Lower email resend/correction rate.

### 6. [P0] Primary communication CTAs in Email Center
Problem: High-frequency actions are buried in template selection flow.
Proposed solution: Dedicated buttons for Send Quote, Send Inventory Link, Send Signature Request.
Why it is better than GRANOT: Faster path for top three actions.
Effort: S
Risk: Low
Dependencies: Existing send endpoint.
Success metric: Lower time from charges-save to first customer send.

### 7. [P0] Unified save-status pattern
Problem: Status feedback differs by tab; error recovery is inconsistent.
Proposed solution: Shared save indicator component with retry affordance.
Why it is better than GRANOT: Consistent trust signal and faster recovery on failures.
Effort: S
Risk: Low
Dependencies: Existing save states.
Success metric: Lower repeated manual-save attempts after transient errors.

### 8. [P0] One-click resend from email history
Problem: Re-sending requires manual re-selection and data re-entry.
Proposed solution: Resend action per prior email row.
Why it is better than GRANOT: Faster recovery for missed/expired links.
Effort: S
Risk: Low
Dependencies: Existing history payload.
Success metric: Reduced clicks to resend by >50%.

## P1 (next wave)

### 9. [P1] Entry form smart defaults by tenant pattern
Problem: Reps repeatedly choose the same lead source/service defaults.
Proposed solution: Persist recent defaults per user/tenant.
Why it is better than GRANOT: Faster first-pass data entry.
Effort: M
Risk: Medium
Dependencies: Preference storage endpoint.
Success metric: Reduced average Entry completion time.

### 10. [P1] Inventory inline bulk text parser
Problem: Bulk additions still require repeated controls for larger item sets.
Proposed solution: Paste-friendly parser (e.g., "10 medium boxes, 5 large boxes").
Why it is better than GRANOT: Faster than row-by-row interaction.
Effort: M
Risk: Medium
Dependencies: Parser and validation rules.
Success metric: Lower inventory completion time for >20-item estimates.

### 11. [P1] Keyboard quantity hotkeys in inventory table
Problem: Mouse-heavy interactions slow down expert users.
Proposed solution: Focused row hotkeys for +/- and +10 changes.
Why it is better than GRANOT: Better power-user throughput.
Effort: M
Risk: Medium
Dependencies: Row focus model.
Success metric: Higher items/minute during inventory editing sessions.

### 12. [P1] Entry field inline examples and format masks
Problem: Phone/ZIP/time formatting errors cause avoidable validation loops.
Proposed solution: Input hints/masks for high-error fields.
Why it is better than GRANOT: Fewer correction cycles.
Effort: M
Risk: Medium
Dependencies: Input utility components.
Success metric: Lower validation error events per estimate.

### 13. [P1] Charges helper microcopy and collapsible explanations
Problem: New reps misinterpret surcharge/discount/liability behavior.
Proposed solution: Contextual helper text and short explainers.
Why it is better than GRANOT: Better onboarding without training docs.
Effort: S
Risk: Low
Dependencies: UI copy review.
Success metric: Fewer support questions on charge semantics.

### 14. [P1] Cross-tab unsaved-change guard
Problem: Users can navigate away before pending autosave settles.
Proposed solution: Unsaved indicator + navigation guard while saving.
Why it is better than GRANOT: Prevents accidental data loss perception.
Effort: M
Risk: Medium
Dependencies: Shared save bus across tabs.
Success metric: Lower reports of “missing recent edits”.

### 15. [P1] Workflow quick presets (follow-up today/tomorrow)
Problem: Sidebar follow-up entry requires full manual datetime input.
Proposed solution: Shortcut chips for common follow-up windows.
Why it is better than GRANOT: Faster routine scheduling.
Effort: S
Risk: Low
Dependencies: Existing workflow patch endpoint.
Success metric: Higher follow-up field completion rate.

### 16. [P1] Payments method presets and inline validation
Problem: Manual method entry creates inconsistent values.
Proposed solution: Method select presets with optional custom override.
Why it is better than GRANOT: Cleaner reporting and less cleanup.
Effort: S
Risk: Low
Dependencies: Payments UI only.
Success metric: Reduction in unique free-text payment method variants.

### 17. [P1] Printed estimate freshness badge
Problem: Users may email stale PDFs after charges/inventory edits.
Proposed solution: “Out of date” badge if source data changed since generation.
Why it is better than GRANOT: Safer quote communication.
Effort: M
Risk: Medium
Dependencies: Document metadata with source snapshot timestamp.
Success metric: Lower stale-document sends.

### 18. [P1] Accessibility pass for keyboard order and ARIA labels
Problem: Dense forms still have non-ideal tab flow and assistive ambiguity.
Proposed solution: Tab order audit, role/label consistency, stronger focus states.
Why it is better than GRANOT: Better WCAG conformance and usability.
Effort: M
Risk: Low
Dependencies: QA pass across all estimate tabs.
Success metric: Improved accessibility audit score.

## P2 (later optimization)

### 19. [P2] Entry-to-inventory guided handoff toast
Problem: New users are unsure what to do immediately after saving entry form.
Proposed solution: Contextual next-step prompt with one-click navigation.
Why it is better than GRANOT: Improves first-time user confidence.
Effort: S
Risk: Low
Dependencies: None.
Success metric: Higher rate of inventory completion within same session.

### 20. [P2] Template personalization snippets
Problem: Template bodies can feel generic and reduce response quality.
Proposed solution: Optional short personalized intro snippets per template.
Why it is better than GRANOT: Better customer communication quality.
Effort: M
Risk: Medium
Dependencies: Email template rendering extension.
Success metric: Higher customer response rate on first outbound email.

### 21. [P2] Inventory anomaly detection hints
Problem: Outlier quantities can be entered accidentally.
Proposed solution: Soft warnings for unusual category totals.
Why it is better than GRANOT: Better data quality without blocking flow.
Effort: M
Risk: Medium
Dependencies: Lightweight heuristics.
Success metric: Fewer post-quote inventory corrections.

### 22. [P2] Performance budget for estimate workspace
Problem: Dense tab components can regress over time with added features.
Proposed solution: Set render and interaction performance budgets with CI checks.
Why it is better than GRANOT: Sustains responsiveness as feature depth grows.
Effort: M
Risk: Medium
Dependencies: profiling tooling and CI instrumentation.
Success metric: P95 interaction latency stays within budget.
