"use client";

import { useCallback, useEffect, useState } from "react";
import { Download, FileText, Loader2, RefreshCw } from "lucide-react";
import { toast } from "sonner";

import { useEstimateWorkspace } from "@/components/estimates/estimate-workspace-context";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { ApiError } from "@/lib/api";
import { generateEstimatePdf, getEstimatePdf, type EstimateDocument } from "@/lib/phase4-api";
import { getApiErrorMessage } from "@/lib/phase2-api";

type SaveState = "idle" | "loading" | "generated" | "error";

export function EstimatePrintedEstimateEditor() {
  const { estimate } = useEstimateWorkspace();
  const [document, setDocument] = useState<EstimateDocument | null>(null);
  const [state, setState] = useState<SaveState>("loading");
  const [statusMessage, setStatusMessage] = useState("Loading document...");
  const [actionLoading, setActionLoading] = useState(false);

  const loadDocument = useCallback(async () => {
    setState("loading");
    setStatusMessage("Loading document...");

    try {
      const response = await getEstimatePdf(estimate.id);
      setDocument(response.document);
      setState("generated");
      setStatusMessage("PDF ready");
    } catch (error) {
      if (error instanceof ApiError && error.code === "document_not_found") {
        setDocument(null);
        setState("idle");
        setStatusMessage("No PDF generated yet");
        return;
      }
      setState("error");
      setStatusMessage(getApiErrorMessage(error));
    }
  }, [estimate.id]);

  useEffect(() => {
    void loadDocument();
  }, [loadDocument]);

  async function handleGenerateClick() {
    setActionLoading(true);
    setStatusMessage("Generating PDF...");
    try {
      const response = await generateEstimatePdf(estimate.id);
      setDocument(response.document);
      setState("generated");
      setStatusMessage("PDF generated");
      toast.success("Printed estimate PDF generated");
    } catch (error) {
      setState("error");
      setStatusMessage("PDF generation failed");
      toast.error(getApiErrorMessage(error));
    } finally {
      setActionLoading(false);
    }
  }

  function handleDownloadClick() {
    if (!document) return;
    const blob = base64ToBlob(document.contentBase64, document.mimeType || "application/pdf");
    const url = URL.createObjectURL(blob);
    const anchor = documentCreateElement("a");
    anchor.href = url;
    anchor.download = document.fileName || `estimate-${estimate.estimateNumber}.pdf`;
    anchor.click();
    URL.revokeObjectURL(url);
  }

  return (
    <div className="space-y-4">
      <Card className="border-border/70 bg-card/70">
        <CardContent className="flex flex-wrap items-center justify-between gap-3 p-4">
          <div>
            <p className="text-sm font-medium">Printed Estimate</p>
            <p className="text-xs text-muted-foreground" aria-live="polite" role="status">
              {statusMessage}
            </p>
          </div>
          <div className="flex items-center gap-2">
            <Button variant="outline" onClick={() => void loadDocument()} disabled={state === "loading" || actionLoading}>
              <RefreshCw className="h-4 w-4" />
              Refresh
            </Button>
            <Button onClick={handleGenerateClick} disabled={actionLoading}>
              {actionLoading ? (
                <span className="inline-flex items-center gap-2">
                  <Loader2 className="h-4 w-4 animate-spin" />
                  Generating...
                </span>
              ) : (
                <span className="inline-flex items-center gap-2">
                  <FileText className="h-4 w-4" />
                  {document ? "Regenerate PDF" : "Generate PDF"}
                </span>
              )}
            </Button>
            <Button variant="secondary" onClick={handleDownloadClick} disabled={!document}>
              <Download className="h-4 w-4" />
              Download
            </Button>
          </div>
        </CardContent>
      </Card>

      {document ? (
        <Card className="border-border/70 bg-card/70">
          <CardHeader>
            <CardTitle className="text-base">Preview</CardTitle>
            <CardDescription>
              Last generated: {new Date(document.createdAt).toLocaleString()} ({(document.sizeBytes / 1024).toFixed(1)} KB)
            </CardDescription>
          </CardHeader>
          <CardContent>
            <iframe
              title="Printed estimate PDF preview"
              src={`data:${document.mimeType};base64,${document.contentBase64}`}
              className="h-[780px] w-full rounded-md border border-border/70"
            />
          </CardContent>
        </Card>
      ) : (
        <Card className="border-border/70 bg-card/70">
          <CardHeader>
            <CardTitle className="text-base">No document generated</CardTitle>
            <CardDescription>
              Generate the printed estimate to preview, download, and send as an e-quote.
            </CardDescription>
          </CardHeader>
        </Card>
      )}
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

function documentCreateElement<K extends keyof HTMLElementTagNameMap>(tagName: K): HTMLElementTagNameMap[K] {
  return window.document.createElement(tagName);
}
