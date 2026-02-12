import type { components, operations } from "@moveops/client";

import { api } from "@/lib/api";
import { getCsrf } from "@/lib/session";

export type StorageStatus = components["schemas"]["StorageStatus"];
export type StorageListItem = components["schemas"]["StorageListItem"];
export type StorageRecord = components["schemas"]["StorageRecord"];
export type CreateStorageRecordRequest = components["schemas"]["CreateStorageRecordRequest"];
export type UpdateStorageRecordRequest = components["schemas"]["UpdateStorageRecordRequest"];

type StorageListResponse = components["schemas"]["StorageListResponse"];
type StorageRecordResponse = components["schemas"]["StorageRecordResponse"];
type StorageQuery = NonNullable<operations["GetStorage"]["parameters"]["query"]>;

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

export async function getStorageRows(params: {
  facility: StorageQuery["facility"];
  q?: StorageQuery["q"];
  status?: StorageQuery["status"];
  hasDateOut?: StorageQuery["hasDateOut"];
  balanceDue?: StorageQuery["balanceDue"];
  pastDueDays?: StorageQuery["pastDueDays"];
  hasContainers?: StorageQuery["hasContainers"];
  limit?: StorageQuery["limit"];
  cursor?: StorageQuery["cursor"];
}) {
  const query = new URLSearchParams({ facility: params.facility });
  if (params.q) query.set("q", params.q);
  if (params.status) query.set("status", params.status);
  if (typeof params.hasDateOut === "boolean") query.set("hasDateOut", String(params.hasDateOut));
  if (typeof params.balanceDue === "boolean") query.set("balanceDue", String(params.balanceDue));
  if (typeof params.pastDueDays === "number") query.set("pastDueDays", String(params.pastDueDays));
  if (typeof params.hasContainers === "boolean") query.set("hasContainers", String(params.hasContainers));
  if (typeof params.limit === "number") query.set("limit", String(params.limit));
  if (params.cursor) query.set("cursor", params.cursor);

  return api.request<StorageListResponse>(`/storage?${query.toString()}`);
}

export async function getStorageRecord(storageRecordId: string) {
  return api.request<StorageRecordResponse>(`/storage/${storageRecordId}`);
}

export async function createStorageRecord(jobId: string, payload: CreateStorageRecordRequest) {
  const csrfToken = await getCsrfToken();
  return api.request<StorageRecordResponse>(`/jobs/${jobId}/storage`, {
    method: "POST",
    body: JSON.stringify(payload),
    headers: {
      "X-CSRF-Token": csrfToken,
    },
  });
}

export async function updateStorageRecord(storageRecordId: string, payload: UpdateStorageRecordRequest) {
  const csrfToken = await getCsrfToken();
  return api.request<StorageRecordResponse>(`/storage/${storageRecordId}`, {
    method: "PUT",
    body: JSON.stringify(payload),
    headers: {
      "X-CSRF-Token": csrfToken,
    },
  });
}
