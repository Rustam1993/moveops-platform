# New Estimate Phase 6 UX Audit

## Scope
This audit compares the estimate workspace journey in GRANOT-style legacy UI patterns vs MoveOps Phase 6 UX.

Journey covered:
- Entry Form -> Inventory -> Charges -> Printed Estimate/Email -> Signature -> Book/Follow-up

## 1) Side-by-side journey map

| Step | GRANOT baseline (from screenshots/pattern) | MoveOps before Phase 6 | MoveOps after Phase 6 quick wins |
|---|---|---|---|
| Entry Form | Dense form blocks, many fields visible at once, weak completion guidance | Modern grouped form, autosave/save states, but no explicit readiness checklist | Added readiness checklist with missing-section guidance and links; clearer "ready to send quote" state |
| Inventory | Effective qty controls but high scanning cost; right utility panel split attention | Modern table + categories + custom item flow; still repetitive for common box-heavy jobs | Added quick-add box presets and `/` focus-search shortcut to reduce repetitive clicks |
| Charges | Powerful but spreadsheet-like; easy to miss dependencies (CF-driven LD pricing) | Modern grouped form with live calculations; summary was lower in scroll path | Added sticky summary and LD zero-CF guardrail warning for error prevention |
| Printed Estimate / Email | Email center integrated but template intent and latest state not always obvious | Functional templates and history | Added primary communication CTAs, template preview, last-sent visibility, and resend action |
| Signature flow | Available but mixed with other controls in legacy UI | Already implemented and tokenized in Phase 4 | Kept parity; improved surrounding send workflow clarity from Email Center |
| Book / Follow-up | Sidebar state controls exist but can feel hidden among many fields | Sidebar workflow controls implemented in Phase 5 | Kept functionality and improved status messaging consistency for confidence |

### Approximate click-path comparison (happy path)
- **GRANOT-style baseline**: Entry save (1) + tab nav to inventory (1) + multiple box item edits (8-20) + tab nav to charges (1) + pricing edits/save (5-10) + tab nav email/send (3-5) = **~19-37 interactions**
- **MoveOps pre-Phase 6**: similar path with better visual hierarchy but still many inventory clicks = **~17-32 interactions**
- **MoveOps Phase 6**: quick-add presets cut repetitive inventory actions significantly = **~10-24 interactions** for common box-heavy jobs

### Where users hesitate
- Deciding if estimate is complete enough to send (no explicit readiness in legacy patterns)
- Repetitive box entry in inventory
- Understanding whether LD pricing is valid when inventory CF is zero
- Choosing the right outbound email template and confirming what will be sent

## 2) Heuristic evaluation

### Clarity of system status
- Strengths in MoveOps: visible save states across major screens; autosave with explicit states.
- Gaps addressed in Phase 6: standardized status indicator component for Entry, Inventory, Charges, Email, Workflow.

### Recognition over recall
- Strengths: tabbed estimate workspace and labeled sections.
- Gaps addressed: readiness checklist now externalizes "what's missing" so users do not mentally track completion.

### Error prevention and recovery
- Strengths: field validation and server-side constraints.
- Gaps addressed: long-distance zero-CF warning before underquoting risk; retry affordance in status indicators.

### Information hierarchy on dense screens
- Strengths: card grouping already better than spreadsheet layouts.
- Gaps addressed: sticky charges summary keeps totals visible without scrolling; template preview reduces surprises.

### Speed for power users
- Strengths: qty controls and autosave.
- Gaps addressed: inventory quick-add presets and keyboard `/` search focus shortcut.

## 3) Concrete GRANOT pain points (justified)

1. **Too many fields without hierarchy**
- Evidence: screenshot patterns show long, table-like forms with many adjacent controls.
- Impact: higher scan time and error probability.

2. **Modal overload / hidden state**
- Evidence: priority and state transitions are modal/sidebar dependent with limited context.
- Impact: users can lose context about what changed.

3. **Weak completeness guidance**
- Evidence: no explicit "ready to send quote" checklist in screenshot patterns.
- Impact: handoff/sending happens with missing data.

4. **Slow inventory entry for common scenarios**
- Evidence: frequent repetitive qty taps for box-heavy estimates.
- Impact: unnecessary interaction count and slower time-to-quote.

## Summary
MoveOps now maintains parity while improving clarity, speed, and error prevention in the core New Estimate workflow. Phase 6 quick wins prioritize lower-risk UX improvements that reduce interaction cost without changing pricing logic or workflow scope.
