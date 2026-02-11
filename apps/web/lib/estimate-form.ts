import type { components } from "@moveops/client";

export type EstimateFormValues = {
  customerName: string;
  primaryPhone: string;
  secondaryPhone: string;
  email: string;
  originAddressLine1: string;
  originCity: string;
  originState: string;
  originPostalCode: string;
  destinationAddressLine1: string;
  destinationCity: string;
  destinationState: string;
  destinationPostalCode: string;
  moveDate: string;
  pickupTime: string;
  leadSource: string;
  moveSize: string;
  locationType: string;
  estimatedTotal: string;
  deposit: string;
  notes: string;
};

export type EstimateFormErrors = Partial<Record<keyof EstimateFormValues, string>>;

export const emptyEstimateForm: EstimateFormValues = {
  customerName: "",
  primaryPhone: "",
  secondaryPhone: "",
  email: "",
  originAddressLine1: "",
  originCity: "",
  originState: "",
  originPostalCode: "",
  destinationAddressLine1: "",
  destinationCity: "",
  destinationState: "",
  destinationPostalCode: "",
  moveDate: "",
  pickupTime: "",
  leadSource: "",
  moveSize: "",
  locationType: "",
  estimatedTotal: "",
  deposit: "",
  notes: "",
};

export function validateEstimateForm(values: EstimateFormValues): EstimateFormErrors {
  const errors: EstimateFormErrors = {};

  const requiredFields: Array<[keyof EstimateFormValues, string]> = [
    ["customerName", "Customer name is required"],
    ["primaryPhone", "Primary phone is required"],
    ["email", "Email is required"],
    ["originAddressLine1", "Origin address is required"],
    ["originCity", "Origin city is required"],
    ["originState", "Origin state is required"],
    ["originPostalCode", "Origin postal code is required"],
    ["destinationAddressLine1", "Destination address is required"],
    ["destinationCity", "Destination city is required"],
    ["destinationState", "Destination state is required"],
    ["destinationPostalCode", "Destination postal code is required"],
    ["moveDate", "Move date is required"],
    ["leadSource", "Lead source is required"],
  ];

  for (const [field, message] of requiredFields) {
    if (!values[field].trim()) {
      errors[field] = message;
    }
  }

  if (values.email.trim() && !/^\S+@\S+\.\S+$/.test(values.email.trim())) {
    errors.email = "Enter a valid email address";
  }

  if (values.estimatedTotal.trim() && parseCurrencyToCents(values.estimatedTotal) === undefined) {
    errors.estimatedTotal = "Enter a valid amount"
  }

  if (values.deposit.trim() && parseCurrencyToCents(values.deposit) === undefined) {
    errors.deposit = "Enter a valid amount"
  }

  return errors;
}

export function toCreateEstimateRequest(values: EstimateFormValues): components["schemas"]["CreateEstimateRequest"] {
  const payload: components["schemas"]["CreateEstimateRequest"] = {
    customerName: values.customerName.trim(),
    primaryPhone: values.primaryPhone.trim(),
    email: values.email.trim(),
    originAddressLine1: values.originAddressLine1.trim(),
    originCity: values.originCity.trim(),
    originState: values.originState.trim(),
    originPostalCode: values.originPostalCode.trim(),
    destinationAddressLine1: values.destinationAddressLine1.trim(),
    destinationCity: values.destinationCity.trim(),
    destinationState: values.destinationState.trim(),
    destinationPostalCode: values.destinationPostalCode.trim(),
    moveDate: values.moveDate,
    leadSource: values.leadSource.trim(),
  };

  const estimatedTotalCents = parseCurrencyToCents(values.estimatedTotal);
  if (estimatedTotalCents !== undefined) payload.estimatedTotalCents = estimatedTotalCents;

  const depositCents = parseCurrencyToCents(values.deposit);
  if (depositCents !== undefined) payload.depositCents = depositCents;

  if (values.secondaryPhone.trim()) payload.secondaryPhone = values.secondaryPhone.trim();
  if (values.pickupTime.trim()) payload.pickupTime = values.pickupTime.trim();
  if (values.moveSize.trim()) payload.moveSize = values.moveSize.trim();
  if (values.locationType.trim()) payload.locationType = values.locationType.trim();
  if (values.notes.trim()) payload.notes = values.notes.trim();

  return payload;
}

export function toUpdateEstimateRequest(values: EstimateFormValues): components["schemas"]["UpdateEstimateRequest"] {
  const payload: components["schemas"]["UpdateEstimateRequest"] = {
    customerName: values.customerName.trim(),
    primaryPhone: values.primaryPhone.trim(),
    email: values.email.trim(),
    originAddressLine1: values.originAddressLine1.trim(),
    originCity: values.originCity.trim(),
    originState: values.originState.trim(),
    originPostalCode: values.originPostalCode.trim(),
    destinationAddressLine1: values.destinationAddressLine1.trim(),
    destinationCity: values.destinationCity.trim(),
    destinationState: values.destinationState.trim(),
    destinationPostalCode: values.destinationPostalCode.trim(),
    moveDate: values.moveDate,
    leadSource: values.leadSource.trim(),
  };

  const estimatedTotalCents = parseCurrencyToCents(values.estimatedTotal);
  if (estimatedTotalCents !== undefined) payload.estimatedTotalCents = estimatedTotalCents;

  const depositCents = parseCurrencyToCents(values.deposit);
  if (depositCents !== undefined) payload.depositCents = depositCents;

  if (values.secondaryPhone.trim()) payload.secondaryPhone = values.secondaryPhone.trim();
  if (values.pickupTime.trim()) payload.pickupTime = values.pickupTime.trim();
  if (values.moveSize.trim()) payload.moveSize = values.moveSize.trim();
  if (values.locationType.trim()) payload.locationType = values.locationType.trim();
  if (values.notes.trim()) payload.notes = values.notes.trim();

  return payload;
}

export function estimateToFormValues(estimate: components["schemas"]["Estimate"]): EstimateFormValues {
  return {
    customerName: estimate.customerName,
    primaryPhone: estimate.primaryPhone,
    secondaryPhone: estimate.secondaryPhone ?? "",
    email: estimate.email,
    originAddressLine1: estimate.originAddressLine1,
    originCity: estimate.originCity,
    originState: estimate.originState,
    originPostalCode: estimate.originPostalCode,
    destinationAddressLine1: estimate.destinationAddressLine1,
    destinationCity: estimate.destinationCity,
    destinationState: estimate.destinationState,
    destinationPostalCode: estimate.destinationPostalCode,
    moveDate: estimate.moveDate,
    pickupTime: estimate.pickupTime ?? "",
    leadSource: estimate.leadSource,
    moveSize: estimate.moveSize ?? "",
    locationType: estimate.locationType ?? "",
    estimatedTotal: centsToCurrency(estimate.estimatedTotalCents),
    deposit: centsToCurrency(estimate.depositCents),
    notes: estimate.notes ?? "",
  };
}

function parseCurrencyToCents(raw: string): number | undefined {
  const normalized = raw.replace(/[$,]/g, "").trim();
  if (!normalized) return undefined;

  const amount = Number(normalized);
  if (!Number.isFinite(amount) || amount < 0) return undefined;

  return Math.round(amount * 100);
}

function centsToCurrency(cents?: number): string {
  if (cents === undefined) return "";
  return (cents / 100).toFixed(2);
}
