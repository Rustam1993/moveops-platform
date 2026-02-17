"use client";

import { EstimateEntryEditor } from "@/components/estimates/estimate-entry-editor";
import { EstimateWorkspaceShell } from "@/components/estimates/estimate-workspace-shell";

export default function NewEstimatePage() {
  return (
    <EstimateWorkspaceShell mode="new" activeTab="entry">
      <EstimateEntryEditor mode="new" />
    </EstimateWorkspaceShell>
  );
}
