export type EstimateWorkspaceTabKey =
  | "entry"
  | "inventory"
  | "items-not-moving"
  | "printed-estimate"
  | "email"
  | "charges"
  | "tasks"
  | "payments"
  | "operations";

export type EstimateWorkspaceTab = {
  key: EstimateWorkspaceTabKey;
  label: string;
  segment: string;
};

export const estimateWorkspaceTabs: EstimateWorkspaceTab[] = [
  { key: "entry", label: "Entry Form", segment: "entry" },
  { key: "inventory", label: "Inventory", segment: "inventory" },
  { key: "items-not-moving", label: "Items not Moving", segment: "items-not-moving" },
  { key: "printed-estimate", label: "Printed Estimate", segment: "printed-estimate" },
  { key: "email", label: "Email Center", segment: "email" },
  { key: "charges", label: "Charges", segment: "charges" },
  { key: "tasks", label: "Tasks List", segment: "tasks" },
  { key: "payments", label: "Payments", segment: "payments" },
  { key: "operations", label: "Operations", segment: "operations" },
];

export function estimateTabHref(estimateId: string, segment: string) {
  return `/estimates/${estimateId}/${segment}`;
}
