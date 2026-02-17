-- +goose Up
-- +goose StatementBegin
ALTER TABLE estimates
  ADD COLUMN total_volume_cf DOUBLE PRECISION NOT NULL DEFAULT 0;

ALTER TABLE estimates
  DROP CONSTRAINT IF EXISTS estimates_positive_amounts_chk;

ALTER TABLE estimates
  ADD CONSTRAINT estimates_positive_amounts_chk CHECK (
    total_volume_cf >= 0
    AND (estimated_total_cents IS NULL OR estimated_total_cents >= 0)
    AND (deposit_cents IS NULL OR deposit_cents >= 0)
  );

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
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS estimate_inventory_share_link_active_idx;
DROP INDEX IF EXISTS estimate_inventory_share_link_tenant_estimate_idx;
DROP TABLE IF EXISTS estimate_inventory_share_link;

DROP INDEX IF EXISTS estimate_inventory_item_unique_idx;
DROP INDEX IF EXISTS estimate_inventory_item_tenant_estimate_idx;
DROP TABLE IF EXISTS estimate_inventory_item;

ALTER TABLE estimates
  DROP CONSTRAINT IF EXISTS estimates_positive_amounts_chk;

ALTER TABLE estimates
  ADD CONSTRAINT estimates_positive_amounts_chk CHECK (
    (estimated_total_cents IS NULL OR estimated_total_cents >= 0)
    AND (deposit_cents IS NULL OR deposit_cents >= 0)
  );

ALTER TABLE estimates
  DROP COLUMN IF EXISTS total_volume_cf;
-- +goose StatementEnd
