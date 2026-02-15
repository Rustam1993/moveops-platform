"use client";

import Link from "next/link";
import { useCallback, useEffect, useMemo, useState } from "react";
import { toast } from "sonner";

import { NotAuthorizedState } from "@/components/layout/not-authorized-state";
import { PageHeader } from "@/components/layout/page-header";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { isForbiddenError } from "@/lib/api";
import {
  getApiErrorMessage,
  getJobsList,
  type JobListItem,
  type JobListJobType,
  type JobListStatus,
} from "@/lib/phase2-api";
import { cn } from "@/lib/utils";
import { useRouter } from "next/navigation";

const statusOptions: Array<{ label: string; value: JobListStatus | "all" }> = [
  { label: "All statuses", value: "all" },
  { label: "Booked", value: "booked" },
  { label: "Scheduled", value: "scheduled" },
  { label: "Completed", value: "completed" },
  { label: "Cancelled", value: "cancelled" },
];

const scheduledOptions = [
  { label: "All jobs", value: "all" as const },
  { label: "Scheduled only", value: "scheduled" as const },
  { label: "Unscheduled only", value: "unscheduled" as const },
];

const jobTypeOptions: Array<{ label: string; value: JobListJobType | "all" }> = [
  { label: "All job types", value: "all" },
  { label: "Local", value: "local" },
  { label: "Long distance", value: "long_distance" },
  { label: "Other", value: "other" },
];

export default function JobsListPage() {
  const router = useRouter();
  const [forbidden, setForbidden] = useState(false);

  const [qInput, setQInput] = useState("");
  const [q, setQ] = useState("");
  const [status, setStatus] = useState<JobListStatus | "all">("all");
  const [scheduledFilter, setScheduledFilter] = useState<(typeof scheduledOptions)[number]["value"]>("all");
  const [jobType, setJobType] = useState<JobListJobType | "all">("all");

  const [loading, setLoading] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [items, setItems] = useState<JobListItem[]>([]);
  const [nextCursor, setNextCursor] = useState<string | null>(null);

  useEffect(() => {
    const timer = setTimeout(() => setQ(qInput.trim()), 300);
    return () => clearTimeout(timer);
  }, [qInput]);

  const scheduled = useMemo(() => {
    if (scheduledFilter === "scheduled") return true;
    if (scheduledFilter === "unscheduled") return false;
    return undefined;
  }, [scheduledFilter]);

  const load = useCallback(
    async (options?: { append?: boolean; cursor?: string }) => {
      const append = options?.append ?? false;
      if (append) setLoadingMore(true);
      else setLoading(true);

      try {
        const response = await getJobsList({
          q: q || undefined,
          status: status === "all" ? undefined : status,
          jobType: jobType === "all" ? undefined : jobType,
          scheduled,
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
    [jobType, q, scheduled, status],
  );

  useEffect(() => {
    void load();
  }, [load]);

  if (forbidden) {
    return (
      <div className="space-y-6">
        <PageHeader title="Jobs" description="View jobs created from estimates." />
        <NotAuthorizedState message="You need job permissions to view this page." />
      </div>
    );
  }

  return (
    <div className="space-y-6 pb-8">
      <PageHeader
        title="Jobs"
        description="Jobs created from estimates. Use this list to find any job, even if it is not scheduled yet."
        actions={
          <Button asChild>
            <Link href="/estimates/new">New estimate</Link>
          </Button>
        }
      />

      <section className="grid gap-3 rounded-xl border border-border/70 bg-card/40 p-4 md:grid-cols-2 xl:grid-cols-5">
        <div className="space-y-2 xl:col-span-2">
          <Label htmlFor="q">Search</Label>
          <Input
            id="q"
            value={qInput}
            onChange={(e) => setQInput(e.target.value)}
            placeholder="Search by job # or customer name"
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor="status">Status</Label>
          <select
            id="status"
            value={status}
            onChange={(e) => setStatus(e.target.value as JobListStatus | "all")}
            className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            {statusOptions.map((opt) => (
              <option key={opt.value} value={opt.value}>
                {opt.label}
              </option>
            ))}
          </select>
        </div>
        <div className="space-y-2">
          <Label htmlFor="scheduled">Scheduled</Label>
          <select
            id="scheduled"
            value={scheduledFilter}
            onChange={(e) => setScheduledFilter(e.target.value as typeof scheduledFilter)}
            className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            {scheduledOptions.map((opt) => (
              <option key={opt.value} value={opt.value}>
                {opt.label}
              </option>
            ))}
          </select>
        </div>
        <div className="space-y-2">
          <Label htmlFor="jobType">Job type</Label>
          <select
            id="jobType"
            value={jobType}
            onChange={(e) => setJobType(e.target.value as JobListJobType | "all")}
            className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            {jobTypeOptions.map((opt) => (
              <option key={opt.value} value={opt.value}>
                {opt.label}
              </option>
            ))}
          </select>
        </div>
      </section>

      <section className="overflow-hidden rounded-xl border border-border/70 bg-card/60">
        <div className="overflow-x-auto">
          <table className="min-w-[1050px] w-full text-sm">
            <thead className="bg-muted/20 text-xs uppercase tracking-wide text-muted-foreground">
              <tr>
                <HeaderCell>Job #</HeaderCell>
                <HeaderCell>Customer</HeaderCell>
                <HeaderCell>Status</HeaderCell>
                <HeaderCell>Scheduled</HeaderCell>
                <HeaderCell>From → To</HeaderCell>
                <HeaderCell className="text-right">Balance due</HeaderCell>
              </tr>
            </thead>
            <tbody>
              {loading ? (
                <TableSkeleton rows={8} cols={6} />
              ) : items.length === 0 ? (
                <tr>
                  <td colSpan={6} className="px-4 py-10 text-center text-sm text-muted-foreground">
                    No jobs found.
                  </td>
                </tr>
              ) : (
                items.map((job) => (
                  <tr
                    key={job.jobId}
                    tabIndex={0}
                    role="button"
                    onClick={() => router.push(`/jobs/${job.jobId}`)}
                    onKeyDown={(event) => {
                      if (event.key === "Enter" || event.key === " ") {
                        event.preventDefault();
                        router.push(`/jobs/${job.jobId}`);
                      }
                    }}
                    className="cursor-pointer border-t border-border/60 hover:bg-muted/20 focus:bg-muted/20 focus:outline-none"
                  >
                    <Cell className="font-medium">{job.jobNumber}</Cell>
                    <Cell>{job.customerName}</Cell>
                    <Cell>
                      <StatusPill value={job.status} />
                      {job.hasStorage ? <span className="ml-2 rounded bg-muted px-2 py-0.5 text-[10px]">STORAGE</span> : null}
                    </Cell>
                    <Cell>{job.scheduledDate ?? <span className="text-muted-foreground">Unscheduled</span>}</Cell>
                    <Cell>
                      <span className="text-muted-foreground">{job.originShort}</span> →{" "}
                      <span className="text-muted-foreground">{job.destinationShort}</span>
                    </Cell>
                    <Cell className="text-right tabular-nums">{formatMoney(job.balanceDueCents)}</Cell>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>

        {nextCursor ? (
          <div className="flex items-center justify-center border-t border-border/60 p-3">
            <Button
              variant="outline"
              disabled={loadingMore}
              onClick={() => void load({ append: true, cursor: nextCursor })}
            >
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
  const className =
    value === "booked"
      ? "bg-blue-500/15 text-blue-200"
      : value === "scheduled"
        ? "bg-amber-500/15 text-amber-200"
        : value === "completed"
          ? "bg-emerald-500/15 text-emerald-200"
          : "bg-rose-500/15 text-rose-200";

  return <span className={cn("inline-flex rounded px-2 py-0.5 text-xs capitalize", className)}>{value.replace("_", " ")}</span>;
}

function formatMoney(cents: number) {
  const dollars = cents / 100;
  return new Intl.NumberFormat("en-US", { style: "currency", currency: "USD" }).format(dollars);
}

