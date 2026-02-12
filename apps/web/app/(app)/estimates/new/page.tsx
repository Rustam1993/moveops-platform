"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { Loader2 } from "lucide-react";
import { toast } from "sonner";

import { EstimateForm } from "@/components/estimates/estimate-form";
import { PageHeader } from "@/components/layout/page-header";
import { Button } from "@/components/ui/button";
import {
  emptyEstimateForm,
  toCreateEstimateRequest,
  validateEstimateForm,
  type EstimateFormErrors,
  type EstimateFormValues,
} from "@/lib/estimate-form";
import {
  convertEstimate,
  createEstimate,
  getApiErrorMessage,
  newIdempotencyKey,
} from "@/lib/phase2-api";

export default function NewEstimatePage() {
  const router = useRouter();
  const [values, setValues] = useState<EstimateFormValues>(emptyEstimateForm);
  const [errors, setErrors] = useState<EstimateFormErrors>({});
  const [saving, setSaving] = useState(false);
  const [converting, setConverting] = useState(false);

  function onFieldChange(field: keyof EstimateFormValues, value: string) {
    setValues((prev) => ({ ...prev, [field]: value }));
    setErrors((prev) => ({ ...prev, [field]: undefined }));
  }

  async function submit(intent: "save" | "convert") {
    const nextErrors = validateEstimateForm(values);
    setErrors(nextErrors);

    if (Object.keys(nextErrors).length > 0) {
      toast.error("Please fix the highlighted fields");
      return;
    }

    if (intent === "save") {
      setSaving(true);
    } else {
      setConverting(true);
    }

    try {
      const estimateResponse = await createEstimate(
        toCreateEstimateRequest(values),
        newIdempotencyKey("estimate"),
      );

      if (intent === "save") {
        toast.success(`Estimate ${estimateResponse.estimate.estimateNumber} saved`);
        router.push(`/estimates/${estimateResponse.estimate.id}`);
        router.refresh();
        return;
      }

      const jobResponse = await convertEstimate(
        estimateResponse.estimate.id,
        newIdempotencyKey("convert"),
      );
      toast.success(`Converted to job ${jobResponse.job.jobNumber}`);
      router.push(`/jobs/${jobResponse.job.id}`);
      router.refresh();
    } catch (error) {
      toast.error(getApiErrorMessage(error));
    } finally {
      setSaving(false);
      setConverting(false);
    }
  }

  return (
    <div className="space-y-6 pb-8">
      <PageHeader
        title="New Estimate"
        description="Capture move details, customer contact, and pricing to create a draft estimate."
        actions={
          <div className="flex flex-wrap items-center gap-2">
            <Button onClick={() => submit("save")} disabled={saving || converting}>
              {saving ? (
                <span className="inline-flex items-center gap-2">
                  <Loader2 className="h-4 w-4 animate-spin" />
                  Saving...
                </span>
              ) : (
                "Save estimate"
              )}
            </Button>
            <Button variant="secondary" onClick={() => submit("convert")} disabled={saving || converting}>
              {converting ? (
                <span className="inline-flex items-center gap-2">
                  <Loader2 className="h-4 w-4 animate-spin" />
                  Saving and converting...
                </span>
              ) : (
                "Convert to job"
              )}
            </Button>
          </div>
        }
      />

      <EstimateForm values={values} errors={errors} onChange={onFieldChange} disabled={saving || converting} />
    </div>
  );
}
