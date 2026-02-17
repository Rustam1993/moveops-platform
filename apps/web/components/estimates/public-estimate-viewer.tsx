"use client";

import { useCallback, useEffect, useState } from "react";
import { Download, FileText, Loader2 } from "lucide-react";

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { ApiError } from "@/lib/api";
import { getPublicEstimate, type PublicEstimateDocumentResponse } from "@/lib/phase4-api";
import { getApiErrorMessage } from "@/lib/phase2-api";

export function PublicEstimateViewer({ token }: { token: string }) {
  const [payload, setPayload] = useState<PublicEstimateDocumentResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const loadPayload = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const response = await getPublicEstimate(token);
      setPayload(response);
    } catch (err) {
      if (err instanceof ApiError && err.code === "quote_share_expired") {
        setError("This estimate link has expired. Please request a new estimate email.");
      } else if (err instanceof ApiError && err.code === "quote_share_not_found") {
        setError("This estimate link is invalid.");
      } else {
        setError(getApiErrorMessage(err));
      }
    } finally {
      setLoading(false);
    }
  }, [token]);

  useEffect(() => {
    void loadPayload();
  }, [loadPayload]);

  function handleDownload() {
    if (!payload) return;
    const blob = base64ToBlob(payload.document.contentBase64, payload.document.mimeType);
    const url = URL.createObjectURL(blob);
    const anchor = window.document.createElement("a");
    anchor.href = url;
    anchor.download = payload.document.fileName;
    anchor.click();
    URL.revokeObjectURL(url);
  }

  if (loading) {
    return (
      <div className="mx-auto max-w-6xl p-6">
        <Card className="border-border/70 bg-card/80">
          <CardContent className="flex h-48 items-center justify-center text-sm text-muted-foreground">
            <Loader2 className="mr-2 h-4 w-4 animate-spin" />
            Loading estimate...
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
            <CardTitle>Estimate link unavailable</CardTitle>
            <CardDescription>{error ?? "Unable to load estimate."}</CardDescription>
          </CardHeader>
        </Card>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-7xl space-y-4 p-4 md:p-6">
      <Card className="border-border/70 bg-card/80">
        <CardContent className="flex flex-wrap items-start justify-between gap-4 p-4">
          <div className="space-y-1">
            <p className="text-xs uppercase tracking-wide text-muted-foreground">E-Quote</p>
            <h1 className="text-xl font-semibold tracking-tight">Your Moving Estimate</h1>
            <p className="text-sm text-muted-foreground">{payload.customerName}</p>
            <p className="text-xs text-muted-foreground">
              Move date: {payload.moveDate} • Expires: {payload.expiresAt.slice(0, 10)}
            </p>
          </div>
          <div className="space-y-2 text-right">
            <p className="text-xs uppercase tracking-wide text-muted-foreground">Volume</p>
            <p className="text-lg font-semibold">{payload.totalVolumeCf.toFixed(2)} cf</p>
            <button
              type="button"
              onClick={handleDownload}
              className="inline-flex items-center gap-2 rounded-md border border-border/70 px-3 py-2 text-sm hover:bg-muted/30"
            >
              <Download className="h-4 w-4" />
              Download PDF
            </button>
          </div>
        </CardContent>
      </Card>

      <Card className="border-border/70 bg-card/80">
        <CardHeader>
          <CardTitle className="text-base">Estimate Preview</CardTitle>
          <CardDescription>
            <span className="inline-flex items-center gap-2">
              <FileText className="h-4 w-4" />
              {payload.document.fileName}
            </span>
          </CardDescription>
        </CardHeader>
        <CardContent>
          <iframe
            title="Estimate PDF"
            src={`data:${payload.document.mimeType};base64,${payload.document.contentBase64}`}
            className="h-[780px] w-full rounded-md border border-border/70"
          />
        </CardContent>
      </Card>
    </div>
  );
}

function base64ToBlob(base64: string, mimeType: string) {
  const binary = atob(base64);
  const length = binary.length;
  const bytes = new Uint8Array(length);
  for (let i = 0; i < length; i += 1) {
    bytes[i] = binary.charCodeAt(i);
  }
  return new Blob([bytes], { type: mimeType });
}
