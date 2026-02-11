"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { Loader2 } from "lucide-react";
import { toast } from "sonner";

import { PageHeader } from "@/components/layout/page-header";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import {
  getApiErrorMessage,
  getJob,
  updateJob,
  type Job,
  type UpdateJobRequest,
} from "@/lib/phase2-api";

type JobFormState = {
  scheduledDate: string;
  pickupTime: string;
  status: "booked" | "scheduled" | "completed" | "cancelled";
};

export default function JobDetailPage() {
  const router = useRouter();
  const params = useParams<{ jobId: string }>();
  const jobId = Array.isArray(params?.jobId) ? params.jobId[0] : params?.jobId;

  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [job, setJob] = useState<Job | null>(null);
  const [form, setForm] = useState<JobFormState | null>(null);

  useEffect(() => {
    if (!jobId) return;

    let cancelled = false;
    setLoading(true);

    getJob(jobId)
      .then((response) => {
        if (cancelled) return;
        setJob(response.job);
        setForm({
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
        setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [jobId]);

  async function saveSchedule() {
    if (!jobId || !form) return;

    setSaving(true);
    try {
      const payload: UpdateJobRequest = {
        status: form.status,
      };
      if (form.scheduledDate) payload.scheduledDate = form.scheduledDate;
      if (form.pickupTime) payload.pickupTime = form.pickupTime;

      const response = await updateJob(jobId, payload);
      setJob(response.job);
      setForm({
        scheduledDate: response.job.scheduledDate ?? "",
        pickupTime: response.job.pickupTime ?? "",
        status: response.job.status,
      });
      toast.success("Job schedule updated");
      router.refresh();
    } catch (error) {
      toast.error(getApiErrorMessage(error));
    } finally {
      setSaving(false);
    }
  }

  if (loading || !job || !form) {
    return (
      <div className="space-y-4">
        <Skeleton className="h-10 w-72" />
        <Skeleton className="h-48 w-full" />
      </div>
    );
  }

  return (
    <div className="space-y-6 pb-8">
      <PageHeader
        title={`Job ${job.jobNumber}`}
        description="Review booking details and update scheduling information."
        actions={
          <Button onClick={saveSchedule} disabled={saving}>
            {saving ? (
              <span className="inline-flex items-center gap-2">
                <Loader2 className="h-4 w-4 animate-spin" />
                Saving...
              </span>
            ) : (
              "Save schedule"
            )}
          </Button>
        }
      />

      <div className="grid gap-6 xl:grid-cols-[2fr,1fr]">
        <Card>
          <CardHeader>
            <CardTitle>Job details</CardTitle>
            <CardDescription>Linked customer and schedule confirmation for this booking.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-5">
            <div className="grid gap-4 text-sm md:grid-cols-2">
              <Summary label="Customer" value={job.customerName} />
              <Summary label="Email" value={job.email} />
              <Summary label="Primary phone" value={job.primaryPhone} />
              <Summary label="Status" value={job.status} />
            </div>

            <div className="grid gap-4 md:grid-cols-3">
              <div className="space-y-2">
                <Label htmlFor="scheduledDate">Scheduled date</Label>
                <Input
                  id="scheduledDate"
                  type="date"
                  value={form.scheduledDate}
                  onChange={(event) => setForm((prev) => (prev ? { ...prev, scheduledDate: event.target.value } : prev))}
                  disabled={saving}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="pickupTime">Pickup time</Label>
                <Input
                  id="pickupTime"
                  type="time"
                  value={form.pickupTime}
                  onChange={(event) => setForm((prev) => (prev ? { ...prev, pickupTime: event.target.value } : prev))}
                  disabled={saving}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="status">Status</Label>
                <select
                  id="status"
                  value={form.status}
                  onChange={(event) =>
                    setForm((prev) =>
                      prev
                        ? {
                            ...prev,
                            status: event.target.value as JobFormState["status"],
                          }
                        : prev,
                    )
                  }
                  disabled={saving}
                  className="flex h-10 w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50"
                >
                  <option value="booked">Booked</option>
                  <option value="scheduled">Scheduled</option>
                  <option value="completed">Completed</option>
                  <option value="cancelled">Cancelled</option>
                </select>
              </div>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Next steps</CardTitle>
            <CardDescription>Continue this workflow in upcoming modules.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3 text-sm">
            <Link href="/calendar" className="block rounded-md border border-border/60 px-3 py-2 hover:bg-accent">
              Plan crew and dispatch in Calendar
            </Link>
            <Link href="/storage" className="block rounded-md border border-border/60 px-3 py-2 hover:bg-accent">
              Manage related storage workflow
            </Link>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}

function Summary({ label, value }: { label: string; value?: string }) {
  return (
    <div className="rounded-lg border border-border/60 bg-muted/20 p-3">
      <p className="text-xs uppercase tracking-wide text-muted-foreground">{label}</p>
      <p className="mt-1 text-sm font-medium">{value || "-"}</p>
    </div>
  );
}
