CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE tenants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    email TEXT NOT NULL,
    full_name TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX users_tenant_email_uidx ON users (tenant_id, lower(email));

CREATE TABLE roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, name)
);

CREATE TABLE permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT ''
);

CREATE TABLE role_permissions (
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (role_id, permission_id)
);

CREATE TABLE user_roles (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, role_id)
);
CREATE INDEX user_roles_tenant_user_idx ON user_roles (tenant_id, user_id);

CREATE TABLE customers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    first_name TEXT NOT NULL,
    last_name TEXT NOT NULL,
    email TEXT,
    phone TEXT,
    created_by UUID REFERENCES users(id),
    updated_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX customers_tenant_idx ON customers (tenant_id);
CREATE UNIQUE INDEX customers_tenant_id_uidx ON customers (tenant_id, id);

CREATE TABLE tenant_counters (
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    counter_type TEXT NOT NULL,
    next_value BIGINT NOT NULL DEFAULT 1,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, counter_type),
    CONSTRAINT tenant_counters_counter_type_chk CHECK (counter_type IN ('estimate', 'job'))
);

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

CREATE TABLE sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    csrf_token TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ
);
CREATE INDEX sessions_token_active_idx ON sessions (token_hash) WHERE revoked_at IS NULL;

CREATE TABLE audit_log (
    id BIGSERIAL PRIMARY KEY,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    action TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id UUID,
    request_id TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX audit_log_tenant_created_idx ON audit_log (tenant_id, created_at DESC);
