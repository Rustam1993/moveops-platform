"use client";

import Link from "next/link";
import { useEffect, useMemo, useState } from "react";
import { Download, Loader2, ShieldAlert, Upload } from "lucide-react";
import { toast } from "sonner";

import { PageHeader } from "@/components/layout/page-header";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import {
  checkImportAccess,
  downloadExportCsv,
  downloadImportErrorsCsv,
  downloadImportReportJson,
  downloadTemplateCsv,
  getApiErrorMessage,
  postImportApply,
  postImportDryRun,
  type ImportOptions,
  type ImportRunResponse,
  type ImportSource,
  type ImportTemplate,
} from "@/lib/import-export-api";

type AccessState = "checking" | "allowed" | "denied";
type Step = 1 | 2 | 3 | 4;

type CanonicalField = {
  key: string;
  label: string;
  required?: boolean;
};

const mappingStorageKey = "moveops.import.mapping.v1";

const stepOrder: Array<{ id: Step; title: string }> = [
  { id: 1, title: "Upload" },
  { id: 2, title: "Map" },
  { id: 3, title: "Dry-run" },
  { id: 4, title: "Apply" },
];

const canonicalFields: CanonicalField[] = [
  { key: "customer_name", label: "Customer Name", required: true },
  { key: "email", label: "Email" },
  { key: "phone_primary", label: "Primary Phone" },
  { key: "phone_secondary", label: "Secondary Phone" },
  { key: "estimate_number", label: "Estimate #" },
  { key: "origin_zip", label: "Origin ZIP" },
  { key: "destination_zip", label: "Destination ZIP" },
  { key: "origin_city", label: "Origin City" },
  { key: "destination_city", label: "Destination City" },
  { key: "requested_pickup_date", label: "Requested Pickup Date" },
  { key: "requested_pickup_time", label: "Requested Pickup Time" },
  { key: "lead_source", label: "Lead Source" },
  { key: "estimated_total", label: "Estimated Total" },
  { key: "deposit", label: "Deposit" },
  { key: "pricing_notes", label: "Pricing Notes" },
  { key: "job_number", label: "Job #", required: true },
  { key: "scheduled_date", label: "Scheduled Date" },
  { key: "pickup_time", label: "Pickup Time" },
  { key: "status", label: "Job Status" },
  { key: "job_type", label: "Job Type" },
  { key: "facility", label: "Facility" },
  { key: "storage_status", label: "Storage Status" },
  { key: "date_in", label: "Date In" },
  { key: "date_out", label: "Date Out" },
  { key: "next_bill_date", label: "Next Bill Date" },
  { key: "lot_number", label: "Lot Number" },
  { key: "location_label", label: "Location Label" },
  { key: "vaults", label: "Vaults" },
  { key: "pads", label: "Pads" },
  { key: "items", label: "Items" },
  { key: "oversize_items", label: "Oversize Items" },
  { key: "volume", label: "Volume" },
  { key: "monthly_rate", label: "Monthly Rate" },
  { key: "storage_balance", label: "Storage Balance" },
  { key: "move_balance", label: "Move Balance" },
];

const templateButtons: Array<{ label: string; template: ImportTemplate }> = [
  { label: "Download customers template", template: "customers" },
  { label: "Download estimates template", template: "estimates" },
  { label: "Download jobs template", template: "jobs" },
  { label: "Download storage template", template: "storage" },
  { label: "Download jobs+storage template", template: "combined" },
];

export default function ImportExportPage() {
  const [accessState, setAccessState] = useState<AccessState>("checking");
  const [step, setStep] = useState<Step>(1);
  const [source, setSource] = useState<ImportSource>("generic");
  const [hasHeader, setHasHeader] = useState(true);

  const [file, setFile] = useState<File | null>(null);
  const [headers, setHeaders] = useState<string[]>([]);
  const [mapping, setMapping] = useState<Record<string, string>>({});

  const [dryRunResult, setDryRunResult] = useState<ImportRunResponse | null>(null);
  const [applyResult, setApplyResult] = useState<ImportRunResponse | null>(null);
  const [runningDryRun, setRunningDryRun] = useState(false);
  const [applyingImport, setApplyingImport] = useState(false);
  const [busyDownload, setBusyDownload] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    checkImportAccess()
      .then((allowed) => {
        if (cancelled) return;
        setAccessState(allowed ? "allowed" : "denied");
      })
      .catch((error) => {
        if (cancelled) return;
        setAccessState("denied");
        toast.error(getApiErrorMessage(error));
      });
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    const raw = localStorage.getItem(mappingStorageKey);
    if (!raw) return;
    try {
      const parsed = JSON.parse(raw) as Record<string, string>;
      setMapping(parsed);
    } catch {
      localStorage.removeItem(mappingStorageKey);
    }
  }, []);

  const mappedCount = useMemo(
    () => canonicalFields.filter((field) => mapping[field.key] && mapping[field.key].trim() !== "").length,
    [mapping],
  );

  const canRunDryRun = Boolean(file && mappedCount > 0);

  async function onFileSelected(nextFile: File | null) {
    setFile(nextFile);
    setDryRunResult(null);
    setApplyResult(null);
    if (!nextFile) {
      setHeaders([]);
      setStep(1);
      return;
    }

    const name = nextFile.name.toLowerCase();
    if (!name.endsWith(".csv") && !name.endsWith(".xlsx")) {
      toast.error("Only CSV uploads are supported in this phase.");
      setHeaders([]);
      return;
    }
    if (name.endsWith(".xlsx")) {
      setHeaders([]);
      setStep(2);
      return;
    }

    try {
      const text = await nextFile.text();
      const firstLine = text.split(/\r?\n/, 1)[0]?.replace(/^\uFEFF/, "") ?? "";
      const parsedHeaders = parseCsvRow(firstLine).map((entry) => entry.trim()).filter(Boolean);
      setHeaders(parsedHeaders);
      if (parsedHeaders.length > 0) {
        setMapping((current) => {
          const alreadyMapped = Object.values(current).some((value) => value && value.trim() !== "");
          return alreadyMapped ? current : autoMapFields(parsedHeaders);
        });
      }
    } catch (error) {
      toast.error(getApiErrorMessage(error));
    } finally {
      setStep(2);
    }
  }

  function saveMappingPreset() {
    localStorage.setItem(mappingStorageKey, JSON.stringify(mapping));
    toast.success("Mapping saved locally");
  }

  function applyAutoMap() {
    if (headers.length === 0) {
      toast.error("Upload a CSV with headers before auto-map");
      return;
    }
    setMapping(autoMapFields(headers));
  }

  async function runDryRun() {
    if (!file) return;
    const payload = buildOptions(source, hasHeader, mapping);
    setRunningDryRun(true);
    try {
      const result = await postImportDryRun(file, payload);
      setDryRunResult(result);
      setStep(3);
      toast.success("Dry-run completed");
    } catch (error) {
      toast.error(getApiErrorMessage(error));
    } finally {
      setRunningDryRun(false);
    }
  }

  async function applyImport() {
    if (!file) return;
    const payload = buildOptions(source, hasHeader, mapping);
    setApplyingImport(true);
    try {
      const result = await postImportApply(file, payload);
      setApplyResult(result);
      setStep(4);
      toast.success("Import applied");
    } catch (error) {
      toast.error(getApiErrorMessage(error));
    } finally {
      setApplyingImport(false);
    }
  }

  async function downloadTemplate(template: ImportTemplate) {
    setBusyDownload(`template:${template}`);
    try {
      const fileResponse = await downloadTemplateCsv(template);
      saveBlob(fileResponse.blob, fileResponse.filename);
    } catch (error) {
      toast.error(getApiErrorMessage(error));
    } finally {
      setBusyDownload(null);
    }
  }

  async function downloadDryRunErrors() {
    if (!dryRunResult) return;
    setBusyDownload("dryrun:errors");
    try {
      const fileResponse = await downloadImportErrorsCsv(dryRunResult.importRunId);
      saveBlob(fileResponse.blob, fileResponse.filename);
    } catch (error) {
      toast.error(getApiErrorMessage(error));
    } finally {
      setBusyDownload(null);
    }
  }

  async function downloadDryRunReport() {
    if (!dryRunResult) return;
    setBusyDownload("dryrun:report");
    try {
      const fileResponse = await downloadImportReportJson(dryRunResult.importRunId);
      saveBlob(fileResponse.blob, fileResponse.filename);
    } catch (error) {
      toast.error(getApiErrorMessage(error));
    } finally {
      setBusyDownload(null);
    }
  }

  async function downloadExport(entity: "customers" | "estimates" | "jobs" | "storage") {
    setBusyDownload(`export:${entity}`);
    try {
      const fileResponse = await downloadExportCsv(entity);
      saveBlob(fileResponse.blob, fileResponse.filename);
    } catch (error) {
      toast.error(getApiErrorMessage(error));
    } finally {
      setBusyDownload(null);
    }
  }

  if (accessState === "checking") {
    return (
      <div className="space-y-6 pb-8">
        <PageHeader title="Import / Export" description="Admin migration tools with dry-run validation and tenant-scoped CSV exports." />
        <Skeleton className="h-32 w-full rounded-xl" />
        <Skeleton className="h-64 w-full rounded-xl" />
      </div>
    );
  }

  if (accessState === "denied") {
    return (
      <div className="space-y-6 pb-8">
        <PageHeader title="Import / Export" description="Admin migration tools with dry-run validation and tenant-scoped CSV exports." />
        <Card className="border-destructive/40">
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-lg">
              <ShieldAlert className="h-5 w-5 text-destructive" />
              Not authorized
            </CardTitle>
            <CardDescription>Import and export tools require `imports.*` and `exports.read` permissions.</CardDescription>
          </CardHeader>
        </Card>
      </div>
    );
  }

  return (
    <div className="space-y-6 pb-8">
      <PageHeader title="Import / Export" description="Upload legacy CSV data, validate in dry-run, apply idempotent imports, and export tenant CSVs." />

      <section className="grid gap-2 rounded-xl border border-border/70 bg-card/40 p-4 md:grid-cols-4">
        {stepOrder.map((entry) => (
          <button
            key={entry.id}
            type="button"
            onClick={() => setStep(entry.id)}
            className={`rounded-md border px-3 py-2 text-left text-sm ${
              step === entry.id ? "border-primary/70 bg-primary/10 text-foreground" : "border-border/60 text-muted-foreground"
            }`}
          >
            <span className="text-xs font-medium uppercase tracking-wide">Step {entry.id}</span>
            <div className="font-medium">{entry.title}</div>
          </button>
        ))}
      </section>

      <Card>
        <CardHeader>
          <CardTitle>1. Upload</CardTitle>
          <CardDescription>CSV is supported now. XLSX will be rejected with `XLSX_NOT_SUPPORTED`.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-4 md:grid-cols-3">
            <div className="space-y-2">
              <Label htmlFor="import-file">File</Label>
              <Input
                id="import-file"
                type="file"
                accept=".csv,.xlsx"
                onChange={(event) => void onFileSelected(event.target.files?.[0] ?? null)}
              />
              <p className="text-xs text-muted-foreground">Max upload size is configured server-side. Default is 15MB.</p>
            </div>
            <div className="space-y-2">
              <Label htmlFor="import-source">Source</Label>
              <select
                id="import-source"
                value={source}
                onChange={(event) => setSource(event.target.value as ImportSource)}
                className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              >
                <option value="generic">Generic</option>
                <option value="granot">Granot</option>
              </select>
            </div>
            <div className="flex items-end pb-2">
              <label className="inline-flex items-center gap-2 text-sm text-muted-foreground">
                <Checkbox checked={hasHeader} onCheckedChange={(value) => setHasHeader(Boolean(value))} />
                CSV includes header row
              </label>
            </div>
          </div>

          <div className="grid gap-2 md:grid-cols-2 xl:grid-cols-3">
            {templateButtons.map((entry) => (
              <Button
                key={entry.template}
                variant="outline"
                onClick={() => void downloadTemplate(entry.template)}
                disabled={busyDownload === `template:${entry.template}`}
                className="justify-start"
              >
                {busyDownload === `template:${entry.template}` ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <Download className="mr-2 h-4 w-4" />}
                {entry.label}
              </Button>
            ))}
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>2. Column Mapping</CardTitle>
          <CardDescription>Detected columns map to canonical import fields. Auto-map uses case-insensitive matching.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex flex-wrap items-center gap-2">
            <Button variant="outline" onClick={applyAutoMap} disabled={headers.length === 0}>
              Auto-map
            </Button>
            <Button variant="outline" onClick={saveMappingPreset}>
              Save mapping to browser
            </Button>
            <span className="text-xs text-muted-foreground">
              {mappedCount} / {canonicalFields.length} fields mapped
            </span>
          </div>

          {headers.length === 0 ? (
            <p className="rounded-md border border-dashed border-border px-3 py-4 text-sm text-muted-foreground">
              Upload a CSV with headers to map columns. If your source is XLSX, export it to CSV first.
            </p>
          ) : (
            <div className="max-h-96 overflow-auto rounded-md border border-border/70">
              <table className="min-w-full text-sm">
                <thead className="sticky top-0 bg-muted/80">
                  <tr className="text-left">
                    <th className="px-3 py-2 font-medium">Canonical field</th>
                    <th className="px-3 py-2 font-medium">Source column</th>
                  </tr>
                </thead>
                <tbody>
                  {canonicalFields.map((field) => (
                    <tr key={field.key} className="border-t border-border/60">
                      <td className="px-3 py-2">
                        <span className="font-medium">{field.label}</span>
                        {field.required ? <span className="ml-2 text-xs text-amber-600">required</span> : null}
                      </td>
                      <td className="px-3 py-2">
                        <select
                          value={mapping[field.key] ?? ""}
                          onChange={(event) => setMapping((current) => ({ ...current, [field.key]: event.target.value }))}
                          className="h-9 w-full rounded-md border border-input bg-background px-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                        >
                          <option value="">Not mapped</option>
                          {headers.map((header) => (
                            <option key={header} value={header}>
                              {header}
                            </option>
                          ))}
                        </select>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>3. Dry-run</CardTitle>
          <CardDescription>Run parse/validate/dedupe simulation before writing any business entities.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <Button onClick={() => void runDryRun()} disabled={!canRunDryRun || runningDryRun}>
            {runningDryRun ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <Upload className="mr-2 h-4 w-4" />}
            Run dry-run
          </Button>

          {dryRunResult ? (
            <div className="space-y-4">
              <ImportSummary run={dryRunResult} />
              <div className="flex flex-wrap gap-2">
                <Button variant="outline" onClick={() => void downloadDryRunErrors()} disabled={busyDownload === "dryrun:errors"}>
                  {busyDownload === "dryrun:errors" ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <Download className="mr-2 h-4 w-4" />}
                  Download errors.csv
                </Button>
                <Button variant="outline" onClick={() => void downloadDryRunReport()} disabled={busyDownload === "dryrun:report"}>
                  {busyDownload === "dryrun:report" ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <Download className="mr-2 h-4 w-4" />}
                  Download report.json
                </Button>
              </div>
              <TopIssues title="Top errors" rows={dryRunResult.topErrors} />
              <TopIssues title="Top warnings" rows={dryRunResult.topWarnings} />
            </div>
          ) : (
            <p className="text-sm text-muted-foreground">Dry-run results will appear here with counts and downloadable reports.</p>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>4. Apply Import</CardTitle>
          <CardDescription>Apply upserts for Customers, Estimates, Jobs, and Storage records.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <Button onClick={() => void applyImport()} disabled={!dryRunResult || applyingImport}>
            {applyingImport ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : null}
            Apply import
          </Button>
          {applyResult ? (
            <div className="space-y-4">
              <ImportSummary run={applyResult} />
              <div className="flex flex-wrap gap-2">
                <Link href="/calendar">
                  <Button variant="outline">Go to Calendar</Button>
                </Link>
                <Link href="/storage">
                  <Button variant="outline">Go to Storage</Button>
                </Link>
              </div>
            </div>
          ) : (
            <p className="text-sm text-muted-foreground">Run dry-run first, then apply to persist data.</p>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>CSV Exports</CardTitle>
          <CardDescription>Download tenant-scoped exports for trust checks and archival snapshots.</CardDescription>
        </CardHeader>
        <CardContent className="grid gap-2 md:grid-cols-2 xl:grid-cols-4">
          <Button variant="outline" onClick={() => void downloadExport("customers")} disabled={busyDownload === "export:customers"}>
            {busyDownload === "export:customers" ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <Download className="mr-2 h-4 w-4" />}
            Customers CSV
          </Button>
          <Button variant="outline" onClick={() => void downloadExport("estimates")} disabled={busyDownload === "export:estimates"}>
            {busyDownload === "export:estimates" ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <Download className="mr-2 h-4 w-4" />}
            Estimates CSV
          </Button>
          <Button variant="outline" onClick={() => void downloadExport("jobs")} disabled={busyDownload === "export:jobs"}>
            {busyDownload === "export:jobs" ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <Download className="mr-2 h-4 w-4" />}
            Jobs CSV
          </Button>
          <Button variant="outline" onClick={() => void downloadExport("storage")} disabled={busyDownload === "export:storage"}>
            {busyDownload === "export:storage" ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <Download className="mr-2 h-4 w-4" />}
            Storage CSV
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}

function ImportSummary({ run }: { run: ImportRunResponse }) {
  return (
    <div className="grid gap-2 md:grid-cols-3 xl:grid-cols-7">
      <Metric label="Rows" value={run.summary.rowsTotal} />
      <Metric label="Valid" value={run.summary.rowsValid} />
      <Metric label="Errors" value={run.summary.rowsError} />
      <Metric label="Customers" value={`C:${run.summary.customer.created} U:${run.summary.customer.updated} S:${run.summary.customer.skipped}`} />
      <Metric label="Estimates" value={`C:${run.summary.estimate.created} U:${run.summary.estimate.updated} S:${run.summary.estimate.skipped}`} />
      <Metric label="Jobs" value={`C:${run.summary.job.created} U:${run.summary.job.updated} S:${run.summary.job.skipped}`} />
      <Metric label="Storage" value={`C:${run.summary.storageRecord.created} U:${run.summary.storageRecord.updated} S:${run.summary.storageRecord.skipped}`} />
    </div>
  );
}

function Metric({ label, value }: { label: string; value: string | number }) {
  return (
    <div className="rounded-md border border-border/70 bg-muted/20 px-3 py-2">
      <p className="text-xs uppercase tracking-wide text-muted-foreground">{label}</p>
      <p className="text-sm font-medium">{value}</p>
    </div>
  );
}

function TopIssues({ title, rows }: { title: string; rows: ImportRunResponse["topErrors"] }) {
  if (rows.length === 0) {
    return (
      <div className="rounded-md border border-dashed border-border px-3 py-3 text-sm text-muted-foreground">
        {title}: none
      </div>
    );
  }

  return (
    <div className="rounded-md border border-border/70">
      <div className="border-b border-border/70 px-3 py-2 text-sm font-medium">{title}</div>
      <div className="max-h-56 overflow-auto">
        <table className="min-w-full text-sm">
          <thead className="bg-muted/60">
            <tr className="text-left">
              <th className="px-3 py-2 font-medium">Row</th>
              <th className="px-3 py-2 font-medium">Entity</th>
              <th className="px-3 py-2 font-medium">Message</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr key={`${title}-${row.rowNumber}-${row.idempotencyKey}`} className="border-t border-border/60">
                <td className="px-3 py-2">{row.rowNumber}</td>
                <td className="px-3 py-2">{row.entityType}</td>
                <td className="px-3 py-2">{row.message}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function autoMapFields(headers: string[]) {
  const lookup = new Map<string, string>();
  for (const header of headers) {
    lookup.set(normalizeFieldName(header), header);
  }

  const mapped: Record<string, string> = {};
  for (const field of canonicalFields) {
    const normalized = normalizeFieldName(field.key);
    const match = lookup.get(normalized);
    if (match) mapped[field.key] = match;
  }
  return mapped;
}

function normalizeFieldName(value: string) {
  return value.toLowerCase().replace(/[^a-z0-9]/g, "");
}

function parseCsvRow(input: string) {
  const result: string[] = [];
  let current = "";
  let inQuotes = false;

  for (let i = 0; i < input.length; i += 1) {
    const char = input[i];
    if (char === '"') {
      if (inQuotes && input[i + 1] === '"') {
        current += '"';
        i += 1;
      } else {
        inQuotes = !inQuotes;
      }
      continue;
    }
    if (char === "," && !inQuotes) {
      result.push(current);
      current = "";
      continue;
    }
    current += char;
  }

  result.push(current);
  return result;
}

function buildOptions(source: ImportSource, hasHeader: boolean, mapping: Record<string, string>): ImportOptions {
  const compact: Record<string, string> = {};
  for (const [field, value] of Object.entries(mapping)) {
    const trimmed = value.trim();
    if (trimmed) compact[field] = trimmed;
  }

  return {
    source,
    hasHeader,
    mapping: compact,
  };
}

function saveBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = filename;
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  URL.revokeObjectURL(url);
}
