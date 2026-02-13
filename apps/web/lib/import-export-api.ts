import type { components } from "@moveops/client";

import { isForbiddenError, requestBlob, requestJSON } from "@/lib/api";

export type ImportSource = components["schemas"]["ImportSource"];
export type ImportTemplate = components["schemas"]["ImportTemplate"];
export type ImportOptions = components["schemas"]["ImportOptions"];
export type ImportRunResponse = components["schemas"]["ImportRunResponse"];
export type ImportRunReportResponse = components["schemas"]["ImportRunReportResponse"];

export function getApiErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : "Request failed";
}

function inferFilename(response: Response, fallback: string) {
  const disposition = response.headers.get("Content-Disposition") ?? "";
  const match = disposition.match(/filename="?([^"]+)"?/i);
  return match?.[1] ?? fallback;
}

async function fetchFile(path: string, init: RequestInit | undefined, fallbackName: string) {
  const response = await requestBlob(path, init);
  return {
    filename: inferFilename(response, fallbackName),
    blob: await response.blob(),
  };
}

export async function checkImportAccess() {
  try {
    await requestBlob("/imports/templates/customers.csv", { method: "GET" }, { suppressAuthRedirect: true });
    return true;
  } catch (error) {
    if (isForbiddenError(error)) {
      return false;
    }
    throw error;
  }
}

export async function postImportDryRun(file: File, options: ImportOptions) {
  const form = new FormData();
  form.append("file", file);
  form.append("options", new Blob([JSON.stringify(options)], { type: "application/json" }));
  return requestJSON<ImportRunResponse>("/imports/dry-run", {
    method: "POST",
    body: form,
  });
}

export async function postImportApply(file: File, options: ImportOptions) {
  const form = new FormData();
  form.append("file", file);
  form.append("options", new Blob([JSON.stringify(options)], { type: "application/json" }));
  return requestJSON<ImportRunResponse>("/imports/apply", {
    method: "POST",
    body: form,
  });
}

export async function getImportRun(importRunId: string) {
  return requestJSON<ImportRunResponse>(`/imports/${importRunId}`);
}

export async function getImportReport(importRunId: string) {
  return requestJSON<ImportRunReportResponse>(`/imports/${importRunId}/report.json`);
}

export async function downloadImportErrorsCsv(importRunId: string) {
  return fetchFile(`/imports/${importRunId}/errors.csv`, undefined, `import-${importRunId}-errors.csv`);
}

export async function downloadImportReportJson(importRunId: string) {
  return fetchFile(`/imports/${importRunId}/report.json`, undefined, `import-${importRunId}-report.json`);
}

export async function downloadTemplateCsv(template: ImportTemplate) {
  return fetchFile(`/imports/templates/${template}.csv`, undefined, `import-template-${template}.csv`);
}

export async function downloadExportCsv(entity: "customers" | "estimates" | "jobs" | "storage") {
  return fetchFile(`/exports/${entity}.csv`, undefined, `${entity}.csv`);
}
