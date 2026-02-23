"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { AlertTriangle, Loader2, Plus, RefreshCw, Trash2 } from "lucide-react";
import { toast } from "sonner";

import { SaveStatusIndicator } from "@/components/estimates/save-status-indicator";
import { useEstimateWorkspace } from "@/components/estimates/estimate-workspace-context";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Separator } from "@/components/ui/separator";
import {
  getEstimateCharges,
  replaceEstimateCharges,
  type EstimateCharges,
} from "@/lib/charges-api";
import {
  calculateChargesPreview,
  emptyChargesForm,
  estimateChargesToFormValues,
  formatCurrency,
  toReplaceEstimateChargesRequest,
  validateChargesField,
  validateChargesForm,
  type ChargesFormErrors,
  type ChargesFormField,
  type ChargesFormValues,
} from "@/lib/charges-form";
import { formatCf } from "@/lib/inventory-catalog";
import { getApiErrorMessage } from "@/lib/phase2-api";
import { generateUUID } from "@/lib/uuid";

type SaveState = "idle" | "saving" | "saved" | "error";

export function EstimateChargesEditor() {
  const { estimate, setEstimate } = useEstimateWorkspace();

  const [values, setValues] = useState<ChargesFormValues>(emptyChargesForm);
  const [errors, setErrors] = useState<ChargesFormErrors>({});
  const [touched, setTouched] = useState<Partial<Record<ChargesFormField, true>>>({});
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [saveState, setSaveState] = useState<SaveState>("idle");
  const [saveMessage, setSaveMessage] = useState("Loading charges...");
  const [lastSavedFingerprint, setLastSavedFingerprint] = useState(JSON.stringify(emptyChargesForm));
  const [actionLoading, setActionLoading] = useState<"save" | null>(null);

  const valuesFingerprint = useMemo(() => JSON.stringify(values), [values]);
  const hasChanges = valuesFingerprint !== lastSavedFingerprint;
  const isSaving = saveState === "saving";
  const totalCf = estimate.totalVolumeCf ?? 0;
  const liveComputed = useMemo(() => calculateChargesPreview(values, totalCf), [totalCf, values]);
  const showZeroCfGuardrail =
    values.mode === "long_distance" &&
    !values.useFixedBaseAmount &&
    totalCf <= 0 &&
    values.longDistanceRatePerCf > 0;

  const loadCharges = useCallback(async () => {
    setLoading(true);
    setLoadError(null);
    try {
      const response = await getEstimateCharges(estimate.id);
      hydrateFromResponse(response.charges);
      setSaveState("saved");
      setSaveMessage("Saved");
    } catch (error) {
      setLoadError(getApiErrorMessage(error));
      setSaveState("error");
      setSaveMessage("Load failed");
    } finally {
      setLoading(false);
    }
  }, [estimate.id]);

  useEffect(() => {
    void loadCharges();
  }, [loadCharges]);

  const persist = useCallback(
    async (mode: "autosave" | "manual") => {
      if (loading || loadError || isSaving || !hasChanges) {
        return true;
      }

      if (mode === "manual") {
        const nextErrors = validateChargesForm(values);
        setErrors(nextErrors);
        if (Object.keys(nextErrors).length > 0) {
          setSaveState("error");
          setSaveMessage("Save failed");
          return false;
        }
      } else if (Object.keys(validateChargesForm(values)).length > 0) {
        return false;
      }

      setSaveState("saving");
      setSaveMessage("Saving...");
      if (mode === "manual") setActionLoading("save");

      try {
        const response = await replaceEstimateCharges(estimate.id, toReplaceEstimateChargesRequest(values));
        hydrateFromResponse(response.charges);
        setSaveState("saved");
        setSaveMessage("Saved");
        if (mode === "manual") {
          toast.success("Charges saved");
        }
        return true;
      } catch (error) {
        setSaveState("error");
        setSaveMessage("Save failed");
        if (mode === "manual") {
          toast.error(getApiErrorMessage(error));
        }
        return false;
      } finally {
        setActionLoading(null);
      }
    },
    [estimate.id, hasChanges, isSaving, loadError, loading, values],
  );

  useEffect(() => {
    if (!hasChanges || loading || loadError || isSaving) return;
    const timer = setTimeout(() => {
      void persist("autosave");
    }, 900);
    return () => clearTimeout(timer);
  }, [hasChanges, isSaving, loadError, loading, persist]);

  function hydrateFromResponse(charges: EstimateCharges) {
    const nextValues = estimateChargesToFormValues(charges);
    const nextFingerprint = JSON.stringify(nextValues);
    setValues(nextValues);
    setLastSavedFingerprint(nextFingerprint);
    setErrors({});
    setTouched({});
    setEstimate({
      ...estimate,
      locationType: charges.mode === "long_distance" ? "Long Distance" : "Local",
      estimatedTotalCents: charges.computed.totalCents,
      depositCents: charges.depositRequiredCents,
    });
  }

  function markDirty() {
    setSaveState("idle");
    setSaveMessage("Unsaved changes");
  }

  function onFieldChange(field: ChargesFormField, nextValue: number | string | boolean) {
    setValues((previous) => {
      const next = {
        ...previous,
        [field]: nextValue,
      } as ChargesFormValues;

      if (touched[field]) {
        setErrors((prev) => ({
          ...prev,
          [field]: validateChargesField(field, next),
        }));
      }

      return next;
    });
    markDirty();
  }

  function onFieldBlur(field: ChargesFormField) {
    setTouched((prev) => ({ ...prev, [field]: true }));
    setErrors((prev) => ({
      ...prev,
      [field]: validateChargesField(field, values),
    }));
  }

  function onNumericFieldChange(field: ChargesFormField, value: string) {
    const parsed = Number(value);
    onFieldChange(field, Number.isFinite(parsed) ? parsed : 0);
  }

  function addLineItem() {
    setValues((previous) => ({
      ...previous,
      otherLineItems: [
        ...previous.otherLineItems,
        {
          id: generateUUID(),
          label: "",
          amount: 0,
        },
      ],
    }));
    markDirty();
  }

  function removeLineItem(id: string) {
    setValues((previous) => ({
      ...previous,
      otherLineItems: previous.otherLineItems.filter((item) => item.id !== id),
    }));
    markDirty();
  }

  function updateLineItem(id: string, field: "label" | "amount", raw: string) {
    setValues((previous) => ({
      ...previous,
      otherLineItems: previous.otherLineItems.map((item) =>
        item.id === id
          ? {
              ...item,
              [field]: field === "amount" ? (Number.isFinite(Number(raw)) ? Number(raw) : 0) : raw,
            }
          : item,
      ),
    }));
    markDirty();
  }

  async function onSaveClick() {
    await persist("manual");
  }

  if (loading) {
    return (
      <Card className="border-border/70 bg-card/70">
        <CardContent className="flex h-48 items-center justify-center">
          <span className="inline-flex items-center gap-2 text-sm text-muted-foreground">
            <Loader2 className="h-4 w-4 animate-spin" />
            Loading charges...
          </span>
        </CardContent>
      </Card>
    );
  }

  if (loadError) {
    return (
      <Card className="border-border/70 bg-card/70">
        <CardHeader>
          <CardTitle>Charges unavailable</CardTitle>
          <CardDescription>{loadError}</CardDescription>
        </CardHeader>
        <CardContent>
          <Button variant="secondary" onClick={() => void loadCharges()}>
            <RefreshCw className="h-4 w-4" />
            Retry
          </Button>
        </CardContent>
      </Card>
    );
  }

  return (
    <div className="space-y-4">
      <Card className="border-border/70 bg-card/70">
        <CardContent className="flex flex-wrap items-center justify-between gap-3 p-4">
          <div>
            <p className="text-sm font-medium">Charges</p>
            <SaveStatusIndicator state={saveState} message={saveMessage} onRetry={onSaveClick} />
          </div>
          <Button onClick={onSaveClick} disabled={isSaving || !hasChanges}>
            {actionLoading === "save" ? (
              <span className="inline-flex items-center gap-2">
                <Loader2 className="h-4 w-4 animate-spin" />
                Saving...
              </span>
            ) : (
              "Save"
            )}
          </Button>
        </CardContent>
      </Card>

      <div className="space-y-4">
        <Card className="border-border/70 bg-card/70">
          <CardHeader className="pb-3">
            <CardTitle className="text-base">Mode and inventory totals</CardTitle>
            <CardDescription>Choose pricing mode and verify inventory-derived volume inputs.</CardDescription>
          </CardHeader>
          <CardContent className="grid gap-4 md:grid-cols-2">
            <div className="space-y-2">
              <Label>Service type</Label>
              <div className="flex flex-wrap gap-2">
                <Button
                  type="button"
                  variant={values.mode === "local" ? "default" : "outline"}
                  onClick={() => onFieldChange("mode", "local")}
                >
                  Local
                </Button>
                <Button
                  type="button"
                  variant={values.mode === "long_distance" ? "default" : "outline"}
                  onClick={() => onFieldChange("mode", "long_distance")}
                >
                  Long Distance
                </Button>
              </div>
            </div>
            <div className="grid gap-2 rounded-md border border-border/70 bg-muted/20 p-3 text-sm">
              <Metric label="Total CF" value={formatCf(liveComputed.totalCf)} />
              <Metric label="Total LBS" value={liveComputed.totalLbs.toFixed(2)} />
              <Metric label="Live total estimate" value={formatCurrency(liveComputed.totalCents)} />
            </div>
          </CardContent>
        </Card>

        <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_320px]">
          <div className="space-y-4">
            <Card className="border-border/70 bg-card/70">
              <CardHeader className="pb-3">
                <CardTitle className="text-base">Core charges</CardTitle>
                <CardDescription>Primary pricing fields for the selected service type.</CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                {values.mode === "long_distance" ? (
                  <div className="grid gap-4 md:grid-cols-3">
                    <FieldWithError label="Rate per CF ($)" error={errors.longDistanceRatePerCf}>
                      <Input
                        id="charges-rate-per-cf"
                        type="number"
                        min="0"
                        step="0.01"
                        value={values.longDistanceRatePerCf}
                        onChange={(event) => onNumericFieldChange("longDistanceRatePerCf", event.target.value)}
                        onBlur={() => onFieldBlur("longDistanceRatePerCf")}
                      />
                    </FieldWithError>
                    <FieldWithError label="CF-LBS Ratio" error={errors.cfLbsRatio}>
                      <Input
                        type="number"
                        min="0"
                        step="0.01"
                        value={values.cfLbsRatio}
                        onChange={(event) => onNumericFieldChange("cfLbsRatio", event.target.value)}
                        onBlur={() => onFieldBlur("cfLbsRatio")}
                      />
                    </FieldWithError>
                    <FieldWithError
                      label="Fuel surcharge (%)"
                      error={errors.fuelSurchargePct}
                      helper="Applied to base charges before discounts."
                    >
                      <Input
                        type="number"
                        min="0"
                        max="100"
                        step="0.01"
                        value={values.fuelSurchargePct}
                        onChange={(event) => onNumericFieldChange("fuelSurchargePct", event.target.value)}
                        onBlur={() => onFieldBlur("fuelSurchargePct")}
                      />
                    </FieldWithError>
                    <div className="md:col-span-3 rounded-md border border-border/70 p-3">
                      <label className="flex items-center gap-2 text-sm font-medium">
                        <input
                          type="checkbox"
                          checked={values.useFixedBaseAmount}
                          onChange={(event) => onFieldChange("useFixedBaseAmount", event.target.checked)}
                        />
                        Use fixed base amount
                      </label>
                      {values.useFixedBaseAmount ? (
                        <FieldWithError label="Fixed base amount ($)" error={errors.longDistanceFixedBaseAmount} className="mt-3">
                          <Input
                            type="number"
                            min="0"
                            step="0.01"
                            value={values.longDistanceFixedBaseAmount}
                            onChange={(event) => onNumericFieldChange("longDistanceFixedBaseAmount", event.target.value)}
                            onBlur={() => onFieldBlur("longDistanceFixedBaseAmount")}
                          />
                        </FieldWithError>
                      ) : null}
                    </div>
                  </div>
                ) : (
                  <div className="grid gap-4 md:grid-cols-4">
                    <FieldWithError label="# Trucks/Vans" error={errors.localTrucks}>
                      <Input
                        type="number"
                        min="0"
                        step="1"
                        value={values.localTrucks}
                        onChange={(event) => onNumericFieldChange("localTrucks", event.target.value)}
                        onBlur={() => onFieldBlur("localTrucks")}
                      />
                    </FieldWithError>
                    <FieldWithError label="# Workers" error={errors.localWorkers}>
                      <Input
                        type="number"
                        min="0"
                        step="1"
                        value={values.localWorkers}
                        onChange={(event) => onNumericFieldChange("localWorkers", event.target.value)}
                        onBlur={() => onFieldBlur("localWorkers")}
                      />
                    </FieldWithError>
                    <FieldWithError label="Labor hours" error={errors.localLaborHours}>
                      <Input
                        type="number"
                        min="0"
                        step="0.01"
                        value={values.localLaborHours}
                        onChange={(event) => onNumericFieldChange("localLaborHours", event.target.value)}
                        onBlur={() => onFieldBlur("localLaborHours")}
                      />
                    </FieldWithError>
                    <FieldWithError label="Labor rate ($/hr)" error={errors.localLaborRate}>
                      <Input
                        type="number"
                        min="0"
                        step="0.01"
                        value={values.localLaborRate}
                        onChange={(event) => onNumericFieldChange("localLaborRate", event.target.value)}
                        onBlur={() => onFieldBlur("localLaborRate")}
                      />
                    </FieldWithError>
                    <FieldWithError label="Travel hours" error={errors.localTravelHours}>
                      <Input
                        type="number"
                        min="0"
                        step="0.01"
                        value={values.localTravelHours}
                        onChange={(event) => onNumericFieldChange("localTravelHours", event.target.value)}
                        onBlur={() => onFieldBlur("localTravelHours")}
                      />
                    </FieldWithError>
                    <FieldWithError label="Travel rate ($/hr)" error={errors.localTravelRate}>
                      <Input
                        type="number"
                        min="0"
                        step="0.01"
                        value={values.localTravelRate}
                        onChange={(event) => onNumericFieldChange("localTravelRate", event.target.value)}
                        onBlur={() => onFieldBlur("localTravelRate")}
                      />
                    </FieldWithError>
                    <FieldWithError label="CF-LBS Ratio" error={errors.cfLbsRatio}>
                      <Input
                        type="number"
                        min="0"
                        step="0.01"
                        value={values.cfLbsRatio}
                        onChange={(event) => onNumericFieldChange("cfLbsRatio", event.target.value)}
                        onBlur={() => onFieldBlur("cfLbsRatio")}
                      />
                    </FieldWithError>
                    <FieldWithError
                      label="Fuel surcharge (%)"
                      error={errors.fuelSurchargePct}
                      helper="Applied to base charges before discounts."
                    >
                      <Input
                        type="number"
                        min="0"
                        max="100"
                        step="0.01"
                        value={values.fuelSurchargePct}
                        onChange={(event) => onNumericFieldChange("fuelSurchargePct", event.target.value)}
                        onBlur={() => onFieldBlur("fuelSurchargePct")}
                      />
                    </FieldWithError>
                  </div>
                )}
              </CardContent>
            </Card>

            <details className="group rounded-md border border-border/70 bg-card/70">
              <summary className="cursor-pointer list-none px-4 py-3 text-sm font-medium">
                Advanced adjustments and protections
              </summary>
              <Separator />
              <div className="space-y-6 p-4">
                <section className="space-y-3">
                  <div className="flex items-center justify-between">
                    <h3 className="text-sm font-semibold">Other line items</h3>
                    <Button type="button" variant="outline" size="sm" onClick={addLineItem}>
                      <Plus className="h-4 w-4" />
                      Add line
                    </Button>
                  </div>
                  {values.otherLineItems.length === 0 ? (
                    <p className="text-xs text-muted-foreground">No additional line items.</p>
                  ) : (
                    <div className="space-y-2">
                      {values.otherLineItems.map((line) => (
                        <div key={line.id} className="grid gap-2 md:grid-cols-[minmax(0,1fr)_180px_auto]">
                          <Input
                            placeholder="Label"
                            value={line.label}
                            onChange={(event) => updateLineItem(line.id, "label", event.target.value)}
                          />
                          <Input
                            type="number"
                            step="0.01"
                            value={line.amount}
                            onChange={(event) => updateLineItem(line.id, "amount", event.target.value)}
                          />
                          <Button type="button" variant="outline" onClick={() => removeLineItem(line.id)} aria-label="Remove line item">
                            <Trash2 className="h-4 w-4" />
                          </Button>
                        </div>
                      ))}
                    </div>
                  )}
                  {errors.otherLineItems ? <p className="text-xs text-destructive">{errors.otherLineItems}</p> : null}
                </section>

                <section className="grid gap-4 md:grid-cols-4">
                  <FieldWithError label="Coupon (%)" error={errors.couponPct} helper="Percent discount on subtotal.">
                    <Input
                      type="number"
                      min="0"
                      max="100"
                      step="0.01"
                      value={values.couponPct}
                      onChange={(event) => onNumericFieldChange("couponPct", event.target.value)}
                      onBlur={() => onFieldBlur("couponPct")}
                    />
                  </FieldWithError>
                  <FieldWithError label="Coupon amount ($)" error={errors.couponAmount}>
                    <Input
                      type="number"
                      min="0"
                      step="0.01"
                      value={values.couponAmount}
                      onChange={(event) => onNumericFieldChange("couponAmount", event.target.value)}
                      onBlur={() => onFieldBlur("couponAmount")}
                    />
                  </FieldWithError>
                  <FieldWithError label="Senior discount (%)" error={errors.seniorPct}>
                    <Input
                      type="number"
                      min="0"
                      max="100"
                      step="0.01"
                      value={values.seniorPct}
                      onChange={(event) => onNumericFieldChange("seniorPct", event.target.value)}
                      onBlur={() => onFieldBlur("seniorPct")}
                    />
                  </FieldWithError>
                  <FieldWithError label="Senior amount ($)" error={errors.seniorAmount}>
                    <Input
                      type="number"
                      min="0"
                      step="0.01"
                      value={values.seniorAmount}
                      onChange={(event) => onNumericFieldChange("seniorAmount", event.target.value)}
                      onBlur={() => onFieldBlur("seniorAmount")}
                    />
                  </FieldWithError>
                </section>

                <section className="grid gap-4 md:grid-cols-4">
                  <FieldWithError label="Packers" error={errors.packingPackers}>
                    <Input
                      type="number"
                      min="0"
                      step="1"
                      value={values.packingPackers}
                      onChange={(event) => onNumericFieldChange("packingPackers", event.target.value)}
                      onBlur={() => onFieldBlur("packingPackers")}
                    />
                  </FieldWithError>
                  <FieldWithError label="Packing hours" error={errors.packingHours}>
                    <Input
                      type="number"
                      min="0"
                      step="0.01"
                      value={values.packingHours}
                      onChange={(event) => onNumericFieldChange("packingHours", event.target.value)}
                      onBlur={() => onFieldBlur("packingHours")}
                    />
                  </FieldWithError>
                  <FieldWithError label="Packing rate ($/hr)" error={errors.packingRate}>
                    <Input
                      type="number"
                      min="0"
                      step="0.01"
                      value={values.packingRate}
                      onChange={(event) => onNumericFieldChange("packingRate", event.target.value)}
                      onBlur={() => onFieldBlur("packingRate")}
                    />
                  </FieldWithError>
                  <FieldWithError
                    label="Liability type"
                    error={errors.liabilityType}
                    helper="Release value is default; full value may add valuation charges."
                  >
                    <select
                      className="flex h-10 w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm shadow-sm"
                      value={values.liabilityType}
                      onChange={(event) => onFieldChange("liabilityType", event.target.value)}
                      onBlur={() => onFieldBlur("liabilityType")}
                    >
                      <option value="release">Release value</option>
                      <option value="full_value">Full value protection</option>
                    </select>
                  </FieldWithError>
                  <FieldWithError label="Liability charge ($)" error={errors.liabilityValuationCharge}>
                    <Input
                      type="number"
                      min="0"
                      step="0.01"
                      value={values.liabilityValuationCharge}
                      onChange={(event) => onNumericFieldChange("liabilityValuationCharge", event.target.value)}
                      onBlur={() => onFieldBlur("liabilityValuationCharge")}
                    />
                  </FieldWithError>
                  <FieldWithError label="Tax rate (%)" error={errors.taxRatePct}>
                    <Input
                      type="number"
                      min="0"
                      max="100"
                      step="0.01"
                      value={values.taxRatePct}
                      onChange={(event) => onNumericFieldChange("taxRatePct", event.target.value)}
                      onBlur={() => onFieldBlur("taxRatePct")}
                    />
                  </FieldWithError>
                  <FieldWithError label="Deposit required ($)" error={errors.depositRequired}>
                    <Input
                      type="number"
                      min="0"
                      step="0.01"
                      value={values.depositRequired}
                      onChange={(event) => onNumericFieldChange("depositRequired", event.target.value)}
                      onBlur={() => onFieldBlur("depositRequired")}
                    />
                  </FieldWithError>
                  <FieldWithError label="Amount paid ($)" error={errors.amountPaid}>
                    <Input
                      type="number"
                      min="0"
                      step="0.01"
                      value={values.amountPaid}
                      onChange={(event) => onNumericFieldChange("amountPaid", event.target.value)}
                      onBlur={() => onFieldBlur("amountPaid")}
                    />
                  </FieldWithError>
                </section>
              </div>
            </details>
          </div>

          <div className="space-y-4 xl:sticky xl:top-24 xl:self-start">
            {showZeroCfGuardrail ? (
              <Card className="border-amber-500/50 bg-amber-500/10">
                <CardContent className="flex items-start gap-2 p-3 text-sm text-amber-800">
                  <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
                  <p>
                    Total CF is currently 0. Long-distance rate-per-CF pricing may underquote unless inventory is entered.
                  </p>
                </CardContent>
              </Card>
            ) : null}

            <Card className="border-border/70 bg-card/70">
              <CardHeader className="pb-3">
                <CardTitle className="text-base">Summary</CardTitle>
                <CardDescription>Live totals use the same calculation model as server-side persistence.</CardDescription>
              </CardHeader>
              <CardContent className="grid gap-2 text-sm">
                <Metric label="Base" value={formatCurrency(liveComputed.baseCents)} />
                <Metric label="Fuel surcharge" value={formatCurrency(liveComputed.fuelSurchargeCents)} />
                <Metric label="Other line items" value={formatCurrency(liveComputed.otherItemsTotalCents)} />
                <Metric label="Packing" value={formatCurrency(liveComputed.packingTotalCents)} />
                <Metric label="Liability" value={formatCurrency(liveComputed.liabilityTotalCents)} />
                <Separator className="my-1" />
                <Metric label="Subtotal" value={formatCurrency(liveComputed.subtotalCents)} />
                <Metric label="Discounts" value={formatCurrency(liveComputed.discountsTotalCents)} />
                <Metric label="Tax" value={formatCurrency(liveComputed.taxTotalCents)} />
                <Metric
                  label="Total estimate"
                  value={formatCurrency(liveComputed.totalCents)}
                  strong
                  dataTestId="charges-total-estimate"
                />
              </CardContent>
            </Card>
          </div>
        </div>
      </div>
    </div>
  );
}

function FieldWithError({
  label,
  error,
  helper,
  children,
  className,
}: {
  label: string;
  error?: string;
  helper?: string;
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <div className={className ? `space-y-1 ${className}` : "space-y-1"}>
      <Label className="space-y-1">
        <span>{label}</span>
        {children}
      </Label>
      {helper ? <p className="text-xs text-muted-foreground">{helper}</p> : null}
      {error ? <p className="text-xs text-destructive">{error}</p> : null}
    </div>
  );
}

function Metric({
  label,
  value,
  strong = false,
  dataTestId,
}: {
  label: string;
  value: string;
  strong?: boolean;
  dataTestId?: string;
}) {
  return (
    <div className="flex items-center justify-between gap-4">
      <span className="text-muted-foreground">{label}</span>
      <span className={strong ? "text-base font-semibold" : "font-medium"} data-testid={dataTestId}>
        {value}
      </span>
    </div>
  );
}
