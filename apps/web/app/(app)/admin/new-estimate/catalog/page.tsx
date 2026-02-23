"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Download, Loader2, Upload } from "lucide-react";
import { toast } from "sonner";

import { NotAuthorizedState } from "@/components/layout/not-authorized-state";
import { PageHeader } from "@/components/layout/page-header";
import { NewEstimateAdminNav } from "@/components/admin/new-estimate-admin-nav";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  createCatalogCategory,
  createCatalogItem,
  deleteCatalogCategory,
  deleteCatalogItem,
  exportCatalogCsv,
  importCatalogCsv,
  listCatalogCategories,
  listCatalogItems,
  updateCatalogCategory,
  updateCatalogItem,
  type AdminCatalogCategory,
  type AdminCatalogItem,
} from "@/lib/new-estimate-admin-api";
import { isForbiddenError } from "@/lib/api";
import { getApiErrorMessage } from "@/lib/phase2-api";

export default function NewEstimateCatalogAdminPage() {
  const [categories, setCategories] = useState<AdminCatalogCategory[]>([]);
  const [items, setItems] = useState<AdminCatalogItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [forbidden, setForbidden] = useState(false);

  const [categoryName, setCategoryName] = useState("");
  const [itemName, setItemName] = useState("");
  const [itemCategoryId, setItemCategoryId] = useState("");
  const [itemVolumeCf, setItemVolumeCf] = useState("1");

  const [importCsv, setImportCsv] = useState("category,item_name,volume_cf,active,item_sort_order\nBoxes,Box Small,1.5,true,1\n");
  const [busyAction, setBusyAction] = useState<string | null>(null);

  const activeCategories = useMemo(() => categories.filter((category) => category.active), [categories]);

  const loadData = useCallback(async () => {
    setLoading(true);
    try {
      const [categoriesResponse, itemsResponse] = await Promise.all([listCatalogCategories(), listCatalogItems()]);
      setCategories(categoriesResponse.categories);
      setItems(itemsResponse.items);
      setForbidden(false);
    } catch (error) {
      if (isForbiddenError(error)) {
        setForbidden(true);
      } else {
        toast.error(getApiErrorMessage(error));
      }
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadData();
  }, [loadData]);

  async function onCreateCategory() {
    const name = categoryName.trim();
    if (!name) {
      toast.error("Category name is required");
      return;
    }
    setBusyAction("create-category");
    try {
      const response = await createCatalogCategory({
        name,
        sortOrder: categories.length + 1,
        active: true,
      });
      setCategories((previous) => [response.category, ...previous]);
      setCategoryName("");
      toast.success("Category created");
    } catch (error) {
      toast.error(getApiErrorMessage(error));
    } finally {
      setBusyAction(null);
    }
  }

  async function onCreateItem() {
    const name = itemName.trim();
    const volume = Number(itemVolumeCf);
    if (!name) {
      toast.error("Item name is required");
      return;
    }
    if (Number.isNaN(volume) || volume < 0) {
      toast.error("Volume must be zero or greater");
      return;
    }

    setBusyAction("create-item");
    try {
      const response = await createCatalogItem({
        itemName: name,
        volumeCf: volume,
        categoryId: itemCategoryId || undefined,
        active: true,
      });
      setItems((previous) => [response.item, ...previous]);
      setItemName("");
      setItemVolumeCf("1");
      toast.success("Catalog item created");
    } catch (error) {
      toast.error(getApiErrorMessage(error));
    } finally {
      setBusyAction(null);
    }
  }

  async function onToggleCategoryActive(category: AdminCatalogCategory) {
    setBusyAction(`category:${category.id}`);
    try {
      const response = await updateCatalogCategory(category.id, { active: !category.active });
      setCategories((previous) => previous.map((entry) => (entry.id === category.id ? response.category : entry)));
    } catch (error) {
      toast.error(getApiErrorMessage(error));
    } finally {
      setBusyAction(null);
    }
  }

  async function onDeleteCategory(categoryId: string) {
    setBusyAction(`delete-category:${categoryId}`);
    try {
      await deleteCatalogCategory(categoryId);
      setCategories((previous) => previous.filter((category) => category.id !== categoryId));
      setItems((previous) => previous.filter((item) => item.categoryId !== categoryId));
      toast.success("Category deleted");
    } catch (error) {
      toast.error(getApiErrorMessage(error));
    } finally {
      setBusyAction(null);
    }
  }

  async function onToggleItemActive(item: AdminCatalogItem) {
    setBusyAction(`item:${item.id}`);
    try {
      const response = await updateCatalogItem(item.id, { active: !item.active });
      setItems((previous) => previous.map((entry) => (entry.id === item.id ? response.item : entry)));
    } catch (error) {
      toast.error(getApiErrorMessage(error));
    } finally {
      setBusyAction(null);
    }
  }

  async function onDeleteItem(itemId: string) {
    setBusyAction(`delete-item:${itemId}`);
    try {
      await deleteCatalogItem(itemId);
      setItems((previous) => previous.filter((item) => item.id !== itemId));
      toast.success("Item deleted");
    } catch (error) {
      toast.error(getApiErrorMessage(error));
    } finally {
      setBusyAction(null);
    }
  }

  async function onImportCsv() {
    setBusyAction("import");
    try {
      const response = await importCatalogCsv(importCsv);
      toast.success(`Import complete (${response.categoriesImported} categories, ${response.itemsImported} items)`);
      await loadData();
    } catch (error) {
      toast.error(getApiErrorMessage(error));
    } finally {
      setBusyAction(null);
    }
  }

  async function onExportCsv() {
    setBusyAction("export");
    try {
      const file = await exportCatalogCsv();
      const url = URL.createObjectURL(file.blob);
      const anchor = document.createElement("a");
      anchor.href = url;
      anchor.download = file.filename;
      anchor.click();
      URL.revokeObjectURL(url);
    } catch (error) {
      toast.error(getApiErrorMessage(error));
    } finally {
      setBusyAction(null);
    }
  }

  if (forbidden) {
    return (
      <div className="space-y-6 pb-8">
        <PageHeader title="New Estimate Catalog" description="Tenant catalog manager for inventory categories and items." />
        <NotAuthorizedState message="This page requires admin.new_estimate permission." />
      </div>
    );
  }

  return (
    <div className="space-y-6 pb-8">
      <PageHeader
        title="New Estimate Catalog"
        description="Manage tenant inventory categories and catalog items used by internal and public inventory entry."
      />
      <NewEstimateAdminNav />

      {loading ? (
        <Card>
          <CardContent className="py-10 text-sm text-muted-foreground">
            <span className="inline-flex items-center gap-2">
              <Loader2 className="h-4 w-4 animate-spin" />
              Loading catalog...
            </span>
          </CardContent>
        </Card>
      ) : null}

      <div className="grid gap-6 xl:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Categories</CardTitle>
            <CardDescription>Create and manage tenant category groupings.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="flex gap-2">
              <Input
                value={categoryName}
                onChange={(event) => setCategoryName(event.target.value)}
                placeholder="e.g., Boxes"
                data-testid="admin-catalog-category-input"
              />
              <Button onClick={() => void onCreateCategory()} disabled={busyAction === "create-category"} data-testid="admin-catalog-category-add">
                Add
              </Button>
            </div>
            <div className="space-y-2">
              {categories.map((category) => (
                <div key={category.id} className="flex items-center justify-between rounded-md border px-3 py-2">
                  <div>
                    <p className="font-medium">{category.name}</p>
                    <p className="text-xs text-muted-foreground">sort: {category.sortOrder}</p>
                  </div>
                  <div className="flex items-center gap-3">
                    <label className="inline-flex items-center gap-2 text-xs text-muted-foreground">
                      <Checkbox
                        checked={category.active}
                        onCheckedChange={() => void onToggleCategoryActive(category)}
                        disabled={busyAction === `category:${category.id}`}
                      />
                      Active
                    </label>
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => void onDeleteCategory(category.id)}
                      disabled={busyAction === `delete-category:${category.id}`}
                    >
                      Delete
                    </Button>
                  </div>
                </div>
              ))}
              {categories.length === 0 ? <p className="text-sm text-muted-foreground">No categories configured yet.</p> : null}
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Items</CardTitle>
            <CardDescription>Add and maintain catalog items with volume values.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid gap-2 md:grid-cols-[1fr_180px_110px_auto]">
              <Input value={itemName} onChange={(event) => setItemName(event.target.value)} placeholder="Item name" data-testid="admin-catalog-item-input" />
              <select
                className="h-10 rounded-md border border-input bg-background px-3 text-sm"
                value={itemCategoryId}
                onChange={(event) => setItemCategoryId(event.target.value)}
                data-testid="admin-catalog-item-category"
              >
                <option value="">No category</option>
                {activeCategories.map((category) => (
                  <option key={category.id} value={category.id}>
                    {category.name}
                  </option>
                ))}
              </select>
              <Input value={itemVolumeCf} onChange={(event) => setItemVolumeCf(event.target.value)} placeholder="cf" data-testid="admin-catalog-item-volume" />
              <Button onClick={() => void onCreateItem()} disabled={busyAction === "create-item"} data-testid="admin-catalog-item-add">
                Add
              </Button>
            </div>

            <div className="space-y-2">
              {items.map((item) => (
                <div key={item.id} className="flex items-center justify-between rounded-md border px-3 py-2" data-testid="admin-catalog-item-row">
                  <div>
                    <p className="font-medium" data-testid="admin-catalog-item-name">{item.itemName}</p>
                    <p className="text-xs text-muted-foreground">
                      {item.categoryName || "No category"} • {item.volumeCf.toFixed(2)} cf
                    </p>
                  </div>
                  <div className="flex items-center gap-3">
                    <label className="inline-flex items-center gap-2 text-xs text-muted-foreground">
                      <Checkbox
                        checked={item.active}
                        onCheckedChange={() => void onToggleItemActive(item)}
                        disabled={busyAction === `item:${item.id}`}
                      />
                      Active
                    </label>
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => void onDeleteItem(item.id)}
                      disabled={busyAction === `delete-item:${item.id}`}
                    >
                      Delete
                    </Button>
                  </div>
                </div>
              ))}
              {items.length === 0 ? <p className="text-sm text-muted-foreground">No items configured yet.</p> : null}
            </div>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Import / Export</CardTitle>
          <CardDescription>Admin-only CSV utilities for bulk catalog management.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <Label htmlFor="catalog-import-text">Catalog CSV</Label>
          <textarea
            id="catalog-import-text"
            className="min-h-[160px] w-full rounded-md border border-input bg-background px-3 py-2 font-mono text-xs"
            value={importCsv}
            onChange={(event) => setImportCsv(event.target.value)}
          />
          <div className="flex flex-wrap gap-2">
            <Button onClick={() => void onImportCsv()} disabled={busyAction === "import"}>
              <Upload className="h-4 w-4" />
              Import CSV
            </Button>
            <Button variant="outline" onClick={() => void onExportCsv()} disabled={busyAction === "export"}>
              <Download className="h-4 w-4" />
              Export CSV
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
