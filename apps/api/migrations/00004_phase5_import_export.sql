-- +goose Up
-- +goose StatementBegin
CREATE UNIQUE INDEX customers_tenant_email_uidx
    ON customers (tenant_id, lower(email))
    WHERE email IS NOT NULL AND btrim(email) <> '';

CREATE TABLE import_run (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    created_by_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    source TEXT NOT NULL,
    filename TEXT NOT NULL,
    file_sha256 TEXT NOT NULL,
    mode TEXT NOT NULL CHECK (mode IN ('dry_run', 'apply')),
    status TEXT NOT NULL CHECK (status IN ('completed', 'failed')),
    mapping_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    summary_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);
CREATE INDEX import_run_tenant_created_idx ON import_run (tenant_id, created_at DESC);
CREATE INDEX import_run_tenant_file_hash_idx ON import_run (tenant_id, file_sha256);

CREATE TABLE import_row_result (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    import_run_id UUID NOT NULL REFERENCES import_run(id) ON DELETE CASCADE,
    row_number INT NOT NULL,
    severity TEXT NOT NULL CHECK (severity IN ('error', 'warn', 'info')),
    entity_type TEXT NOT NULL CHECK (entity_type IN ('customer', 'estimate', 'job', 'storage_record')),
    idempotency_key TEXT NOT NULL,
    result TEXT NOT NULL CHECK (result IN ('created', 'updated', 'skipped', 'error')),
    field TEXT,
    message TEXT NOT NULL,
    raw_value TEXT,
    target_entity_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, import_run_id, entity_type, idempotency_key)
);
CREATE INDEX import_row_result_tenant_run_idx ON import_row_result (tenant_id, import_run_id, row_number);
CREATE INDEX import_row_result_tenant_run_severity_idx ON import_row_result (tenant_id, import_run_id, severity);

CREATE TABLE import_idempotency (
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    entity_type TEXT NOT NULL CHECK (entity_type IN ('customer', 'estimate', 'job', 'storage_record')),
    idempotency_key TEXT NOT NULL,
    target_entity_id UUID NOT NULL,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, entity_type, idempotency_key)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS import_idempotency;

DROP INDEX IF EXISTS import_row_result_tenant_run_severity_idx;
DROP INDEX IF EXISTS import_row_result_tenant_run_idx;
DROP TABLE IF EXISTS import_row_result;

DROP INDEX IF EXISTS import_run_tenant_file_hash_idx;
DROP INDEX IF EXISTS import_run_tenant_created_idx;
DROP TABLE IF EXISTS import_run;

DROP INDEX IF EXISTS customers_tenant_email_uidx;
-- +goose StatementEnd
