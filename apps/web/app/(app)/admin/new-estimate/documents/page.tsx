"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Loader2, Save } from "lucide-react";
import { toast } from "sonner";

import { NewEstimateAdminNav } from "@/components/admin/new-estimate-admin-nav";
import { NotAuthorizedState } from "@/components/layout/not-authorized-state";
import { PageHeader } from "@/components/layout/page-header";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { isForbiddenError } from "@/lib/api";
import {
  getDocumentBranding,
  saveDocumentBranding,
  type NewEstimateDocumentBranding,
} from "@/lib/new-estimate-admin-api";
import { getApiErrorMessage } from "@/lib/phase2-api";

type SaveState = "idle" | "saving" | "saved" | "error";

export default function NewEstimateDocumentsAdminPage() {
  const [branding, setBranding] = useState<NewEstimateDocumentBranding>({});
  const [loading, setLoading] = useState(true);
  const [forbidden, setForbidden] = useState(false);
  const [saveState, setSaveState] = useState<SaveState>("idle");

  const status = useMemo(() => {
    if (saveState === "saving") return "Saving...";
    if (saveState === "saved") return "Saved";
    if (saveState === "error") return "Save failed";
    return "Ready";
  }, [saveState]);

  const loadData = useCallback(async () => {
    setLoading(true);
    try {
      const response = await getDocumentBranding();
      setBranding(response.branding);
      setForbidden(false);
      setSaveState("idle");
    } catch (error) {
      if (isForbiddenError(error)) {
        setForbidden(true);
      } else {
        toast.error(getApiErrorMessage(error));
      }
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadData();
  }, [loadData]);

  function onFieldChange(field: keyof NewEstimateDocumentBranding, value: string) {
    setBranding((previous) => ({ ...previous, [field]: value }));
    setSaveState("idle");
  }

  async function onSave() {
    setSaveState("saving");
    try {
      const payload = normalizeBranding(branding);
      const response = await saveDocumentBranding(payload);
      setBranding(response.branding);
      setSaveState("saved");
      toast.success("Document branding saved");
    } catch (error) {
      setSaveState("error");
      toast.error(getApiErrorMessage(error));
    }
  }

  if (forbidden) {
    return (
      <div className="space-y-6 pb-8">
        <PageHeader title="Document Branding" description="Branding fields used in printed estimate PDFs." />
        <NewEstimateAdminNav />
        <NotAuthorizedState message="This page requires admin.new_estimate permission." />
      </div>
    );
  }

  return (
    <div className="space-y-6 pb-8">
      <PageHeader
        title="Document Branding"
        description="Configure company information and terms snippet used by generated estimate PDFs."
      />
      <NewEstimateAdminNav />

      {loading ? (
        <Card>
          <CardContent className="py-10 text-sm text-muted-foreground">
            <span className="inline-flex items-center gap-2">
              <Loader2 className="h-4 w-4 animate-spin" />
              Loading document branding...
            </span>
          </CardContent>
        </Card>
      ) : (
        <>
          <Card className="border-border/70 bg-card/70">
            <CardContent className="flex items-center justify-between gap-3 p-4">
              <p className="text-sm text-muted-foreground" aria-live="polite" role="status">
                {status}
              </p>
              <Button onClick={() => void onSave()} disabled={saveState === "saving"}>
                {saveState === "saving" ? (
                  <span className="inline-flex items-center gap-2">
                    <Loader2 className="h-4 w-4 animate-spin" />
                    Saving...
                  </span>
                ) : (
                  <span className="inline-flex items-center gap-2">
                    <Save className="h-4 w-4" />
                    Save branding
                  </span>
                )}
              </Button>
            </CardContent>
          </Card>

          <Card className="border-border/70 bg-card/70">
            <CardHeader>
              <CardTitle>PDF branding</CardTitle>
              <CardDescription>
                These values appear in generated estimate PDFs and signed documents. Logo upload is not enabled in this MVP.
              </CardDescription>
            </CardHeader>
            <CardContent className="grid gap-4 md:grid-cols-2">
              <Field label="Company display name">
                <Input
                  value={branding.companyDisplayName ?? ""}
                  onChange={(event) => onFieldChange("companyDisplayName", event.target.value)}
                  placeholder="MoveOps Movers"
                />
              </Field>
              <Field label="Company phone">
                <Input
                  value={branding.companyPhone ?? ""}
                  onChange={(event) => onFieldChange("companyPhone", event.target.value)}
                  placeholder="(555) 123-1234"
                />
              </Field>
              <Field label="Company email">
                <Input
                  value={branding.companyEmail ?? ""}
                  onChange={(event) => onFieldChange("companyEmail", event.target.value)}
                  placeholder="support@moveops.example"
                />
              </Field>
              <Field label="Logo URL (optional)">
                <Input
                  value={branding.logoUrl ?? ""}
                  onChange={(event) => onFieldChange("logoUrl", event.target.value)}
                  placeholder="https://cdn.example.com/logo.png"
                />
              </Field>
              <div className="space-y-2 md:col-span-2">
                <Label>Terms snippet</Label>
                <Textarea
                  rows={6}
                  value={branding.termsSnippet ?? ""}
                  onChange={(event) => onFieldChange("termsSnippet", event.target.value)}
                  placeholder="All services subject to contract terms and mover tariffs..."
                />
              </div>
            </CardContent>
          </Card>
        </>
      )}
    </div>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="space-y-2">
      <Label>{label}</Label>
      {children}
    </div>
  );
}

function normalizeBranding(input: NewEstimateDocumentBranding): NewEstimateDocumentBranding {
  return {
    companyDisplayName: trimToUndefined(input.companyDisplayName),
    companyPhone: trimToUndefined(input.companyPhone),
    companyEmail: trimToUndefined(input.companyEmail),
    termsSnippet: trimToUndefined(input.termsSnippet),
    logoUrl: trimToUndefined(input.logoUrl),
  };
}

function trimToUndefined(value: string | undefined) {
  if (!value) return undefined;
  const trimmed = value.trim();
  return trimmed.length > 0 ? trimmed : undefined;
}
