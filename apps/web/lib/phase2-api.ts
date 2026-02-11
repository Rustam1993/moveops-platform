import type { components } from "@moveops/client";

import { api } from "@/lib/api";
import { getCsrf } from "@/lib/session";

export type Estimate = components["schemas"]["Estimate"];
export type Job = components["schemas"]["Job"];
export type CreateEstimateRequest = components["schemas"]["CreateEstimateRequest"];
export type UpdateEstimateRequest = components["schemas"]["UpdateEstimateRequest"];
export type UpdateJobRequest = components["schemas"]["UpdateJobRequest"];

type EstimateResponse = components["schemas"]["EstimateResponse"];
type JobResponse = components["schemas"]["JobResponse"];

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
