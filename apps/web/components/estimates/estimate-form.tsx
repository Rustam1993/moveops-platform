import { type ChangeEvent } from "react";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import type { EstimateFormErrors, EstimateFormValues } from "@/lib/estimate-form";
import { cn } from "@/lib/utils";

type Props = {
  values: EstimateFormValues;
  errors: EstimateFormErrors;
  disabled?: boolean;
  onChange: (field: keyof EstimateFormValues, value: string) => void;
};

const selectClassName =
  "flex h-10 w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50";

const leadSourceOptions = ["", "Website", "Phone Inquiry", "Referral", "Walk-in", "Other"];
const moveSizeOptions = ["", "Studio", "1 Bedroom", "2 Bedroom", "3 Bedroom", "4+ Bedroom", "Office"];
const locationTypeOptions = ["", "House", "Apartment", "Condo", "Storage", "Office", "Other"];

export function EstimateForm({ values, errors, disabled, onChange }: Props) {
  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle>Customer contact</CardTitle>
        </CardHeader>
        <CardContent className="grid gap-4 md:grid-cols-2">
          <FormField
            id="customerName"
            label="Customer name"
            required
            value={values.customerName}
            error={errors.customerName}
            disabled={disabled}
            onChange={(value) => onChange("customerName", value)}
          />
          <FormField
            id="email"
            label="Email"
            required
            type="email"
            value={values.email}
            error={errors.email}
            disabled={disabled}
            onChange={(value) => onChange("email", value)}
          />
          <FormField
            id="primaryPhone"
            label="Primary phone"
            required
            value={values.primaryPhone}
            error={errors.primaryPhone}
            disabled={disabled}
            onChange={(value) => onChange("primaryPhone", value)}
          />
          <FormField
            id="secondaryPhone"
            label="Secondary phone"
            value={values.secondaryPhone}
            error={errors.secondaryPhone}
            disabled={disabled}
            onChange={(value) => onChange("secondaryPhone", value)}
          />
        </CardContent>
      </Card>

      <div className="grid gap-6 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Origin (Moving From)</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <FormField
              id="originAddressLine1"
              label="Address"
              required
              value={values.originAddressLine1}
              error={errors.originAddressLine1}
              disabled={disabled}
              onChange={(value) => onChange("originAddressLine1", value)}
            />
            <div className="grid gap-4 md:grid-cols-3">
              <FormField
                id="originCity"
                label="City"
                required
                value={values.originCity}
                error={errors.originCity}
                disabled={disabled}
                onChange={(value) => onChange("originCity", value)}
              />
              <FormField
                id="originState"
                label="State"
                required
                value={values.originState}
                error={errors.originState}
                disabled={disabled}
                onChange={(value) => onChange("originState", value)}
              />
              <FormField
                id="originPostalCode"
                label="Postal code"
                required
                value={values.originPostalCode}
                error={errors.originPostalCode}
                disabled={disabled}
                onChange={(value) => onChange("originPostalCode", value)}
              />
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Destination (Moving To)</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <FormField
              id="destinationAddressLine1"
              label="Address"
              required
              value={values.destinationAddressLine1}
              error={errors.destinationAddressLine1}
              disabled={disabled}
              onChange={(value) => onChange("destinationAddressLine1", value)}
            />
            <div className="grid gap-4 md:grid-cols-3">
              <FormField
                id="destinationCity"
                label="City"
                required
                value={values.destinationCity}
                error={errors.destinationCity}
                disabled={disabled}
                onChange={(value) => onChange("destinationCity", value)}
              />
              <FormField
                id="destinationState"
                label="State"
                required
                value={values.destinationState}
                error={errors.destinationState}
                disabled={disabled}
                onChange={(value) => onChange("destinationState", value)}
              />
              <FormField
                id="destinationPostalCode"
                label="Postal code"
                required
                value={values.destinationPostalCode}
                error={errors.destinationPostalCode}
                disabled={disabled}
                onChange={(value) => onChange("destinationPostalCode", value)}
              />
            </div>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Move details</CardTitle>
        </CardHeader>
        <CardContent className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
          <FormField
            id="moveDate"
            label="Move date"
            required
            type="date"
            value={values.moveDate}
            error={errors.moveDate}
            disabled={disabled}
            onChange={(value) => onChange("moveDate", value)}
          />
          <FormField
            id="pickupTime"
            label="Pickup time"
            type="time"
            value={values.pickupTime}
            error={errors.pickupTime}
            disabled={disabled}
            onChange={(value) => onChange("pickupTime", value)}
          />
          <SelectField
            id="leadSource"
            label="Lead source"
            required
            value={values.leadSource}
            options={leadSourceOptions}
            error={errors.leadSource}
            disabled={disabled}
            onChange={(value) => onChange("leadSource", value)}
          />
          <SelectField
            id="moveSize"
            label="Move size"
            value={values.moveSize}
            options={moveSizeOptions}
            error={errors.moveSize}
            disabled={disabled}
            onChange={(value) => onChange("moveSize", value)}
          />
          <SelectField
            id="locationType"
            label="Location type"
            value={values.locationType}
            options={locationTypeOptions}
            error={errors.locationType}
            disabled={disabled}
            onChange={(value) => onChange("locationType", value)}
          />
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Pricing</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-4 md:grid-cols-2">
            <FormField
              id="estimatedTotal"
              label="Estimated total"
              placeholder="0.00"
              value={values.estimatedTotal}
              error={errors.estimatedTotal}
              disabled={disabled}
              onChange={(value) => onChange("estimatedTotal", value)}
            />
            <FormField
              id="deposit"
              label="Deposit"
              placeholder="0.00"
              value={values.deposit}
              error={errors.deposit}
              disabled={disabled}
              onChange={(value) => onChange("deposit", value)}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="notes">Notes</Label>
            <Textarea
              id="notes"
              value={values.notes}
              onChange={(event) => onChange("notes", event.target.value)}
              disabled={disabled}
              placeholder="Add pricing or scope notes"
            />
            <FieldError message={errors.notes} />
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

function FormField({
  id,
  label,
  value,
  onChange,
  error,
  required,
  disabled,
  type = "text",
  placeholder,
}: {
  id: string;
  label: string;
  value: string;
  onChange: (value: string) => void;
  error?: string;
  required?: boolean;
  disabled?: boolean;
  type?: string;
  placeholder?: string;
}) {
  function handleChange(event: ChangeEvent<HTMLInputElement>) {
    onChange(event.target.value);
  }

  return (
    <div className="space-y-2">
      <Label htmlFor={id}>
        {label}
        {required ? <span className="ml-1 text-destructive">*</span> : null}
      </Label>
      <Input
        id={id}
        type={type}
        value={value}
        onChange={handleChange}
        disabled={disabled}
        placeholder={placeholder}
        className={cn(error ? "border-destructive focus-visible:ring-destructive" : "")}
      />
      <FieldError message={error} />
    </div>
  );
}

function SelectField({
  id,
  label,
  value,
  options,
  onChange,
  error,
  required,
  disabled,
}: {
  id: string;
  label: string;
  value: string;
  options: string[];
  onChange: (value: string) => void;
  error?: string;
  required?: boolean;
  disabled?: boolean;
}) {
  return (
    <div className="space-y-2">
      <Label htmlFor={id}>
        {label}
        {required ? <span className="ml-1 text-destructive">*</span> : null}
      </Label>
      <select
        id={id}
        className={cn(selectClassName, error ? "border-destructive focus-visible:ring-destructive" : "")}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        disabled={disabled}
      >
        {options.map((option) => (
          <option key={option || "empty"} value={option}>
            {option || "Select"}
          </option>
        ))}
      </select>
      <FieldError message={error} />
    </div>
  );
}

function FieldError({ message }: { message?: string }) {
  if (!message) return null;
  return <p className="text-xs text-destructive">{message}</p>;
}
