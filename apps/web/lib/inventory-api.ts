import { api, requestJSON } from "@/lib/api";

export type InventoryItem = {
  category: string;
  itemName: string;
  volumeCf: number;
  qty: number;
  isCustom?: boolean;
};

export type EstimateInventoryResponse = {
  estimateId: string;
  items: InventoryItem[];
  totalVolumeCf: number;
  requestId: string;
};

export type ReplaceEstimateInventoryRequest = {
  items: InventoryItem[];
};

export type CreateInventoryShareLinkRequest = {
  expiresInDays?: number;
};

export type CreateInventoryShareLinkResponse = {
  shareLinkId: string;
  estimateId: string;
  recipientEmail: string;
  shareUrl: string;
  expiresAt: string;
  deliveryMode: "log" | "smtp";
  requestId: string;
};

export type PublicInventoryResponse = {
  estimateId: string;
  customerName: string;
  moveDate: string;
  items: InventoryItem[];
  totalVolumeCf: number;
  expiresAt: string;
  requestId: string;
};

export type CatalogCategory = {
  id: string;
  name: string;
  sortOrder: number;
  active: boolean;
};

export type CatalogItem = {
  id: string;
  categoryId?: string | null;
  categoryName?: string | null;
  itemName: string;
  volumeCf: number;
  sortOrder: number;
  active: boolean;
};

export type EstimateInventoryCatalogResponse = {
  categories: CatalogCategory[];
  items: CatalogItem[];
  requestId: string;
};

export async function getEstimateInventory(estimateId: string) {
  return api.request<EstimateInventoryResponse>(`/estimates/${estimateId}/inventory`);
}

export async function replaceEstimateInventory(estimateId: string, payload: ReplaceEstimateInventoryRequest) {
  return api.request<EstimateInventoryResponse>(`/estimates/${estimateId}/inventory`, {
    method: "PUT",
    body: JSON.stringify(payload),
  });
}

export async function createEstimateInventoryShareLink(estimateId: string, payload?: CreateInventoryShareLinkRequest) {
  return api.request<CreateInventoryShareLinkResponse>(`/estimates/${estimateId}/inventory-share-links`, {
    method: "POST",
    body: JSON.stringify(payload ?? {}),
  });
}

export async function getEstimateInventoryCatalog(estimateId: string) {
  return api.request<EstimateInventoryCatalogResponse>(`/estimates/${estimateId}/inventory/catalog`);
}

export async function getPublicInventory(token: string) {
  return requestJSON<PublicInventoryResponse>(`/public/inventory/${token}`, undefined, {
    withCsrf: false,
    suppressAuthRedirect: true,
  });
}

export async function updatePublicInventory(token: string, payload: ReplaceEstimateInventoryRequest) {
  return requestJSON<PublicInventoryResponse>(
    `/public/inventory/${token}`,
    {
      method: "PUT",
      body: JSON.stringify(payload),
    },
    {
      withCsrf: false,
      suppressAuthRedirect: true,
    },
  );
}
