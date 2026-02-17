import { z } from "zod";
import type { components } from "@moveops/client";

export type ServiceType = "local" | "long_distance";

export type EstimateFormValues = {
  firstName: string;
  lastName: string;
  email: string;
  phone: string;
  movingFromStreet: string;
  movingFromCity: string;
  movingFromState: string;
  movingFromZip: string;
  movingToStreet: string;
  movingToCity: string;
  movingToState: string;
  movingToZip: string;
  moveDate: string;
  preferredTimeWindow: string;
  serviceType: ServiceType;
  leadSource: string;
  notes: string;
};

export type EstimateFormField = keyof EstimateFormValues;
export type EstimateFormErrors = Partial<Record<EstimateFormField, string>>;

const zipRegex = /^\d{5}(?:-\d{4})?$/;
const phoneRegex = /^\+?[0-9().\-\s]{7,20}$/;

export const estimateFormSchema = z.object({
  firstName: z.string().trim().min(1, "First name is required"),
  lastName: z.string().trim().min(1, "Last name is required"),
  email: z.string().trim().email("Enter a valid email address"),
  phone: z.string().trim().regex(phoneRegex, "Enter a valid phone number"),
  movingFromStreet: z.string().trim().min(1, "Moving from street is required"),
  movingFromCity: z.string().trim().min(1, "Moving from city is required"),
  movingFromState: z.string().trim().min(2, "State is required"),
  movingFromZip: z.string().trim().regex(zipRegex, "Enter a valid ZIP code"),
  movingToStreet: z.string().trim().min(1, "Moving to street is required"),
  movingToCity: z.string().trim().min(1, "Moving to city is required"),
  movingToState: z.string().trim().min(2, "State is required"),
  movingToZip: z.string().trim().regex(zipRegex, "Enter a valid ZIP code"),
  moveDate: z.string().trim().min(1, "Move date is required"),
  preferredTimeWindow: z.string().trim().optional().or(z.literal("")),
  serviceType: z.enum(["local", "long_distance"], {
    errorMap: () => ({ message: "Service type is required" }),
  }),
  leadSource: z.string().trim().min(1, "Referral source is required"),
  notes: z.string().trim().max(2000, "Notes must be 2000 characters or less").optional().or(z.literal("")),
});

export const serviceTypeOptions: Array<{ label: string; value: ServiceType }> = [
  { label: "Local", value: "local" },
  { label: "Long Distance", value: "long_distance" },
];

export const emptyEstimateForm: EstimateFormValues = {
  firstName: "",
  lastName: "",
  email: "",
  phone: "",
  movingFromStreet: "",
  movingFromCity: "",
  movingFromState: "",
  movingFromZip: "",
  movingToStreet: "",
  movingToCity: "",
  movingToState: "",
  movingToZip: "",
  moveDate: "",
  preferredTimeWindow: "",
  serviceType: "local",
  leadSource: "Website",
  notes: "",
};

export function validateEstimateForm(values: EstimateFormValues): EstimateFormErrors {
  const parsed = estimateFormSchema.safeParse(values);
  if (parsed.success) return {};
  return parsed.error.flatten().fieldErrors as EstimateFormErrors;
}

export function validateEstimateField(field: EstimateFormField, values: EstimateFormValues) {
  const fieldSchema = estimateFormSchema.shape[field];
  const parsed = fieldSchema.safeParse(values[field]);
  if (parsed.success) return undefined;
  return parsed.error.issues[0]?.message;
}

export function toCreateEstimateRequest(values: EstimateFormValues): components["schemas"]["CreateEstimateRequest"] {
  return {
    customerName: buildCustomerName(values.firstName, values.lastName),
    primaryPhone: values.phone.trim(),
    email: values.email.trim(),
    originAddressLine1: values.movingFromStreet.trim(),
    originCity: values.movingFromCity.trim(),
    originState: values.movingFromState.trim(),
    originPostalCode: values.movingFromZip.trim(),
    destinationAddressLine1: values.movingToStreet.trim(),
    destinationCity: values.movingToCity.trim(),
    destinationState: values.movingToState.trim(),
    destinationPostalCode: values.movingToZip.trim(),
    moveDate: values.moveDate,
    pickupTime: values.preferredTimeWindow.trim() || undefined,
    leadSource: values.leadSource.trim(),
    locationType: toLocationType(values.serviceType),
    notes: values.notes.trim() || undefined,
  };
}

export function toUpdateEstimateRequest(values: EstimateFormValues): components["schemas"]["UpdateEstimateRequest"] {
  return {
    customerName: buildCustomerName(values.firstName, values.lastName),
    primaryPhone: values.phone.trim(),
    email: values.email.trim(),
    originAddressLine1: values.movingFromStreet.trim(),
    originCity: values.movingFromCity.trim(),
    originState: values.movingFromState.trim(),
    originPostalCode: values.movingFromZip.trim(),
    destinationAddressLine1: values.movingToStreet.trim(),
    destinationCity: values.movingToCity.trim(),
    destinationState: values.movingToState.trim(),
    destinationPostalCode: values.movingToZip.trim(),
    moveDate: values.moveDate,
    pickupTime: values.preferredTimeWindow.trim() || undefined,
    leadSource: values.leadSource.trim(),
    locationType: toLocationType(values.serviceType),
    notes: values.notes.trim() || undefined,
  };
}

export function estimateToFormValues(estimate: components["schemas"]["Estimate"]): EstimateFormValues {
  const [firstName, ...restName] = estimate.customerName.trim().split(/\s+/);
  const lastName = restName.join(" ");

  return {
    firstName: firstName || "",
    lastName: lastName || "",
    email: estimate.email,
    phone: estimate.primaryPhone,
    movingFromStreet: estimate.originAddressLine1,
    movingFromCity: estimate.originCity,
    movingFromState: estimate.originState,
    movingFromZip: estimate.originPostalCode,
    movingToStreet: estimate.destinationAddressLine1,
    movingToCity: estimate.destinationCity,
    movingToState: estimate.destinationState,
    movingToZip: estimate.destinationPostalCode,
    moveDate: estimate.moveDate,
    preferredTimeWindow: estimate.pickupTime ?? "",
    serviceType: fromLocationType(estimate.locationType),
    leadSource: estimate.leadSource || "Website",
    notes: estimate.notes ?? "",
  };
}

function buildCustomerName(firstName: string, lastName: string) {
  return `${firstName.trim()} ${lastName.trim()}`.trim();
}

function toLocationType(serviceType: ServiceType) {
  return serviceType === "long_distance" ? "Long Distance" : "Local";
}

function fromLocationType(locationType?: string): ServiceType {
  if (!locationType) return "local";
  return locationType.toLowerCase().includes("long") ? "long_distance" : "local";
}
