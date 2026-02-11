import { Archive } from "lucide-react";

import { EmptyState } from "@/components/layout/empty-state";
import { PageHeader } from "@/components/layout/page-header";

export default function StoragePage() {
  return (
    <div className="space-y-6">
      <PageHeader
        title="Storage"
        description="Track storage records and statuses. Data entry and filtering are provided in subsequent phases."
      />
      <EmptyState
        title="No storage records"
        description="Storage entries connected to operations will surface here once jobs are flowing through the system."
        ctaLabel="Create storage record"
        icon={<Archive className="h-5 w-5" />}
      />
    </div>
  );
}
