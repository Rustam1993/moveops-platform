"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Loader2, Mail, Save, Send } from "lucide-react";
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
  getEmailTemplates,
  saveEmailTemplates,
  testSendEmailTemplate,
  type NewEstimateEmailTemplate,
  type NewEstimateEmailTemplates,
} from "@/lib/new-estimate-admin-api";
import { getApiErrorMessage } from "@/lib/phase2-api";

type SaveState = "idle" | "saving" | "saved" | "error";
type TemplateTab = "eQuote" | "inventoryLink" | "eSign";

const templateTabs: Array<{ key: TemplateTab; label: string; apiKey: "e_quote" | "inventory_link" | "e_sign" }> = [
  { key: "eQuote", label: "E-Quote", apiKey: "e_quote" },
  { key: "inventoryLink", label: "Inventory Link", apiKey: "inventory_link" },
  { key: "eSign", label: "E-Sign", apiKey: "e_sign" },
];

const sampleData: Record<string, string> = {
  customer_name: "Alex Johnson",
  quote_link: "https://app.moveops.example/public/estimate/sample-token",
  inventory_link: "https://app.moveops.example/public/inventory/sample-token",
  signature_link: "https://app.moveops.example/public/sign/sample-token",
  estimate_number: "E-10293",
  move_date: "2026-03-15",
};

export default function NewEstimateEmailTemplatesAdminPage() {
  const [templates, setTemplates] = useState<NewEstimateEmailTemplates>({});
  const [allowedVariables, setAllowedVariables] = useState<string[]>([]);
  const [activeTab, setActiveTab] = useState<TemplateTab>("eQuote");
  const [loading, setLoading] = useState(true);
  const [forbidden, setForbidden] = useState(false);
  const [saveState, setSaveState] = useState<SaveState>("idle");
  const [testEmail, setTestEmail] = useState("");
  const [sendingTest, setSendingTest] = useState(false);

  const activeTemplate = templates[activeTab] ?? {};

  const preview = useMemo(() => {
    const subject = renderTemplateText(activeTemplate.subject ?? "", sampleData);
    const textBody = renderTemplateText(activeTemplate.textBody ?? "", sampleData);
    return { subject, textBody };
  }, [activeTemplate.subject, activeTemplate.textBody]);

  const loadData = useCallback(async () => {
    setLoading(true);
    try {
      const response = await getEmailTemplates();
      setTemplates(normalizeTemplates(response.templates));
      setAllowedVariables(response.allowedVariables);
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

  function onTemplateFieldChange(field: keyof NewEstimateEmailTemplate, value: string) {
    setTemplates((previous) => ({
      ...previous,
      [activeTab]: {
        ...(previous[activeTab] ?? {}),
        [field]: value,
      },
    }));
    setSaveState("idle");
  }

  async function onSave() {
    setSaveState("saving");
    try {
      const response = await saveEmailTemplates(templates);
      setTemplates(normalizeTemplates(response.templates));
      setAllowedVariables(response.allowedVariables);
      setSaveState("saved");
      toast.success("Email templates saved");
    } catch (error) {
      setSaveState("error");
      toast.error(getApiErrorMessage(error));
    }
  }

  async function onTestSend() {
    const toEmail = testEmail.trim();
    if (!toEmail) {
      toast.error("Test recipient email is required");
      return;
    }

    const tab = templateTabs.find((entry) => entry.key === activeTab);
    if (!tab) return;

    setSendingTest(true);
    try {
      const response = await testSendEmailTemplate({
        templateKey: tab.apiKey,
        toEmail,
      });
      toast.success(`Test email ${response.status}`);
    } catch (error) {
      toast.error(getApiErrorMessage(error));
    } finally {
      setSendingTest(false);
    }
  }

  if (forbidden) {
    return (
      <div className="space-y-6 pb-8">
        <PageHeader title="Email Templates" description="Tenant-scoped transactional templates for New Estimate." />
        <NewEstimateAdminNav />
        <NotAuthorizedState message="This page requires admin.new_estimate permission." />
      </div>
    );
  }

  return (
    <div className="space-y-6 pb-8">
      <PageHeader
        title="Email Templates"
        description="Configure E-Quote, Inventory Link, and E-Sign templates. Defaults are used when a template field is blank."
      />
      <NewEstimateAdminNav />

      {loading ? (
        <Card>
          <CardContent className="py-10 text-sm text-muted-foreground">
            <span className="inline-flex items-center gap-2">
              <Loader2 className="h-4 w-4 animate-spin" />
              Loading email templates...
            </span>
          </CardContent>
        </Card>
      ) : (
        <>
          <Card className="border-border/70 bg-card/70">
            <CardContent className="flex flex-wrap items-center justify-between gap-3 p-4">
              <p className="text-sm text-muted-foreground" aria-live="polite" role="status">
                {saveState === "saving" ? "Saving..." : saveState === "saved" ? "Saved" : saveState === "error" ? "Save failed" : "Ready"}
              </p>
              <div className="flex items-center gap-2">
                <Button variant="outline" onClick={() => void onTestSend()} disabled={sendingTest}>
                  {sendingTest ? (
                    <span className="inline-flex items-center gap-2">
                      <Loader2 className="h-4 w-4 animate-spin" />
                      Sending test...
                    </span>
                  ) : (
                    <span className="inline-flex items-center gap-2">
                      <Send className="h-4 w-4" />
                      Test send
                    </span>
                  )}
                </Button>
                <Button onClick={() => void onSave()} disabled={saveState === "saving"}>
                  {saveState === "saving" ? (
                    <span className="inline-flex items-center gap-2">
                      <Loader2 className="h-4 w-4 animate-spin" />
                      Saving...
                    </span>
                  ) : (
                    <span className="inline-flex items-center gap-2">
                      <Save className="h-4 w-4" />
                      Save templates
                    </span>
                  )}
                </Button>
              </div>
            </CardContent>
          </Card>

          <div className="grid gap-6 xl:grid-cols-[220px_minmax(0,1fr)_360px]">
            <Card className="border-border/70 bg-card/70">
              <CardHeader className="pb-3">
                <CardTitle className="text-base">Template keys</CardTitle>
                <CardDescription>Select a template to edit.</CardDescription>
              </CardHeader>
              <CardContent className="space-y-2">
                {templateTabs.map((tab) => (
                  <Button
                    key={tab.key}
                    variant={activeTab === tab.key ? "default" : "outline"}
                    className="w-full justify-start"
                    onClick={() => setActiveTab(tab.key)}
                  >
                    {tab.label}
                  </Button>
                ))}
              </CardContent>
            </Card>

            <Card className="border-border/70 bg-card/70">
              <CardHeader>
                <CardTitle>{templateTabs.find((tab) => tab.key === activeTab)?.label}</CardTitle>
                <CardDescription>Subject, HTML, and plain-text bodies. Plain-text is used as fallback.</CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <Field label="Subject">
                  <Input
                    value={activeTemplate.subject ?? ""}
                    onChange={(event) => onTemplateFieldChange("subject", event.target.value)}
                    placeholder="Your moving estimate"
                  />
                </Field>
                <Field label="Text body">
                  <Textarea
                    rows={10}
                    value={activeTemplate.textBody ?? ""}
                    onChange={(event) => onTemplateFieldChange("textBody", event.target.value)}
                    placeholder="Hi {{customer_name}}, ..."
                  />
                </Field>
                <Field label="HTML body (optional)">
                  <Textarea
                    rows={8}
                    value={activeTemplate.htmlBody ?? ""}
                    onChange={(event) => onTemplateFieldChange("htmlBody", event.target.value)}
                    placeholder="<p>Hi {{customer_name}}, ...</p>"
                  />
                </Field>
                <Field label="Test recipient">
                  <Input
                    type="email"
                    value={testEmail}
                    onChange={(event) => setTestEmail(event.target.value)}
                    placeholder="admin@example.com"
                  />
                </Field>
              </CardContent>
            </Card>

            <div className="space-y-6">
              <Card className="border-border/70 bg-card/70">
                <CardHeader className="pb-3">
                  <CardTitle className="text-base">Allowed variables</CardTitle>
                  <CardDescription>Use these placeholders in subject/body values.</CardDescription>
                </CardHeader>
                <CardContent className="space-y-2">
                  {allowedVariables.length === 0 ? (
                    <p className="text-sm text-muted-foreground">No variables available.</p>
                  ) : (
                    allowedVariables.map((item) => (
                      <code key={item} className="block rounded bg-muted/40 px-2 py-1 text-xs">
                        {`{{${item}}}`}
                      </code>
                    ))
                  )}
                </CardContent>
              </Card>

              <Card className="border-border/70 bg-card/70">
                <CardHeader className="pb-3">
                  <CardTitle className="text-base">Rendered preview</CardTitle>
                  <CardDescription>Preview with sample values.</CardDescription>
                </CardHeader>
                <CardContent className="space-y-2 text-sm">
                  <p className="font-medium">{preview.subject || "(Empty subject)"}</p>
                  <pre className="whitespace-pre-wrap rounded-md bg-muted/30 p-3 text-xs text-muted-foreground">
                    {preview.textBody || "(Empty body)"}
                  </pre>
                  <p className="inline-flex items-center gap-2 text-xs text-muted-foreground">
                    <Mail className="h-3.5 w-3.5" />
                    Transactional emails do not include marketing unsubscribe links.
                  </p>
                </CardContent>
              </Card>
            </div>
          </div>
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

function normalizeTemplates(templates: NewEstimateEmailTemplates) {
  return {
    eQuote: templates.eQuote ?? {},
    inventoryLink: templates.inventoryLink ?? {},
    eSign: templates.eSign ?? {},
  } satisfies NewEstimateEmailTemplates;
}

function renderTemplateText(template: string, values: Record<string, string>) {
  let output = template;
  for (const [key, value] of Object.entries(values)) {
    output = output.replaceAll(`{{${key}}}`, value);
    output = output.replaceAll(`{{ ${key} }}`, value);
  }
  return output;
}
