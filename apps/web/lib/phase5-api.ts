import { api } from "@/lib/api";

export type EstimateWorkflowStatus =
  | "draft"
  | "open"
  | "follow_up"
  | "quoted"
  | "booked"
  | "on_hold"
  | "canceled";

export type EstimateWorkflow = {
  estimateId: string;
  status: EstimateWorkflowStatus;
  priorityLevel: number;
  followUpAt?: string;
  followUpNote?: string;
  vip: boolean;
  bookedAt?: string;
  holdReason?: string;
  updatedAt: string;
};

export type EstimateWorkflowResponse = {
  workflow: EstimateWorkflow;
  requestId: string;
};

export type UpdateEstimateWorkflowRequest = {
  status?: EstimateWorkflowStatus;
  priorityLevel?: number;
  followUpAt?: string;
  clearFollowUpAt?: boolean;
  followUpNote?: string;
  clearFollowUpNote?: boolean;
  vip?: boolean;
};

export type HoldEstimateRequest = {
  holdReason?: string;
};

export type EstimateTask = {
  id: string;
  estimateId: string;
  title: string;
  isDone: boolean;
  dueAt?: string;
  createdAt: string;
  updatedAt: string;
};

export type EstimateTaskListResponse = {
  tasks: EstimateTask[];
  requestId: string;
};

export type EstimateTaskResponse = {
  task: EstimateTask;
  requestId: string;
};

export type CreateEstimateTaskRequest = {
  title: string;
  dueAt?: string;
};

export type UpdateEstimateTaskRequest = {
  title?: string;
  isDone?: boolean;
  dueAt?: string;
  clearDueAt?: boolean;
};

export type EstimatePayment = {
  id: string;
  estimateId: string;
  amountCents: number;
  method: string;
  paidAt: string;
  notes?: string;
  createdAt: string;
};

export type EstimatePaymentSummary = {
  depositRequiredCents?: number;
  totalEstimateCents?: number;
  amountPaidCents: number;
  remainingBalanceCents?: number;
};

export type EstimatePaymentListResponse = {
  summary: EstimatePaymentSummary;
  payments: EstimatePayment[];
  requestId: string;
};

export type EstimatePaymentResponse = {
  payment: EstimatePayment;
  requestId: string;
};

export type CreateEstimatePaymentRequest = {
  amountCents: number;
  method: string;
  paidAt?: string;
  notes?: string;
};

export async function getEstimateWorkflow(estimateId: string) {
  return api.request<EstimateWorkflowResponse>(`/estimates/${estimateId}/workflow`);
}

export async function updateEstimateWorkflow(estimateId: string, payload: UpdateEstimateWorkflowRequest) {
  return api.request<EstimateWorkflowResponse>(`/estimates/${estimateId}/workflow`, {
    method: "PATCH",
    body: JSON.stringify(payload),
  });
}

export async function bookEstimate(estimateId: string) {
  return api.request<EstimateWorkflowResponse>(`/estimates/${estimateId}/book`, {
    method: "POST",
  });
}

export async function releaseEstimateBooking(estimateId: string) {
  return api.request<EstimateWorkflowResponse>(`/estimates/${estimateId}/release-book`, {
    method: "POST",
  });
}

export async function holdEstimate(estimateId: string, payload?: HoldEstimateRequest) {
  return api.request<EstimateWorkflowResponse>(`/estimates/${estimateId}/hold`, {
    method: "POST",
    body: JSON.stringify(payload ?? {}),
  });
}

export async function getEstimateTasks(estimateId: string) {
  return api.request<EstimateTaskListResponse>(`/estimates/${estimateId}/tasks`);
}

export async function createEstimateTask(estimateId: string, payload: CreateEstimateTaskRequest) {
  return api.request<EstimateTaskResponse>(`/estimates/${estimateId}/tasks`, {
    method: "POST",
    body: JSON.stringify(payload),
  });
}

export async function updateEstimateTask(estimateId: string, taskId: string, payload: UpdateEstimateTaskRequest) {
  return api.request<EstimateTaskResponse>(`/estimates/${estimateId}/tasks/${taskId}`, {
    method: "PATCH",
    body: JSON.stringify(payload),
  });
}

export async function deleteEstimateTask(estimateId: string, taskId: string) {
  return api.request<void>(`/estimates/${estimateId}/tasks/${taskId}`, {
    method: "DELETE",
  });
}

export async function getEstimatePayments(estimateId: string) {
  return api.request<EstimatePaymentListResponse>(`/estimates/${estimateId}/payments`);
}

export async function createEstimatePayment(estimateId: string, payload: CreateEstimatePaymentRequest) {
  return api.request<EstimatePaymentResponse>(`/estimates/${estimateId}/payments`, {
    method: "POST",
    body: JSON.stringify(payload),
  });
}

export async function deleteEstimatePayment(estimateId: string, paymentId: string) {
  return api.request<void>(`/estimates/${estimateId}/payments/${paymentId}`, {
    method: "DELETE",
  });
}
