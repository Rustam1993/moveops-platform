-- +goose Up
-- +goose StatementBegin
CREATE TABLE estimate_document (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  estimate_id UUID NOT NULL REFERENCES estimates(id) ON DELETE CASCADE,
  document_type TEXT NOT NULL CHECK (document_type IN ('estimate_pdf', 'signed_estimate_pdf')),
  file_name TEXT NOT NULL,
  mime_type TEXT NOT NULL DEFAULT 'application/pdf',
  content_bytes BYTEA NOT NULL,
  content_sha256 TEXT NOT NULL,
  size_bytes INT NOT NULL CHECK (size_bytes >= 0),
  metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  generated_by UUID REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX estimate_document_tenant_estimate_type_created_idx
  ON estimate_document (tenant_id, estimate_id, document_type, created_at DESC);

CREATE TABLE estimate_email_log (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  estimate_id UUID NOT NULL REFERENCES estimates(id) ON DELETE CASCADE,
  template_key TEXT NOT NULL,
  email_to TEXT NOT NULL,
  email_cc TEXT,
  email_from TEXT NOT NULL,
  subject TEXT NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('queued', 'sent', 'failed')),
  provider_message_id TEXT,
  delivery_mode TEXT NOT NULL DEFAULT 'log' CHECK (delivery_mode IN ('log', 'smtp')),
  error_message TEXT,
  rendered_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_by UUID REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX estimate_email_log_tenant_estimate_created_idx
  ON estimate_email_log (tenant_id, estimate_id, created_at DESC);

CREATE TABLE estimate_quote_share_link (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  estimate_id UUID NOT NULL REFERENCES estimates(id) ON DELETE CASCADE,
  document_id UUID REFERENCES estimate_document(id) ON DELETE SET NULL,
  token_hash TEXT NOT NULL UNIQUE,
  recipient_email TEXT NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  last_accessed_at TIMESTAMPTZ,
  revoked_at TIMESTAMPTZ,
  created_by UUID REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX estimate_quote_share_link_tenant_estimate_created_idx
  ON estimate_quote_share_link (tenant_id, estimate_id, created_at DESC);
CREATE INDEX estimate_quote_share_link_active_idx
  ON estimate_quote_share_link (token_hash, expires_at)
  WHERE revoked_at IS NULL;

CREATE TABLE estimate_signature_request (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  estimate_id UUID NOT NULL REFERENCES estimates(id) ON DELETE CASCADE,
  document_id UUID REFERENCES estimate_document(id) ON DELETE SET NULL,
  token_hash TEXT NOT NULL UNIQUE,
  recipient_email TEXT NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  used_at TIMESTAMPTZ,
  last_accessed_at TIMESTAMPTZ,
  revoked_at TIMESTAMPTZ,
  created_by UUID REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX estimate_signature_request_tenant_estimate_created_idx
  ON estimate_signature_request (tenant_id, estimate_id, created_at DESC);
CREATE INDEX estimate_signature_request_active_idx
  ON estimate_signature_request (token_hash, expires_at)
  WHERE revoked_at IS NULL;

CREATE TABLE estimate_signature (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  estimate_id UUID NOT NULL REFERENCES estimates(id) ON DELETE CASCADE,
  signature_request_id UUID NOT NULL REFERENCES estimate_signature_request(id) ON DELETE CASCADE,
  document_id UUID REFERENCES estimate_document(id) ON DELETE SET NULL,
  signer_name TEXT NOT NULL,
  signer_email TEXT NOT NULL,
  signature_type TEXT NOT NULL CHECK (signature_type IN ('typed')),
  signature_value TEXT NOT NULL,
  agreed_terms BOOLEAN NOT NULL DEFAULT TRUE,
  ip_address TEXT,
  user_agent TEXT,
  signed_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (signature_request_id)
);
CREATE INDEX estimate_signature_tenant_estimate_signed_idx
  ON estimate_signature (tenant_id, estimate_id, signed_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS estimate_signature_tenant_estimate_signed_idx;
DROP TABLE IF EXISTS estimate_signature;

DROP INDEX IF EXISTS estimate_signature_request_active_idx;
DROP INDEX IF EXISTS estimate_signature_request_tenant_estimate_created_idx;
DROP TABLE IF EXISTS estimate_signature_request;

DROP INDEX IF EXISTS estimate_quote_share_link_active_idx;
DROP INDEX IF EXISTS estimate_quote_share_link_tenant_estimate_created_idx;
DROP TABLE IF EXISTS estimate_quote_share_link;

DROP INDEX IF EXISTS estimate_email_log_tenant_estimate_created_idx;
DROP TABLE IF EXISTS estimate_email_log;

DROP INDEX IF EXISTS estimate_document_tenant_estimate_type_created_idx;
DROP TABLE IF EXISTS estimate_document;
-- +goose StatementEnd
