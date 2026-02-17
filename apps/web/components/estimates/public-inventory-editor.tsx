"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Loader2 } from "lucide-react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ApiError } from "@/lib/api";
import { getPublicInventory, updatePublicInventory, type InventoryItem } from "@/lib/inventory-api";
import {
  calculateTotalVolumeCf,
  formatCf,
  INVENTORY_CATALOG,
  INVENTORY_CATEGORIES,
  inventoryItemKey,
  normalize,
  roundCf,
  sortInventoryItems,
} from "@/lib/inventory-catalog";
import { getApiErrorMessage } from "@/lib/phase2-api";

const selectClassName =
  "flex h-10 w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50";

type InventoryRow = {
  category: string;
  itemName: string;
  volumeCf: number;
  isCustom?: boolean;
};

type PublicInventoryContext = {
  customerName: string;
  moveDate: string;
  expiresAt: string;
  totalVolumeCf: number;
};

export function PublicInventoryEditor({ token }: { token: string }) {
  const [context, setContext] = useState<PublicInventoryContext | null>(null);
  const [items, setItems] = useState<InventoryItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadingError, setLoadingError] = useState<string | null>(null);

  const [selectedCategory, setSelectedCategory] = useState("Boxes");
  const [search, setSearch] = useState("");

  const [customItemName, setCustomItemName] = useState("");
  const [customCategory, setCustomCategory] = useState("Boxes");
  const [customVolumeCf, setCustomVolumeCf] = useState("1");
  const [customQty, setCustomQty] = useState("1");

  const [saving, setSaving] = useState(false);
  const [statusMessage, setStatusMessage] = useState("Loading inventory...");
  const [lastSavedFingerprint, setLastSavedFingerprint] = useState("[]");

  const itemMap = useMemo(() => {
    const map = new Map<string, InventoryItem>();
    for (const item of items) {
      map.set(inventoryItemKey(item), item);
    }
    return map;
  }, [items]);

  const categoryOptions = useMemo(() => {
    const dynamicCategories = items
      .map((item) => item.category.trim())
      .filter((category) => category.length > 0);

    const unique = new Set([...INVENTORY_CATEGORIES, ...dynamicCategories]);
    return Array.from(unique);
  }, [items]);

  const currentFingerprint = useMemo(() => inventoryFingerprint(items), [items]);
  const hasChanges = currentFingerprint !== lastSavedFingerprint;
  const totalVolumeCf = useMemo(() => calculateTotalVolumeCf(items), [items]);

  const tableRows = useMemo(() => {
    const catalogRows: InventoryRow[] = INVENTORY_CATALOG.filter((item) => item.category === selectedCategory).map((item) => ({
      ...item,
      isCustom: false,
    }));

    const catalogKeySet = new Set(catalogRows.map((row) => inventoryItemKey(row)));

    const extraRows: InventoryRow[] = items
      .filter((item) => item.category === selectedCategory)
      .filter((item) => item.isCustom || !catalogKeySet.has(inventoryItemKey({ ...item, isCustom: false })))
      .map((item) => ({
        category: item.category,
        itemName: item.itemName,
        volumeCf: item.volumeCf,
        isCustom: item.isCustom,
      }));

    const merged = sortInventoryItems([...catalogRows, ...extraRows]);
    return merged.filter((row) => normalize(row.itemName).includes(normalize(search)));
  }, [items, search, selectedCategory]);

  const loadPublicInventory = useCallback(async () => {
    setLoading(true);
    setLoadingError(null);

    try {
      const response = await getPublicInventory(token);
      const normalizedItems = normalizeInventoryItems(response.items);
      setItems(normalizedItems);
      setLastSavedFingerprint(inventoryFingerprint(normalizedItems));
      setContext({
        customerName: response.customerName,
        moveDate: response.moveDate,
        expiresAt: response.expiresAt,
        totalVolumeCf: response.totalVolumeCf,
      });
      setStatusMessage("Inventory loaded");
    } catch (error) {
      if (error instanceof ApiError && error.code === "inventory_share_expired") {
        setLoadingError("This inventory link has expired. Please contact your move coordinator for a new link.");
      } else if (error instanceof ApiError && error.code === "inventory_share_not_found") {
        setLoadingError("This inventory link is invalid.");
      } else {
        setLoadingError(getApiErrorMessage(error));
      }
      setStatusMessage("Unable to load inventory");
    } finally {
      setLoading(false);
    }
  }, [token]);

  useEffect(() => {
    void loadPublicInventory();
  }, [loadPublicInventory]);

  const upsertItem = useCallback((row: InventoryRow, nextQty: number) => {
    const clampedQty = Math.max(0, Math.floor(nextQty));

    setItems((previous) => {
      const key = inventoryItemKey(row);
      const index = previous.findIndex((item) => inventoryItemKey(item) === key);

      if (index === -1) {
        if (clampedQty <= 0) {
          return previous;
        }
        return sortInventoryItems([
          ...previous,
          {
            category: row.category.trim(),
            itemName: row.itemName.trim(),
            volumeCf: roundCf(row.volumeCf),
            qty: clampedQty,
            isCustom: !!row.isCustom,
          },
        ]);
      }

      if (clampedQty <= 0) {
        return sortInventoryItems(previous.filter((_, itemIndex) => itemIndex !== index));
      }

      const next = [...previous];
      const existing = next[index];
      next[index] = {
        ...existing,
        category: row.category.trim(),
        itemName: row.itemName.trim(),
        volumeCf: roundCf(row.volumeCf),
        qty: clampedQty,
        isCustom: !!row.isCustom,
      };
      return sortInventoryItems(next);
    });

    setStatusMessage("Unsaved changes");
  }, []);

  function itemQty(row: InventoryRow) {
    return itemMap.get(inventoryItemKey(row))?.qty ?? 0;
  }

  function handleAddCustomItem() {
    const name = customItemName.trim();
    const category = customCategory.trim() || "Boxes";
    const volume = Number(customVolumeCf);
    const qty = Math.max(1, Math.floor(Number(customQty) || 1));

    if (!name) {
      toast.error("Item name is required");
      return;
    }
    if (Number.isNaN(volume) || volume < 0) {
      toast.error("Volume must be zero or greater");
      return;
    }

    const row: InventoryRow = {
      category,
      itemName: name,
      volumeCf: roundCf(volume),
      isCustom: true,
    };

    const existingQty = itemQty(row);
    upsertItem(row, existingQty + qty);

    setSelectedCategory(category);
    setSearch(name);
    setCustomItemName("");
    setCustomVolumeCf("1");
    setCustomQty("1");
  }

  async function handleSubmit() {
    if (!hasChanges) {
      setStatusMessage("No new changes to submit");
      return;
    }

    setSaving(true);
    setStatusMessage("Submitting...");

    try {
      const response = await updatePublicInventory(token, {
        items: serializeInventory(items),
      });

      const normalizedItems = normalizeInventoryItems(response.items);
      setItems(normalizedItems);
      setLastSavedFingerprint(inventoryFingerprint(normalizedItems));
      setContext((previous) =>
        previous
          ? {
              ...previous,
              totalVolumeCf: response.totalVolumeCf,
            }
          : previous,
      );
      setStatusMessage("Saved");
      toast.success("Inventory submitted");
    } catch (error) {
      setStatusMessage("Submit failed");
      toast.error(getApiErrorMessage(error));
    } finally {
      setSaving(false);
    }
  }

  if (loading) {
    return (
      <div className="mx-auto max-w-6xl p-6">
        <Card className="border-border/70 bg-card/80">
          <CardContent className="flex h-48 items-center justify-center">
            <span className="inline-flex items-center gap-2 text-sm text-muted-foreground">
              <Loader2 className="h-4 w-4 animate-spin" />
              Loading inventory...
            </span>
          </CardContent>
        </Card>
      </div>
    );
  }

  if (loadingError) {
    return (
      <div className="mx-auto max-w-2xl p-6">
        <Card className="border-border/70 bg-card/80">
          <CardHeader>
            <CardTitle>Inventory link unavailable</CardTitle>
            <CardDescription>{loadingError}</CardDescription>
          </CardHeader>
        </Card>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-7xl space-y-4 p-4 md:p-6">
      <Card className="border-border/70 bg-card/80">
        <CardContent className="flex flex-wrap items-start justify-between gap-4 p-4">
          <div className="space-y-1">
            <p className="text-xs uppercase tracking-wide text-muted-foreground">Customer inventory</p>
            <h1 className="text-xl font-semibold tracking-tight">Complete your move inventory</h1>
            <p className="text-sm text-muted-foreground">{context?.customerName}</p>
            <p className="text-sm text-muted-foreground">Move date: {context?.moveDate ?? "-"}</p>
            <p className="text-xs text-muted-foreground" aria-live="polite">
              {statusMessage}
            </p>
          </div>
          <div className="space-y-1 text-right">
            <p className="text-xs uppercase tracking-wide text-muted-foreground">Total volume</p>
            <p className="text-lg font-semibold">{formatCf(totalVolumeCf)}</p>
            <p className="text-xs text-muted-foreground">Link expires: {context?.expiresAt ? context.expiresAt.slice(0, 10) : "-"}</p>
          </div>
        </CardContent>
      </Card>

      <div className="grid gap-4 xl:grid-cols-[220px_minmax(0,1fr)_300px]">
        <Card className="border-border/70 bg-card/80">
          <CardHeader className="pb-3">
            <CardTitle className="text-base">Categories</CardTitle>
          </CardHeader>
          <CardContent className="space-y-1">
            {categoryOptions.map((category) => {
              const active = category === selectedCategory;
              return (
                <button
                  key={category}
                  type="button"
                  className={`w-full rounded-md border px-3 py-2 text-left text-sm transition-colors ${
                    active
                      ? "border-primary bg-primary/10 text-primary"
                      : "border-border/70 text-muted-foreground hover:bg-muted/40 hover:text-foreground"
                  }`}
                  onClick={() => setSelectedCategory(category)}
                >
                  {category}
                </button>
              );
            })}
          </CardContent>
        </Card>

        <Card className="border-border/70 bg-card/80">
          <CardHeader>
            <CardTitle className="text-base">{selectedCategory}</CardTitle>
            <CardDescription>Use -1, +1, +10 to adjust quantities quickly.</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="overflow-x-auto">
              <table className="w-full min-w-[620px] text-sm">
                <thead>
                  <tr className="border-b border-border/70 text-left text-xs uppercase tracking-wide text-muted-foreground">
                    <th className="px-2 py-2">Item</th>
                    <th className="px-2 py-2">Volume (cf)</th>
                    <th className="px-2 py-2">Qty</th>
                    <th className="px-2 py-2">Controls</th>
                  </tr>
                </thead>
                <tbody>
                  {tableRows.length === 0 ? (
                    <tr>
                      <td colSpan={4} className="px-2 py-8 text-center text-sm text-muted-foreground">
                        No items match your search.
                      </td>
                    </tr>
                  ) : (
                    tableRows.map((row) => {
                      const qty = itemQty(row);
                      return (
                        <tr key={inventoryItemKey(row)} className="border-b border-border/50">
                          <td className="px-2 py-2">
                            <div className="flex items-center gap-2">
                              <span>{row.itemName}</span>
                              {row.isCustom ? (
                                <span className="rounded bg-muted px-2 py-0.5 text-[11px] text-muted-foreground">Custom</span>
                              ) : null}
                            </div>
                          </td>
                          <td className="px-2 py-2">{roundCf(row.volumeCf).toFixed(2)}</td>
                          <td className="px-2 py-2 text-base font-semibold">{qty}</td>
                          <td className="px-2 py-2">
                            <div className="flex flex-wrap items-center gap-1">
                              <Button type="button" variant="outline" size="sm" onClick={() => upsertItem(row, qty - 1)}>
                                -1
                              </Button>
                              <Button type="button" variant="outline" size="sm" onClick={() => upsertItem(row, qty + 1)}>
                                +1
                              </Button>
                              <Button type="button" variant="outline" size="sm" onClick={() => upsertItem(row, qty + 10)}>
                                +10
                              </Button>
                              <Button type="button" variant="outline" size="sm" onClick={() => upsertItem(row, 0)} disabled={qty === 0}>
                                Remove
                              </Button>
                            </div>
                          </td>
                        </tr>
                      );
                    })
                  )}
                </tbody>
              </table>
            </div>
          </CardContent>
        </Card>

        <Card className="border-border/70 bg-card/80">
          <CardHeader>
            <CardTitle className="text-base">Tools</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="public-inventory-search">Search by item name</Label>
              <Input
                id="public-inventory-search"
                value={search}
                onChange={(event) => setSearch(event.target.value)}
                placeholder="e.g., box"
              />
            </div>

            <div className="space-y-2 rounded-md border border-border/70 p-3">
              <p className="text-sm font-medium">Add New Item</p>
              <div className="space-y-2">
                <Label htmlFor="public-custom-item-name">Item name</Label>
                <Input
                  id="public-custom-item-name"
                  value={customItemName}
                  onChange={(event) => setCustomItemName(event.target.value)}
                  placeholder="Custom item"
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="public-custom-item-category">Category</Label>
                <select
                  id="public-custom-item-category"
                  value={customCategory}
                  onChange={(event) => setCustomCategory(event.target.value)}
                  className={selectClassName}
                >
                  {categoryOptions.map((category) => (
                    <option key={category} value={category}>
                      {category}
                    </option>
                  ))}
                </select>
              </div>
              <div className="grid grid-cols-2 gap-2">
                <div className="space-y-2">
                  <Label htmlFor="public-custom-volume">Volume (cf)</Label>
                  <Input
                    id="public-custom-volume"
                    type="number"
                    min="0"
                    step="0.01"
                    value={customVolumeCf}
                    onChange={(event) => setCustomVolumeCf(event.target.value)}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="public-custom-qty">Qty</Label>
                  <Input
                    id="public-custom-qty"
                    type="number"
                    min="1"
                    step="1"
                    value={customQty}
                    onChange={(event) => setCustomQty(event.target.value)}
                  />
                </div>
              </div>
              <Button type="button" variant="secondary" onClick={handleAddCustomItem}>
                Add Item
              </Button>
            </div>

            <Button type="button" className="w-full" onClick={handleSubmit} disabled={saving || !hasChanges}>
              {saving ? (
                <span className="inline-flex items-center gap-2">
                  <Loader2 className="h-4 w-4 animate-spin" />
                  Submitting...
                </span>
              ) : (
                "Submit Inventory"
              )}
            </Button>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}

function serializeInventory(items: InventoryItem[]) {
  return sortInventoryItems(
    items
      .map((item) => ({
        category: item.category.trim(),
        itemName: item.itemName.trim(),
        volumeCf: roundCf(item.volumeCf),
        qty: Math.max(0, Math.floor(item.qty)),
        isCustom: !!item.isCustom,
      }))
      .filter((item) => item.category && item.itemName && item.qty > 0),
  );
}

function normalizeInventoryItems(items: InventoryItem[]) {
  return serializeInventory(items);
}

function inventoryFingerprint(items: InventoryItem[]) {
  return JSON.stringify(serializeInventory(items));
}
