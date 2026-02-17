-- +goose Up
-- +goose StatementBegin
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
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS estimate_charge_tenant_idx;
DROP INDEX IF EXISTS estimate_charge_tenant_estimate_uidx;
DROP TABLE IF EXISTS estimate_charge;
-- +goose StatementEnd
