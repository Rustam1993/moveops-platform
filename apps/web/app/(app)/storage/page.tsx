"use client";

import { useCallback, useEffect, useMemo, useRef, useState, type Dispatch, type SetStateAction } from "react";
import { Loader2 } from "lucide-react";
import { toast } from "sonner";

import { NotAuthorizedState } from "@/components/layout/not-authorized-state";
import { PageHeader } from "@/components/layout/page-header";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Sheet, SheetContent } from "@/components/ui/sheet";
import { Skeleton } from "@/components/ui/skeleton";
import { Textarea } from "@/components/ui/textarea";
import {
  createStorageRecord,
  getApiErrorMessage,
  getStorageRecord,
  getStorageRows,
  updateStorageRecord,
  type CreateStorageRecordRequest,
  type StorageListItem,
  type StorageRecord,
  type StorageStatus,
  type UpdateStorageRecordRequest,
} from "@/lib/storage-api";
import { isForbiddenError } from "@/lib/api";

const facilities = ["Main Facility", "North Warehouse", "South Annex"];
const statusOptions: Array<{ label: string; value: StorageStatus | "all" }> = [
  { label: "All statuses", value: "all" },
  { label: "In storage", value: "in_storage" },
  { label: "SIT", value: "sit" },
  { label: "Out", value: "out" },
];

type StorageFormState = {
  facility: string;
  status: StorageStatus;
  dateIn: string;
  dateOut: string;
  nextBillDate: string;
  lotNumber: string;
  locationLabel: string;
  vaults: string;
  pads: string;
  items: string;
  oversizeItems: string;
  volume: string;
  monthlyRateCents: string;
  storageBalanceCents: string;
  moveBalanceCents: string;
  notes: string;
};

export default function StoragePage() {
  const facilityInputRef = useRef<HTMLInputElement>(null);

  const [facility, setFacility] = useState("");
  const [searchInput, setSearchInput] = useState("");
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState<StorageStatus | "all">("all");
  const [deliveryScheduled, setDeliveryScheduled] = useState(false);
  const [balanceDue, setBalanceDue] = useState(false);
  const [hasContainers, setHasContainers] = useState(false);

  const [loading, setLoading] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [items, setItems] = useState<StorageListItem[]>([]);
  const [nextCursor, setNextCursor] = useState<string | null>(null);
  const [forbidden, setForbidden] = useState(false);

  const [drawerOpen, setDrawerOpen] = useState(false);
  const [drawerLoading, setDrawerLoading] = useState(false);
  const [drawerSaving, setDrawerSaving] = useState(false);
  const [selectedRow, setSelectedRow] = useState<StorageListItem | null>(null);
  const [storageRecord, setStorageRecord] = useState<StorageRecord | null>(null);
  const [form, setForm] = useState<StorageFormState | null>(null);

  useEffect(() => {
    const timer = setTimeout(() => setSearch(searchInput.trim()), 300);
    return () => clearTimeout(timer);
  }, [searchInput]);

  const loadStorageRows = useCallback(
    async (options?: { append?: boolean; cursor?: string }) => {
      if (!facility) {
        setItems([]);
        setNextCursor(null);
        return;
      }

      const append = options?.append ?? false;
      if (append) {
        setLoadingMore(true);
      } else {
        setLoading(true);
      }

      try {
        const response = await getStorageRows({
          facility,
          q: search || undefined,
          status: status === "all" ? undefined : status,
          hasDateOut: deliveryScheduled ? true : undefined,
          balanceDue: balanceDue ? true : undefined,
          hasContainers: hasContainers ? true : undefined,
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
        if (append) {
          setLoadingMore(false);
        } else {
          setLoading(false);
        }
      }
    },
    [facility, search, status, deliveryScheduled, balanceDue, hasContainers],
  );

  useEffect(() => {
    void loadStorageRows();
  }, [loadStorageRows]);

  useEffect(() => {
    if (!drawerOpen || drawerLoading) return;
    const timer = setTimeout(() => facilityInputRef.current?.focus(), 30);
    return () => clearTimeout(timer);
  }, [drawerOpen, drawerLoading]);

  const dateRangeWarning = useMemo(() => {
    if (!form?.dateIn || !form.dateOut) return null;
    if (form.dateIn <= form.dateOut) return null;
    return "Date In should be on or before Date Out.";
  }, [form]);

  async function openDrawer(row: StorageListItem) {
    setSelectedRow(row);
    setStorageRecord(null);
    setForm(null);
    setDrawerOpen(true);

    if (!row.storageRecordId) {
      setForm(buildCreateForm(row, facility || row.facility));
      return;
    }

    setDrawerLoading(true);
    try {
      const response = await getStorageRecord(row.storageRecordId);
      setStorageRecord(response.storage);
      setForm(buildFormFromRecord(response.storage));
    } catch (error) {
      toast.error(getApiErrorMessage(error));
    } finally {
      setDrawerLoading(false);
    }
  }

  function closeDrawer(nextOpen: boolean) {
    setDrawerOpen(nextOpen);
    if (nextOpen) return;
    setSelectedRow(null);
    setStorageRecord(null);
    setForm(null);
    setDrawerLoading(false);
    setDrawerSaving(false);
  }

  async function saveStorageRecord() {
    if (!selectedRow || !form) return;

    const trimmedFacility = form.facility.trim();
    if (!trimmedFacility) {
      toast.error("Facility is required");
      return;
    }
    if (dateRangeWarning) {
      toast.error(dateRangeWarning);
      return;
    }

    const vaults = toNonNegativeInteger(form.vaults);
    const pads = toNonNegativeInteger(form.pads);
    const itemCount = toNonNegativeInteger(form.items);
    const oversizeItems = toNonNegativeInteger(form.oversizeItems);
    const volume = toNonNegativeInteger(form.volume);
    const storageBalanceCents = toNonNegativeInteger(form.storageBalanceCents);
    const moveBalanceCents = toNonNegativeInteger(form.moveBalanceCents);
    const monthlyRateCents = optionalNonNegativeInteger(form.monthlyRateCents);
    const notes = form.notes.trim();
    const lotNumber = form.lotNumber.trim();
    const locationLabel = form.locationLabel.trim();

    setDrawerSaving(true);
    try {
      if (storageRecord?.id || selectedRow.storageRecordId) {
        const payload: UpdateStorageRecordRequest = {
          facility: trimmedFacility,
          status: form.status,
          vaults,
          pads,
          items: itemCount,
          oversizeItems,
          volume,
          storageBalanceCents,
          moveBalanceCents,
        };
        if (form.dateIn) payload.dateIn = form.dateIn;
        if (form.dateOut) payload.dateOut = form.dateOut;
        if (form.nextBillDate) payload.nextBillDate = form.nextBillDate;
        if (lotNumber) payload.lotNumber = lotNumber;
        if (locationLabel) payload.locationLabel = locationLabel;
        if (monthlyRateCents !== undefined) payload.monthlyRateCents = monthlyRateCents;
        if (notes) payload.notes = notes;

        const targetId = storageRecord?.id ?? selectedRow.storageRecordId ?? "";
        const response = await updateStorageRecord(targetId, payload);
        setStorageRecord(response.storage);
        setForm(buildFormFromRecord(response.storage));
        toast.success("Storage record updated");
      } else {
        const payload: CreateStorageRecordRequest = {
          facility: trimmedFacility,
          status: form.status,
          vaults,
          pads,
          items: itemCount,
          oversizeItems,
          volume,
          storageBalanceCents,
          moveBalanceCents,
        };
        if (form.dateIn) payload.dateIn = form.dateIn;
        if (form.dateOut) payload.dateOut = form.dateOut;
        if (form.nextBillDate) payload.nextBillDate = form.nextBillDate;
        if (lotNumber) payload.lotNumber = lotNumber;
        if (locationLabel) payload.locationLabel = locationLabel;
        if (monthlyRateCents !== undefined) payload.monthlyRateCents = monthlyRateCents;
        if (notes) payload.notes = notes;

        const response = await createStorageRecord(selectedRow.jobId, payload);
        setStorageRecord(response.storage);
        setForm(buildFormFromRecord(response.storage));
        toast.success("Storage record created");
      }

      await loadStorageRows();
    } catch (error) {
      toast.error(getApiErrorMessage(error));
    } finally {
      setDrawerSaving(false);
    }
  }

  const selectedSummary = storageRecord
    ? {
        jobNumber: storageRecord.jobNumber,
        customerName: storageRecord.customerName,
        fromShort: storageRecord.fromShort,
        toShort: storageRecord.toShort,
      }
    : selectedRow
      ? {
          jobNumber: selectedRow.jobNumber,
          customerName: selectedRow.customerName,
          fromShort: selectedRow.fromShort,
          toShort: selectedRow.toShort,
        }
      : null;

  return (
    <div className="space-y-6 pb-8">
      {forbidden ? (
        <>
          <PageHeader title="Storage" description="Track storage occupancy, balances, and container counts by facility." />
          <NotAuthorizedState message="You need storage permissions to view this page." />
        </>
      ) : (
        <>
      <PageHeader
        title="Storage"
        description="Track storage occupancy, balances, and container counts by facility."
        actions={
          <div className="grid w-full gap-2 sm:w-auto sm:grid-cols-[200px,260px]">
            <select
              value={facility}
              onChange={(event) => setFacility(event.target.value)}
              className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            >
              <option value="">Select facility</option>
              {facilities.map((entry) => (
                <option key={entry} value={entry}>
                  {entry}
                </option>
              ))}
            </select>
            <Input
              value={searchInput}
              onChange={(event) => setSearchInput(event.target.value)}
              placeholder="Search by job # or customer"
            />
          </div>
        }
      />

      <section className="grid gap-3 rounded-xl border border-border/70 bg-card/40 p-4 md:grid-cols-2 xl:grid-cols-6">
        <div className="space-y-2">
          <Label htmlFor="statusFilter">Status</Label>
          <select
            id="statusFilter"
            value={status}
            onChange={(event) => setStatus(event.target.value as StorageStatus | "all")}
            className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            {statusOptions.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
        </div>

        <ToggleField
          id="deliveryScheduled"
          label="Delivery scheduled"
          description="Date out is set"
          checked={deliveryScheduled}
          onChange={setDeliveryScheduled}
        />
        <ToggleField
          id="balanceDue"
          label="Balance due"
          description="Storage balance > 0"
          checked={balanceDue}
          onChange={setBalanceDue}
        />
        <ToggleField
          id="hasContainers"
          label="Has containers"
          description="Vaults, pads, items, or oversize"
          checked={hasContainers}
          onChange={setHasContainers}
        />
        <ToggleField
          id="pastDueStub"
          label="Past 30 days without payment"
          description="Disabled in Phase 4"
          checked={false}
          disabled
          onChange={() => {}}
        />
        <div className="flex items-end">
          <Button variant="outline" className="w-full" disabled>
            Monthly invoice cycle actions (Phase 6)
          </Button>
        </div>
      </section>

      {!facility ? (
        <div className="rounded-xl border border-dashed border-border p-8 text-center text-sm text-muted-foreground">
          Select a facility to view storage jobs.
        </div>
      ) : (
        <section className="overflow-hidden rounded-xl border border-border/70 bg-card/60">
          <div className="overflow-x-auto">
            <table className="min-w-[1300px] w-full text-sm">
              <thead className="bg-muted/20 text-xs uppercase tracking-wide text-muted-foreground">
                <tr>
                  <HeaderCell>Job #</HeaderCell>
                  <HeaderCell>Type</HeaderCell>
                  <HeaderCell>Customer</HeaderCell>
                  <HeaderCell>From → To</HeaderCell>
                  <HeaderCell>Date In</HeaderCell>
                  <HeaderCell>Date Out</HeaderCell>
                  <HeaderCell>Next Bill</HeaderCell>
                  <HeaderCell>Lot</HeaderCell>
                  <HeaderCell>Location</HeaderCell>
                  <HeaderCell>Vaults</HeaderCell>
                  <HeaderCell>Pads</HeaderCell>
                  <HeaderCell>Items / Oversize</HeaderCell>
                  <HeaderCell>Volume</HeaderCell>
                  <HeaderCell>Monthly</HeaderCell>
                  <HeaderCell>Storage Balance</HeaderCell>
                  <HeaderCell>Move Balance</HeaderCell>
                </tr>
              </thead>
              <tbody>
                {loading ? (
                  <StorageTableSkeleton />
                ) : items.length === 0 ? (
                  <tr>
                    <td colSpan={16} className="px-4 py-10 text-center text-sm text-muted-foreground">
                      No storage records found.
                    </td>
                  </tr>
                ) : (
                  items.map((item) => (
                    <tr
                      key={`${item.jobId}-${item.storageRecordId ?? "new"}`}
                      tabIndex={0}
                      role="button"
                      onClick={() => void openDrawer(item)}
                      onKeyDown={(event) => {
                        if (event.key === "Enter" || event.key === " ") {
                          event.preventDefault();
                          void openDrawer(item);
                        }
                      }}
                      className="cursor-pointer border-b border-border/60 hover:bg-accent/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                    >
                      <BodyCell className="font-semibold text-primary">{item.jobNumber}</BodyCell>
                      <BodyCell>{formatMoveType(item.moveType)}</BodyCell>
                      <BodyCell>{item.customerName}</BodyCell>
                      <BodyCell>
                        <div className="max-w-56 truncate">
                          {item.fromShort} → {item.toShort}
                        </div>
                      </BodyCell>
                      <BodyCell>{formatDate(item.dateIn)}</BodyCell>
                      <BodyCell>{formatDate(item.dateOut)}</BodyCell>
                      <BodyCell>{formatDate(item.nextBillDate)}</BodyCell>
                      <BodyCell>{item.lotNumber ?? "-"}</BodyCell>
                      <BodyCell>{item.locationLabel ?? "-"}</BodyCell>
                      <BodyCell>{item.vaults}</BodyCell>
                      <BodyCell>{item.pads}</BodyCell>
                      <BodyCell>
                        {item.items} / {item.oversizeItems}
                      </BodyCell>
                      <BodyCell>{item.volume}</BodyCell>
                      <BodyCell>{item.monthlyRateCents == null ? "-" : formatCurrency(item.monthlyRateCents)}</BodyCell>
                      <BodyCell>{formatCurrency(item.storageBalanceCents)}</BodyCell>
                      <BodyCell>{formatCurrency(item.moveBalanceCents)}</BodyCell>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>

          {nextCursor ? (
            <div className="border-t border-border/70 p-3">
              <Button
                variant="outline"
                onClick={() => void loadStorageRows({ append: true, cursor: nextCursor })}
                disabled={loadingMore}
              >
                {loadingMore ? (
                  <span className="inline-flex items-center gap-2">
                    <Loader2 className="h-4 w-4 animate-spin" />
                    Loading...
                  </span>
                ) : (
                  "Load more"
                )}
              </Button>
            </div>
          ) : null}
        </section>
      )}

      <Sheet open={drawerOpen} onOpenChange={closeDrawer}>
        <SheetContent className="left-auto right-0 w-full border-l border-r-0 p-0 sm:w-[520px]">
          {drawerLoading || !selectedRow || !form ? (
            <div className="space-y-3 p-6 pt-14">
              <Skeleton className="h-7 w-40" />
              <Skeleton className="h-16 w-full" />
              <Skeleton className="h-[420px] w-full" />
            </div>
          ) : (
            <div className="flex h-full flex-col">
              <div className="border-b border-border/70 px-6 pb-4 pt-14">
                <h2 className="text-lg font-semibold">
                  {storageRecord?.id || selectedRow.storageRecordId ? "Edit storage record" : "Create storage record"}
                </h2>
                {selectedSummary ? (
                  <p className="mt-1 text-sm text-muted-foreground">
                    {selectedSummary.jobNumber} • {selectedSummary.customerName} • {selectedSummary.fromShort} → {selectedSummary.toShort}
                  </p>
                ) : null}
              </div>

              <div className="flex-1 space-y-4 overflow-y-auto px-6 py-5">
                <div className="grid gap-3 md:grid-cols-2">
                  <Field label="Facility">
                    <Input
                      ref={facilityInputRef}
                      value={form.facility}
                      disabled={drawerSaving}
                      onChange={(event) => updateForm(setForm, "facility", event.target.value)}
                    />
                  </Field>
                  <Field label="Status">
                    <select
                      value={form.status}
                      disabled={drawerSaving}
                      onChange={(event) => updateForm(setForm, "status", event.target.value as StorageStatus)}
                      className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                    >
                      <option value="in_storage">In storage</option>
                      <option value="sit">SIT</option>
                      <option value="out">Out</option>
                    </select>
                  </Field>
                </div>

                <div className="grid gap-3 md:grid-cols-3">
                  <Field label="Date In">
                    <Input
                      type="date"
                      value={form.dateIn}
                      disabled={drawerSaving}
                      onChange={(event) => updateForm(setForm, "dateIn", event.target.value)}
                    />
                  </Field>
                  <Field label="Date Out">
                    <Input
                      type="date"
                      value={form.dateOut}
                      disabled={drawerSaving}
                      onChange={(event) => updateForm(setForm, "dateOut", event.target.value)}
                    />
                  </Field>
                  <Field label="Next Bill Date">
                    <Input
                      type="date"
                      value={form.nextBillDate}
                      disabled={drawerSaving}
                      onChange={(event) => updateForm(setForm, "nextBillDate", event.target.value)}
                    />
                  </Field>
                </div>

                {dateRangeWarning ? <p className="text-xs text-destructive">{dateRangeWarning}</p> : null}

                <div className="grid gap-3 md:grid-cols-2">
                  <Field label="Lot">
                    <Input
                      value={form.lotNumber}
                      disabled={drawerSaving}
                      onChange={(event) => updateForm(setForm, "lotNumber", event.target.value)}
                    />
                  </Field>
                  <Field label="Location">
                    <Input
                      value={form.locationLabel}
                      disabled={drawerSaving}
                      onChange={(event) => updateForm(setForm, "locationLabel", event.target.value)}
                    />
                  </Field>
                </div>

                <div className="grid gap-3 md:grid-cols-4">
                  <NumberField label="Vaults" value={form.vaults} disabled={drawerSaving} onChange={(value) => updateForm(setForm, "vaults", value)} />
                  <NumberField label="Pads" value={form.pads} disabled={drawerSaving} onChange={(value) => updateForm(setForm, "pads", value)} />
                  <NumberField label="Items" value={form.items} disabled={drawerSaving} onChange={(value) => updateForm(setForm, "items", value)} />
                  <NumberField
                    label="Oversize"
                    value={form.oversizeItems}
                    disabled={drawerSaving}
                    onChange={(value) => updateForm(setForm, "oversizeItems", value)}
                  />
                </div>

                <div className="grid gap-3 md:grid-cols-3">
                  <NumberField label="Volume" value={form.volume} disabled={drawerSaving} onChange={(value) => updateForm(setForm, "volume", value)} />
                  <NumberField
                    label="Monthly rate (cents)"
                    value={form.monthlyRateCents}
                    disabled={drawerSaving}
                    onChange={(value) => updateForm(setForm, "monthlyRateCents", value)}
                  />
                  <NumberField
                    label="Storage balance (cents)"
                    value={form.storageBalanceCents}
                    disabled={drawerSaving}
                    onChange={(value) => updateForm(setForm, "storageBalanceCents", value)}
                  />
                </div>

                <div className="grid gap-3 md:grid-cols-2">
                  <NumberField
                    label="Move balance (cents)"
                    value={form.moveBalanceCents}
                    disabled={drawerSaving}
                    onChange={(value) => updateForm(setForm, "moveBalanceCents", value)}
                  />
                </div>

                <Field label="Notes">
                  <Textarea
                    value={form.notes}
                    disabled={drawerSaving}
                    onChange={(event) => updateForm(setForm, "notes", event.target.value)}
                  />
                </Field>
              </div>

              <div className="border-t border-border/70 px-6 py-4">
                <Button onClick={() => void saveStorageRecord()} disabled={drawerSaving || !!dateRangeWarning}>
                  {drawerSaving ? (
                    <span className="inline-flex items-center gap-2">
                      <Loader2 className="h-4 w-4 animate-spin" />
                      Saving...
                    </span>
                  ) : (
                    "Save"
                  )}
                </Button>
              </div>
            </div>
          )}
        </SheetContent>
      </Sheet>
        </>
      )}
    </div>
  );
}

function HeaderCell({ children }: { children: React.ReactNode }) {
  return <th className="px-3 py-3 text-left font-semibold">{children}</th>;
}

function BodyCell({ children, className }: { children: React.ReactNode; className?: string }) {
  return <td className={`px-3 py-3 align-top ${className ?? ""}`}>{children}</td>;
}

function StorageTableSkeleton() {
  return (
    <>
      {Array.from({ length: 6 }).map((_, rowIndex) => (
        <tr key={`skeleton-${rowIndex}`} className="border-b border-border/60">
          {Array.from({ length: 16 }).map((_, columnIndex) => (
            <td key={`skeleton-${rowIndex}-${columnIndex}`} className="px-3 py-3">
              <Skeleton className="h-4 w-full max-w-28" />
            </td>
          ))}
        </tr>
      ))}
    </>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="space-y-1.5">
      <Label>{label}</Label>
      {children}
    </div>
  );
}

function NumberField({
  label,
  value,
  disabled,
  onChange,
}: {
  label: string;
  value: string;
  disabled: boolean;
  onChange: (value: string) => void;
}) {
  return (
    <Field label={label}>
      <Input
        type="number"
        min={0}
        inputMode="numeric"
        value={value}
        disabled={disabled}
        onChange={(event) => onChange(clampNumberInput(event.target.value))}
      />
    </Field>
  );
}

function ToggleField({
  id,
  label,
  description,
  checked,
  onChange,
  disabled,
}: {
  id: string;
  label: string;
  description: string;
  checked: boolean;
  onChange: (value: boolean) => void;
  disabled?: boolean;
}) {
  return (
    <div className="flex items-end">
      <label
        htmlFor={id}
        className={`flex w-full items-start gap-2 rounded-md border border-border/60 px-3 py-2 text-sm ${disabled ? "opacity-60" : ""}`}
      >
        <Checkbox
          id={id}
          checked={checked}
          disabled={disabled}
          onCheckedChange={(value) => onChange(value === true)}
          className="mt-1"
        />
        <span>
          <span className="block font-medium">{label}</span>
          <span className="block text-xs text-muted-foreground">{description}</span>
        </span>
      </label>
    </div>
  );
}

function buildCreateForm(row: StorageListItem, selectedFacility: string): StorageFormState {
  return {
    facility: selectedFacility || row.facility,
    status: (row.status ?? "in_storage") as StorageStatus,
    dateIn: row.dateIn ?? "",
    dateOut: row.dateOut ?? "",
    nextBillDate: row.nextBillDate ?? "",
    lotNumber: row.lotNumber ?? "",
    locationLabel: row.locationLabel ?? "",
    vaults: String(row.vaults),
    pads: String(row.pads),
    items: String(row.items),
    oversizeItems: String(row.oversizeItems),
    volume: String(row.volume),
    monthlyRateCents: row.monthlyRateCents ? String(row.monthlyRateCents) : "",
    storageBalanceCents: String(row.storageBalanceCents),
    moveBalanceCents: String(row.moveBalanceCents),
    notes: "",
  };
}

function buildFormFromRecord(record: StorageRecord): StorageFormState {
  return {
    facility: record.facility,
    status: record.status,
    dateIn: record.dateIn ?? "",
    dateOut: record.dateOut ?? "",
    nextBillDate: record.nextBillDate ?? "",
    lotNumber: record.lotNumber ?? "",
    locationLabel: record.locationLabel ?? "",
    vaults: String(record.vaults),
    pads: String(record.pads),
    items: String(record.items),
    oversizeItems: String(record.oversizeItems),
    volume: String(record.volume),
    monthlyRateCents: record.monthlyRateCents ? String(record.monthlyRateCents) : "",
    storageBalanceCents: String(record.storageBalanceCents),
    moveBalanceCents: String(record.moveBalanceCents),
    notes: record.notes ?? "",
  };
}

function updateForm<K extends keyof StorageFormState>(
  setForm: Dispatch<SetStateAction<StorageFormState | null>>,
  key: K,
  value: StorageFormState[K],
) {
  setForm((prev) => (prev ? { ...prev, [key]: value } : prev));
}

function toNonNegativeInteger(value: string) {
  const parsed = Number.parseInt(value || "0", 10);
  if (!Number.isFinite(parsed) || Number.isNaN(parsed)) return 0;
  return Math.max(0, parsed);
}

function optionalNonNegativeInteger(value: string) {
  if (!value) return undefined;
  return toNonNegativeInteger(value);
}

function clampNumberInput(value: string) {
  if (value === "") return "";
  const parsed = Number.parseInt(value, 10);
  if (!Number.isFinite(parsed) || Number.isNaN(parsed)) return "";
  return String(Math.max(0, parsed));
}

function formatDate(value?: string | null) {
  if (!value) return "-";
  return new Intl.DateTimeFormat("en-US", { month: "short", day: "numeric", year: "numeric" }).format(new Date(`${value}T00:00:00Z`));
}

function formatCurrency(cents: number) {
  return new Intl.NumberFormat("en-US", { style: "currency", currency: "USD" }).format(cents / 100);
}

function formatMoveType(value?: string | null) {
  if (!value) return "-";
  if (value === "long_distance") return "Long distance";
  if (value === "local") return "Local";
  if (value === "other") return "Other";
  return value.replace("_", " ");
}
