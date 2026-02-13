"use client";

import { type ReactNode, useCallback, useEffect, useMemo, useState } from "react";
import { ChevronLeft, ChevronRight, Loader2 } from "lucide-react";
import { toast } from "sonner";

import { NotAuthorizedState } from "@/components/layout/not-authorized-state";
import { PageHeader } from "@/components/layout/page-header";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Sheet, SheetContent } from "@/components/ui/sheet";
import { Skeleton } from "@/components/ui/skeleton";
import {
  getApiErrorMessage,
  getCalendar,
  getJob,
  updateJob,
  type CalendarJobCard,
  type CalendarJobType,
  type CalendarPhase,
  type Job,
  type UpdateJobRequest,
} from "@/lib/phase2-api";
import { isForbiddenError } from "@/lib/api";

const weekdays = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];

const phaseOptions: Array<{ label: string; value: CalendarPhase | "all" }> = [
  { label: "Booked", value: "booked" },
  { label: "Scheduled", value: "scheduled" },
  { label: "Completed", value: "completed" },
  { label: "Cancelled", value: "cancelled" },
  { label: "All", value: "all" },
];

const jobTypeOptions: Array<{ label: string; value: CalendarJobType | "all" }> = [
  { label: "All job types", value: "all" },
  { label: "Local", value: "local" },
  { label: "Long distance", value: "long_distance" },
  { label: "Other", value: "other" },
];

type JobEditorState = {
  scheduledDate: string;
  pickupTime: string;
  status: Job["status"];
};

export default function CalendarPage() {
  const [monthCursor, setMonthCursor] = useState(() => startOfMonth(new Date()));
  const [phaseFilter, setPhaseFilter] = useState<CalendarPhase | "all">("booked");
  const [jobTypeFilter, setJobTypeFilter] = useState<CalendarJobType | "all">("all");

  const [loading, setLoading] = useState(true);
  const [jobs, setJobs] = useState<CalendarJobCard[]>([]);
  const [forbidden, setForbidden] = useState(false);

  const [drawerOpen, setDrawerOpen] = useState(false);
  const [selectedJobId, setSelectedJobId] = useState<string | null>(null);
  const [selectedJob, setSelectedJob] = useState<Job | null>(null);
  const [editor, setEditor] = useState<JobEditorState | null>(null);
  const [jobLoading, setJobLoading] = useState(false);
  const [jobSaving, setJobSaving] = useState(false);

  const dateRange = useMemo(() => {
    const from = startOfMonth(monthCursor);
    const to = startOfMonth(addMonths(from, 1));
    return { from, to, fromISO: formatDate(from), toISO: formatDate(to) };
  }, [monthCursor]);

  const loadCalendar = useCallback(
    async (showLoading = true) => {
      if (showLoading) setLoading(true);
      try {
        const response = await getCalendar({
          from: dateRange.fromISO,
          to: dateRange.toISO,
          phase: phaseFilter === "all" ? undefined : phaseFilter,
          jobType: jobTypeFilter === "all" ? undefined : jobTypeFilter,
        });
        setJobs(response.jobs);
        setForbidden(false);
      } catch (error) {
        if (isForbiddenError(error)) {
          setForbidden(true);
        } else {
          toast.error(getApiErrorMessage(error));
        }
      } finally {
        if (showLoading) setLoading(false);
      }
    },
    [dateRange.fromISO, dateRange.toISO, phaseFilter, jobTypeFilter],
  );

  useEffect(() => {
    void loadCalendar(true);
  }, [loadCalendar]);

  useEffect(() => {
    if (!drawerOpen || !selectedJobId) return;

    let cancelled = false;
    setJobLoading(true);
    getJob(selectedJobId)
      .then((response) => {
        if (cancelled) return;
        setSelectedJob(response.job);
        setEditor({
          scheduledDate: response.job.scheduledDate ?? "",
          pickupTime: response.job.pickupTime ?? "",
          status: response.job.status,
        });
      })
      .catch((error) => {
        if (cancelled) return;
        toast.error(getApiErrorMessage(error));
      })
      .finally(() => {
        if (cancelled) return;
        setJobLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [drawerOpen, selectedJobId]);

  const calendarDays = useMemo(() => buildCalendarDays(monthCursor), [monthCursor]);

  const jobsByDate = useMemo(() => {
    const grouped = new Map<string, CalendarJobCard[]>();
    for (const job of jobs) {
      const existing = grouped.get(job.scheduledDate) ?? [];
      existing.push(job);
      grouped.set(job.scheduledDate, existing);
    }
    return grouped;
  }, [jobs]);

  const monthLabel = useMemo(
    () => new Intl.DateTimeFormat("en-US", { month: "long", year: "numeric" }).format(monthCursor),
    [monthCursor],
  );

  function moveMonth(offset: number) {
    setMonthCursor((current) => startOfMonth(addMonths(current, offset)));
  }

  function openJobEditor(jobId: string) {
    setSelectedJobId(jobId);
    setDrawerOpen(true);
  }

  function closeDrawer(nextOpen: boolean) {
    setDrawerOpen(nextOpen);
    if (nextOpen) return;
    setSelectedJobId(null);
    setSelectedJob(null);
    setEditor(null);
  }

  async function saveJob() {
    if (!selectedJobId || !editor) return;
    setJobSaving(true);
    try {
      const payload: UpdateJobRequest = { status: editor.status };
      if (editor.scheduledDate) payload.scheduledDate = editor.scheduledDate;
      if (editor.pickupTime) payload.pickupTime = editor.pickupTime;

      const response = await updateJob(selectedJobId, payload);
      setSelectedJob(response.job);
      setEditor({
        scheduledDate: response.job.scheduledDate ?? "",
        pickupTime: response.job.pickupTime ?? "",
        status: response.job.status,
      });
      await loadCalendar(false);
      toast.success("Calendar job updated");
    } catch (error) {
      toast.error(getApiErrorMessage(error));
    } finally {
      setJobSaving(false);
    }
  }

  return (
    <div className="space-y-6 pb-8">
      {forbidden ? (
        <>
          <PageHeader title="Calendar" description="Monthly operations planning by scheduled jobs." />
          <NotAuthorizedState message="You need calendar permissions to view this page." />
        </>
      ) : (
        <>
      <PageHeader
        title="Calendar"
        description="Monthly operations planning by scheduled jobs."
        actions={
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="icon"
              aria-label="Previous month"
              onClick={() => moveMonth(-1)}
            >
              <ChevronLeft className="h-4 w-4" />
            </Button>
            <div className="min-w-40 rounded-md border border-border/70 bg-card px-3 py-2 text-center text-sm font-medium">
              {monthLabel}
            </div>
            <Button
              variant="outline"
              size="icon"
              aria-label="Next month"
              onClick={() => moveMonth(1)}
            >
              <ChevronRight className="h-4 w-4" />
            </Button>
          </div>
        }
      />

      <section className="grid gap-3 rounded-xl border border-border/70 bg-card/40 p-4 md:grid-cols-2 xl:grid-cols-5">
        <FilterField
          id="phase"
          label="Phase"
          value={phaseFilter}
          onChange={(value) => setPhaseFilter(value as CalendarPhase | "all")}
          options={phaseOptions}
        />
        <FilterField
          id="jobType"
          label="Job type"
          value={jobTypeFilter}
          onChange={(value) => setJobTypeFilter(value as CalendarJobType | "all")}
          options={jobTypeOptions}
        />
        <FilterField
          id="department"
          label="Department (stub)"
          value=""
          disabled
          onChange={() => {}}
          options={[{ value: "", label: "Coming soon" }]}
        />
        <FilterField
          id="user"
          label="User (stub)"
          value=""
          disabled
          onChange={() => {}}
          options={[{ value: "", label: "Coming soon" }]}
        />
        <div className="rounded-md border border-border/60 bg-background/70 px-3 py-2 text-sm">
          <p className="text-xs uppercase tracking-wide text-muted-foreground">Range</p>
          <p className="mt-1 font-medium">
            {dateRange.fromISO} to {dateRange.toISO}
          </p>
          <p className="mt-1 text-xs text-muted-foreground">`to` is exclusive.</p>
        </div>
      </section>

      <section className="overflow-hidden rounded-xl border border-border/70 bg-card/60">
        <div className="grid grid-cols-7 border-b border-border/70 bg-muted/20">
          {weekdays.map((day) => (
            <div key={day} className="px-2 py-2 text-center text-xs font-semibold uppercase tracking-wide text-muted-foreground">
              {day}
            </div>
          ))}
        </div>

        {loading ? (
          <CalendarGridSkeleton />
        ) : (
          <div className="grid grid-cols-7">
            {calendarDays.map((day) => {
              const key = formatDate(day.date);
              const dayJobs = jobsByDate.get(key) ?? [];
              return (
                <div
                  key={key}
                  className="min-h-40 border-b border-r border-border/70 p-2 last:border-r-0 [&:nth-child(7n)]:border-r-0"
                >
                  <p className={day.inCurrentMonth ? "text-xs font-semibold" : "text-xs font-semibold text-muted-foreground/70"}>
                    {day.date.getDate()}
                  </p>
                  <div className="mt-2 space-y-2">
                    {dayJobs.map((job) => (
                      <button
                        key={job.jobId}
                        type="button"
                        onClick={() => openJobEditor(job.jobId)}
                        className="w-full rounded-md border border-border/70 bg-background/80 p-2 text-left hover:bg-accent/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                      >
                        <p className="text-xs font-semibold text-primary">{job.jobNumber}</p>
                        <p className="text-xs text-muted-foreground">{job.pickupTime || "Time TBD"}</p>
                        <p className="truncate text-xs font-medium">{job.customerName}</p>
                        <p className="truncate text-[11px] text-muted-foreground">
                          {job.originShort} → {job.destinationShort}
                        </p>
                        <div className="mt-1 flex flex-wrap gap-1">
                          {job.hasStorage ? <Badge>STORAGE</Badge> : null}
                          {job.balanceDueCents > 0 ? <Badge>Balance {formatCurrency(job.balanceDueCents)}</Badge> : null}
                        </div>
                      </button>
                    ))}
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </section>

      {!loading && jobs.length === 0 ? (
        <p className="rounded-md border border-dashed border-border px-4 py-6 text-center text-sm text-muted-foreground">
          No jobs found for this month and filter combination.
        </p>
      ) : null}

      <Sheet open={drawerOpen} onOpenChange={closeDrawer}>
        <SheetContent className="left-auto right-0 w-full border-l border-r-0 sm:w-[440px]">
          {jobLoading || !selectedJob || !editor ? (
            <div className="space-y-3 pt-8">
              <Skeleton className="h-7 w-36" />
              <Skeleton className="h-16 w-full" />
              <Skeleton className="h-40 w-full" />
            </div>
          ) : (
            <div className="space-y-5 pt-8">
              <div>
                <h2 className="text-lg font-semibold">Job {selectedJob.jobNumber}</h2>
                <p className="text-sm text-muted-foreground">{selectedJob.customerName}</p>
              </div>

              <div className="rounded-lg border border-border/70 bg-muted/20 p-3 text-sm">
                <p className="font-medium">{selectedJob.primaryPhone || "No phone"}</p>
                <p className="text-muted-foreground">{selectedJob.email}</p>
              </div>

              <div className="space-y-3">
                <div className="space-y-2">
                  <Label htmlFor="drawerScheduledDate">Scheduled date</Label>
                  <Input
                    id="drawerScheduledDate"
                    type="date"
                    value={editor.scheduledDate}
                    disabled={jobSaving}
                    onChange={(event) => setEditor((prev) => (prev ? { ...prev, scheduledDate: event.target.value } : prev))}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="drawerPickupTime">Pickup time</Label>
                  <Input
                    id="drawerPickupTime"
                    type="time"
                    value={editor.pickupTime}
                    disabled={jobSaving}
                    onChange={(event) => setEditor((prev) => (prev ? { ...prev, pickupTime: event.target.value } : prev))}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="drawerPhase">Phase</Label>
                  <select
                    id="drawerPhase"
                    className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50"
                    value={editor.status}
                    disabled={jobSaving}
                    onChange={(event) => setEditor((prev) => (prev ? { ...prev, status: event.target.value as Job["status"] } : prev))}
                  >
                    <option value="booked">Booked</option>
                    <option value="scheduled">Scheduled</option>
                    <option value="completed">Completed</option>
                    <option value="cancelled">Cancelled</option>
                  </select>
                </div>
              </div>

              <Button className="w-full" disabled={jobSaving} onClick={saveJob}>
                {jobSaving ? (
                  <span className="inline-flex items-center gap-2">
                    <Loader2 className="h-4 w-4 animate-spin" />
                    Saving...
                  </span>
                ) : (
                  "Save changes"
                )}
              </Button>
            </div>
          )}
        </SheetContent>
      </Sheet>
        </>
      )}
    </div>
  );
}

function FilterField({
  id,
  label,
  value,
  options,
  onChange,
  disabled,
}: {
  id: string;
  label: string;
  value: string;
  options: Array<{ value: string; label: string }>;
  onChange: (value: string) => void;
  disabled?: boolean;
}) {
  return (
    <div className="space-y-2">
      <Label htmlFor={id}>{label}</Label>
      <select
        id={id}
        value={value}
        disabled={disabled}
        onChange={(event) => onChange(event.target.value)}
        className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-60"
      >
        {options.map((option) => (
          <option key={`${id}-${option.value || "empty"}`} value={option.value}>
            {option.label}
          </option>
        ))}
      </select>
    </div>
  );
}

function Badge({ children }: { children: ReactNode }) {
  return (
    <span className="rounded bg-secondary px-1.5 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-secondary-foreground">
      {children}
    </span>
  );
}

function CalendarGridSkeleton() {
  return (
    <div className="grid grid-cols-7">
      {Array.from({ length: 35 }).map((_, index) => (
        <div key={index} className="min-h-40 border-b border-r border-border/70 p-2 [&:nth-child(7n)]:border-r-0">
          <Skeleton className="h-4 w-6" />
          <div className="mt-2 space-y-2">
            <Skeleton className="h-14 w-full" />
            <Skeleton className="h-12 w-full" />
          </div>
        </div>
      ))}
    </div>
  );
}

function formatDate(value: Date) {
  const year = value.getFullYear();
  const month = String(value.getMonth() + 1).padStart(2, "0");
  const day = String(value.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

function startOfMonth(value: Date) {
  return new Date(value.getFullYear(), value.getMonth(), 1);
}

function addMonths(value: Date, months: number) {
  return new Date(value.getFullYear(), value.getMonth() + months, 1);
}

function buildCalendarDays(month: Date) {
  const monthStart = startOfMonth(month);
  const monthEnd = new Date(monthStart.getFullYear(), monthStart.getMonth() + 1, 0);

  const gridStart = new Date(monthStart);
  gridStart.setDate(monthStart.getDate() - monthStart.getDay());

  const gridEnd = new Date(monthEnd);
  gridEnd.setDate(monthEnd.getDate() + (6 - monthEnd.getDay()));

  const days: Array<{ date: Date; inCurrentMonth: boolean }> = [];
  for (let cursor = new Date(gridStart); cursor <= gridEnd; cursor.setDate(cursor.getDate() + 1)) {
    days.push({
      date: new Date(cursor),
      inCurrentMonth: cursor.getMonth() === monthStart.getMonth(),
    });
  }
  return days;
}

function formatCurrency(cents: number) {
  return new Intl.NumberFormat("en-US", { style: "currency", currency: "USD", maximumFractionDigits: 0 }).format(cents / 100);
}
