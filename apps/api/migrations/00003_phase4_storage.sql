-- +goose Up
-- +goose StatementBegin
CREATE TABLE storage_record (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    job_id UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    facility TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'in_storage' CHECK (status IN ('in_storage', 'sit', 'out')),
    date_in DATE,
    date_out DATE,
    next_bill_date DATE,
    lot_number TEXT,
    location_label TEXT,
    vaults INT NOT NULL DEFAULT 0 CHECK (vaults >= 0),
    pads INT NOT NULL DEFAULT 0 CHECK (pads >= 0),
    items INT NOT NULL DEFAULT 0 CHECK (items >= 0),
    oversize_items INT NOT NULL DEFAULT 0 CHECK (oversize_items >= 0),
    volume INT NOT NULL DEFAULT 0 CHECK (volume >= 0),
    monthly_rate_cents BIGINT CHECK (monthly_rate_cents IS NULL OR monthly_rate_cents >= 0),
    storage_balance_cents BIGINT NOT NULL DEFAULT 0 CHECK (storage_balance_cents >= 0),
    move_balance_cents BIGINT NOT NULL DEFAULT 0 CHECK (move_balance_cents >= 0),
    last_payment_at TIMESTAMPTZ,
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX storage_record_tenant_job_uidx ON storage_record (tenant_id, job_id);
CREATE INDEX storage_record_tenant_facility_idx ON storage_record (tenant_id, facility);
CREATE INDEX storage_record_tenant_balance_idx ON storage_record (tenant_id, storage_balance_cents);
CREATE INDEX storage_record_tenant_date_in_idx ON storage_record (tenant_id, date_in);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS storage_record_tenant_date_in_idx;
DROP INDEX IF EXISTS storage_record_tenant_balance_idx;
DROP INDEX IF EXISTS storage_record_tenant_facility_idx;
DROP INDEX IF EXISTS storage_record_tenant_job_uidx;
DROP TABLE IF EXISTS storage_record;
-- +goose StatementEnd
