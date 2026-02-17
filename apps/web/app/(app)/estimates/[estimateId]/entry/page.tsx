"use client";

import { EstimateEntryEditor } from "@/components/estimates/estimate-entry-editor";
import { useEstimateWorkspace } from "@/components/estimates/estimate-workspace-context";

export default function EstimateEntryPage() {
  const { estimate, setEstimate } = useEstimateWorkspace();

  return (
    <EstimateEntryEditor
      mode="existing"
      estimateId={estimate.id}
      estimate={estimate}
      onEstimateSaved={setEstimate}
    />
  );
}
