import { FileCog } from "lucide-react";

import { EmptyState } from "@/components/layout/empty-state";
import { PageHeader } from "@/components/layout/page-header";

export default function ImportExportPage() {
  return (
    <div className="space-y-6">
      <PageHeader
        title="Import / Export"
        description="Prepare tenant migration files and exports. Role-based admin gating can be enforced as a follow-up UI pass."
      />
      <EmptyState
        title="No import jobs yet"
        description="Dry-run previews and import summaries will be available here once migration actions are enabled in the UI."
        ctaLabel="Start import dry-run"
        icon={<FileCog className="h-5 w-5" />}
      />
    </div>
  );
}
