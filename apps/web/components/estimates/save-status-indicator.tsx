"use client";

import { AlertCircle, CheckCircle2, Loader2 } from "lucide-react";

import { Button } from "@/components/ui/button";

export type SaveStatusState = "idle" | "saving" | "saved" | "error";

export function SaveStatusIndicator({
  state,
  message,
  onRetry,
}: {
  state: SaveStatusState;
  message: string;
  onRetry?: () => void;
}) {
  const icon =
    state === "saving" ? (
      <Loader2 className="h-3.5 w-3.5 animate-spin" />
    ) : state === "saved" ? (
      <CheckCircle2 className="h-3.5 w-3.5" />
    ) : state === "error" ? (
      <AlertCircle className="h-3.5 w-3.5" />
    ) : (
      <span className="h-2 w-2 rounded-full bg-amber-500" />
    );

  const toneClassName =
    state === "saved"
      ? "text-emerald-700"
      : state === "error"
        ? "text-destructive"
        : state === "saving"
          ? "text-primary"
          : "text-muted-foreground";

  return (
    <div className="flex items-center gap-2">
      <p
        className={`inline-flex items-center gap-1.5 text-xs font-medium ${toneClassName}`}
        aria-live="polite"
        role="status"
      >
        {icon}
        {message}
      </p>
      {state === "error" && onRetry ? (
        <Button type="button" variant="ghost" size="sm" className="h-7 px-2 text-xs" onClick={onRetry}>
          Retry
        </Button>
      ) : null}
    </div>
  );
}
