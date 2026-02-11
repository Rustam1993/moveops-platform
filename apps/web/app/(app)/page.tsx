"use client";

import { ArrowUpRight, Building2, Clock3, UsersRound } from "lucide-react";
import { useEffect, useState } from "react";

import { EmptyState } from "@/components/layout/empty-state";
import { PageHeader } from "@/components/layout/page-header";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { getMe, type SessionPayload } from "@/lib/session";

const stats = [
  { label: "Open Estimates", value: "0", icon: UsersRound },
  { label: "Upcoming Jobs", value: "0", icon: Clock3 },
  { label: "Storage Records", value: "0", icon: Building2 },
];

export default function DashboardPage() {
  const [session, setSession] = useState<SessionPayload | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    getMe()
      .then(setSession)
      .finally(() => setLoading(false));
  }, []);

  return (
    <div className="space-y-6">
      <PageHeader
        title="Dashboard"
        description="A quick snapshot of your moving operations. Use the navigation on the left to start the core workflows."
        actions={
          <Button variant="outline">
            View activity
            <ArrowUpRight className="h-4 w-4" />
          </Button>
        }
      />

      <section className="grid gap-4 md:grid-cols-3">
        {stats.map((stat) => {
          const Icon = stat.icon;
          return (
            <Card key={stat.label}>
              <CardHeader className="pb-3">
                <CardDescription>{stat.label}</CardDescription>
                <CardTitle className="text-2xl">{stat.value}</CardTitle>
              </CardHeader>
              <CardContent>
                <Icon className="h-5 w-5 text-muted-foreground" />
              </CardContent>
            </Card>
          );
        })}
      </section>

      <Card>
        <CardHeader>
          <CardTitle>Active session</CardTitle>
          <CardDescription>Signed-in context used for tenant isolation and RBAC.</CardDescription>
        </CardHeader>
        <CardContent>
          {loading ? (
            <div className="space-y-2">
              <Skeleton className="h-4 w-1/3" />
              <Skeleton className="h-4 w-2/3" />
            </div>
          ) : (
            <pre className="overflow-x-auto rounded-md border bg-muted/40 p-3 text-xs">{JSON.stringify(session, null, 2)}</pre>
          )}
        </CardContent>
      </Card>

      <EmptyState
        title="No operational activity yet"
        description="Create your first estimate or open the calendar to start organizing jobs for this tenant."
        ctaLabel="Create first estimate"
        icon={<UsersRound className="h-5 w-5" />}
      />
    </div>
  );
}
