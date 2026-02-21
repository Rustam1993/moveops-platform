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
CREATE UNIQUE INDEX customers_tenant_email_uidx
    ON customers (tenant_id, lower(email))
    WHERE email IS NOT NULL AND btrim(email) <> '';

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
    total_volume_cf DOUBLE PRECISION NOT NULL DEFAULT 0,
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
        total_volume_cf >= 0
        AND
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

CREATE TABLE estimate_inventory_item (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    estimate_id UUID NOT NULL REFERENCES estimates(id) ON DELETE CASCADE,
    category TEXT NOT NULL,
    item_name TEXT NOT NULL,
    volume_cf DOUBLE PRECISION NOT NULL CHECK (volume_cf >= 0),
    qty INT NOT NULL CHECK (qty >= 0),
    is_custom BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX estimate_inventory_item_tenant_estimate_idx ON estimate_inventory_item (tenant_id, estimate_id);
CREATE UNIQUE INDEX estimate_inventory_item_unique_idx
    ON estimate_inventory_item (tenant_id, estimate_id, lower(category), lower(item_name), is_custom);

CREATE TABLE estimate_inventory_share_link (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    estimate_id UUID NOT NULL REFERENCES estimates(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    recipient_email TEXT NOT NULL,
    delivery_mode TEXT NOT NULL DEFAULT 'log',
    delivery_error TEXT,
    created_by UUID REFERENCES users(id),
    expires_at TIMESTAMPTZ NOT NULL,
    last_accessed_at TIMESTAMPTZ,
    last_updated_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX estimate_inventory_share_link_tenant_estimate_idx
    ON estimate_inventory_share_link (tenant_id, estimate_id, created_at DESC);
CREATE INDEX estimate_inventory_share_link_active_idx
    ON estimate_inventory_share_link (token_hash, expires_at)
    WHERE revoked_at IS NULL;

CREATE TABLE estimate_charge (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    estimate_id UUID NOT NULL REFERENCES estimates(id) ON DELETE CASCADE,
    mode TEXT NOT NULL CHECK (mode IN ('local', 'long_distance')),
    calculation_version TEXT NOT NULL DEFAULT 'v1',
    cf_lbs_ratio DOUBLE PRECISION NOT NULL DEFAULT 7 CHECK (cf_lbs_ratio >= 0),
    fuel_surcharge_pct DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (fuel_surcharge_pct >= 0 AND fuel_surcharge_pct <= 100),
    ld_rate_per_cf DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (ld_rate_per_cf >= 0),
    ld_fixed_base_amount_cents BIGINT CHECK (ld_fixed_base_amount_cents IS NULL OR ld_fixed_base_amount_cents >= 0),
    local_trucks INT NOT NULL DEFAULT 0 CHECK (local_trucks >= 0),
    local_workers INT NOT NULL DEFAULT 0 CHECK (local_workers >= 0),
    local_labor_hours DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (local_labor_hours >= 0),
    local_labor_rate_cents BIGINT NOT NULL DEFAULT 0 CHECK (local_labor_rate_cents >= 0),
    local_travel_hours DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (local_travel_hours >= 0),
    local_travel_rate_cents BIGINT NOT NULL DEFAULT 0 CHECK (local_travel_rate_cents >= 0),
    other_line_items_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    discount_coupon_pct DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (discount_coupon_pct >= 0 AND discount_coupon_pct <= 100),
    discount_coupon_amount_cents BIGINT NOT NULL DEFAULT 0 CHECK (discount_coupon_amount_cents >= 0),
    discount_senior_pct DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (discount_senior_pct >= 0 AND discount_senior_pct <= 100),
    discount_senior_amount_cents BIGINT NOT NULL DEFAULT 0 CHECK (discount_senior_amount_cents >= 0),
    packing_packers INT NOT NULL DEFAULT 0 CHECK (packing_packers >= 0),
    packing_hours DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (packing_hours >= 0),
    packing_rate_cents BIGINT NOT NULL DEFAULT 0 CHECK (packing_rate_cents >= 0),
    liability_type TEXT NOT NULL DEFAULT 'release' CHECK (liability_type IN ('release', 'full_value')),
    liability_valuation_charge_cents BIGINT NOT NULL DEFAULT 0 CHECK (liability_valuation_charge_cents >= 0),
    tax_rate_pct DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (tax_rate_pct >= 0 AND tax_rate_pct <= 100),
    deposit_required_cents BIGINT CHECK (deposit_required_cents IS NULL OR deposit_required_cents >= 0),
    amount_paid_cents BIGINT NOT NULL DEFAULT 0 CHECK (amount_paid_cents >= 0),
    computed_total_cf DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (computed_total_cf >= 0),
    computed_total_lbs DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (computed_total_lbs >= 0),
    computed_base_cents BIGINT NOT NULL DEFAULT 0,
    computed_fuel_surcharge_cents BIGINT NOT NULL DEFAULT 0,
    computed_other_items_cents BIGINT NOT NULL DEFAULT 0,
    computed_packing_cents BIGINT NOT NULL DEFAULT 0,
    computed_liability_cents BIGINT NOT NULL DEFAULT 0,
    computed_subtotal_cents BIGINT NOT NULL DEFAULT 0,
    computed_discounts_cents BIGINT NOT NULL DEFAULT 0,
    computed_tax_cents BIGINT NOT NULL DEFAULT 0,
    computed_total_cents BIGINT NOT NULL DEFAULT 0,
    created_by UUID REFERENCES users(id),
    updated_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX estimate_charge_tenant_estimate_uidx ON estimate_charge (tenant_id, estimate_id);
CREATE INDEX estimate_charge_tenant_idx ON estimate_charge (tenant_id, updated_at DESC);

CREATE TABLE estimate_workflow (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    estimate_id UUID NOT NULL REFERENCES estimates(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'open', 'follow_up', 'quoted', 'booked', 'on_hold', 'canceled')),
    priority_level INT NOT NULL DEFAULT 0 CHECK (priority_level BETWEEN 0 AND 8),
    follow_up_at TIMESTAMPTZ,
    follow_up_note TEXT,
    vip BOOLEAN NOT NULL DEFAULT FALSE,
    booked_at TIMESTAMPTZ,
    hold_reason TEXT,
    created_by UUID REFERENCES users(id),
    updated_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, estimate_id)
);
CREATE INDEX estimate_workflow_tenant_status_idx
    ON estimate_workflow (tenant_id, status, updated_at DESC);

CREATE TABLE estimate_task (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    estimate_id UUID NOT NULL REFERENCES estimates(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    is_done BOOLEAN NOT NULL DEFAULT FALSE,
    due_at TIMESTAMPTZ,
    created_by UUID REFERENCES users(id),
    updated_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);
CREATE INDEX estimate_task_tenant_estimate_idx
    ON estimate_task (tenant_id, estimate_id, created_at DESC);
CREATE INDEX estimate_task_active_idx
    ON estimate_task (tenant_id, estimate_id, is_done, due_at)
    WHERE deleted_at IS NULL;

CREATE TABLE estimate_payment (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    estimate_id UUID NOT NULL REFERENCES estimates(id) ON DELETE CASCADE,
    amount_cents BIGINT NOT NULL CHECK (amount_cents > 0),
    method TEXT NOT NULL,
    paid_at TIMESTAMPTZ NOT NULL,
    notes TEXT,
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);
CREATE INDEX estimate_payment_tenant_estimate_idx
    ON estimate_payment (tenant_id, estimate_id, paid_at DESC, created_at DESC);
CREATE INDEX estimate_payment_active_idx
    ON estimate_payment (tenant_id, estimate_id)
    WHERE deleted_at IS NULL;

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

CREATE TABLE new_estimate_catalog_category (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    sort_order INT NOT NULL DEFAULT 0,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_by UUID REFERENCES users(id),
    updated_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, name)
);
CREATE INDEX new_estimate_catalog_category_tenant_sort_idx
    ON new_estimate_catalog_category (tenant_id, active DESC, sort_order ASC, name ASC);

CREATE TABLE new_estimate_catalog_item (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    category_id UUID REFERENCES new_estimate_catalog_category(id) ON DELETE SET NULL,
    name TEXT NOT NULL,
    volume_cf DOUBLE PRECISION NOT NULL CHECK (volume_cf >= 0),
    sort_order INT NOT NULL DEFAULT 0,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_by UUID REFERENCES users(id),
    updated_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, category_id, name)
);
CREATE INDEX new_estimate_catalog_item_tenant_category_sort_idx
    ON new_estimate_catalog_item (tenant_id, category_id, active DESC, sort_order ASC, name ASC);

CREATE TABLE tenant_new_estimate_settings (
    tenant_id UUID PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
    pricing_defaults_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    email_templates_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    document_branding_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    updated_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE analytics_event (
    id BIGSERIAL PRIMARY KEY,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    estimate_id UUID REFERENCES estimates(id) ON DELETE SET NULL,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    event_name TEXT NOT NULL,
    properties_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX analytics_event_tenant_name_created_idx
    ON analytics_event (tenant_id, event_name, created_at DESC);
CREATE INDEX analytics_event_tenant_estimate_created_idx
    ON analytics_event (tenant_id, estimate_id, created_at DESC)
    WHERE estimate_id IS NOT NULL;

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
