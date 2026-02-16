"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { Loader2 } from "lucide-react";
import { toast } from "sonner";

import { EstimateForm } from "@/components/estimates/estimate-form";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import {
  emptyEstimateForm,
  estimateFormSchema,
  estimateToFormValues,
  toCreateEstimateRequest,
  toUpdateEstimateRequest,
  validateEstimateField,
  validateEstimateForm,
  type EstimateFormErrors,
  type EstimateFormField,
  type EstimateFormValues,
} from "@/lib/estimate-form";
import {
  createEstimate,
  getApiErrorMessage,
  newIdempotencyKey,
  updateEstimate,
  type Estimate,
} from "@/lib/phase2-api";

type Props = {
  mode: "new" | "existing";
  estimate?: Estimate;
  estimateId?: string;
  onEstimateSaved?: (estimate: Estimate) => void;
};

type SaveIntent = "save" | "next" | "autosave";
type SaveState = "idle" | "saving" | "saved" | "error";

export function EstimateEntryEditor({ mode, estimate, estimateId, onEstimateSaved }: Props) {
  const router = useRouter();
  const createIdempotencyKey = useRef(newIdempotencyKey("estimate"));

  const initialValues = useMemo<EstimateFormValues>(() => {
    if (mode === "existing" && estimate) {
      return estimateToFormValues(estimate);
    }
    return emptyEstimateForm;
  }, [mode, estimate]);

  const [values, setValues] = useState<EstimateFormValues>(initialValues);
  const [errors, setErrors] = useState<EstimateFormErrors>({});
  const [touched, setTouched] = useState<Partial<Record<EstimateFormField, true>>>({});
  const [saveState, setSaveState] = useState<SaveState>("idle");
  const [saveMessage, setSaveMessage] = useState<string>("Not saved yet");
  const [actionLoading, setActionLoading] = useState<SaveIntent | null>(null);

  const valuesFingerprint = useMemo(() => JSON.stringify(values), [values]);
  const [lastSavedFingerprint, setLastSavedFingerprint] = useState(valuesFingerprint);
  const hasChanges = valuesFingerprint !== lastSavedFingerprint;
  const isSaving = saveState === "saving";

  useEffect(() => {
    setValues(initialValues);
    const nextFingerprint = JSON.stringify(initialValues);
    setLastSavedFingerprint(nextFingerprint);
    setErrors({});
    setTouched({});
    setSaveState(mode === "new" ? "idle" : "saved");
    setSaveMessage(mode === "new" ? "Not saved yet" : "Saved");
  }, [initialValues, mode]);

  const persist = useCallback(
    async (intent: SaveIntent) => {
      if (intent === "autosave" && (mode !== "existing" || !estimateId || !hasChanges || isSaving)) {
        return null;
      }

      if (intent !== "autosave") {
        const nextErrors = validateEstimateForm(values);
        setErrors(nextErrors);
        if (Object.keys(nextErrors).length > 0) {
          setSaveState("error");
          setSaveMessage("Save failed");
          return null;
        }
      } else if (!estimateFormSchema.safeParse(values).success) {
        return null;
      }

      if (isSaving) return null;

      setSaveState("saving");
      setSaveMessage("Saving...");
      setActionLoading(intent);

      try {
        const response =
          mode === "existing" && estimateId
            ? await updateEstimate(estimateId, toUpdateEstimateRequest(values))
            : await createEstimate(toCreateEstimateRequest(values), createIdempotencyKey.current);

        const nextEstimate = response.estimate;
        const nextValues = estimateToFormValues(nextEstimate);
        const nextFingerprint = JSON.stringify(nextValues);

        setValues(nextValues);
        setLastSavedFingerprint(nextFingerprint);
        setErrors({});
        setSaveState("saved");
        setSaveMessage("Saved");
        onEstimateSaved?.(nextEstimate);

        if (intent === "next") {
          router.push(`/estimates/${nextEstimate.id}/inventory`);
          return nextEstimate;
        }

        if (mode === "new") {
          router.replace(`/estimates/${nextEstimate.id}/entry`);
        }

        return nextEstimate;
      } catch (error) {
        const message = getApiErrorMessage(error);
        setSaveState("error");
        setSaveMessage("Save failed");

        if (intent !== "autosave") {
          toast.error(message);
        }

        return null;
      } finally {
        setActionLoading(null);
      }
    },
    [estimateId, hasChanges, isSaving, mode, onEstimateSaved, router, values],
  );

  useEffect(() => {
    if (mode !== "existing" || !estimateId || !hasChanges || isSaving) return;
    const timer = setTimeout(() => {
      void persist("autosave");
    }, 900);
    return () => clearTimeout(timer);
  }, [estimateId, hasChanges, isSaving, mode, persist, valuesFingerprint]);

  function onFieldChange(field: EstimateFormField, value: string) {
    setValues((prev) => {
      const next = { ...prev, [field]: value };
      if (touched[field]) {
        setErrors((previous) => ({
          ...previous,
          [field]: validateEstimateField(field, next),
        }));
      }
      return next;
    });
  }

  function onFieldBlur(field: EstimateFormField) {
    setTouched((prev) => ({ ...prev, [field]: true }));
    setErrors((prev) => ({
      ...prev,
      [field]: validateEstimateField(field, values),
    }));
  }

  async function handleSaveClick() {
    await persist("save");
  }

  async function handleNextClick() {
    if (mode === "existing" && estimateId && !hasChanges) {
      router.push(`/estimates/${estimateId}/inventory`);
      return;
    }
    await persist("next");
  }

  return (
    <div className="space-y-4">
      <Card className="border-border/70 bg-card/70">
        <CardContent className="flex flex-wrap items-center justify-between gap-3 p-4">
          <p
            className="text-sm text-muted-foreground"
            aria-live="polite"
            role="status"
            data-save-state={saveState}
          >
            {saveMessage}
          </p>
          <div className="flex items-center gap-2">
            <Button onClick={handleSaveClick} disabled={isSaving}>
              {actionLoading === "save" ? (
                <span className="inline-flex items-center gap-2">
                  <Loader2 className="h-4 w-4 animate-spin" />
                  Saving...
                </span>
              ) : (
                "Save"
              )}
            </Button>
            <Button variant="secondary" onClick={handleNextClick} disabled={isSaving}>
              {actionLoading === "next" ? (
                <span className="inline-flex items-center gap-2">
                  <Loader2 className="h-4 w-4 animate-spin" />
                  Saving...
                </span>
              ) : (
                "Next: Inventory"
              )}
            </Button>
          </div>
        </CardContent>
      </Card>

      <EstimateForm
        values={values}
        errors={errors}
        onChange={onFieldChange}
        onBlur={onFieldBlur}
        disabled={isSaving}
      />
    </div>
  );
}
