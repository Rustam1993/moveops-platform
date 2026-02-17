# New Estimate Phase 4

## Scope
Phase 4 delivers estimate-scoped customer communication and document workflows:
- Printed Estimate PDF generation and preview
- Email Center template-based transactional sending
- E-Quote public link flow
- E-Sign public token signing flow

Implemented routes and APIs are tenant-scoped and RBAC-protected for internal actions, with token-based access for public customer pages.

## Routes
Internal workspace routes:
- `/estimates/:id/printed-estimate`
- `/estimates/:id/email`

Public routes (no login):
- `/public/estimate/[token]`
- `/public/sign/[token]`

## PDF generation approach
Internal endpoints:
- `POST /estimates/{estimateId}/documents/estimate-pdf`
- `GET /estimates/{estimateId}/documents/estimate-pdf`

Implementation details:
- PDF is generated server-side using `gofpdf` from current estimate data:
  - customer + origin/destination + move date
  - `totalVolumeCf` from inventory (Phase 2)
  - pricing summary from charges totals (Phase 3) when available
  - static terms section
- Generated PDF is persisted in `estimate_document` (`tenant_id` scoped) as `BYTEA` for MVP.
- Each document row stores:
  - `document_type` (`estimate_pdf` or `signed_estimate_pdf`)
  - content bytes, SHA-256 hash, size, metadata
- Audit event:
  - `document.generated`

## Email Center templates and variables
Internal endpoints:
- `GET /estimates/{estimateId}/emails`
- `POST /estimates/{estimateId}/emails/send`

MVP templates:
- `moving_estimate` (E-Quote)
- `update_inventory` (Link to Inventory)
- `signature_request` (E-Sign)
- placeholders: `credit_card_authorization`, `waiver_cancellation`, `follow_up_move`

Email behavior:
- Transactional email only (plain-text body)
- Configurable sender headers:
  - `EMAIL_FROM`
  - `EMAIL_REPLY_TO`
- Delivery modes:
  - `EMAIL_MODE=log` (dev fallback logs content)
  - `EMAIL_MODE=smtp` (SMTP provider path)
- Email history is persisted to `estimate_email_log` with status and delivery metadata.

Template variable snapshot (stored in `rendered_json`) includes estimate id/customer and generated URLs when applicable:
- `quoteUrl`
- `inventoryUrl`
- `signatureUrl`

Audit events:
- `email.sent`
- `email.failed`

## Public token security
Public API endpoints:
- `GET /public/estimate/{token}`
- `GET /public/sign/{token}`
- `POST /public/sign/{token}`

Security model:
- High-entropy random tokens are generated server-side.
- Only hashed tokens are stored (`token_hash`), never plaintext tokens.
- Token records map to `tenant_id` + `estimate_id`.
- `expires_at` enforced on every public request.
- Signature tokens are one-time-use:
  - `used_at` set on completion
  - repeat completion attempts rejected
- Public endpoints are IP rate-limited.
- No cookie/session auth is used for public token endpoints.

Error behaviors:
- invalid token -> `*_not_found`
- expired token -> `*_expired`
- reused sign token -> `signature_already_completed`

## E-Quote and E-Sign flows
E-Quote:
- Email Center `moving_estimate` generates a quote-share token and sends a secure link.
- Public quote page displays estimate context and PDF preview/download.

E-Sign:
- Email Center `signature_request` (or direct signature-request endpoint) creates a signature token and sends link.
- Public sign page shows estimate document and captures typed signature + explicit agreement checkbox.
- On completion:
  - signature request is marked used
  - `estimate_signature` row is created
  - a signed document record (`signed_estimate_pdf`) is generated and associated

Captured signing data:
- `signer_name`
- `signer_email`
- `signature_type` (`typed`)
- `signature_value` (typed signature text)
- `signed_at`
- `ip_address` (best effort)
- `user_agent` (best effort)
- `document_id` of signed document

Audit events:
- `signature.requested`
- `signature.completed`
- `signature.failed`

## Data model additions
Migration: `apps/api/migrations/00007_phase8_documents_email_signature.sql`

Added tables:
- `estimate_document`
- `estimate_email_log`
- `estimate_quote_share_link`
- `estimate_signature_request`
- `estimate_signature`

All new rows include `tenant_id` and enforce tenant isolation via query scoping.
