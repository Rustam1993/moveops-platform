"use client";

import { useCallback, useEffect, useState } from "react";
import { Loader2, PenLine } from "lucide-react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ApiError } from "@/lib/api";
import {
  completePublicSign,
  getPublicSign,
  type CompleteSignatureResponse,
  type PublicSignContextResponse,
} from "@/lib/phase4-api";
import { getApiErrorMessage } from "@/lib/phase2-api";

export function PublicSignEstimate({ token }: { token: string }) {
  const [payload, setPayload] = useState<PublicSignContextResponse | null>(null);
  const [completion, setCompletion] = useState<CompleteSignatureResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [signerName, setSignerName] = useState("");
  const [signerEmail, setSignerEmail] = useState("");
  const [signatureText, setSignatureText] = useState("");
  const [agreeToTerms, setAgreeToTerms] = useState(false);

  const loadContext = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const response = await getPublicSign(token);
      setPayload(response);
      setSignerEmail(response.signerEmail);
    } catch (err) {
      if (err instanceof ApiError && err.code === "signature_request_expired") {
        setError("This signature request has expired. Please request a new signature email.");
      } else if (err instanceof ApiError && err.code === "signature_request_not_found") {
        setError("This signature request is invalid.");
      } else {
        setError(getApiErrorMessage(err));
      }
    } finally {
      setLoading(false);
    }
  }, [token]);

  useEffect(() => {
    void loadContext();
  }, [loadContext]);

  async function handleSubmit() {
    if (!payload) return;
    if (!signerName.trim() || !signatureText.trim() || !signerEmail.trim()) {
      toast.error("Name, email, and typed signature are required");
      return;
    }
    if (!agreeToTerms) {
      toast.error("You must agree to terms before signing");
      return;
    }

    setSubmitting(true);
    try {
      const response = await completePublicSign(token, {
        signerName: signerName.trim(),
        signerEmail: signerEmail.trim(),
        signatureText: signatureText.trim(),
        agreeToTerms,
      });
      setCompletion(response);
      setPayload((previous) =>
        previous
          ? {
              ...previous,
              alreadySigned: true,
              document: response.document,
            }
          : previous,
      );
      toast.success("Signature completed");
    } catch (err) {
      toast.error(getApiErrorMessage(err));
    } finally {
      setSubmitting(false);
    }
  }

  if (loading) {
    return (
      <div className="mx-auto max-w-6xl p-6">
        <Card className="border-border/70 bg-card/80">
          <CardContent className="flex h-48 items-center justify-center text-sm text-muted-foreground">
            <Loader2 className="mr-2 h-4 w-4 animate-spin" />
            Loading signature request...
          </CardContent>
        </Card>
      </div>
    );
  }

  if (error || !payload) {
    return (
      <div className="mx-auto max-w-3xl p-6">
        <Card className="border-border/70 bg-card/80">
          <CardHeader>
            <CardTitle>Signature request unavailable</CardTitle>
            <CardDescription>{error ?? "Unable to load this signature request."}</CardDescription>
          </CardHeader>
        </Card>
      </div>
    );
  }

  const signedAtLabel = completion ? new Date(completion.signedAt).toLocaleString() : null;

  return (
    <div className="mx-auto max-w-7xl space-y-4 p-4 md:p-6">
      <Card className="border-border/70 bg-card/80">
        <CardContent className="flex flex-wrap items-start justify-between gap-4 p-4">
          <div className="space-y-1">
            <p className="text-xs uppercase tracking-wide text-muted-foreground">E-Sign</p>
            <h1 className="text-xl font-semibold tracking-tight">Sign Your Moving Estimate</h1>
            <p className="text-sm text-muted-foreground">{payload.customerName}</p>
            <p className="text-xs text-muted-foreground">
              Move date: {payload.moveDate} • Expires: {payload.expiresAt.slice(0, 10)}
            </p>
          </div>
          <div className="text-right">
            <p className="text-xs uppercase tracking-wide text-muted-foreground">Total Volume</p>
            <p className="text-lg font-semibold">{payload.totalVolumeCf.toFixed(2)} cf</p>
          </div>
        </CardContent>
      </Card>

      {payload.alreadySigned ? (
        <Card className="border-border/70 bg-card/70">
          <CardHeader>
            <CardTitle>Estimate already signed</CardTitle>
            <CardDescription>
              {signedAtLabel ? `Signed at ${signedAtLabel}.` : "This signature request has already been completed."}
            </CardDescription>
          </CardHeader>
        </Card>
      ) : (
        <Card className="border-border/70 bg-card/70">
          <CardHeader>
            <CardTitle className="text-base">Signer details</CardTitle>
            <CardDescription>Type your legal name and signature, then confirm agreement.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid gap-4 md:grid-cols-2">
              <div className="space-y-2">
                <Label htmlFor="signer-name">Signer name</Label>
                <Input
                  id="signer-name"
                  value={signerName}
                  onChange={(event) => setSignerName(event.target.value)}
                  placeholder="Full legal name"
                  disabled={submitting}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="signer-email">Signer email</Label>
                <Input
                  id="signer-email"
                  type="email"
                  value={signerEmail}
                  onChange={(event) => setSignerEmail(event.target.value)}
                  placeholder="email@example.com"
                  disabled={submitting}
                />
              </div>
            </div>

            <div className="space-y-2">
              <Label htmlFor="signature-text">Typed signature</Label>
              <Input
                id="signature-text"
                value={signatureText}
                onChange={(event) => setSignatureText(event.target.value)}
                placeholder="Type your name as signature"
                disabled={submitting}
              />
            </div>

            <label htmlFor="agree-terms" className="inline-flex cursor-pointer items-center gap-2 text-sm">
              <Checkbox
                id="agree-terms"
                checked={agreeToTerms}
                onCheckedChange={(checked) => setAgreeToTerms(checked === true)}
                disabled={submitting}
              />
              I agree that this typed signature is my electronic signature.
            </label>

            <Button onClick={handleSubmit} disabled={submitting}>
              {submitting ? (
                <span className="inline-flex items-center gap-2">
                  <Loader2 className="h-4 w-4 animate-spin" />
                  Submitting...
                </span>
              ) : (
                <span className="inline-flex items-center gap-2">
                  <PenLine className="h-4 w-4" />
                  Sign Estimate
                </span>
              )}
            </Button>
          </CardContent>
        </Card>
      )}

      <Card className="border-border/70 bg-card/80">
        <CardHeader>
          <CardTitle className="text-base">Estimate Document</CardTitle>
          <CardDescription>Review the estimate before signing.</CardDescription>
        </CardHeader>
        <CardContent>
          <iframe
            title="Estimate document"
            src={`data:${payload.document.mimeType};base64,${payload.document.contentBase64}`}
            className="h-[760px] w-full rounded-md border border-border/70"
          />
        </CardContent>
      </Card>
    </div>
  );
}
