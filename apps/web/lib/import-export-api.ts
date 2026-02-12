import type { components } from "@moveops/client";

import { getCsrf } from "@/lib/session";

const apiBase = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080/api";

export type ImportSource = components["schemas"]["ImportSource"];
export type ImportTemplate = components["schemas"]["ImportTemplate"];
export type ImportOptions = components["schemas"]["ImportOptions"];
export type ImportRunResponse = components["schemas"]["ImportRunResponse"];
export type ImportRunReportResponse = components["schemas"]["ImportRunReportResponse"];

type ErrorEnvelope = components["schemas"]["ErrorEnvelope"];

export function getApiErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : "Request failed";
}

async function getCsrfToken() {
  const cached = sessionStorage.getItem("csrfToken");
  if (cached) return cached;
  const data = await getCsrf();
  sessionStorage.setItem("csrfToken", data.csrfToken);
  return data.csrfToken;
}

async function throwApiError(response: Response): Promise<never> {
  const fallback = `HTTP ${response.status}`;
  let message = fallback;
  try {
    const payload = (await response.json()) as ErrorEnvelope;
    if (payload?.error?.message) {
      message = payload.error.message;
    }
  } catch {
    message = fallback;
  }
  throw new Error(message);
}

function inferFilename(response: Response, fallback: string) {
  const disposition = response.headers.get("Content-Disposition") ?? "";
  const match = disposition.match(/filename="?([^"]+)"?/i);
  return match?.[1] ?? fallback;
}

async function fetchJSON<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${apiBase}${path}`, {
    ...init,
    credentials: "include",
  });
  if (!response.ok) {
    return throwApiError(response);
  }
  return (await response.json()) as T;
}

async function fetchFile(path: string, init: RequestInit | undefined, fallbackName: string) {
  const response = await fetch(`${apiBase}${path}`, {
    ...init,
    credentials: "include",
  });
  if (!response.ok) {
    return throwApiError(response);
  }
  return {
    filename: inferFilename(response, fallbackName),
    blob: await response.blob(),
  };
}

export async function checkImportAccess() {
  const response = await fetch(`${apiBase}/imports/templates/customers.csv`, {
    method: "GET",
    credentials: "include",
  });
  if (response.status === 403) {
    return false;
  }
  if (!response.ok) {
    return throwApiError(response);
  }
  return true;
}

export async function postImportDryRun(file: File, options: ImportOptions) {
  const csrfToken = await getCsrfToken();
  const form = new FormData();
  form.append("file", file);
  form.append("options", new Blob([JSON.stringify(options)], { type: "application/json" }));
  return fetchJSON<ImportRunResponse>("/imports/dry-run", {
    method: "POST",
    headers: {
      "X-CSRF-Token": csrfToken,
    },
    body: form,
  });
}

export async function postImportApply(file: File, options: ImportOptions) {
  const csrfToken = await getCsrfToken();
  const form = new FormData();
  form.append("file", file);
  form.append("options", new Blob([JSON.stringify(options)], { type: "application/json" }));
  return fetchJSON<ImportRunResponse>("/imports/apply", {
    method: "POST",
    headers: {
      "X-CSRF-Token": csrfToken,
    },
    body: form,
  });
}

export async function getImportRun(importRunId: string) {
  return fetchJSON<ImportRunResponse>(`/imports/${importRunId}`);
}

export async function getImportReport(importRunId: string) {
  return fetchJSON<ImportRunReportResponse>(`/imports/${importRunId}/report.json`);
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
