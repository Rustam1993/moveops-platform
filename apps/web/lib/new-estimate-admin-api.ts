import { api, requestBlob } from "@/lib/api";

export type AdminCatalogCategory = {
  id: string;
  name: string;
  sortOrder: number;
  active: boolean;
};

export type AdminCatalogItem = {
  id: string;
  categoryId?: string | null;
  categoryName?: string | null;
  itemName: string;
  volumeCf: number;
  sortOrder: number;
  active: boolean;
};

export type NewEstimatePricingDefaults = {
  longDistance?: {
    ratePerCf?: number;
    fuelSurchargePct?: number;
    taxRatePct?: number;
  };
  local?: {
    laborRateCents?: number;
    travelRateCents?: number;
    fuelSurchargePct?: number;
    taxRatePct?: number;
  };
  discounts?: {
    seniorPct?: number;
    couponPct?: number;
  };
  liability?: {
    type?: "release" | "full_value";
    valuationChargeCents?: number;
  };
};

export type NewEstimateEmailTemplate = {
  subject?: string;
  htmlBody?: string;
  textBody?: string;
};

export type NewEstimateEmailTemplates = {
  eQuote?: NewEstimateEmailTemplate;
  inventoryLink?: NewEstimateEmailTemplate;
  eSign?: NewEstimateEmailTemplate;
};

export type NewEstimateDocumentBranding = {
  companyDisplayName?: string;
  companyPhone?: string;
  companyEmail?: string;
  termsSnippet?: string;
  logoUrl?: string;
};

export type NewEstimateMetrics = {
  medianTimeToQuoteMinutes: number;
  quoteToSign: { convertedCount: number; totalCount: number; rate: number };
  signToBook: { convertedCount: number; totalCount: number; rate: number };
  inventoryCompletion: { convertedCount: number; totalCount: number; rate: number };
  stuckEstimatesCount: number;
};

export type AdminAuditLogEntry = {
  id: number;
  userId?: string;
  action: string;
  entityType: string;
  entityId?: string;
  requestId?: string;
  metadata: Record<string, unknown>;
  createdAt: string;
};

export async function listCatalogCategories() {
  return api.request<{ categories: AdminCatalogCategory[]; requestId: string }>("/admin/new-estimate/catalog/categories");
}

export async function createCatalogCategory(payload: { name: string; sortOrder?: number; active?: boolean }) {
  return api.request<{ category: AdminCatalogCategory; requestId: string }>("/admin/new-estimate/catalog/categories", {
    method: "POST",
    body: JSON.stringify(payload),
  });
}

export async function updateCatalogCategory(categoryId: string, payload: { name?: string; sortOrder?: number; active?: boolean }) {
  return api.request<{ category: AdminCatalogCategory; requestId: string }>(`/admin/new-estimate/catalog/categories/${categoryId}`, {
    method: "PATCH",
    body: JSON.stringify(payload),
  });
}

export async function deleteCatalogCategory(categoryId: string) {
  return api.request<void>(`/admin/new-estimate/catalog/categories/${categoryId}`, {
    method: "DELETE",
  });
}

export async function listCatalogItems() {
  return api.request<{ items: AdminCatalogItem[]; requestId: string }>("/admin/new-estimate/catalog/items");
}

export async function createCatalogItem(payload: {
  categoryId?: string;
  itemName: string;
  volumeCf: number;
  sortOrder?: number;
  active?: boolean;
}) {
  return api.request<{ item: AdminCatalogItem; requestId: string }>("/admin/new-estimate/catalog/items", {
    method: "POST",
    body: JSON.stringify(payload),
  });
}

export async function updateCatalogItem(
  itemId: string,
  payload: {
    categoryId?: string | null;
    itemName?: string;
    volumeCf?: number;
    sortOrder?: number;
    active?: boolean;
  },
) {
  return api.request<{ item: AdminCatalogItem; requestId: string }>(`/admin/new-estimate/catalog/items/${itemId}`, {
    method: "PATCH",
    body: JSON.stringify(payload),
  });
}

export async function deleteCatalogItem(itemId: string) {
  return api.request<void>(`/admin/new-estimate/catalog/items/${itemId}`, {
    method: "DELETE",
  });
}

export async function importCatalogCsv(csvText: string) {
  return api.request<{ categoriesImported: number; itemsImported: number; errors?: string[]; requestId: string }>(
    "/admin/new-estimate/catalog/import",
    {
      method: "POST",
      headers: {
        "Content-Type": "text/csv",
      },
      body: csvText,
    },
  );
}

export async function exportCatalogCsv() {
  const response = await requestBlob("/admin/new-estimate/catalog/export");
  const blob = await response.blob();
  return {
    blob,
    filename: extractFilename(response.headers.get("content-disposition")) ?? "new-estimate-catalog.csv",
  };
}

export async function getPricingDefaults() {
  return api.request<{ pricing: NewEstimatePricingDefaults; requestId: string }>("/admin/new-estimate/pricing");
}

export async function savePricingDefaults(payload: NewEstimatePricingDefaults) {
  return api.request<{ pricing: NewEstimatePricingDefaults; requestId: string }>("/admin/new-estimate/pricing", {
    method: "PUT",
    body: JSON.stringify(payload),
  });
}

export async function getEmailTemplates() {
  return api.request<{ templates: NewEstimateEmailTemplates; allowedVariables: string[]; requestId: string }>(
    "/admin/new-estimate/email-templates",
  );
}

export async function saveEmailTemplates(payload: NewEstimateEmailTemplates) {
  return api.request<{ templates: NewEstimateEmailTemplates; allowedVariables: string[]; requestId: string }>(
    "/admin/new-estimate/email-templates",
    {
      method: "PUT",
      body: JSON.stringify(payload),
    },
  );
}

export async function testSendEmailTemplate(payload: { templateKey: "e_quote" | "inventory_link" | "e_sign"; toEmail: string }) {
  return api.request<{ status: "sent" | "failed" | "logged"; requestId: string }>(
    "/admin/new-estimate/email-templates/test-send",
    {
      method: "POST",
      body: JSON.stringify(payload),
    },
  );
}

export async function getDocumentBranding() {
  return api.request<{ branding: NewEstimateDocumentBranding; requestId: string }>("/admin/new-estimate/documents");
}

export async function saveDocumentBranding(payload: NewEstimateDocumentBranding) {
  return api.request<{ branding: NewEstimateDocumentBranding; requestId: string }>("/admin/new-estimate/documents", {
    method: "PUT",
    body: JSON.stringify(payload),
  });
}

export async function getNewEstimateMetrics(params?: { from?: string; to?: string }) {
  const query = new URLSearchParams();
  if (params?.from) query.set("from", params.from);
  if (params?.to) query.set("to", params.to);
  const suffix = query.toString();
  return api.request<{ metrics: NewEstimateMetrics; requestId: string }>(`/admin/new-estimate/metrics${suffix ? `?${suffix}` : ""}`);
}

export async function listAuditLogs(params?: {
  from?: string;
  to?: string;
  actorUserId?: string;
  action?: string;
  entityType?: string;
  entityId?: string;
  limit?: number;
  offset?: number;
}) {
  const query = new URLSearchParams();
  if (params?.from) query.set("from", params.from);
  if (params?.to) query.set("to", params.to);
  if (params?.actorUserId) query.set("actorUserId", params.actorUserId);
  if (params?.action) query.set("action", params.action);
  if (params?.entityType) query.set("entityType", params.entityType);
  if (params?.entityId) query.set("entityId", params.entityId);
  if (params?.limit !== undefined) query.set("limit", String(params.limit));
  if (params?.offset !== undefined) query.set("offset", String(params.offset));
  const suffix = query.toString();

  return api.request<{ items: AdminAuditLogEntry[]; total: number; limit: number; offset: number; requestId: string }>(
    `/admin/audit-logs${suffix ? `?${suffix}` : ""}`,
  );
}

function extractFilename(contentDisposition: string | null) {
  if (!contentDisposition) return null;
  const match = /filename\*=UTF-8''([^;]+)|filename="?([^";]+)"?/i.exec(contentDisposition);
  const value = match?.[1] ?? match?.[2];
  if (!value) return null;
  try {
    return decodeURIComponent(value);
  } catch {
    return value;
  }
}
