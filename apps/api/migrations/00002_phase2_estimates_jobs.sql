-- +goose Up
-- +goose StatementBegin
CREATE TABLE tenant_counters (
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    counter_type TEXT NOT NULL,
    next_value BIGINT NOT NULL DEFAULT 1,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, counter_type),
    CONSTRAINT tenant_counters_counter_type_chk CHECK (counter_type IN ('estimate', 'job'))
);

CREATE UNIQUE INDEX customers_tenant_id_uidx ON customers (tenant_id, id);

CREATE TABLE estimates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    estimate_number TEXT NOT NULL,
    customer_id UUID NOT NULL REFERENCES customers(id),
    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'converted')),
    customer_name TEXT NOT NULL,
    primary_phone TEXT NOT NULL,
    secondary_phone TEXT,
    email TEXT NOT NULL,
    origin_address_line1 TEXT NOT NULL,
    origin_city TEXT NOT NULL,
    origin_state TEXT NOT NULL,
    origin_postal_code TEXT NOT NULL,
    destination_address_line1 TEXT NOT NULL,
    destination_city TEXT NOT NULL,
    destination_state TEXT NOT NULL,
    destination_postal_code TEXT NOT NULL,
    move_date DATE NOT NULL,
    pickup_time TEXT,
    lead_source TEXT NOT NULL,
    move_size TEXT,
    location_type TEXT,
    estimated_total_cents BIGINT,
    deposit_cents BIGINT,
    notes TEXT,
    idempotency_key TEXT,
    idempotency_payload_hash TEXT,
    created_by UUID REFERENCES users(id),
    updated_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT estimates_positive_amounts_chk CHECK (
        (estimated_total_cents IS NULL OR estimated_total_cents >= 0)
        AND (deposit_cents IS NULL OR deposit_cents >= 0)
    )
);
CREATE INDEX estimates_tenant_idx ON estimates (tenant_id);
CREATE UNIQUE INDEX estimates_tenant_number_uidx ON estimates (tenant_id, estimate_number);
CREATE UNIQUE INDEX estimates_tenant_id_uidx ON estimates (tenant_id, id);
CREATE UNIQUE INDEX estimates_tenant_idempotency_uidx
    ON estimates (tenant_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;

CREATE TABLE jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    job_number TEXT NOT NULL,
    estimate_id UUID REFERENCES estimates(id),
    customer_id UUID NOT NULL REFERENCES customers(id),
    status TEXT NOT NULL DEFAULT 'booked' CHECK (status IN ('booked', 'scheduled', 'completed', 'cancelled')),
    scheduled_date DATE,
    pickup_time TEXT,
    convert_idempotency_key TEXT,
    created_by UUID REFERENCES users(id),
    updated_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX jobs_tenant_idx ON jobs (tenant_id);
CREATE UNIQUE INDEX jobs_tenant_number_uidx ON jobs (tenant_id, job_number);
CREATE UNIQUE INDEX jobs_tenant_estimate_uidx
    ON jobs (tenant_id, estimate_id)
    WHERE estimate_id IS NOT NULL;
CREATE UNIQUE INDEX jobs_tenant_convert_idempotency_uidx
    ON jobs (tenant_id, convert_idempotency_key)
    WHERE convert_idempotency_key IS NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS jobs_tenant_convert_idempotency_uidx;
DROP INDEX IF EXISTS jobs_tenant_estimate_uidx;
DROP INDEX IF EXISTS jobs_tenant_number_uidx;
DROP INDEX IF EXISTS jobs_tenant_idx;
DROP TABLE IF EXISTS jobs;

DROP INDEX IF EXISTS estimates_tenant_idempotency_uidx;
DROP INDEX IF EXISTS estimates_tenant_id_uidx;
DROP INDEX IF EXISTS estimates_tenant_number_uidx;
DROP INDEX IF EXISTS estimates_tenant_idx;
DROP TABLE IF EXISTS estimates;

DROP INDEX IF EXISTS customers_tenant_id_uidx;
DROP TABLE IF EXISTS tenant_counters;
-- +goose StatementEnd
