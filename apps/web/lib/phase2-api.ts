import type { components, operations } from "@moveops/client";

import { api } from "@/lib/api";

export type Estimate = components["schemas"]["Estimate"];
export type Job = components["schemas"]["Job"];
export type CalendarJobCard = components["schemas"]["CalendarJobCard"];
export type JobListItem = components["schemas"]["JobListItem"];
export type EstimateListItem = components["schemas"]["EstimateListItem"];
export type CreateEstimateRequest = components["schemas"]["CreateEstimateRequest"];
export type UpdateEstimateRequest = components["schemas"]["UpdateEstimateRequest"];
export type UpdateJobRequest = components["schemas"]["UpdateJobRequest"];
export type CalendarPhase = Exclude<NonNullable<operations["GetCalendar"]["parameters"]["query"]>["phase"], undefined>;
export type CalendarJobType = Exclude<NonNullable<operations["GetCalendar"]["parameters"]["query"]>["jobType"], undefined>;
export type JobListStatus = Exclude<NonNullable<operations["GetJobs"]["parameters"]["query"]>["status"], undefined>;
export type JobListJobType = Exclude<NonNullable<operations["GetJobs"]["parameters"]["query"]>["jobType"], undefined>;
export type EstimateListStatus = Exclude<NonNullable<operations["GetEstimates"]["parameters"]["query"]>["status"], undefined>;

type EstimateResponse = components["schemas"]["EstimateResponse"];
type JobResponse = components["schemas"]["JobResponse"];
type CalendarResponse = components["schemas"]["CalendarResponse"];
type JobListResponse = components["schemas"]["JobListResponse"];
type EstimateListResponse = components["schemas"]["EstimateListResponse"];
type DashboardSummaryResponse = components["schemas"]["DashboardSummaryResponse"];

export function newIdempotencyKey(prefix: "estimate" | "convert") {
  return `${prefix}-${crypto.randomUUID()}`;
}

export function getApiErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : "Request failed";
}

export async function createEstimate(payload: CreateEstimateRequest, idempotencyKey: string) {
  return api.request<EstimateResponse>("/estimates", {
    method: "POST",
    body: JSON.stringify(payload),
    headers: {
      "Idempotency-Key": idempotencyKey,
    },
  });
}

export async function getEstimate(estimateId: string) {
  return api.request<EstimateResponse>(`/estimates/${estimateId}`);
}

export async function updateEstimate(estimateId: string, payload: UpdateEstimateRequest) {
  return api.request<EstimateResponse>(`/estimates/${estimateId}`, {
    method: "PATCH",
    body: JSON.stringify(payload),
  });
}

export async function convertEstimate(estimateId: string, idempotencyKey: string) {
  return api.request<JobResponse>(`/estimates/${estimateId}/convert`, {
    method: "POST",
    headers: {
      "Idempotency-Key": idempotencyKey,
    },
  });
}

export async function getJob(jobId: string) {
  return api.request<JobResponse>(`/jobs/${jobId}`);
}

export async function getJobsList(params?: {
  q?: string;
  status?: JobListStatus;
  jobType?: JobListJobType;
  scheduled?: boolean;
  scheduledFrom?: string;
  scheduledTo?: string;
  limit?: number;
  cursor?: string;
}) {
  const query = new URLSearchParams();
  if (params?.q) query.set("q", params.q);
  if (params?.status) query.set("status", params.status);
  if (params?.jobType) query.set("jobType", params.jobType);
  if (params?.scheduled !== undefined) query.set("scheduled", String(params.scheduled));
  if (params?.scheduledFrom) query.set("scheduledFrom", params.scheduledFrom);
  if (params?.scheduledTo) query.set("scheduledTo", params.scheduledTo);
  if (params?.limit) query.set("limit", String(params.limit));
  if (params?.cursor) query.set("cursor", params.cursor);
  const suffix = query.toString();
  return api.request<JobListResponse>(`/jobs${suffix ? `?${suffix}` : ""}`);
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
  return api.request<JobResponse>(`/jobs/${jobId}`, {
    method: "PATCH",
    body: JSON.stringify(payload),
  });
}

export async function getEstimatesList(params?: {
  q?: string;
  status?: EstimateListStatus;
  limit?: number;
  cursor?: string;
}) {
  const query = new URLSearchParams();
  if (params?.q) query.set("q", params.q);
  if (params?.status) query.set("status", params.status);
  if (params?.limit) query.set("limit", String(params.limit));
  if (params?.cursor) query.set("cursor", params.cursor);
  const suffix = query.toString();
  return api.request<EstimateListResponse>(`/estimates${suffix ? `?${suffix}` : ""}`);
}

export async function getDashboardSummary() {
  return api.request<DashboardSummaryResponse>("/dashboard/summary");
}
