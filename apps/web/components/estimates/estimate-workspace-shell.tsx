"use client";

import Link from "next/link";
import { Mail, MessageSquare, ShieldAlert } from "lucide-react";

import { EstimateWorkflowSidebar } from "@/components/estimates/estimate-workflow-sidebar";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { cn } from "@/lib/utils";
import {
  estimateTabHref,
  estimateWorkspaceTabs,
  type EstimateWorkspaceTabKey,
} from "@/lib/estimate-workspace";
import type { Estimate } from "@/lib/phase2-api";

type Props = {
  mode: "new" | "existing";
  activeTab: EstimateWorkspaceTabKey;
  estimate?: Pick<Estimate, "id" | "estimateNumber" | "customerName" | "status" | "totalVolumeCf" | "locationType" | "estimatedTotalCents">;
  children: React.ReactNode;
};

const statusLabel: Record<string, string> = {
  draft: "Draft",
  converted: "Converted",
};

export function EstimateWorkspaceShell({ mode, activeTab, estimate, children }: Props) {
  const isNew = mode === "new";

  return (
    <div className="space-y-5 pb-8">
      <div className="sticky top-16 z-20 space-y-3 bg-background/95 pb-1 backdrop-blur">
        <Card className="border-border/70 bg-card/70">
          <CardContent className="flex flex-wrap items-start justify-between gap-4 p-4">
            <div className="space-y-3">
              <div>
                <p className="text-xs uppercase tracking-wide text-muted-foreground">Estimate workspace</p>
                <h1 className="text-xl font-semibold tracking-tight">
                  {estimate?.estimateNumber ? `Estimate ${estimate.estimateNumber}` : "New estimate"}
                </h1>
              </div>
              <div className="grid gap-2 text-sm md:grid-cols-2">
                <MetaItem label="Customer" value={estimate?.customerName || "Not saved yet"} />
                <MetaItem label="Status" value={estimate?.status ? (statusLabel[estimate.status] ?? estimate.status) : "Draft"} />
              </div>
            </div>

            <div className="flex flex-wrap items-center justify-end gap-2">
              <select
                aria-label="Estimate status"
                disabled
                value={estimate?.status ?? "draft"}
                className="flex h-10 min-w-36 rounded-md border border-input bg-transparent px-3 py-2 text-sm text-muted-foreground shadow-sm disabled:cursor-not-allowed disabled:opacity-80"
              >
                <option value="draft">Draft</option>
                <option value="converted">Converted</option>
              </select>
              <Button variant="outline" disabled>
                <ShieldAlert className="h-4 w-4" />
                Priority
              </Button>
              <Button variant="outline" disabled>
                Preview
              </Button>
              <Button variant="outline" disabled>
                <Mail className="h-4 w-4" />
                Email
              </Button>
              <Button variant="outline" disabled>
                <MessageSquare className="h-4 w-4" />
                SMS
              </Button>
            </div>
          </CardContent>
        </Card>

        <Card className="border-border/70 bg-card/70">
          <CardContent className="overflow-x-auto p-2">
            <div className="flex min-w-max gap-1" role="tablist" aria-label="Estimate workspace tabs">
              {estimateWorkspaceTabs.map((tab) => {
                const isActive = tab.key === activeTab;
                const disabled = isNew && tab.key !== "entry";

                if (disabled) {
                  return (
                    <button
                      key={tab.key}
                      type="button"
                      disabled
                      className={cn(
                        "rounded-md px-3 py-2 text-sm font-medium text-muted-foreground",
                        isActive ? "bg-primary/10 text-primary" : "hover:bg-muted/40",
                      )}
                    >
                      {tab.label}
                    </button>
                  );
                }

                if (!estimate?.id) {
                  return (
                    <span
                      key={tab.key}
                      className={cn(
                        "rounded-md px-3 py-2 text-sm font-medium",
                        isActive ? "bg-primary text-primary-foreground shadow-sm" : "text-muted-foreground",
                      )}
                    >
                      {tab.label}
                    </span>
                  );
                }

                return (
                  <Link
                    key={tab.key}
                    href={estimateTabHref(estimate.id, tab.segment)}
                    className={cn(
                      "rounded-md px-3 py-2 text-sm font-medium transition-colors",
                      isActive
                        ? "bg-primary text-primary-foreground shadow-sm"
                        : "text-muted-foreground hover:bg-muted/50 hover:text-foreground",
                    )}
                  >
                    {tab.label}
                  </Link>
                );
              })}
            </div>
          </CardContent>
        </Card>
      </div>

      <div className="grid gap-5 xl:grid-cols-[minmax(0,1fr)_320px]">
        <main className="min-w-0">{children}</main>
        <aside>
          <EstimateWorkflowSidebar mode={mode} estimate={estimate} />
        </aside>
      </div>
    </div>
  );
}

function MetaItem({ label, value }: { label: string; value: string }) {
  return (
    <p className="text-sm">
      <span className="text-muted-foreground">{label}: </span>
      <span className="font-medium">{value}</span>
    </p>
  );
}
