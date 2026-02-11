import { CalendarRange } from "lucide-react";

import { EmptyState } from "@/components/layout/empty-state";
import { PageHeader } from "@/components/layout/page-header";

export default function CalendarPage() {
  return (
    <div className="space-y-6">
      <PageHeader
        title="Calendar"
        description="Plan and review move schedules. Monthly planning and filters will expand as product workflows land."
      />
      <EmptyState
        title="Nothing scheduled yet"
        description="Jobs created from estimates will appear here for dispatcher and operations planning."
        ctaLabel="Open scheduling"
        icon={<CalendarRange className="h-5 w-5" />}
      />
    </div>
  );
}
