"use client";

import Link from "next/link";
import { CheckCircle2, CircleAlert } from "lucide-react";

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { estimateTabHref } from "@/lib/estimate-workspace";
import type { Estimate } from "@/lib/phase2-api";

type Props = {
  mode: "new" | "existing";
  estimate?: Estimate;
};

type ReadinessItem = {
  key: "contact" | "move_details" | "inventory" | "charges";
  label: string;
  complete: boolean;
  href?: string;
};

export function EstimateReadinessChecklist({ mode, estimate }: Props) {
  const isExisting = mode === "existing" && !!estimate?.id;
  const items: ReadinessItem[] = [
    {
      key: "contact",
      label: "Customer contact is complete",
      complete: !!estimate?.customerName && !!estimate?.primaryPhone && !!estimate?.email,
      href: isExisting && estimate?.id ? estimateTabHref(estimate.id, "entry") : undefined,
    },
    {
      key: "move_details",
      label: "Move details are complete",
      complete:
        !!estimate?.originAddressLine1 &&
        !!estimate?.originCity &&
        !!estimate?.originState &&
        !!estimate?.originPostalCode &&
        !!estimate?.destinationAddressLine1 &&
        !!estimate?.destinationCity &&
        !!estimate?.destinationState &&
        !!estimate?.destinationPostalCode &&
        !!estimate?.moveDate,
      href: isExisting && estimate?.id ? estimateTabHref(estimate.id, "entry") : undefined,
    },
    {
      key: "inventory",
      label: "Inventory has been captured",
      complete: (estimate?.totalVolumeCf ?? 0) > 0,
      href: isExisting && estimate?.id ? estimateTabHref(estimate.id, "inventory") : undefined,
    },
    {
      key: "charges",
      label: "Pricing has been finalized",
      complete: (estimate?.estimatedTotalCents ?? 0) > 0,
      href: isExisting && estimate?.id ? estimateTabHref(estimate.id, "charges") : undefined,
    },
  ];

  const completeCount = items.filter((item) => item.complete).length;
  const remainingCount = items.length - completeCount;
  const ready = remainingCount === 0;

  return (
    <Card className="border-border/70 bg-card/70">
      <CardHeader className="pb-3">
        <CardTitle className="text-base">Estimate readiness</CardTitle>
        <CardDescription data-testid="readiness-state">
          {ready ? "Ready to send quote" : `${remainingCount} checks remaining`}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-2">
        {items.map((item) => (
          <div
            key={item.key}
            className="flex items-center justify-between gap-3 rounded-md border border-border/60 bg-muted/10 px-3 py-2 text-sm"
            data-testid={`readiness-item-${item.key}`}
          >
            <div className="flex min-w-0 items-center gap-2">
              {item.complete ? (
                <CheckCircle2 className="h-4 w-4 shrink-0 text-emerald-600" />
              ) : (
                <CircleAlert className="h-4 w-4 shrink-0 text-amber-600" />
              )}
              <span className="truncate">{item.label}</span>
            </div>
            <div className="flex items-center gap-2">
              <span
                className={`rounded-full px-2 py-0.5 text-xs font-medium ${
                  item.complete ? "bg-emerald-500/15 text-emerald-700" : "bg-amber-500/15 text-amber-700"
                }`}
              >
                {item.complete ? "Complete" : "Missing"}
              </span>
              {!item.complete && item.href ? (
                <Link className="text-xs font-medium text-primary hover:underline" href={item.href}>
                  Open
                </Link>
              ) : null}
            </div>
          </div>
        ))}
      </CardContent>
    </Card>
  );
}
