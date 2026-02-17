import { z } from "zod";

import type { EstimateCharges, ReplaceEstimateChargesRequest } from "@/lib/charges-api";

export type ChargesMode = "local" | "long_distance";
export type LiabilityType = "release" | "full_value";

export type ChargesLineItemForm = {
  id: string;
  label: string;
  amount: number;
};

export type ChargesFormValues = {
  mode: ChargesMode;
  cfLbsRatio: number;
  fuelSurchargePct: number;
  longDistanceRatePerCf: number;
  useFixedBaseAmount: boolean;
  longDistanceFixedBaseAmount: number;
  localTrucks: number;
  localWorkers: number;
  localLaborHours: number;
  localLaborRate: number;
  localTravelHours: number;
  localTravelRate: number;
  otherLineItems: ChargesLineItemForm[];
  couponPct: number;
  couponAmount: number;
  seniorPct: number;
  seniorAmount: number;
  packingPackers: number;
  packingHours: number;
  packingRate: number;
  liabilityType: LiabilityType;
  liabilityValuationCharge: number;
  taxRatePct: number;
  depositRequired: number;
  amountPaid: number;
};

export type ChargesFormField = keyof ChargesFormValues;
export type ChargesFormErrors = Partial<Record<ChargesFormField, string>>;

const numberNonNegative = z.number().finite().min(0, "Must be zero or greater");
const percentage = z.number().finite().min(0, "Must be zero or greater").max(100, "Must be 100 or less");
const lineItemSchema = z.object({
  id: z.string().min(1),
  label: z.string().trim().max(120, "Label must be 120 characters or less"),
  amount: z.number().finite(),
});

const baseChargesFormSchema = z.object({
  mode: z.enum(["local", "long_distance"]),
  cfLbsRatio: numberNonNegative,
  fuelSurchargePct: percentage,
  longDistanceRatePerCf: numberNonNegative,
  useFixedBaseAmount: z.boolean(),
  longDistanceFixedBaseAmount: numberNonNegative,
  localTrucks: z.number().int("Use whole numbers").min(0, "Must be zero or greater"),
  localWorkers: z.number().int("Use whole numbers").min(0, "Must be zero or greater"),
  localLaborHours: numberNonNegative,
  localLaborRate: numberNonNegative,
  localTravelHours: numberNonNegative,
  localTravelRate: numberNonNegative,
  otherLineItems: z.array(lineItemSchema).max(25, "Use 25 or fewer line items"),
  couponPct: percentage,
  couponAmount: numberNonNegative,
  seniorPct: percentage,
  seniorAmount: numberNonNegative,
  packingPackers: z.number().int("Use whole numbers").min(0, "Must be zero or greater"),
  packingHours: numberNonNegative,
  packingRate: numberNonNegative,
  liabilityType: z.enum(["release", "full_value"]),
  liabilityValuationCharge: numberNonNegative,
  taxRatePct: percentage,
  depositRequired: numberNonNegative,
  amountPaid: numberNonNegative,
});

export const chargesFormSchema = baseChargesFormSchema.refine(
  (values) => values.otherLineItems.every((item) => item.amount === 0 || item.label.trim().length > 0),
  { message: "Line item label is required when amount is not zero", path: ["otherLineItems"] },
);

export const emptyChargesForm: ChargesFormValues = {
  mode: "local",
  cfLbsRatio: 7,
  fuelSurchargePct: 0,
  longDistanceRatePerCf: 0,
  useFixedBaseAmount: false,
  longDistanceFixedBaseAmount: 0,
  localTrucks: 0,
  localWorkers: 0,
  localLaborHours: 0,
  localLaborRate: 0,
  localTravelHours: 0,
  localTravelRate: 0,
  otherLineItems: [],
  couponPct: 0,
  couponAmount: 0,
  seniorPct: 0,
  seniorAmount: 0,
  packingPackers: 0,
  packingHours: 0,
  packingRate: 0,
  liabilityType: "release",
  liabilityValuationCharge: 0,
  taxRatePct: 0,
  depositRequired: 0,
  amountPaid: 0,
};

export type ChargesComputedPreview = {
  baseCents: number;
  fuelSurchargeCents: number;
  otherItemsTotalCents: number;
  packingTotalCents: number;
  liabilityTotalCents: number;
  subtotalCents: number;
  discountsTotalCents: number;
  taxTotalCents: number;
  totalCents: number;
  totalCf: number;
  totalLbs: number;
};

export function estimateChargesToFormValues(charges: EstimateCharges): ChargesFormValues {
  return {
    mode: charges.mode,
    cfLbsRatio: round2(charges.cfLbsRatio),
    fuelSurchargePct: round2(charges.fuelSurchargePct),
    longDistanceRatePerCf: round2(charges.longDistance.ratePerCf ?? 0),
    useFixedBaseAmount: charges.longDistance.fixedBaseAmountCents !== undefined,
    longDistanceFixedBaseAmount: centsToDollars(charges.longDistance.fixedBaseAmountCents ?? 0),
    localTrucks: charges.local.trucks ?? 0,
    localWorkers: charges.local.workers ?? 0,
    localLaborHours: round2(charges.local.laborHours ?? 0),
    localLaborRate: centsToDollars(charges.local.laborRateCents ?? 0),
    localTravelHours: round2(charges.local.travelHours ?? 0),
    localTravelRate: centsToDollars(charges.local.travelRateCents ?? 0),
    otherLineItems: (charges.otherLineItems ?? []).map((line, index) => ({
      id: `line-${index + 1}`,
      label: line.label,
      amount: centsToDollars(line.amountCents),
    })),
    couponPct: round2(charges.discounts.couponPct ?? 0),
    couponAmount: centsToDollars(charges.discounts.couponAmountCents ?? 0),
    seniorPct: round2(charges.discounts.seniorPct ?? 0),
    seniorAmount: centsToDollars(charges.discounts.seniorAmountCents ?? 0),
    packingPackers: charges.packing.packers ?? 0,
    packingHours: round2(charges.packing.hours ?? 0),
    packingRate: centsToDollars(charges.packing.rateCents ?? 0),
    liabilityType: (charges.liability.type ?? "release") as LiabilityType,
    liabilityValuationCharge: centsToDollars(charges.liability.valuationChargeCents ?? 0),
    taxRatePct: round2(charges.taxRatePct ?? 0),
    depositRequired: centsToDollars(charges.depositRequiredCents ?? 0),
    amountPaid: centsToDollars(charges.amountPaidCents ?? 0),
  };
}

export function toReplaceEstimateChargesRequest(values: ChargesFormValues): ReplaceEstimateChargesRequest {
  return {
    mode: values.mode,
    cfLbsRatio: round2(values.cfLbsRatio),
    fuelSurchargePct: round2(values.fuelSurchargePct),
    longDistance: {
      ratePerCf: round2(values.longDistanceRatePerCf),
      fixedBaseAmountCents: values.useFixedBaseAmount ? dollarsToCents(values.longDistanceFixedBaseAmount) : undefined,
    },
    local: {
      trucks: Math.max(0, Math.floor(values.localTrucks)),
      workers: Math.max(0, Math.floor(values.localWorkers)),
      laborHours: round2(values.localLaborHours),
      laborRateCents: dollarsToCents(values.localLaborRate),
      travelHours: round2(values.localTravelHours),
      travelRateCents: dollarsToCents(values.localTravelRate),
    },
    otherLineItems: values.otherLineItems
      .map((item) => ({
        label: item.label.trim(),
        amountCents: dollarsToCents(item.amount),
      }))
      .filter((item) => item.label.length > 0 || item.amountCents !== 0),
    discounts: {
      couponPct: round2(values.couponPct),
      couponAmountCents: dollarsToCents(values.couponAmount),
      seniorPct: round2(values.seniorPct),
      seniorAmountCents: dollarsToCents(values.seniorAmount),
    },
    packing: {
      packers: Math.max(0, Math.floor(values.packingPackers)),
      hours: round2(values.packingHours),
      rateCents: dollarsToCents(values.packingRate),
    },
    liability: {
      type: values.liabilityType,
      valuationChargeCents: dollarsToCents(values.liabilityValuationCharge),
    },
    taxRatePct: round2(values.taxRatePct),
    depositRequiredCents: values.depositRequired > 0 ? dollarsToCents(values.depositRequired) : undefined,
    amountPaidCents: dollarsToCents(values.amountPaid),
  };
}

export function calculateChargesPreview(values: ChargesFormValues, totalCf: number): ChargesComputedPreview {
  const normalizedTotalCf = round2(Math.max(0, totalCf));
  const ratio = round2(Math.max(0, values.cfLbsRatio));
  const totalLbs = round2(normalizedTotalCf * ratio);

  let baseCents = 0;
  if (values.mode === "long_distance") {
    baseCents = values.useFixedBaseAmount
      ? dollarsToCents(values.longDistanceFixedBaseAmount)
      : Math.round(normalizedTotalCf * round2(values.longDistanceRatePerCf) * 100);
  } else {
    const workersMultiplier = values.localWorkers > 0 ? values.localWorkers : 1;
    const laborCents = values.localLaborHours * workersMultiplier * dollarsToCents(values.localLaborRate);
    const travelCents = values.localTravelHours * dollarsToCents(values.localTravelRate);
    baseCents = Math.round(laborCents + travelCents);
  }

  const fuelSurchargeCents = Math.round((baseCents * values.fuelSurchargePct) / 100);
  const otherItemsTotalCents = values.otherLineItems.reduce((sum, item) => sum + dollarsToCents(item.amount), 0);
  const packingTotalCents = Math.round(values.packingPackers * values.packingHours * dollarsToCents(values.packingRate));
  const liabilityTotalCents = dollarsToCents(values.liabilityValuationCharge);

  const subtotalCents = baseCents + fuelSurchargeCents + otherItemsTotalCents + packingTotalCents + liabilityTotalCents;
  const discountBase = Math.max(subtotalCents, 0);
  let discountsTotalCents =
    Math.round((discountBase * values.couponPct) / 100) +
    dollarsToCents(values.couponAmount) +
    Math.round((discountBase * values.seniorPct) / 100) +
    dollarsToCents(values.seniorAmount);
  discountsTotalCents = Math.min(Math.max(discountsTotalCents, 0), discountBase);

  const taxableCents = Math.max(subtotalCents - discountsTotalCents, 0);
  const taxTotalCents = Math.round((taxableCents * values.taxRatePct) / 100);
  const totalCents = Math.max(subtotalCents - discountsTotalCents + taxTotalCents, 0);

  return {
    baseCents,
    fuelSurchargeCents,
    otherItemsTotalCents,
    packingTotalCents,
    liabilityTotalCents,
    subtotalCents,
    discountsTotalCents,
    taxTotalCents,
    totalCents,
    totalCf: normalizedTotalCf,
    totalLbs,
  };
}

export function validateChargesForm(values: ChargesFormValues): ChargesFormErrors {
  const parsed = chargesFormSchema.safeParse(values);
  if (parsed.success) return {};
  return parsed.error.flatten().fieldErrors as ChargesFormErrors;
}

export function validateChargesField(field: ChargesFormField, values: ChargesFormValues) {
  const fieldSchema = baseChargesFormSchema.shape[field];
  const parsed = fieldSchema.safeParse(values[field]);
  if (parsed.success) return undefined;
  return parsed.error.issues[0]?.message;
}

export function dollarsToCents(value: number | null | undefined) {
  if (value === null || value === undefined || Number.isNaN(value)) return 0;
  return Math.round(value * 100);
}

export function centsToDollars(value: number | null | undefined) {
  if (value === null || value === undefined || Number.isNaN(value)) return 0;
  return round2(value / 100);
}

export function round2(value: number) {
  return Math.round(value * 100) / 100;
}

export function formatCurrency(cents: number | null | undefined) {
  if (cents === null || cents === undefined || Number.isNaN(cents)) return "—";
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: "USD",
  }).format(cents / 100);
}
