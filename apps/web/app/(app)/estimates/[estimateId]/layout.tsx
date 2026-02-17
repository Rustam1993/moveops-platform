"use client";

import { useEffect, useMemo, useState } from "react";
import { useParams, usePathname } from "next/navigation";
import { toast } from "sonner";

import { EstimateWorkspaceProvider } from "@/components/estimates/estimate-workspace-context";
import { EstimateWorkspaceShell } from "@/components/estimates/estimate-workspace-shell";
import { NotAuthorizedState } from "@/components/layout/not-authorized-state";
import { Skeleton } from "@/components/ui/skeleton";
import { isForbiddenError } from "@/lib/api";
import { getApiErrorMessage, getEstimate, type Estimate } from "@/lib/phase2-api";
import type { EstimateWorkspaceTabKey } from "@/lib/estimate-workspace";

const segmentToTab: Record<string, EstimateWorkspaceTabKey> = {
  entry: "entry",
  inventory: "inventory",
  "items-not-moving": "items-not-moving",
  "printed-estimate": "printed-estimate",
  email: "email",
  charges: "charges",
  tasks: "tasks",
  payments: "payments",
  operations: "operations",
};

export default function EstimateWorkspaceLayout({ children }: { children: React.ReactNode }) {
  const params = useParams<{ estimateId: string }>();
  const pathname = usePathname();
  const estimateId = Array.isArray(params?.estimateId) ? params.estimateId[0] : params?.estimateId;

  const activeTab = useMemo<EstimateWorkspaceTabKey>(() => {
    const segment = pathname?.split("/").at(-1) ?? "entry";
    return segmentToTab[segment] ?? "entry";
  }, [pathname]);

  const [estimate, setEstimate] = useState<Estimate | null>(null);
  const [loading, setLoading] = useState(true);
  const [forbidden, setForbidden] = useState(false);

  useEffect(() => {
    if (!estimateId) return;
    let cancelled = false;
    setLoading(true);

    getEstimate(estimateId)
      .then((response) => {
        if (cancelled) return;
        setEstimate(response.estimate);
        setForbidden(false);
      })
      .catch((error) => {
        if (cancelled) return;
        if (isForbiddenError(error)) {
          setForbidden(true);
          return;
        }
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

  if (forbidden) {
    return <NotAuthorizedState message="You need estimate permissions to view this estimate workspace." />;
  }

  if (loading || !estimate) {
    return (
      <div className="space-y-4">
        <Skeleton className="h-44 w-full" />
        <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_320px]">
          <Skeleton className="h-[600px] w-full" />
          <Skeleton className="h-[280px] w-full" />
        </div>
      </div>
    );
  }

  return (
    <EstimateWorkspaceProvider value={{ estimate, setEstimate: (next) => setEstimate(next) }}>
      <EstimateWorkspaceShell mode="existing" activeTab={activeTab} estimate={estimate}>
        {children}
      </EstimateWorkspaceShell>
    </EstimateWorkspaceProvider>
  );
}
