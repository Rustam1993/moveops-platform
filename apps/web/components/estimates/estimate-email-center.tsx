"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Loader2, Mail, Send } from "lucide-react";
import { toast } from "sonner";

import { useEstimateWorkspace } from "@/components/estimates/estimate-workspace-context";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  listEstimateEmails,
  sendEstimateEmail,
  type EstimateEmailLog,
  type EstimateEmailTemplateKey,
  type SendEstimateEmailResponse,
} from "@/lib/phase4-api";
import { getApiErrorMessage } from "@/lib/phase2-api";

type SaveState = "idle" | "sending" | "sent" | "error";

type TemplateOption = {
  key: EstimateEmailTemplateKey;
  name: string;
  description: string;
  helper: string;
};

const TEMPLATE_OPTIONS: TemplateOption[] = [
  {
    key: "moving_estimate",
    name: "Moving Estimate",
    description: "E-Quote",
    helper: "Sends a secure link to the latest estimate PDF.",
  },
  {
    key: "update_inventory",
    name: "Update Your Inventory",
    description: "Link to Inventory",
    helper: "Sends the customer self-serve inventory link.",
  },
  {
    key: "signature_request",
    name: "Signature Request",
    description: "E-Sign",
    helper: "Sends a secure signature request link.",
  },
  {
    key: "credit_card_authorization",
    name: "Credit Card Authorization Form",
    description: "Placeholder",
    helper: "Reserved for a future phase.",
  },
  {
    key: "waiver_cancellation",
    name: "Waiver of Cancellation Period",
    description: "Placeholder",
    helper: "Reserved for a future phase.",
  },
  {
    key: "follow_up_move",
    name: "Follow-Up on Your Move",
    description: "Placeholder",
    helper: "Reserved for a future phase.",
  },
];

export function EstimateEmailCenter() {
  const { estimate } = useEstimateWorkspace();
  const [selectedTemplate, setSelectedTemplate] = useState<EstimateEmailTemplateKey>("moving_estimate");
  const [toEmail, setToEmail] = useState(estimate.email ?? "");
  const [ccMe, setCcMe] = useState(false);

  const [emails, setEmails] = useState<EstimateEmailLog[]>([]);
  const [loading, setLoading] = useState(true);
  const [state, setState] = useState<SaveState>("idle");
  const [statusMessage, setStatusMessage] = useState("Loading email history...");
  const [lastResult, setLastResult] = useState<SendEstimateEmailResponse | null>(null);

  const selected = useMemo(
    () => TEMPLATE_OPTIONS.find((option) => option.key === selectedTemplate) ?? TEMPLATE_OPTIONS[0],
    [selectedTemplate],
  );

  const loadHistory = useCallback(async () => {
    setLoading(true);
    setStatusMessage("Loading email history...");
    try {
      const response = await listEstimateEmails(estimate.id);
      setEmails(response.emails);
      setStatusMessage("Ready to send");
    } catch (error) {
      setStatusMessage(getApiErrorMessage(error));
    } finally {
      setLoading(false);
    }
  }, [estimate.id]);

  useEffect(() => {
    void loadHistory();
  }, [loadHistory]);

  async function handleSendEmail() {
    setState("sending");
    setStatusMessage("Sending email...");

    try {
      const response = await sendEstimateEmail(estimate.id, {
        templateKey: selectedTemplate,
        toEmail: toEmail.trim() || undefined,
        ccMe,
      });
      setLastResult(response);
      setEmails((previous) => [response.email, ...previous]);
      setState("sent");
      setStatusMessage(response.email.status === "failed" ? "Email send failed" : "Email sent");

      if (response.email.status === "failed") {
        toast.error(response.email.errorMessage ?? "Email failed");
      } else {
        toast.success("Email sent");
      }
    } catch (error) {
      setState("error");
      setStatusMessage("Email send failed");
      toast.error(getApiErrorMessage(error));
    }
  }

  return (
    <div className="space-y-4">
      <Card className="border-border/70 bg-card/70">
        <CardContent className="flex flex-wrap items-center justify-between gap-3 p-4">
          <div>
            <p className="text-sm font-medium">Email Center</p>
            <p className="text-xs text-muted-foreground" aria-live="polite" role="status">
              {statusMessage}
            </p>
          </div>
          <Button onClick={handleSendEmail} disabled={state === "sending" || loading}>
            {state === "sending" ? (
              <span className="inline-flex items-center gap-2">
                <Loader2 className="h-4 w-4 animate-spin" />
                Sending...
              </span>
            ) : (
              <span className="inline-flex items-center gap-2">
                <Send className="h-4 w-4" />
                Send Email
              </span>
            )}
          </Button>
        </CardContent>
      </Card>

      <Card className="border-border/70 bg-card/70">
        <CardHeader className="pb-3">
          <CardTitle className="text-base">Templates</CardTitle>
          <CardDescription>Select a transactional template and send to the customer.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-2">
            {TEMPLATE_OPTIONS.map((option) => {
              const active = option.key === selectedTemplate;
              return (
                <button
                  key={option.key}
                  type="button"
                  className={`rounded-md border px-3 py-3 text-left transition-colors ${
                    active
                      ? "border-primary bg-primary/10"
                      : "border-border/70 bg-muted/10 hover:bg-muted/20"
                  }`}
                  onClick={() => setSelectedTemplate(option.key)}
                >
                  <div className="flex items-center justify-between gap-3">
                    <p className="text-sm font-medium">{option.name}</p>
                    <p className="text-xs uppercase tracking-wide text-muted-foreground">{option.description}</p>
                  </div>
                  <p className="mt-1 text-xs text-muted-foreground">{option.helper}</p>
                </button>
              );
            })}
          </div>

          <div className="grid gap-4 rounded-md border border-border/70 bg-muted/10 p-4 md:grid-cols-2">
            <div className="space-y-2">
              <Label htmlFor="email-to">Email to</Label>
              <Input
                id="email-to"
                type="email"
                value={toEmail}
                onChange={(event) => setToEmail(event.target.value)}
                placeholder="customer@example.com"
              />
            </div>
            <div className="flex items-end">
              <label htmlFor="cc-me" className="inline-flex cursor-pointer items-center gap-2 text-sm">
                <Checkbox
                  id="cc-me"
                  checked={ccMe}
                  onCheckedChange={(checked) => setCcMe(checked === true)}
                />
                CC me on this email
              </label>
            </div>
          </div>

          <div className="rounded-md border border-border/70 bg-muted/10 p-3 text-sm">
            <p className="font-medium">Selected template: {selected.name}</p>
            <p className="mt-1 text-xs text-muted-foreground">Delivery target: {toEmail || estimate.email || "Not set"}</p>
          </div>

          {lastResult?.generatedLinks ? (
            <div className="rounded-md border border-border/70 bg-muted/10 p-3 text-sm">
              <p className="mb-2 font-medium">Last generated links</p>
              {lastResult.generatedLinks.quoteUrl ? <LinkRow label="E-Quote" url={lastResult.generatedLinks.quoteUrl} /> : null}
              {lastResult.generatedLinks.inventoryUrl ? (
                <LinkRow label="Inventory" url={lastResult.generatedLinks.inventoryUrl} />
              ) : null}
              {lastResult.generatedLinks.signatureUrl ? (
                <LinkRow label="E-Sign" url={lastResult.generatedLinks.signatureUrl} />
              ) : null}
            </div>
          ) : null}
        </CardContent>
      </Card>

      <Card className="border-border/70 bg-card/70">
        <CardHeader className="pb-3">
          <CardTitle className="text-base">Previous Emails</CardTitle>
          <CardDescription>Transactional email history for this estimate.</CardDescription>
        </CardHeader>
        <CardContent>
          {loading ? (
            <div className="flex items-center gap-2 py-8 text-sm text-muted-foreground">
              <Loader2 className="h-4 w-4 animate-spin" />
              Loading email history...
            </div>
          ) : emails.length === 0 ? (
            <div className="py-8 text-sm text-muted-foreground">No emails sent yet.</div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full min-w-[680px] text-sm">
                <thead>
                  <tr className="border-b border-border/70 text-left text-xs uppercase tracking-wide text-muted-foreground">
                    <th className="px-2 py-2">Sent At</th>
                    <th className="px-2 py-2">Subject</th>
                    <th className="px-2 py-2">To</th>
                    <th className="px-2 py-2">Template</th>
                    <th className="px-2 py-2">Status</th>
                  </tr>
                </thead>
                <tbody>
                  {emails.map((email) => (
                    <tr key={email.id} className="border-b border-border/50 align-top">
                      <td className="px-2 py-2 text-muted-foreground">{new Date(email.createdAt).toLocaleString()}</td>
                      <td className="px-2 py-2">
                        <p className="font-medium">{email.subject}</p>
                        {email.errorMessage ? (
                          <p className="mt-1 text-xs text-destructive">{email.errorMessage}</p>
                        ) : null}
                      </td>
                      <td className="px-2 py-2">{email.to}</td>
                      <td className="px-2 py-2">{email.templateKey}</td>
                      <td className="px-2 py-2">
                        <span
                          className={`rounded-full px-2 py-1 text-xs font-medium ${
                            email.status === "sent"
                              ? "bg-emerald-500/15 text-emerald-700"
                              : email.status === "failed"
                                ? "bg-destructive/10 text-destructive"
                                : "bg-muted text-muted-foreground"
                          }`}
                        >
                          {email.status}
                        </span>
                        <p className="mt-1 text-xs text-muted-foreground">{email.deliveryMode}</p>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

function LinkRow({ label, url }: { label: string; url: string }) {
  return (
    <p className="mb-1 flex items-center gap-2">
      <Mail className="h-3.5 w-3.5 text-muted-foreground" />
      <span className="text-xs font-medium uppercase tracking-wide text-muted-foreground">{label}:</span>
      <a href={url} target="_blank" rel="noreferrer" className="truncate text-sm text-primary underline-offset-4 hover:underline">
        {url}
      </a>
    </p>
  );
}
