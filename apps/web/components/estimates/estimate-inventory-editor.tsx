"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Loader2, Mail, RefreshCw } from "lucide-react";
import { toast } from "sonner";

import { useEstimateWorkspace } from "@/components/estimates/estimate-workspace-context";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  createEstimateInventoryShareLink,
  getEstimateInventory,
  replaceEstimateInventory,
  type InventoryItem,
} from "@/lib/inventory-api";
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

type SaveState = "idle" | "saving" | "saved" | "error";

type InventoryRow = {
  category: string;
  itemName: string;
  volumeCf: number;
  isCustom?: boolean;
};

export function EstimateInventoryEditor() {
  const { estimate, setEstimate } = useEstimateWorkspace();

  const [items, setItems] = useState<InventoryItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [selectedCategory, setSelectedCategory] = useState("Boxes");
  const [search, setSearch] = useState("");

  const [customItemName, setCustomItemName] = useState("");
  const [customCategory, setCustomCategory] = useState("Boxes");
  const [customVolumeCf, setCustomVolumeCf] = useState("1");
  const [customQty, setCustomQty] = useState("1");

  const [saveState, setSaveState] = useState<SaveState>("idle");
  const [saveMessage, setSaveMessage] = useState("Loading inventory...");
  const [lastSavedFingerprint, setLastSavedFingerprint] = useState("[]");
  const [sendingLink, setSendingLink] = useState(false);

  const isSaving = saveState === "saving";
  const totalVolumeCf = useMemo(() => calculateTotalVolumeCf(items), [items]);

  const categoryOptions = useMemo(() => {
    const dynamicCategories = items
      .map((item) => item.category.trim())
      .filter((category) => category.length > 0);

    const unique = new Set([...INVENTORY_CATEGORIES, ...dynamicCategories]);
    return Array.from(unique);
  }, [items]);

  const itemMap = useMemo(() => {
    const map = new Map<string, InventoryItem>();
    for (const item of items) {
      map.set(inventoryItemKey(item), item);
    }
    return map;
  }, [items]);

  const categoryQtyMap = useMemo(() => {
    const map = new Map<string, number>();
    for (const item of items) {
      const key = item.category.trim();
      map.set(key, (map.get(key) ?? 0) + item.qty);
    }
    return map;
  }, [items]);

  const currentFingerprint = useMemo(() => inventoryFingerprint(items), [items]);
  const hasChanges = currentFingerprint !== lastSavedFingerprint;

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
    const filtered = merged.filter((row) => normalize(row.itemName).includes(normalize(search)));

    const deduped = new Map<string, InventoryRow>();
    for (const row of filtered) {
      deduped.set(inventoryItemKey(row), row);
    }

    return Array.from(deduped.values());
  }, [items, search, selectedCategory]);

  const loadInventory = useCallback(async () => {
    setLoading(true);
    setLoadError(null);

    try {
      const response = await getEstimateInventory(estimate.id);
      const normalizedItems = normalizeInventoryItems(response.items);
      setItems(normalizedItems);
      setLastSavedFingerprint(inventoryFingerprint(normalizedItems));
      setSaveState("saved");
      setSaveMessage("Saved");
      setEstimate({
        ...estimate,
        totalVolumeCf: response.totalVolumeCf,
      });
    } catch (error) {
      setLoadError(getApiErrorMessage(error));
      setSaveState("error");
      setSaveMessage("Load failed");
    } finally {
      setLoading(false);
    }
  }, [estimate.id, setEstimate]);

  useEffect(() => {
    void loadInventory();
  }, [loadInventory]);

  const persist = useCallback(
    async (mode: "autosave" | "manual") => {
      if (loading || loadError || isSaving || !hasChanges) {
        return true;
      }

      setSaveState("saving");
      setSaveMessage("Saving...");

      try {
        const response = await replaceEstimateInventory(estimate.id, {
          items: serializeInventory(items),
        });

        const normalizedItems = normalizeInventoryItems(response.items);
        setItems(normalizedItems);
        setLastSavedFingerprint(inventoryFingerprint(normalizedItems));
        setSaveState("saved");
        setSaveMessage("Saved");
        setEstimate({
          ...estimate,
          totalVolumeCf: response.totalVolumeCf,
        });

        if (mode === "manual") {
          toast.success("Inventory saved");
        }

        return true;
      } catch (error) {
        setSaveState("error");
        setSaveMessage("Save failed");
        if (mode === "manual") {
          toast.error(getApiErrorMessage(error));
        }
        return false;
      }
    },
    [estimate, hasChanges, isSaving, items, loadError, loading, setEstimate],
  );

  useEffect(() => {
    if (!hasChanges || loading || loadError || isSaving) return;
    const timer = setTimeout(() => {
      void persist("autosave");
    }, 900);

    return () => clearTimeout(timer);
  }, [hasChanges, isSaving, loadError, loading, persist]);

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

    setSaveState("idle");
    setSaveMessage("Unsaved changes");
  }, []);

  function itemQty(row: InventoryRow) {
    return itemMap.get(inventoryItemKey(row))?.qty ?? 0;
  }

  async function handleSaveClick() {
    await persist("manual");
  }

  async function handleSendInventoryLink() {
    if (hasChanges) {
      const saved = await persist("manual");
      if (!saved) return;
    }

    setSendingLink(true);
    try {
      const response = await createEstimateInventoryShareLink(estimate.id);
      if (response.deliveryMode === "smtp") {
        toast.success(`Inventory link sent to ${response.recipientEmail}`);
      } else {
        toast.success("Inventory link generated", {
          description: "Email delivery is in log mode. The link was written to API logs.",
        });
      }
    } catch (error) {
      toast.error(getApiErrorMessage(error));
    } finally {
      setSendingLink(false);
    }
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

  if (loading) {
    return (
      <Card className="border-border/70 bg-card/70">
        <CardContent className="flex h-48 items-center justify-center">
          <span className="inline-flex items-center gap-2 text-sm text-muted-foreground">
            <Loader2 className="h-4 w-4 animate-spin" />
            Loading inventory...
          </span>
        </CardContent>
      </Card>
    );
  }

  if (loadError) {
    return (
      <Card className="border-border/70 bg-card/70">
        <CardHeader>
          <CardTitle>Inventory unavailable</CardTitle>
          <CardDescription>{loadError}</CardDescription>
        </CardHeader>
        <CardContent>
          <Button variant="secondary" onClick={() => void loadInventory()}>
            <RefreshCw className="h-4 w-4" />
            Retry
          </Button>
        </CardContent>
      </Card>
    );
  }

  return (
    <div className="space-y-4">
      <Card className="border-border/70 bg-card/70">
        <CardContent className="flex flex-wrap items-center justify-between gap-3 p-4">
          <div>
            <p className="text-sm font-medium">Inventory</p>
            <p className="text-xs text-muted-foreground" aria-live="polite" role="status">
              {saveMessage}
            </p>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <Button variant="outline" onClick={handleSaveClick} disabled={isSaving || !hasChanges}>
              {isSaving ? (
                <span className="inline-flex items-center gap-2">
                  <Loader2 className="h-4 w-4 animate-spin" />
                  Saving...
                </span>
              ) : (
                "Save"
              )}
            </Button>
            <Button variant="secondary" onClick={handleSendInventoryLink} disabled={sendingLink || isSaving}>
              {sendingLink ? (
                <span className="inline-flex items-center gap-2">
                  <Loader2 className="h-4 w-4 animate-spin" />
                  Sending...
                </span>
              ) : (
                <>
                  <Mail className="h-4 w-4" />
                  Send Inventory Link
                </>
              )}
            </Button>
          </div>
        </CardContent>
      </Card>

      <div className="grid gap-4 xl:grid-cols-[220px_minmax(0,1fr)_300px]">
        <Card className="border-border/70 bg-card/70">
          <CardHeader className="pb-3">
            <CardTitle className="text-base">Categories</CardTitle>
          </CardHeader>
          <CardContent className="space-y-1">
            {categoryOptions.map((category) => {
              const isActive = category === selectedCategory;
              const qty = categoryQtyMap.get(category) ?? 0;
              return (
                <button
                  key={category}
                  type="button"
                  onClick={() => setSelectedCategory(category)}
                  className={`flex w-full items-center justify-between rounded-md border px-3 py-2 text-left text-sm transition-colors ${
                    isActive
                      ? "border-primary bg-primary/10 text-primary"
                      : "border-border/70 text-muted-foreground hover:bg-muted/40 hover:text-foreground"
                  }`}
                >
                  <span>{category}</span>
                  <span className="text-xs">{qty}</span>
                </button>
              );
            })}
          </CardContent>
        </Card>

        <Card className="border-border/70 bg-card/70">
          <CardHeader>
            <CardTitle className="text-base">{selectedCategory}</CardTitle>
            <CardDescription>Adjust quantities quickly with -1, +1, and +10 controls.</CardDescription>
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
                          <td className="px-2 py-2 align-middle">
                            <div className="flex items-center gap-2">
                              <span>{row.itemName}</span>
                              {row.isCustom ? (
                                <span className="rounded bg-muted px-2 py-0.5 text-[11px] text-muted-foreground">Custom</span>
                              ) : null}
                            </div>
                          </td>
                          <td className="px-2 py-2 align-middle">{roundCf(row.volumeCf).toFixed(2)}</td>
                          <td className="px-2 py-2 align-middle text-base font-semibold" data-testid={`qty-${inventoryItemKey(row)}`}>
                            {qty}
                          </td>
                          <td className="px-2 py-2 align-middle">
                            <div className="flex flex-wrap items-center gap-1">
                              <Button
                                type="button"
                                variant="outline"
                                size="sm"
                                onClick={() => upsertItem(row, qty - 1)}
                                aria-label={`Subtract 1 ${row.itemName}`}
                              >
                                -1
                              </Button>
                              <Button
                                type="button"
                                variant="outline"
                                size="sm"
                                onClick={() => upsertItem(row, qty + 1)}
                                aria-label={`Add 1 ${row.itemName}`}
                              >
                                +1
                              </Button>
                              <Button
                                type="button"
                                variant="outline"
                                size="sm"
                                onClick={() => upsertItem(row, qty + 10)}
                                aria-label={`Add 10 ${row.itemName}`}
                              >
                                +10
                              </Button>
                              <Button
                                type="button"
                                variant="outline"
                                size="sm"
                                onClick={() => upsertItem(row, 0)}
                                disabled={qty === 0}
                                aria-label={`Remove ${row.itemName}`}
                              >
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

        <Card className="border-border/70 bg-card/70">
          <CardHeader>
            <CardTitle className="text-base">Inventory tools</CardTitle>
            <CardDescription>Search and add custom items without leaving this estimate.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-5">
            <div className="space-y-2">
              <Label htmlFor="inventory-search">Search by item name</Label>
              <Input
                id="inventory-search"
                value={search}
                onChange={(event) => setSearch(event.target.value)}
                placeholder="e.g., box"
              />
            </div>

            <div className="space-y-2 rounded-md border border-border/70 p-3">
              <p className="text-sm font-medium">Add New Item</p>
              <div className="space-y-2">
                <Label htmlFor="custom-item-name">Item name</Label>
                <Input
                  id="custom-item-name"
                  value={customItemName}
                  onChange={(event) => setCustomItemName(event.target.value)}
                  placeholder="Custom item"
                />
              </div>
              <div className="grid gap-2">
                <div className="space-y-2">
                  <Label htmlFor="custom-item-category">Category</Label>
                  <select
                    id="custom-item-category"
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
                    <Label htmlFor="custom-item-volume">Volume (cf)</Label>
                    <Input
                      id="custom-item-volume"
                      type="number"
                      min="0"
                      step="0.01"
                      value={customVolumeCf}
                      onChange={(event) => setCustomVolumeCf(event.target.value)}
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="custom-item-qty">Qty</Label>
                    <Input
                      id="custom-item-qty"
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
            </div>

            <div className="rounded-md border border-border/70 bg-muted/20 p-3">
              <p className="text-xs uppercase tracking-wide text-muted-foreground">Total volume</p>
              <p className="text-lg font-semibold" data-testid="inventory-total-cf">
                {formatCf(totalVolumeCf)}
              </p>
              <p className="text-xs text-muted-foreground">
                Saved on estimate: {formatCf(estimate.totalVolumeCf)}
              </p>
            </div>
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
