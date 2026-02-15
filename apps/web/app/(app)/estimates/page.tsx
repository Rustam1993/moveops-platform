"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";

import { NotAuthorizedState } from "@/components/layout/not-authorized-state";
import { PageHeader } from "@/components/layout/page-header";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { isForbiddenError } from "@/lib/api";
import { getApiErrorMessage, getEstimatesList, type EstimateListItem, type EstimateListStatus } from "@/lib/phase2-api";
import { cn } from "@/lib/utils";
import { useRouter } from "next/navigation";

const statusOptions: Array<{ label: string; value: EstimateListStatus | "all" }> = [
  { label: "All statuses", value: "all" },
  { label: "Draft", value: "draft" },
  { label: "Converted", value: "converted" },
];

export default function EstimatesListPage() {
  const router = useRouter();
  const [forbidden, setForbidden] = useState(false);

  const [qInput, setQInput] = useState("");
  const [q, setQ] = useState("");
  const [status, setStatus] = useState<EstimateListStatus | "all">("all");

  const [loading, setLoading] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [items, setItems] = useState<EstimateListItem[]>([]);
  const [nextCursor, setNextCursor] = useState<string | null>(null);

  useEffect(() => {
    const timer = setTimeout(() => setQ(qInput.trim()), 300);
    return () => clearTimeout(timer);
  }, [qInput]);

  const load = useCallback(
    async (options?: { append?: boolean; cursor?: string }) => {
      const append = options?.append ?? false;
      if (append) setLoadingMore(true);
      else setLoading(true);

      try {
        const response = await getEstimatesList({
          q: q || undefined,
          status: status === "all" ? undefined : status,
          limit: 25,
          cursor: options?.cursor,
        });

        setItems((prev) => (append ? [...prev, ...response.items] : response.items));
        setNextCursor(response.nextCursor ?? null);
        setForbidden(false);
      } catch (error) {
        if (isForbiddenError(error)) {
          setForbidden(true);
        } else {
          toast.error(getApiErrorMessage(error));
        }
      } finally {
        if (append) setLoadingMore(false);
        else setLoading(false);
      }
    },
    [q, status],
  );

  useEffect(() => {
    void load();
  }, [load]);

  if (forbidden) {
    return (
      <div className="space-y-6">
        <PageHeader title="Estimates" description="View saved estimates." />
        <NotAuthorizedState message="You need estimate permissions to view this page." />
      </div>
    );
  }

  return (
    <div className="space-y-6 pb-8">
      <PageHeader
        title="Estimates"
        description="Draft and converted estimates for this tenant."
        actions={
          <Button asChild>
            <Link href="/estimates/new">New estimate</Link>
          </Button>
        }
      />

      <section className="grid gap-3 rounded-xl border border-border/70 bg-card/40 p-4 md:grid-cols-2 xl:grid-cols-3">
        <div className="space-y-2 xl:col-span-2">
          <Label htmlFor="q">Search</Label>
          <Input
            id="q"
            value={qInput}
            onChange={(e) => setQInput(e.target.value)}
            placeholder="Search by estimate #, customer, email, or phone"
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor="status">Status</Label>
          <select
            id="status"
            value={status}
            onChange={(e) => setStatus(e.target.value as EstimateListStatus | "all")}
            className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            {statusOptions.map((opt) => (
              <option key={opt.value} value={opt.value}>
                {opt.label}
              </option>
            ))}
          </select>
        </div>
      </section>

      <section className="overflow-hidden rounded-xl border border-border/70 bg-card/60">
        <div className="overflow-x-auto">
          <table className="min-w-[950px] w-full text-sm">
            <thead className="bg-muted/20 text-xs uppercase tracking-wide text-muted-foreground">
              <tr>
                <HeaderCell>Estimate #</HeaderCell>
                <HeaderCell>Customer</HeaderCell>
                <HeaderCell>Status</HeaderCell>
                <HeaderCell>Move date</HeaderCell>
                <HeaderCell>Converted job</HeaderCell>
              </tr>
            </thead>
            <tbody>
              {loading ? (
                <TableSkeleton rows={8} cols={5} />
              ) : items.length === 0 ? (
                <tr>
                  <td colSpan={5} className="px-4 py-10 text-center text-sm text-muted-foreground">
                    No estimates found.
                  </td>
                </tr>
              ) : (
                items.map((estimate) => (
                  <tr
                    key={estimate.estimateId}
                    tabIndex={0}
                    role="button"
                    onClick={() => router.push(`/estimates/${estimate.estimateId}`)}
                    onKeyDown={(event) => {
                      if (event.key === "Enter" || event.key === " ") {
                        event.preventDefault();
                        router.push(`/estimates/${estimate.estimateId}`);
                      }
                    }}
                    className="cursor-pointer border-t border-border/60 hover:bg-muted/20 focus:bg-muted/20 focus:outline-none"
                  >
                    <Cell className="font-medium">{estimate.estimateNumber}</Cell>
                    <Cell>
                      <div className="font-medium">{estimate.customerName}</div>
                      <div className="text-xs text-muted-foreground">{estimate.email ?? "-"}</div>
                    </Cell>
                    <Cell>
                      <StatusPill value={estimate.status} />
                    </Cell>
                    <Cell>{estimate.moveDate}</Cell>
                    <Cell>{estimate.convertedJobId ? <span className="font-medium">Yes</span> : <span className="text-muted-foreground">No</span>}</Cell>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>

        {nextCursor ? (
          <div className="flex items-center justify-center border-t border-border/60 p-3">
            <Button variant="outline" disabled={loadingMore} onClick={() => void load({ append: true, cursor: nextCursor })}>
              {loadingMore ? "Loading..." : "Load more"}
            </Button>
          </div>
        ) : null}
      </section>
    </div>
  );
}

function HeaderCell({ className, children }: { className?: string; children: React.ReactNode }) {
  return <th className={cn("px-4 py-3 text-left font-semibold", className)}>{children}</th>;
}

function Cell({ className, children }: { className?: string; children: React.ReactNode }) {
  return <td className={cn("px-4 py-3 align-top", className)}>{children}</td>;
}

function TableSkeleton({ rows, cols }: { rows: number; cols: number }) {
  return (
    <>
      {Array.from({ length: rows }).map((_, idx) => (
        <tr key={idx} className="border-t border-border/60">
          {Array.from({ length: cols }).map((__, cellIdx) => (
            <td key={cellIdx} className="px-4 py-3">
              <Skeleton className="h-4 w-full" />
            </td>
          ))}
        </tr>
      ))}
    </>
  );
}

function StatusPill({ value }: { value: string }) {
  const className = value === "draft" ? "bg-blue-500/15 text-blue-200" : "bg-emerald-500/15 text-emerald-200";
  return <span className={cn("inline-flex rounded px-2 py-0.5 text-xs capitalize", className)}>{value}</span>;
}

