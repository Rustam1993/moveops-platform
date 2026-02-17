export type CatalogInventoryItem = {
  category: string;
  itemName: string;
  volumeCf: number;
};

export type InventoryLike = {
  category: string;
  itemName: string;
  volumeCf?: number;
  qty?: number;
  isCustom?: boolean;
};

export const INVENTORY_CATEGORIES: string[] = [
  "Bedroom",
  "Living Room",
  "Dining Room",
  "Kitchen",
  "Appliances",
  "Office",
  "Garage",
  "Patio Furniture",
  "Boxes",
  "Miscellaneous",
  "Nursery",
  "Attic",
  "Basement",
  "Play Room",
  "Military",
];

export const INVENTORY_CATALOG: CatalogInventoryItem[] = [
  { category: "Boxes", itemName: "Box, Book Small", volumeCf: 1.5 },
  { category: "Boxes", itemName: "Box, Medium 18x18x16", volumeCf: 3 },
  { category: "Boxes", itemName: "Box, Large 18x18x24", volumeCf: 5 },
  { category: "Boxes", itemName: "Box, Wardrobe", volumeCf: 15 },
  { category: "Boxes", itemName: "Box, Dishpack", volumeCf: 6 },
  { category: "Boxes", itemName: "Box, File", volumeCf: 2 },
  { category: "Bedroom", itemName: "Bed, Queen (Mattress & Box)", volumeCf: 65 },
  { category: "Bedroom", itemName: "Dresser", volumeCf: 25 },
  { category: "Living Room", itemName: "Sofa, 3 Seat", volumeCf: 55 },
  { category: "Living Room", itemName: "Coffee Table", volumeCf: 10 },
  { category: "Dining Room", itemName: "Dining Table", volumeCf: 35 },
  { category: "Kitchen", itemName: "Kitchen Cart", volumeCf: 12 },
  { category: "Appliances", itemName: "Refrigerator", volumeCf: 45 },
  { category: "Office", itemName: "Desk", volumeCf: 28 },
  { category: "Garage", itemName: "Tool Chest", volumeCf: 20 },
];

export function inventoryItemKey(item: Pick<InventoryLike, "category" | "itemName" | "isCustom">) {
  return `${normalize(item.category)}|${normalize(item.itemName)}|${item.isCustom ? "1" : "0"}`;
}

export function normalize(text: string) {
  return text.trim().toLowerCase();
}

export function roundCf(value: number) {
  return Math.round(value * 100) / 100;
}

export function calculateTotalVolumeCf(items: Array<Pick<InventoryLike, "volumeCf" | "qty">>) {
  return roundCf(
    items.reduce((sum, item) => {
      const volume = item.volumeCf ?? 0;
      const qty = item.qty ?? 0;
      if (qty <= 0 || volume < 0) return sum;
      return sum + volume * qty;
    }, 0),
  );
}

export function sortInventoryItems<T extends Pick<InventoryLike, "category" | "itemName" | "isCustom">>(items: T[]): T[] {
  return [...items].sort((left, right) => {
    const categoryCompare = normalize(left.category).localeCompare(normalize(right.category));
    if (categoryCompare !== 0) return categoryCompare;

    const nameCompare = normalize(left.itemName).localeCompare(normalize(right.itemName));
    if (nameCompare !== 0) return nameCompare;

    if (!!left.isCustom === !!right.isCustom) return 0;
    return left.isCustom ? 1 : -1;
  });
}

export function formatCf(value: number | null | undefined) {
  if (value === null || value === undefined || Number.isNaN(value)) return "-";
  return `${roundCf(value).toFixed(2)} cf`;
}
