"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Loader2, RefreshCw } from "lucide-react";
import { toast } from "sonner";

import { NewEstimateAdminNav } from "@/components/admin/new-estimate-admin-nav";
import { NotAuthorizedState } from "@/components/layout/not-authorized-state";
import { PageHeader } from "@/components/layout/page-header";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { isForbiddenError } from "@/lib/api";
import { listAuditLogs, type AdminAuditLogEntry } from "@/lib/new-estimate-admin-api";
import { getApiErrorMessage } from "@/lib/phase2-api";

const pageSize = 50;

export default function AdminAuditLogsPage() {
  const [items, setItems] = useState<AdminAuditLogEntry[]>([]);
  const [total, setTotal] = useState(0);
  const [offset, setOffset] = useState(0);

  const [fromDate, setFromDate] = useState("");
  const [toDate, setToDate] = useState("");
  const [actorUserId, setActorUserId] = useState("");
  const [action, setAction] = useState("");
  const [entityType, setEntityType] = useState("");
  const [entityId, setEntityId] = useState("");

  const [loading, setLoading] = useState(true);
  const [forbidden, setForbidden] = useState(false);

  const canPrev = offset > 0;
  const canNext = offset + pageSize < total;

  const filters = useMemo(
    () => ({
      from: fromDate ? `${fromDate}T00:00:00Z` : undefined,
      to: toDate ? `${toDate}T23:59:59Z` : undefined,
      actorUserId: actorUserId.trim() || undefined,
      action: action.trim() || undefined,
      entityType: entityType.trim() || undefined,
      entityId: entityId.trim() || undefined,
      limit: pageSize,
      offset,
    }),
    [action, actorUserId, entityId, entityType, fromDate, offset, toDate],
  );

  const loadData = useCallback(async () => {
    setLoading(true);
    try {
      const response = await listAuditLogs(filters);
      setItems(response.items);
      setTotal(response.total);
      setForbidden(false);
    } catch (error) {
      if (isForbiddenError(error)) {
        setForbidden(true);
      } else {
        toast.error(getApiErrorMessage(error));
      }
    } finally {
      setLoading(false);
    }
  }, [filters]);

  useEffect(() => {
    void loadData();
  }, [loadData]);

  function onApplyFilters() {
    setOffset(0);
    void loadData();
  }

  if (forbidden) {
    return (
      <div className="space-y-6 pb-8">
        <PageHeader title="Audit Logs" description="Tenant-scoped audit trail for admin and workflow actions." />
        <NewEstimateAdminNav />
        <NotAuthorizedState message="This page requires admin.new_estimate permission." />
      </div>
    );
  }

  return (
    <div className="space-y-6 pb-8">
      <PageHeader
        title="Audit Logs"
        description="Filter by date, actor, action, and entity to review sensitive activity and configuration changes."
      />
      <NewEstimateAdminNav />

      <Card className="border-border/70 bg-card/70">
        <CardHeader className="pb-3">
          <CardTitle className="text-base">Filters</CardTitle>
        </CardHeader>
        <CardContent className="grid gap-3 md:grid-cols-3 xl:grid-cols-6">
          <Field label="From">
            <Input type="date" value={fromDate} onChange={(event) => setFromDate(event.target.value)} />
          </Field>
          <Field label="To">
            <Input type="date" value={toDate} onChange={(event) => setToDate(event.target.value)} />
          </Field>
          <Field label="Actor user id">
            <Input value={actorUserId} onChange={(event) => setActorUserId(event.target.value)} placeholder="UUID" />
          </Field>
          <Field label="Action">
            <Input value={action} onChange={(event) => setAction(event.target.value)} placeholder="booking.booked" />
          </Field>
          <Field label="Entity type">
            <Input value={entityType} onChange={(event) => setEntityType(event.target.value)} placeholder="estimate" />
          </Field>
          <Field label="Entity id">
            <Input value={entityId} onChange={(event) => setEntityId(event.target.value)} placeholder="UUID" />
          </Field>
          <div className="md:col-span-3 xl:col-span-6">
            <div className="flex flex-wrap gap-2">
              <Button type="button" onClick={onApplyFilters} disabled={loading}>
                {loading ? (
                  <span className="inline-flex items-center gap-2">
                    <Loader2 className="h-4 w-4 animate-spin" />
                    Applying...
                  </span>
                ) : (
                  "Apply filters"
                )}
              </Button>
              <Button type="button" variant="outline" onClick={() => void loadData()} disabled={loading}>
                <RefreshCw className="h-4 w-4" />
                Refresh
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>

      <Card className="border-border/70 bg-card/70">
        <CardHeader className="pb-3">
          <CardTitle className="text-base">Entries</CardTitle>
          <CardDescription>{total} total records</CardDescription>
        </CardHeader>
        <CardContent>
          {loading ? (
            <div className="py-10 text-sm text-muted-foreground">
              <span className="inline-flex items-center gap-2">
                <Loader2 className="h-4 w-4 animate-spin" />
                Loading audit logs...
              </span>
            </div>
          ) : items.length === 0 ? (
            <div className="rounded-md border border-dashed border-border/70 p-8 text-center text-sm text-muted-foreground">
              No audit log entries found for this filter.
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full min-w-[980px] text-sm">
                <thead>
                  <tr className="border-b border-border/70 text-left text-xs uppercase tracking-wide text-muted-foreground">
                    <th className="px-2 py-2">When</th>
                    <th className="px-2 py-2">Action</th>
                    <th className="px-2 py-2">Entity</th>
                    <th className="px-2 py-2">Actor</th>
                    <th className="px-2 py-2">Request</th>
                    <th className="px-2 py-2">Metadata</th>
                  </tr>
                </thead>
                <tbody>
                  {items.map((entry) => (
                    <tr key={entry.id} className="border-b border-border/50 align-top">
                      <td className="px-2 py-2 text-muted-foreground">{formatDate(entry.createdAt)}</td>
                      <td className="px-2 py-2 font-medium">{entry.action}</td>
                      <td className="px-2 py-2">
                        <p>{entry.entityType}</p>
                        <p className="text-xs text-muted-foreground">{entry.entityId ?? "-"}</p>
                      </td>
                      <td className="px-2 py-2 text-xs text-muted-foreground">{entry.userId ?? "system"}</td>
                      <td className="px-2 py-2 text-xs text-muted-foreground">{entry.requestId ?? "-"}</td>
                      <td className="px-2 py-2 text-xs text-muted-foreground">
                        <pre className="max-w-[320px] overflow-x-auto whitespace-pre-wrap break-words rounded bg-muted/30 p-2">
                          {formatMetadata(entry.metadata)}
                        </pre>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}

          <div className="mt-4 flex items-center justify-between">
            <p className="text-xs text-muted-foreground">
              Showing {Math.min(offset + 1, total)} - {Math.min(offset + pageSize, total)} of {total}
            </p>
            <div className="flex gap-2">
              <Button
                type="button"
                variant="outline"
                onClick={() => setOffset((previous) => Math.max(0, previous - pageSize))}
                disabled={!canPrev || loading}
              >
                Previous
              </Button>
              <Button
                type="button"
                variant="outline"
                onClick={() => setOffset((previous) => previous + pageSize)}
                disabled={!canNext || loading}
              >
                Next
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="space-y-2">
      <Label>{label}</Label>
      {children}
    </div>
  );
}

function formatDate(value: string) {
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return value;
  return new Intl.DateTimeFormat("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(parsed);
}

function formatMetadata(metadata: Record<string, unknown>) {
  const keys = Object.keys(metadata);
  if (keys.length === 0) return "{}";
  try {
    return JSON.stringify(metadata, null, 2);
  } catch {
    return "[metadata unavailable]";
  }
}
