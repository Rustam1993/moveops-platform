"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { BarChart3, Loader2, RefreshCw } from "lucide-react";
import { toast } from "sonner";

import { NewEstimateAdminNav } from "@/components/admin/new-estimate-admin-nav";
import { NotAuthorizedState } from "@/components/layout/not-authorized-state";
import { PageHeader } from "@/components/layout/page-header";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { isForbiddenError } from "@/lib/api";
import { getNewEstimateMetrics, type NewEstimateMetrics } from "@/lib/new-estimate-admin-api";
import { getApiErrorMessage } from "@/lib/phase2-api";

export default function NewEstimateMetricsAdminPage() {
  const [metrics, setMetrics] = useState<NewEstimateMetrics | null>(null);
  const [fromDate, setFromDate] = useState(defaultFromDate());
  const [toDate, setToDate] = useState(defaultToDate());
  const [loading, setLoading] = useState(true);
  const [forbidden, setForbidden] = useState(false);

  const params = useMemo(() => {
    const from = fromDate ? `${fromDate}T00:00:00Z` : undefined;
    const to = toDate ? `${toDate}T23:59:59Z` : undefined;
    return { from, to };
  }, [fromDate, toDate]);

  const loadData = useCallback(async () => {
    setLoading(true);
    try {
      const response = await getNewEstimateMetrics(params);
      setMetrics(response.metrics);
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
  }, [params]);

  useEffect(() => {
    void loadData();
  }, [loadData]);

  if (forbidden) {
    return (
      <div className="space-y-6 pb-8">
        <PageHeader title="New Estimate Metrics" description="Tenant-scoped conversion and cycle-time metrics." />
        <NewEstimateAdminNav />
        <NotAuthorizedState message="This page requires admin.new_estimate permission." />
      </div>
    );
  }

  return (
    <div className="space-y-6 pb-8">
      <PageHeader
        title="New Estimate Metrics"
        description="Track speed and conversion across the Entry → Inventory → Charges → Quote → Sign → Book funnel."
      />
      <NewEstimateAdminNav />

      <Card className="border-border/70 bg-card/70">
        <CardContent className="grid gap-4 p-4 md:grid-cols-[180px_180px_auto] md:items-end">
          <div className="space-y-2">
            <Label htmlFor="metrics-from">From</Label>
            <Input id="metrics-from" type="date" value={fromDate} onChange={(event) => setFromDate(event.target.value)} />
          </div>
          <div className="space-y-2">
            <Label htmlFor="metrics-to">To</Label>
            <Input id="metrics-to" type="date" value={toDate} onChange={(event) => setToDate(event.target.value)} />
          </div>
          <div className="flex gap-2">
            <Button onClick={() => void loadData()} disabled={loading}>
              {loading ? (
                <span className="inline-flex items-center gap-2">
                  <Loader2 className="h-4 w-4 animate-spin" />
                  Loading...
                </span>
              ) : (
                <span className="inline-flex items-center gap-2">
                  <RefreshCw className="h-4 w-4" />
                  Refresh
                </span>
              )}
            </Button>
          </div>
        </CardContent>
      </Card>

      {loading && !metrics ? (
        <Card>
          <CardContent className="py-10 text-sm text-muted-foreground">
            <span className="inline-flex items-center gap-2">
              <Loader2 className="h-4 w-4 animate-spin" />
              Loading metrics...
            </span>
          </CardContent>
        </Card>
      ) : metrics ? (
        <>
          <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-5">
            <MetricCard
              title="Median Time to Quote"
              value={`${metrics.medianTimeToQuoteMinutes.toFixed(1)} min`}
              helper="entry_started → quote_sent"
            />
            <MetricCard
              title="Quote → Sign"
              value={formatRate(metrics.quoteToSign.rate)}
              helper={`${metrics.quoteToSign.convertedCount}/${metrics.quoteToSign.totalCount}`}
            />
            <MetricCard
              title="Sign → Book"
              value={formatRate(metrics.signToBook.rate)}
              helper={`${metrics.signToBook.convertedCount}/${metrics.signToBook.totalCount}`}
            />
            <MetricCard
              title="Inventory Completion"
              value={formatRate(metrics.inventoryCompletion.rate)}
              helper={`${metrics.inventoryCompletion.convertedCount}/${metrics.inventoryCompletion.totalCount}`}
            />
            <MetricCard title="Stuck Estimates" value={String(metrics.stuckEstimatesCount)} helper="Open/draft > 7 days" />
          </div>

          <Card className="border-border/70 bg-card/70">
            <CardHeader>
              <CardTitle className="text-base">Interpretation</CardTitle>
              <CardDescription>These numbers are tenant-scoped and aggregated from analytics events.</CardDescription>
            </CardHeader>
            <CardContent className="text-sm text-muted-foreground">
              <p className="inline-flex items-center gap-2">
                <BarChart3 className="h-4 w-4" />
                Use this panel to validate whether catalog, pricing, and communication defaults reduce cycle time and increase conversion.
              </p>
            </CardContent>
          </Card>
        </>
      ) : (
        <Card>
          <CardHeader>
            <CardTitle>No metrics available</CardTitle>
            <CardDescription>No data matched the selected date range.</CardDescription>
          </CardHeader>
        </Card>
      )}
    </div>
  );
}

function MetricCard({ title, value, helper }: { title: string; value: string; helper: string }) {
  return (
    <Card className="border-border/70 bg-card/70">
      <CardHeader className="pb-2">
        <CardDescription>{title}</CardDescription>
      </CardHeader>
      <CardContent>
        <p className="text-2xl font-semibold tracking-tight">{value}</p>
        <p className="text-xs text-muted-foreground">{helper}</p>
      </CardContent>
    </Card>
  );
}

function defaultFromDate() {
  const date = new Date();
  date.setDate(date.getDate() - 30);
  return toDateInput(date);
}

function defaultToDate() {
  return toDateInput(new Date());
}

function toDateInput(date: Date) {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

function formatRate(value: number) {
  return `${(value * 100).toFixed(1)}%`;
}
