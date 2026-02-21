"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Loader2, Save } from "lucide-react";
import { toast } from "sonner";

import { NewEstimateAdminNav } from "@/components/admin/new-estimate-admin-nav";
import { NotAuthorizedState } from "@/components/layout/not-authorized-state";
import { PageHeader } from "@/components/layout/page-header";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { isForbiddenError } from "@/lib/api";
import {
  getPricingDefaults,
  savePricingDefaults,
  type NewEstimatePricingDefaults,
} from "@/lib/new-estimate-admin-api";
import { getApiErrorMessage } from "@/lib/phase2-api";

type SaveState = "idle" | "saving" | "saved" | "error";

type PricingFormState = {
  ldRatePerCf: string;
  ldFuelPct: string;
  ldTaxPct: string;
  localLaborRate: string;
  localTravelRate: string;
  localFuelPct: string;
  localTaxPct: string;
  seniorPct: string;
  couponPct: string;
  liabilityType: "release" | "full_value";
  valuationCharge: string;
};

const emptyForm: PricingFormState = {
  ldRatePerCf: "",
  ldFuelPct: "",
  ldTaxPct: "",
  localLaborRate: "",
  localTravelRate: "",
  localFuelPct: "",
  localTaxPct: "",
  seniorPct: "",
  couponPct: "",
  liabilityType: "release",
  valuationCharge: "",
};

export default function NewEstimatePricingAdminPage() {
  const [form, setForm] = useState<PricingFormState>(emptyForm);
  const [loading, setLoading] = useState(true);
  const [forbidden, setForbidden] = useState(false);
  const [saveState, setSaveState] = useState<SaveState>("idle");

  const statusText = useMemo(() => {
    if (saveState === "saving") return "Saving...";
    if (saveState === "saved") return "Saved";
    if (saveState === "error") return "Save failed";
    return "Ready";
  }, [saveState]);

  const loadData = useCallback(async () => {
    setLoading(true);
    try {
      const response = await getPricingDefaults();
      setForm(fromPricingDefaults(response.pricing));
      setForbidden(false);
      setSaveState("idle");
    } catch (error) {
      if (isForbiddenError(error)) {
        setForbidden(true);
      } else {
        toast.error(getApiErrorMessage(error));
      }
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadData();
  }, [loadData]);

  function onInputChange(field: keyof PricingFormState, value: string) {
    setForm((previous) => ({ ...previous, [field]: value }));
    setSaveState("idle");
  }

  async function onSave() {
    setSaveState("saving");
    try {
      const payload = toPricingDefaults(form);
      const response = await savePricingDefaults(payload);
      setForm(fromPricingDefaults(response.pricing));
      setSaveState("saved");
      toast.success("Pricing defaults saved");
    } catch (error) {
      setSaveState("error");
      toast.error(getApiErrorMessage(error));
    }
  }

  if (forbidden) {
    return (
      <div className="space-y-6 pb-8">
        <PageHeader title="Pricing Defaults" description="Tenant defaults for new estimate charges." />
        <NewEstimateAdminNav />
        <NotAuthorizedState message="This page requires admin.new_estimate permission." />
      </div>
    );
  }

  return (
    <div className="space-y-6 pb-8">
      <PageHeader
        title="Pricing Defaults"
        description="Prefill Charges for new estimates and first-time charge setup. Existing estimate charges are never overwritten."
      />
      <NewEstimateAdminNav />

      {loading ? (
        <Card>
          <CardContent className="py-10 text-sm text-muted-foreground">
            <span className="inline-flex items-center gap-2">
              <Loader2 className="h-4 w-4 animate-spin" />
              Loading pricing defaults...
            </span>
          </CardContent>
        </Card>
      ) : (
        <>
          <Card className="border-border/70 bg-card/70">
            <CardContent className="flex items-center justify-between gap-3 p-4">
              <p className="text-sm text-muted-foreground" aria-live="polite" role="status">
                {statusText}
              </p>
              <Button onClick={() => void onSave()} disabled={saveState === "saving"}>
                {saveState === "saving" ? (
                  <span className="inline-flex items-center gap-2">
                    <Loader2 className="h-4 w-4 animate-spin" />
                    Saving...
                  </span>
                ) : (
                  <span className="inline-flex items-center gap-2">
                    <Save className="h-4 w-4" />
                    Save defaults
                  </span>
                )}
              </Button>
            </CardContent>
          </Card>

          <div className="grid gap-6 lg:grid-cols-2">
            <Card className="border-border/70 bg-card/70">
              <CardHeader>
                <CardTitle>Long Distance defaults</CardTitle>
                <CardDescription>Applied when service mode is Long Distance.</CardDescription>
              </CardHeader>
              <CardContent className="grid gap-4 md:grid-cols-3">
                <Field label="Rate per CF ($)">
                  <Input type="number" step="0.01" value={form.ldRatePerCf} onChange={(event) => onInputChange("ldRatePerCf", event.target.value)} />
                </Field>
                <Field label="Fuel surcharge (%)">
                  <Input type="number" step="0.01" value={form.ldFuelPct} onChange={(event) => onInputChange("ldFuelPct", event.target.value)} />
                </Field>
                <Field label="Tax rate (%)">
                  <Input type="number" step="0.01" value={form.ldTaxPct} onChange={(event) => onInputChange("ldTaxPct", event.target.value)} />
                </Field>
              </CardContent>
            </Card>

            <Card className="border-border/70 bg-card/70">
              <CardHeader>
                <CardTitle>Local defaults</CardTitle>
                <CardDescription>Applied when service mode is Local.</CardDescription>
              </CardHeader>
              <CardContent className="grid gap-4 md:grid-cols-2">
                <Field label="Labor rate ($/hr)">
                  <Input
                    type="number"
                    step="0.01"
                    value={form.localLaborRate}
                    onChange={(event) => onInputChange("localLaborRate", event.target.value)}
                  />
                </Field>
                <Field label="Travel rate ($/hr)">
                  <Input
                    type="number"
                    step="0.01"
                    value={form.localTravelRate}
                    onChange={(event) => onInputChange("localTravelRate", event.target.value)}
                  />
                </Field>
                <Field label="Fuel surcharge (%)">
                  <Input type="number" step="0.01" value={form.localFuelPct} onChange={(event) => onInputChange("localFuelPct", event.target.value)} />
                </Field>
                <Field label="Tax rate (%)">
                  <Input type="number" step="0.01" value={form.localTaxPct} onChange={(event) => onInputChange("localTaxPct", event.target.value)} />
                </Field>
              </CardContent>
            </Card>

            <Card className="border-border/70 bg-card/70">
              <CardHeader>
                <CardTitle>Discount presets</CardTitle>
                <CardDescription>Used to prefill discount fields in Charges.</CardDescription>
              </CardHeader>
              <CardContent className="grid gap-4 md:grid-cols-2">
                <Field label="Senior discount (%)">
                  <Input type="number" step="0.01" value={form.seniorPct} onChange={(event) => onInputChange("seniorPct", event.target.value)} />
                </Field>
                <Field label="Coupon discount (%)">
                  <Input type="number" step="0.01" value={form.couponPct} onChange={(event) => onInputChange("couponPct", event.target.value)} />
                </Field>
              </CardContent>
            </Card>

            <Card className="border-border/70 bg-card/70">
              <CardHeader>
                <CardTitle>Liability defaults</CardTitle>
                <CardDescription>Default selection shown in Charges liability section.</CardDescription>
              </CardHeader>
              <CardContent className="grid gap-4 md:grid-cols-2">
                <Field label="Default type">
                  <select
                    className="h-10 w-full rounded-md border border-input bg-background px-3 text-sm"
                    value={form.liabilityType}
                    onChange={(event) => onInputChange("liabilityType", event.target.value as PricingFormState["liabilityType"])}
                  >
                    <option value="release">Release Value</option>
                    <option value="full_value">Full Value Protection</option>
                  </select>
                </Field>
                <Field label="Valuation fee ($)">
                  <Input
                    type="number"
                    step="0.01"
                    value={form.valuationCharge}
                    onChange={(event) => onInputChange("valuationCharge", event.target.value)}
                  />
                </Field>
              </CardContent>
            </Card>
          </div>
        </>
      )}
    </div>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="space-y-2">
      <Label>{label}</Label>
      {children}
    </div>
  );
}

function fromPricingDefaults(input: NewEstimatePricingDefaults): PricingFormState {
  return {
    ldRatePerCf: toInput(input.longDistance?.ratePerCf),
    ldFuelPct: toInput(input.longDistance?.fuelSurchargePct),
    ldTaxPct: toInput(input.longDistance?.taxRatePct),
    localLaborRate: toInput(centsToDollars(input.local?.laborRateCents)),
    localTravelRate: toInput(centsToDollars(input.local?.travelRateCents)),
    localFuelPct: toInput(input.local?.fuelSurchargePct),
    localTaxPct: toInput(input.local?.taxRatePct),
    seniorPct: toInput(input.discounts?.seniorPct),
    couponPct: toInput(input.discounts?.couponPct),
    liabilityType: input.liability?.type ?? "release",
    valuationCharge: toInput(centsToDollars(input.liability?.valuationChargeCents)),
  };
}

function toPricingDefaults(form: PricingFormState): NewEstimatePricingDefaults {
  const payload: NewEstimatePricingDefaults = {};

  const longDistance: NonNullable<NewEstimatePricingDefaults["longDistance"]> = {};
  const ldRatePerCf = toNumber(form.ldRatePerCf);
  const ldFuelPct = toNumber(form.ldFuelPct);
  const ldTaxPct = toNumber(form.ldTaxPct);
  if (ldRatePerCf !== undefined) longDistance.ratePerCf = ldRatePerCf;
  if (ldFuelPct !== undefined) longDistance.fuelSurchargePct = ldFuelPct;
  if (ldTaxPct !== undefined) longDistance.taxRatePct = ldTaxPct;
  if (Object.keys(longDistance).length > 0) payload.longDistance = longDistance;

  const local: NonNullable<NewEstimatePricingDefaults["local"]> = {};
  const localLaborRate = toNumber(form.localLaborRate);
  const localTravelRate = toNumber(form.localTravelRate);
  const localFuelPct = toNumber(form.localFuelPct);
  const localTaxPct = toNumber(form.localTaxPct);
  if (localLaborRate !== undefined) local.laborRateCents = Math.round(localLaborRate * 100);
  if (localTravelRate !== undefined) local.travelRateCents = Math.round(localTravelRate * 100);
  if (localFuelPct !== undefined) local.fuelSurchargePct = localFuelPct;
  if (localTaxPct !== undefined) local.taxRatePct = localTaxPct;
  if (Object.keys(local).length > 0) payload.local = local;

  const discounts: NonNullable<NewEstimatePricingDefaults["discounts"]> = {};
  const seniorPct = toNumber(form.seniorPct);
  const couponPct = toNumber(form.couponPct);
  if (seniorPct !== undefined) discounts.seniorPct = seniorPct;
  if (couponPct !== undefined) discounts.couponPct = couponPct;
  if (Object.keys(discounts).length > 0) payload.discounts = discounts;

  const liability: NonNullable<NewEstimatePricingDefaults["liability"]> = {
    type: form.liabilityType,
  };
  const valuationCharge = toNumber(form.valuationCharge);
  if (valuationCharge !== undefined) liability.valuationChargeCents = Math.round(valuationCharge * 100);
  payload.liability = liability;

  return payload;
}

function toInput(value: number | undefined) {
  if (value === undefined) return "";
  return String(value);
}

function toNumber(value: string) {
  const trimmed = value.trim();
  if (!trimmed) return undefined;
  const parsed = Number(trimmed);
  if (!Number.isFinite(parsed)) return undefined;
  return parsed;
}

function centsToDollars(value: number | undefined) {
  if (value === undefined) return undefined;
  return value / 100;
}
