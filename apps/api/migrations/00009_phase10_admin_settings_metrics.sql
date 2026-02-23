-- +goose Up
-- +goose StatementBegin
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
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS analytics_event_tenant_estimate_created_idx;
DROP INDEX IF EXISTS analytics_event_tenant_name_created_idx;
DROP TABLE IF EXISTS analytics_event;

DROP TABLE IF EXISTS tenant_new_estimate_settings;

DROP INDEX IF EXISTS new_estimate_catalog_item_tenant_category_sort_idx;
DROP TABLE IF EXISTS new_estimate_catalog_item;

DROP INDEX IF EXISTS new_estimate_catalog_category_tenant_sort_idx;
DROP TABLE IF EXISTS new_estimate_catalog_category;
-- +goose StatementEnd
