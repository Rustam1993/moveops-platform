import type { components, operations } from "@moveops/client";

import { api } from "@/lib/api";
import { getCsrf } from "@/lib/session";

export type Estimate = components["schemas"]["Estimate"];
export type Job = components["schemas"]["Job"];
export type CalendarJobCard = components["schemas"]["CalendarJobCard"];
export type CreateEstimateRequest = components["schemas"]["CreateEstimateRequest"];
export type UpdateEstimateRequest = components["schemas"]["UpdateEstimateRequest"];
export type UpdateJobRequest = components["schemas"]["UpdateJobRequest"];
export type CalendarPhase = Exclude<NonNullable<operations["GetCalendar"]["parameters"]["query"]>["phase"], undefined>;
export type CalendarJobType = Exclude<NonNullable<operations["GetCalendar"]["parameters"]["query"]>["jobType"], undefined>;

type EstimateResponse = components["schemas"]["EstimateResponse"];
type JobResponse = components["schemas"]["JobResponse"];
type CalendarResponse = components["schemas"]["CalendarResponse"];

export function newIdempotencyKey(prefix: "estimate" | "convert") {
  return `${prefix}-${crypto.randomUUID()}`;
}

export function getApiErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : "Request failed";
}

async function getCsrfToken() {
  const cached = sessionStorage.getItem("csrfToken");
  if (cached) {
    return cached;
  }

  const data = await getCsrf();
  sessionStorage.setItem("csrfToken", data.csrfToken);
  return data.csrfToken;
}

export async function createEstimate(payload: CreateEstimateRequest, idempotencyKey: string) {
  const csrfToken = await getCsrfToken();
  return api.request<EstimateResponse>("/estimates", {
    method: "POST",
    body: JSON.stringify(payload),
    headers: {
      "X-CSRF-Token": csrfToken,
      "Idempotency-Key": idempotencyKey,
    },
  });
}

export async function getEstimate(estimateId: string) {
  return api.request<EstimateResponse>(`/estimates/${estimateId}`);
}

export async function updateEstimate(estimateId: string, payload: UpdateEstimateRequest) {
  const csrfToken = await getCsrfToken();
  return api.request<EstimateResponse>(`/estimates/${estimateId}`, {
    method: "PATCH",
    body: JSON.stringify(payload),
    headers: {
      "X-CSRF-Token": csrfToken,
    },
  });
}

export async function convertEstimate(estimateId: string, idempotencyKey: string) {
  const csrfToken = await getCsrfToken();
  return api.request<JobResponse>(`/estimates/${estimateId}/convert`, {
    method: "POST",
    headers: {
      "X-CSRF-Token": csrfToken,
      "Idempotency-Key": idempotencyKey,
    },
  });
}

export async function getJob(jobId: string) {
  return api.request<JobResponse>(`/jobs/${jobId}`);
}

export async function getCalendar(params: {
  from: string;
  to: string;
  phase?: CalendarPhase;
  jobType?: CalendarJobType;
  userId?: string;
  departmentId?: string;
}) {
  const query = new URLSearchParams({
    from: params.from,
    to: params.to,
  });
  if (params.phase) query.set("phase", params.phase);
  if (params.jobType) query.set("jobType", params.jobType);
  if (params.userId) query.set("userId", params.userId);
  if (params.departmentId) query.set("departmentId", params.departmentId);

  return api.request<CalendarResponse>(`/calendar?${query.toString()}`);
}

export async function updateJob(jobId: string, payload: UpdateJobRequest) {
  const csrfToken = await getCsrfToken();
  return api.request<JobResponse>(`/jobs/${jobId}`, {
    method: "PATCH",
    body: JSON.stringify(payload),
    headers: {
      "X-CSRF-Token": csrfToken,
    },
  });
}
