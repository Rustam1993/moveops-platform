import { FilePlus2 } from "lucide-react";

import { EmptyState } from "@/components/layout/empty-state";
import { PageHeader } from "@/components/layout/page-header";

export default function NewEstimatePage() {
  return (
    <div className="space-y-6">
      <PageHeader
        title="New Estimate"
        description="Start a new estimate record. Detailed pricing and workflow forms are introduced in later phases."
      />
      <EmptyState
        title="No estimate draft yet"
        description="Use this entry point to capture customer requirements and create an estimate pipeline item."
        ctaLabel="Create estimate draft"
        icon={<FilePlus2 className="h-5 w-5" />}
      />
    </div>
  );
}
