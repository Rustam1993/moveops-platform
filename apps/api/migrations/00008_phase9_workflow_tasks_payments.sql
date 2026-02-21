-- +goose Up
-- +goose StatementBegin
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
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS estimate_payment_active_idx;
DROP INDEX IF EXISTS estimate_payment_tenant_estimate_idx;
DROP TABLE IF EXISTS estimate_payment;

DROP INDEX IF EXISTS estimate_task_active_idx;
DROP INDEX IF EXISTS estimate_task_tenant_estimate_idx;
DROP TABLE IF EXISTS estimate_task;

DROP INDEX IF EXISTS estimate_workflow_tenant_status_idx;
DROP TABLE IF EXISTS estimate_workflow;
-- +goose StatementEnd
