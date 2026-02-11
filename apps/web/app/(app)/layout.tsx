"use client";

import { useEffect, useState } from "react";
import { usePathname, useRouter } from "next/navigation";

import { AppShell } from "@/components/layout/app-shell";
import { Skeleton } from "@/components/ui/skeleton";
import { getMe, type SessionPayload } from "@/lib/session";

export default function ProtectedLayout({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const pathname = usePathname();
  const [session, setSession] = useState<SessionPayload | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let mounted = true;
    getMe()
      .then((data) => {
        if (!mounted) return;
        setSession(data);
      })
      .catch(() => {
        if (!mounted) return;
        if (pathname !== "/login") {
          router.replace("/login");
        }
      })
      .finally(() => {
        if (!mounted) return;
        setLoading(false);
      });

    return () => {
      mounted = false;
    };
  }, [pathname, router]);

  if (loading) {
    return (
      <div className="mx-auto max-w-6xl space-y-4 p-6">
        <Skeleton className="h-10 w-56" />
        <Skeleton className="h-56 w-full" />
      </div>
    );
  }

  return <AppShell session={session}>{children}</AppShell>;
}
