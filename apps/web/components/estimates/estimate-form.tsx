import { type ChangeEvent } from "react";

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import {
  serviceTypeOptions,
  type EstimateFormErrors,
  type EstimateFormField,
  type EstimateFormValues,
} from "@/lib/estimate-form";
import { cn } from "@/lib/utils";

type Props = {
  values: EstimateFormValues;
  errors: EstimateFormErrors;
  disabled?: boolean;
  onChange: (field: EstimateFormField, value: string) => void;
  onBlur: (field: EstimateFormField) => void;
};

const selectClassName =
  "flex h-10 w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50";

export function EstimateForm({ values, errors, disabled, onChange, onBlur }: Props) {
  return (
    <div className="space-y-5">
      <Card className="border-border/70">
        <CardHeader>
          <CardTitle>Customer</CardTitle>
          <CardDescription>Primary contact for this estimate.</CardDescription>
        </CardHeader>
        <CardContent className="grid gap-4 md:grid-cols-2">
          <FormField
            id="firstName"
            label="First name"
            required
            value={values.firstName}
            error={errors.firstName}
            disabled={disabled}
            onChange={(value) => onChange("firstName", value)}
            onBlur={() => onBlur("firstName")}
          />
          <FormField
            id="lastName"
            label="Last name"
            required
            value={values.lastName}
            error={errors.lastName}
            disabled={disabled}
            onChange={(value) => onChange("lastName", value)}
            onBlur={() => onBlur("lastName")}
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
            onBlur={() => onBlur("email")}
          />
          <FormField
            id="phone"
            label="Phone"
            required
            value={values.phone}
            error={errors.phone}
            disabled={disabled}
            onChange={(value) => onChange("phone", value)}
            onBlur={() => onBlur("phone")}
          />
        </CardContent>
      </Card>

      <div className="grid gap-5 lg:grid-cols-2">
        <Card className="border-border/70">
          <CardHeader>
            <CardTitle>Moving From</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <FormField
              id="movingFromStreet"
              label="Street"
              required
              value={values.movingFromStreet}
              error={errors.movingFromStreet}
              disabled={disabled}
              onChange={(value) => onChange("movingFromStreet", value)}
              onBlur={() => onBlur("movingFromStreet")}
            />
            <div className="grid gap-4 md:grid-cols-3">
              <FormField
                id="movingFromCity"
                label="City"
                required
                value={values.movingFromCity}
                error={errors.movingFromCity}
                disabled={disabled}
                onChange={(value) => onChange("movingFromCity", value)}
                onBlur={() => onBlur("movingFromCity")}
              />
              <FormField
                id="movingFromState"
                label="State"
                required
                value={values.movingFromState}
                error={errors.movingFromState}
                disabled={disabled}
                onChange={(value) => onChange("movingFromState", value)}
                onBlur={() => onBlur("movingFromState")}
              />
              <FormField
                id="movingFromZip"
                label="ZIP"
                required
                value={values.movingFromZip}
                error={errors.movingFromZip}
                disabled={disabled}
                onChange={(value) => onChange("movingFromZip", value)}
                onBlur={() => onBlur("movingFromZip")}
              />
            </div>
          </CardContent>
        </Card>

        <Card className="border-border/70">
          <CardHeader>
            <CardTitle>Moving To</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <FormField
              id="movingToStreet"
              label="Street"
              required
              value={values.movingToStreet}
              error={errors.movingToStreet}
              disabled={disabled}
              onChange={(value) => onChange("movingToStreet", value)}
              onBlur={() => onBlur("movingToStreet")}
            />
            <div className="grid gap-4 md:grid-cols-3">
              <FormField
                id="movingToCity"
                label="City"
                required
                value={values.movingToCity}
                error={errors.movingToCity}
                disabled={disabled}
                onChange={(value) => onChange("movingToCity", value)}
                onBlur={() => onBlur("movingToCity")}
              />
              <FormField
                id="movingToState"
                label="State"
                required
                value={values.movingToState}
                error={errors.movingToState}
                disabled={disabled}
                onChange={(value) => onChange("movingToState", value)}
                onBlur={() => onBlur("movingToState")}
              />
              <FormField
                id="movingToZip"
                label="ZIP"
                required
                value={values.movingToZip}
                error={errors.movingToZip}
                disabled={disabled}
                onChange={(value) => onChange("movingToZip", value)}
                onBlur={() => onBlur("movingToZip")}
              />
            </div>
          </CardContent>
        </Card>
      </div>

      <Card className="border-border/70">
        <CardHeader>
          <CardTitle>Move Basics</CardTitle>
          <CardDescription>Date, time preference, and service type for scheduling.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-4 md:grid-cols-3">
            <FormField
              id="moveDate"
              label="Move date"
              required
              type="date"
              value={values.moveDate}
              error={errors.moveDate}
              disabled={disabled}
              onChange={(value) => onChange("moveDate", value)}
              onBlur={() => onBlur("moveDate")}
            />
            <FormField
              id="preferredTimeWindow"
              label="Preferred time window"
              type="text"
              placeholder="e.g., 9:00 AM - 12:00 PM"
              value={values.preferredTimeWindow}
              error={errors.preferredTimeWindow}
              disabled={disabled}
              onChange={(value) => onChange("preferredTimeWindow", value)}
              onBlur={() => onBlur("preferredTimeWindow")}
            />
            <SelectField
              id="serviceType"
              label="Service type"
              required
              value={values.serviceType}
              error={errors.serviceType}
              disabled={disabled}
              options={serviceTypeOptions.map((option) => ({ value: option.value, label: option.label }))}
              onChange={(value) => onChange("serviceType", value)}
              onBlur={() => onBlur("serviceType")}
            />
          </div>

          <details className="rounded-md border border-border/70 bg-muted/10 p-4">
            <summary className="cursor-pointer text-sm font-medium">Advanced</summary>
            <div className="mt-4 grid gap-4 md:grid-cols-2">
              <FormField
                id="leadSource"
                label="Referral source"
                required
                value={values.leadSource}
                error={errors.leadSource}
                disabled={disabled}
                onChange={(value) => onChange("leadSource", value)}
                onBlur={() => onBlur("leadSource")}
              />
            </div>
          </details>

          <div className="space-y-2">
            <Label htmlFor="notes">Internal notes</Label>
            <Textarea
              id="notes"
              value={values.notes}
              onChange={(event) => onChange("notes", event.target.value)}
              onBlur={() => onBlur("notes")}
              disabled={disabled}
              placeholder="Add context for sales and operations teams"
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
  onBlur,
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
  onBlur: () => void;
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
        onBlur={onBlur}
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
  onBlur,
  error,
  required,
  disabled,
}: {
  id: string;
  label: string;
  value: string;
  options: Array<{ value: string; label: string }>;
  onChange: (value: string) => void;
  onBlur: () => void;
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
        onBlur={onBlur}
        disabled={disabled}
      >
        {options.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
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
