"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Loader2 } from "lucide-react";

import { SaveStatusIndicator } from "@/components/estimates/save-status-indicator";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { formatCf } from "@/lib/inventory-catalog";
import { getApiErrorMessage } from "@/lib/phase2-api";
import {
  bookEstimate,
  getEstimateWorkflow,
  holdEstimate,
  releaseEstimateBooking,
  updateEstimateWorkflow,
  type EstimateWorkflow,
  type EstimateWorkflowStatus,
} from "@/lib/phase5-api";

type SidebarEstimate = {
  id: string;
  totalVolumeCf?: number | null;
  locationType?: string | null;
  estimatedTotalCents?: number | null;
};

type Props = {
  mode: "new" | "existing";
  estimate?: SidebarEstimate;
};

type SaveState = "idle" | "saving" | "saved" | "error";

const workflowStatuses: Array<{ value: EstimateWorkflowStatus; label: string }> = [
  { value: "draft", label: "Draft" },
  { value: "open", label: "Open" },
  { value: "follow_up", label: "Follow-up" },
  { value: "quoted", label: "Quoted" },
  { value: "booked", label: "Booked" },
  { value: "on_hold", label: "On hold" },
  { value: "canceled", label: "Canceled" },
];

export function EstimateWorkflowSidebar({ mode, estimate }: Props) {
  const isNew = mode === "new" || !estimate?.id;
  const [workflow, setWorkflow] = useState<EstimateWorkflow | null>(null);
  const [loading, setLoading] = useState(!isNew);
  const [error, setError] = useState<string | null>(null);
  const [saveState, setSaveState] = useState<SaveState>("idle");
  const [saveMessage, setSaveMessage] = useState("Ready");
  const [priorityDialogOpen, setPriorityDialogOpen] = useState(false);
  const [followUpInput, setFollowUpInput] = useState("");
  const [followUpNoteInput, setFollowUpNoteInput] = useState("");
  const [holdReasonInput, setHoldReasonInput] = useState("");

  const statusLabel = useMemo(() => {
    if (!workflow) return "Draft";
    return workflowStatuses.find((entry) => entry.value === workflow.status)?.label ?? workflow.status;
  }, [workflow]);

  const isBooked = workflow?.status === "booked";
  const isOnHold = workflow?.status === "on_hold";
  const isCanceled = workflow?.status === "canceled";

  const loadWorkflow = useCallback(async () => {
    if (!estimate?.id || isNew) return;
    setLoading(true);
    setError(null);

    try {
      const response = await getEstimateWorkflow(estimate.id);
      setWorkflow(response.workflow);
      setFollowUpInput(toDateTimeLocal(response.workflow.followUpAt));
      setFollowUpNoteInput(response.workflow.followUpNote ?? "");
      setHoldReasonInput(response.workflow.holdReason ?? "");
      setSaveState("saved");
      setSaveMessage("Saved");
    } catch (loadError) {
      setError(getApiErrorMessage(loadError));
      setSaveState("error");
      setSaveMessage("Load failed");
    } finally {
      setLoading(false);
    }
  }, [estimate?.id, isNew]);

  useEffect(() => {
    if (isNew) {
      setLoading(false);
      return;
    }
    void loadWorkflow();
  }, [isNew, loadWorkflow]);

  async function persistWorkflow(
    action: () => Promise<{ workflow: EstimateWorkflow }>,
    optimisticLabel?: string,
  ) {
    if (isNew) return;
    setSaveState("saving");
    setSaveMessage(optimisticLabel ?? "Saving...");
    setError(null);
    try {
      const response = await action();
      setWorkflow(response.workflow);
      setFollowUpInput(toDateTimeLocal(response.workflow.followUpAt));
      setFollowUpNoteInput(response.workflow.followUpNote ?? "");
      setHoldReasonInput(response.workflow.holdReason ?? "");
      setSaveState("saved");
      setSaveMessage("Saved");
    } catch (persistError) {
      setSaveState("error");
      setSaveMessage("Save failed");
      setError(getApiErrorMessage(persistError));
    }
  }

  function onStatusChange(nextStatus: EstimateWorkflowStatus) {
    if (!estimate?.id || !workflow) return;
    void persistWorkflow(() => updateEstimateWorkflow(estimate.id, { status: nextStatus }), "Saving...");
  }

  function onPrioritySelect(priorityLevel: number) {
    if (!estimate?.id || !workflow) return;
    setPriorityDialogOpen(false);
    void persistWorkflow(() => updateEstimateWorkflow(estimate.id, { priorityLevel }), "Saving...");
  }

  function onVipToggle(nextVip: boolean) {
    if (!estimate?.id || !workflow) return;
    void persistWorkflow(() => updateEstimateWorkflow(estimate.id, { vip: nextVip }), "Saving...");
  }

  function onFollowUpBlur() {
    if (!estimate?.id || !workflow) return;
    const trimmed = followUpInput.trim();
    if (!trimmed) {
      void persistWorkflow(() => updateEstimateWorkflow(estimate.id, { clearFollowUpAt: true }), "Saving...");
      return;
    }
    const asIso = toIsoString(trimmed);
    if (!asIso) {
      setError("Follow-up date/time is invalid");
      setSaveState("error");
      setSaveMessage("Save failed");
      return;
    }
    void persistWorkflow(() => updateEstimateWorkflow(estimate.id, { followUpAt: asIso }), "Saving...");
  }

  function onFollowUpNoteBlur() {
    if (!estimate?.id || !workflow) return;
    const trimmed = followUpNoteInput.trim();
    if (!trimmed) {
      void persistWorkflow(
        () => updateEstimateWorkflow(estimate.id, { clearFollowUpNote: true }),
        "Saving...",
      );
      return;
    }
    void persistWorkflow(() => updateEstimateWorkflow(estimate.id, { followUpNote: trimmed }), "Saving...");
  }

  function onBookClick() {
    if (!estimate?.id) return;
    void persistWorkflow(() => bookEstimate(estimate.id), "Booking...");
  }

  function onReleaseBookClick() {
    if (!estimate?.id) return;
    void persistWorkflow(() => releaseEstimateBooking(estimate.id), "Releasing...");
  }

  function onHoldClick() {
    if (!estimate?.id) return;
    void persistWorkflow(() => holdEstimate(estimate.id, { holdReason: holdReasonInput.trim() || undefined }), "Saving...");
  }

  const priorityLabel = workflow
    ? workflow.priorityLevel === 0
      ? "General Priority Pool"
      : `Priority ${workflow.priorityLevel}`
    : "Level 0";

  return (
    <Card className="border-border/70 bg-card/70">
      <CardContent className="space-y-4 p-4 text-sm">
        <SidebarGroupTitle title="Workflow" />

        {isNew ? (
          <p className="text-muted-foreground">Save the estimate first to enable workflow controls.</p>
        ) : loading ? (
          <p className="inline-flex items-center gap-2 text-muted-foreground">
            <Loader2 className="h-4 w-4 animate-spin" />
            Loading workflow...
          </p>
        ) : (
          <>
            <SaveStatusIndicator state={saveState} message={saveMessage} onRetry={() => void loadWorkflow()} />
            {error ? <p className="text-xs text-destructive">{error}</p> : null}

            {workflow?.status === "booked" ? (
              <div className="rounded-md border border-emerald-500/40 bg-emerald-500/10 px-3 py-2 text-sm font-medium text-emerald-700">
                Job is Booked
              </div>
            ) : null}

            <div className="space-y-2">
              <Label htmlFor="workflow-status">Status</Label>
              <select
                id="workflow-status"
                value={workflow?.status ?? "draft"}
                onChange={(event) => onStatusChange(event.target.value as EstimateWorkflowStatus)}
                className="flex h-10 w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              >
                {workflowStatuses.map((item) => (
                  <option key={item.value} value={item.value}>
                    {item.label}
                  </option>
                ))}
              </select>
            </div>

            <div className="space-y-2">
              <Label>Priority</Label>
              <Button type="button" variant="outline" className="w-full justify-start" onClick={() => setPriorityDialogOpen(true)}>
                {priorityLabel}
              </Button>
            </div>

            <div className="space-y-2">
              <Label htmlFor="workflow-follow-up">Follow-up date/time</Label>
              <Input
                id="workflow-follow-up"
                type="datetime-local"
                value={followUpInput}
                onChange={(event) => setFollowUpInput(event.target.value)}
                onBlur={onFollowUpBlur}
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="workflow-follow-up-note">Follow-up note</Label>
              <Textarea
                id="workflow-follow-up-note"
                rows={2}
                placeholder="Optional follow-up note"
                value={followUpNoteInput}
                onChange={(event) => setFollowUpNoteInput(event.target.value)}
                onBlur={onFollowUpNoteBlur}
              />
            </div>

            <label className="flex items-center gap-2 text-sm">
              <Checkbox checked={workflow?.vip ?? false} onCheckedChange={(checked) => onVipToggle(Boolean(checked))} />
              VIP customer
            </label>

            <div className="space-y-2">
              <Label htmlFor="workflow-hold-reason">Hold reason</Label>
              <Input
                id="workflow-hold-reason"
                placeholder="Optional reason for hold"
                value={holdReasonInput}
                onChange={(event) => setHoldReasonInput(event.target.value)}
              />
            </div>

            <div className="grid gap-2">
              <Button type="button" onClick={onBookClick} disabled={isBooked || isCanceled}>
                Book This Job
              </Button>
              <Button type="button" variant="outline" onClick={onReleaseBookClick} disabled={!isBooked}>
                Release Book
              </Button>
              <Button type="button" variant="secondary" onClick={onHoldClick} disabled={isCanceled || isOnHold}>
                Move On Hold
              </Button>
            </div>
          </>
        )}

        <div className="h-px bg-border/70" />

        <SidebarGroupTitle title="Derived totals" />
        <SidebarField label="Status" value={statusLabel} />
        <SidebarField label="Service type" value={formatServiceType(estimate?.locationType)} />
        <SidebarField label="Total CF" value={formatCf(estimate?.totalVolumeCf)} />
        <SidebarField label="Total estimate" value={formatCurrency(estimate?.estimatedTotalCents)} />
      </CardContent>

      <Dialog open={priorityDialogOpen} onOpenChange={setPriorityDialogOpen}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>Select priority</DialogTitle>
            <DialogDescription>Choose a queue level from Priority 1 through Priority 8.</DialogDescription>
          </DialogHeader>
          <div className="grid grid-cols-2 gap-2">
            {Array.from({ length: 8 }, (_, index) => index + 1).map((level) => (
              <Button
                key={level}
                type="button"
                variant={workflow?.priorityLevel === level ? "default" : "outline"}
                onClick={() => onPrioritySelect(level)}
              >
                Priority {level}
              </Button>
            ))}
          </div>
          <Button
            type="button"
            variant={workflow?.priorityLevel === 0 ? "default" : "outline"}
            onClick={() => onPrioritySelect(0)}
          >
            General Priority Pool
          </Button>
        </DialogContent>
      </Dialog>
    </Card>
  );
}

function SidebarGroupTitle({ title }: { title: string }) {
  return <h2 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">{title}</h2>;
}

function SidebarField({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between gap-3">
      <span className="text-muted-foreground">{label}</span>
      <span className="font-medium">{value}</span>
    </div>
  );
}

function formatServiceType(locationType?: string | null) {
  if (!locationType) return "Local";
  return locationType.toLowerCase().includes("long") ? "Long Distance" : "Local";
}

function formatCurrency(value: number | null | undefined) {
  if (value === null || value === undefined) return "—";
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: "USD",
  }).format(value / 100);
}

function toDateTimeLocal(value?: string) {
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  const hours = String(date.getHours()).padStart(2, "0");
  const minutes = String(date.getMinutes()).padStart(2, "0");
  return `${year}-${month}-${day}T${hours}:${minutes}`;
}

function toIsoString(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return null;
  return date.toISOString();
}
