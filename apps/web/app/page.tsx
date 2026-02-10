"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import type { AuthSessionResponse } from "@moveops/client";
import { api } from "@/lib/api";
import { Card } from "@/components/ui";

export default function HomePage() {
  const [session, setSession] = useState<AuthSessionResponse | null>(null);
  const [error, setError] = useState<string>("");

  useEffect(() => {
    api
      .request<AuthSessionResponse>("/auth/me")
      .then(setSession)
      .catch(() => setError("Not logged in"));
  }, []);

  return (
    <main className="mx-auto flex min-h-screen max-w-4xl flex-col gap-6 px-6 py-10">
      <Card>
        <h1 className="text-3xl font-bold">MoveOps Dashboard</h1>
        <p className="mt-2 text-slate-600">Phase 1 foundation is running.</p>
      </Card>

      <Card>
        <h2 className="text-xl font-semibold">Navigation placeholders</h2>
        <nav className="mt-3 flex gap-4">
          <Link href="#">New Estimate</Link>
          <Link href="#">Calendar</Link>
          <Link href="#">Storage</Link>
          <Link href="/login">Login</Link>
        </nav>
      </Card>

      <Card>
        <h2 className="text-xl font-semibold">Current user</h2>
        {session ? (
          <pre className="mt-2 overflow-x-auto rounded bg-slate-100 p-3 text-sm">
            {JSON.stringify(session, null, 2)}
          </pre>
        ) : (
          <p className="mt-2 text-slate-700">{error || "Loading..."}</p>
        )}
      </Card>
    </main>
  );
}
