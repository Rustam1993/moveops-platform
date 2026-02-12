"use client";

import { useEffect, useMemo, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { Loader2 } from "lucide-react";
import { toast } from "sonner";

import { EstimateForm } from "@/components/estimates/estimate-form";
import { PageHeader } from "@/components/layout/page-header";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import {
  estimateToFormValues,
  toUpdateEstimateRequest,
  validateEstimateForm,
  type EstimateFormErrors,
  type EstimateFormValues,
} from "@/lib/estimate-form";
import {
  convertEstimate,
  getApiErrorMessage,
  getEstimate,
  newIdempotencyKey,
  updateEstimate,
  type Estimate,
} from "@/lib/phase2-api";

export default function EstimateDetailPage() {
  const router = useRouter();
  const params = useParams<{ estimateId: string }>();
  const estimateId = Array.isArray(params?.estimateId) ? params.estimateId[0] : params?.estimateId;

  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [converting, setConverting] = useState(false);
  const [estimate, setEstimate] = useState<Estimate | null>(null);
  const [values, setValues] = useState<EstimateFormValues | null>(null);
  const [errors, setErrors] = useState<EstimateFormErrors>({});

  useEffect(() => {
    if (!estimateId) return;

    let cancelled = false;
    setLoading(true);

    getEstimate(estimateId)
      .then((response) => {
        if (cancelled) return;
        setEstimate(response.estimate);
        setValues(estimateToFormValues(response.estimate));
      })
      .catch((error) => {
        if (cancelled) return;
        toast.error(getApiErrorMessage(error));
      })
      .finally(() => {
        if (cancelled) return;
        setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [estimateId]);

  const canConvert = useMemo(() => {
    if (!estimate) return false;
    return estimate.status !== "converted";
  }, [estimate]);

  function onFieldChange(field: keyof EstimateFormValues, value: string) {
    setValues((prev) => (prev ? { ...prev, [field]: value } : prev));
    setErrors((prev) => ({ ...prev, [field]: undefined }));
  }

  async function saveChanges() {
    if (!estimate || !values || !estimateId) return;

    const nextErrors = validateEstimateForm(values);
    setErrors(nextErrors);
    if (Object.keys(nextErrors).length > 0) {
      toast.error("Please fix the highlighted fields");
      return;
    }

    setSaving(true);
    try {
      const response = await updateEstimate(estimateId, toUpdateEstimateRequest(values));
      setEstimate(response.estimate);
      setValues(estimateToFormValues(response.estimate));
      toast.success("Estimate updated");
      router.refresh();
    } catch (error) {
      toast.error(getApiErrorMessage(error));
    } finally {
      setSaving(false);
    }
  }

  async function convertToJob() {
    if (!estimate || !estimateId) return;

    if (estimate.status === "converted" && estimate.convertedJobId) {
      router.push(`/jobs/${estimate.convertedJobId}`);
      return;
    }

    setConverting(true);
    try {
      const response = await convertEstimate(estimateId, newIdempotencyKey("convert"));
      toast.success(`Converted to job ${response.job.jobNumber}`);
      router.push(`/jobs/${response.job.id}`);
      router.refresh();
    } catch (error) {
      toast.error(getApiErrorMessage(error));
    } finally {
      setConverting(false);
    }
  }

  if (loading || !values || !estimate) {
    return (
      <div className="space-y-4">
        <Skeleton className="h-10 w-80" />
        <Skeleton className="h-56 w-full" />
      </div>
    );
  }

  return (
    <div className="space-y-6 pb-8">
      <PageHeader
        title={`Estimate ${estimate.estimateNumber}`}
        description="Review and update estimate details, then convert to a job when ready."
        actions={
          <div className="flex flex-wrap items-center gap-2">
            <Button onClick={saveChanges} disabled={saving || converting}>
              {saving ? (
                <span className="inline-flex items-center gap-2">
                  <Loader2 className="h-4 w-4 animate-spin" />
                  Saving...
                </span>
              ) : (
                "Save changes"
              )}
            </Button>
            <Button
              variant={canConvert ? "secondary" : "outline"}
              onClick={convertToJob}
              disabled={saving || converting}
            >
              {converting ? (
                <span className="inline-flex items-center gap-2">
                  <Loader2 className="h-4 w-4 animate-spin" />
                  Converting...
                </span>
              ) : canConvert ? (
                "Convert to job"
              ) : (
                "Open converted job"
              )}
            </Button>
          </div>
        }
      />

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Estimate summary</CardTitle>
        </CardHeader>
        <CardContent className="grid gap-4 text-sm md:grid-cols-4">
          <Summary label="Status" value={estimate.status} />
          <Summary label="Lead source" value={estimate.leadSource} />
          <Summary label="Move date" value={estimate.moveDate} />
          <Summary label="Customer" value={estimate.customerName} />
        </CardContent>
      </Card>

      <EstimateForm values={values} errors={errors} onChange={onFieldChange} disabled={saving || converting} />
    </div>
  );
}

function Summary({ label, value }: { label: string; value?: string }) {
  return (
    <div className="rounded-lg border border-border/60 bg-muted/20 p-3">
      <p className="text-xs uppercase tracking-wide text-muted-foreground">{label}</p>
      <p className="mt-1 text-sm font-medium">{value || "-"}</p>
    </div>
  );
}
