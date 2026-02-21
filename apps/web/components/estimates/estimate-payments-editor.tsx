"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Loader2, Plus, RefreshCw, Trash2 } from "lucide-react";
import { toast } from "sonner";

import { useEstimateWorkspace } from "@/components/estimates/estimate-workspace-context";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import {
  createEstimatePayment,
  deleteEstimatePayment,
  getEstimatePayments,
  type EstimatePayment,
  type EstimatePaymentSummary,
} from "@/lib/phase5-api";
import { getApiErrorMessage } from "@/lib/phase2-api";

export function EstimatePaymentsEditor() {
  const { estimate } = useEstimateWorkspace();

  const [summary, setSummary] = useState<EstimatePaymentSummary | null>(null);
  const [payments, setPayments] = useState<EstimatePayment[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const [amountDollars, setAmountDollars] = useState("");
  const [method, setMethod] = useState("cash");
  const [paidDate, setPaidDate] = useState(todayDateInput());
  const [notes, setNotes] = useState("");

  const sortedPayments = useMemo(
    () => [...payments].sort((a, b) => b.paidAt.localeCompare(a.paidAt)),
    [payments],
  );

  const loadPayments = useCallback(async () => {
    setLoading(true);
    setLoadError(null);
    try {
      const response = await getEstimatePayments(estimate.id);
      setSummary(response.summary);
      setPayments(response.payments);
    } catch (error) {
      setLoadError(getApiErrorMessage(error));
    } finally {
      setLoading(false);
    }
  }, [estimate.id]);

  useEffect(() => {
    void loadPayments();
  }, [loadPayments]);

  async function onAddPayment() {
    const parsedAmount = Number(amountDollars);
    if (!Number.isFinite(parsedAmount) || parsedAmount <= 0) {
      toast.error("Amount must be greater than zero");
      return;
    }
    const cleanedMethod = method.trim();
    if (!cleanedMethod) {
      toast.error("Payment method is required");
      return;
    }

    setSaving(true);
    try {
      await createEstimatePayment(estimate.id, {
        amountCents: Math.round(parsedAmount * 100),
        method: cleanedMethod,
        paidAt: paidDate ? new Date(`${paidDate}T09:00`).toISOString() : undefined,
        notes: notes.trim() || undefined,
      });
      setAmountDollars("");
      setMethod("cash");
      setPaidDate(todayDateInput());
      setNotes("");
      toast.success("Payment added");
      await loadPayments();
    } catch (error) {
      toast.error(getApiErrorMessage(error));
    } finally {
      setSaving(false);
    }
  }

  async function onDeletePayment(payment: EstimatePayment) {
    setSaving(true);
    try {
      await deleteEstimatePayment(estimate.id, payment.id);
      toast.success("Payment deleted");
      await loadPayments();
    } catch (error) {
      toast.error(getApiErrorMessage(error));
    } finally {
      setSaving(false);
    }
  }

  if (loading) {
    return (
      <Card className="border-border/70 bg-card/70">
        <CardContent className="flex h-48 items-center justify-center">
          <span className="inline-flex items-center gap-2 text-sm text-muted-foreground">
            <Loader2 className="h-4 w-4 animate-spin" />
            Loading payments...
          </span>
        </CardContent>
      </Card>
    );
  }

  if (loadError) {
    return (
      <Card className="border-border/70 bg-card/70">
        <CardHeader>
          <CardTitle>Payments unavailable</CardTitle>
          <CardDescription>{loadError}</CardDescription>
        </CardHeader>
        <CardContent>
          <Button variant="secondary" onClick={() => void loadPayments()}>
            <RefreshCw className="h-4 w-4" />
            Retry
          </Button>
        </CardContent>
      </Card>
    );
  }

  return (
    <div className="space-y-4">
      <div className="grid gap-4 md:grid-cols-3">
        <SummaryCard title="Deposit required" value={formatCurrency(summary?.depositRequiredCents)} />
        <SummaryCard title="Amount paid" value={formatCurrency(summary?.amountPaidCents ?? 0)} />
        <SummaryCard title="Remaining balance" value={formatCurrency(resolveRemaining(summary))} />
      </div>

      <Card className="border-border/70 bg-card/70">
        <CardHeader className="pb-3">
          <CardTitle className="text-base">Add payment</CardTitle>
          <CardDescription>Manual payment tracking only. No processor integration in this phase.</CardDescription>
        </CardHeader>
        <CardContent className="grid gap-3 md:grid-cols-4">
          <div className="space-y-2">
            <Label htmlFor="payment-amount">Amount (USD)</Label>
            <Input
              id="payment-amount"
              type="number"
              min="0"
              step="0.01"
              value={amountDollars}
              onChange={(event) => setAmountDollars(event.target.value)}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="payment-method">Method</Label>
            <Input id="payment-method" value={method} onChange={(event) => setMethod(event.target.value)} />
          </div>
          <div className="space-y-2">
            <Label htmlFor="payment-date">Paid date</Label>
            <Input id="payment-date" type="date" value={paidDate} onChange={(event) => setPaidDate(event.target.value)} />
          </div>
          <div className="space-y-2 md:col-span-4">
            <Label htmlFor="payment-notes">Notes</Label>
            <Textarea id="payment-notes" rows={2} value={notes} onChange={(event) => setNotes(event.target.value)} />
          </div>
          <div className="md:col-span-4">
            <Button type="button" onClick={onAddPayment} disabled={saving}>
              <Plus className="h-4 w-4" />
              Add payment
            </Button>
          </div>
        </CardContent>
      </Card>

      <Card className="border-border/70 bg-card/70">
        <CardHeader className="pb-3">
          <CardTitle className="text-base">Payments</CardTitle>
          <CardDescription>{payments.length === 0 ? "No payments recorded." : `${payments.length} payment(s)`}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-2">
          {sortedPayments.map((payment) => (
            <div key={payment.id} className="grid gap-3 rounded-md border border-border/70 p-3 md:grid-cols-[160px_120px_120px_minmax(0,1fr)_auto] md:items-center">
              <span className="text-sm font-medium">{formatDate(payment.paidAt)}</span>
              <span className="text-sm">{formatCurrency(payment.amountCents)}</span>
              <span className="text-sm capitalize">{payment.method}</span>
              <span className="text-sm text-muted-foreground">{payment.notes || "—"}</span>
              <Button
                size="icon"
                variant="ghost"
                onClick={() => void onDeletePayment(payment)}
                aria-label={`Delete payment ${payment.id}`}
              >
                <Trash2 className="h-4 w-4" />
              </Button>
            </div>
          ))}
          {sortedPayments.length === 0 ? (
            <div className="rounded-md border border-dashed border-border/70 p-6 text-center text-sm text-muted-foreground">
              Add a payment to start tracking deposits and balances.
            </div>
          ) : null}
        </CardContent>
      </Card>
    </div>
  );
}

function SummaryCard({ title, value }: { title: string; value: string }) {
  return (
    <Card className="border-border/70 bg-card/70">
      <CardHeader className="pb-2">
        <CardDescription>{title}</CardDescription>
      </CardHeader>
      <CardContent>
        <p className="text-2xl font-semibold tracking-tight">{value}</p>
      </CardContent>
    </Card>
  );
}

function resolveRemaining(summary: EstimatePaymentSummary | null) {
  if (!summary) return undefined;
  if (summary.remainingBalanceCents !== undefined) return summary.remainingBalanceCents;
  if (summary.totalEstimateCents !== undefined) {
    return summary.totalEstimateCents - summary.amountPaidCents;
  }
  return undefined;
}

function formatCurrency(value: number | null | undefined) {
  if (value === null || value === undefined) return "—";
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: "USD",
  }).format(value / 100);
}

function formatDate(value: string) {
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return value;
  return new Intl.DateTimeFormat("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
  }).format(parsed);
}

function todayDateInput() {
  const date = new Date();
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}
