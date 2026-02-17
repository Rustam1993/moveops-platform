import { api } from "@/lib/api";

export type EstimateChargesMode = "local" | "long_distance";
export type EstimateLiabilityType = "release" | "full_value";

export type EstimateChargesLineItem = {
  label: string;
  amountCents: number;
};

export type EstimateChargesLongDistanceInput = {
  ratePerCf?: number;
  fixedBaseAmountCents?: number;
};

export type EstimateChargesLocalInput = {
  trucks?: number;
  workers?: number;
  laborHours?: number;
  laborRateCents?: number;
  travelHours?: number;
  travelRateCents?: number;
};

export type EstimateChargesDiscountInput = {
  couponPct?: number;
  couponAmountCents?: number;
  seniorPct?: number;
  seniorAmountCents?: number;
};

export type EstimateChargesPackingInput = {
  packers?: number;
  hours?: number;
  rateCents?: number;
};

export type EstimateChargesLiabilityInput = {
  type?: EstimateLiabilityType;
  valuationChargeCents?: number;
};

export type EstimateChargesComputed = {
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

export type EstimateCharges = {
  estimateId: string;
  mode: EstimateChargesMode;
  calculationVersion: string;
  cfLbsRatio: number;
  fuelSurchargePct: number;
  longDistance: EstimateChargesLongDistanceInput;
  local: EstimateChargesLocalInput;
  otherLineItems: EstimateChargesLineItem[];
  discounts: EstimateChargesDiscountInput;
  packing: EstimateChargesPackingInput;
  liability: EstimateChargesLiabilityInput;
  taxRatePct: number;
  depositRequiredCents?: number;
  amountPaidCents: number;
  computed: EstimateChargesComputed;
  updatedAt: string;
};

export type EstimateChargesResponse = {
  charges: EstimateCharges;
  requestId: string;
};

export type ReplaceEstimateChargesRequest = {
  mode: EstimateChargesMode;
  cfLbsRatio?: number;
  fuelSurchargePct?: number;
  longDistance?: EstimateChargesLongDistanceInput;
  local?: EstimateChargesLocalInput;
  otherLineItems?: EstimateChargesLineItem[];
  discounts?: EstimateChargesDiscountInput;
  packing?: EstimateChargesPackingInput;
  liability?: EstimateChargesLiabilityInput;
  taxRatePct?: number;
  depositRequiredCents?: number;
  amountPaidCents?: number;
};

export async function getEstimateCharges(estimateId: string) {
  return api.request<EstimateChargesResponse>(`/estimates/${estimateId}/charges`);
}

export async function replaceEstimateCharges(estimateId: string, payload: ReplaceEstimateChargesRequest) {
  return api.request<EstimateChargesResponse>(`/estimates/${estimateId}/charges`, {
    method: "PUT",
    body: JSON.stringify(payload),
  });
}
