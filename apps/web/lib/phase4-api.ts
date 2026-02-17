import { api, requestJSON } from "@/lib/api";

export type EstimateDocumentType = "estimate_pdf" | "signed_estimate_pdf";

export type EstimateDocument = {
  id: string;
  estimateId: string;
  documentType: EstimateDocumentType;
  fileName: string;
  mimeType: string;
  sizeBytes: number;
  contentBase64: string;
  metadata?: Record<string, unknown>;
  createdAt: string;
};

export type EstimateDocumentResponse = {
  document: EstimateDocument;
  requestId: string;
};

export type EstimateEmailTemplateKey =
  | "moving_estimate"
  | "update_inventory"
  | "signature_request"
  | "credit_card_authorization"
  | "waiver_cancellation"
  | "follow_up_move";

export type EstimateEmailLog = {
  id: string;
  estimateId: string;
  templateKey: EstimateEmailTemplateKey;
  to: string;
  cc?: string;
  from: string;
  subject: string;
  status: "queued" | "sent" | "failed";
  deliveryMode: "log" | "smtp";
  providerMessageId?: string;
  errorMessage?: string;
  createdAt: string;
};

export type EstimateEmailLogListResponse = {
  emails: EstimateEmailLog[];
  requestId: string;
};

export type SendEstimateEmailRequest = {
  templateKey: EstimateEmailTemplateKey;
  toEmail?: string;
  ccMe?: boolean;
};

export type SendEstimateEmailResponse = {
  email: EstimateEmailLog;
  generatedLinks?: {
    quoteUrl?: string;
    inventoryUrl?: string;
    signatureUrl?: string;
  };
  requestId: string;
};

export type PublicEstimateDocumentResponse = {
  estimateId: string;
  customerName: string;
  moveDate: string;
  totalVolumeCf: number;
  totalEstimateCents?: number;
  document: EstimateDocument;
  expiresAt: string;
  requestId: string;
};

export type PublicSignContextResponse = {
  estimateId: string;
  customerName: string;
  moveDate: string;
  totalVolumeCf: number;
  totalEstimateCents?: number;
  document: EstimateDocument;
  signerEmail: string;
  expiresAt: string;
  alreadySigned: boolean;
  requestId: string;
};

export type CompleteSignatureRequest = {
  signerName: string;
  signerEmail: string;
  signatureText: string;
  agreeToTerms: boolean;
};

export type CompleteSignatureResponse = {
  estimateId: string;
  signatureId: string;
  signerName: string;
  signerEmail: string;
  signedAt: string;
  document: EstimateDocument;
  requestId: string;
};

export async function generateEstimatePdf(estimateId: string) {
  return api.request<EstimateDocumentResponse>(`/estimates/${estimateId}/documents/estimate-pdf`, {
    method: "POST",
  });
}

export async function getEstimatePdf(estimateId: string) {
  return api.request<EstimateDocumentResponse>(`/estimates/${estimateId}/documents/estimate-pdf`);
}

export async function listEstimateEmails(estimateId: string) {
  return api.request<EstimateEmailLogListResponse>(`/estimates/${estimateId}/emails`);
}

export async function sendEstimateEmail(estimateId: string, payload: SendEstimateEmailRequest) {
  return api.request<SendEstimateEmailResponse>(`/estimates/${estimateId}/emails/send`, {
    method: "POST",
    body: JSON.stringify(payload),
  });
}

export async function getPublicEstimate(token: string) {
  return requestJSON<PublicEstimateDocumentResponse>(`/public/estimate/${token}`, undefined, {
    withCsrf: false,
    suppressAuthRedirect: true,
  });
}

export async function getPublicSign(token: string) {
  return requestJSON<PublicSignContextResponse>(`/public/sign/${token}`, undefined, {
    withCsrf: false,
    suppressAuthRedirect: true,
  });
}

export async function completePublicSign(token: string, payload: CompleteSignatureRequest) {
  return requestJSON<CompleteSignatureResponse>(
    `/public/sign/${token}`,
    {
      method: "POST",
      body: JSON.stringify(payload),
    },
    {
      withCsrf: false,
      suppressAuthRedirect: true,
    },
  );
}
