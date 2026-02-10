"use client";

import { FormEvent, useState } from "react";
import { useRouter } from "next/navigation";
import type { LoginRequest } from "@moveops/client";
import { api } from "@/lib/api";
import { Button, Card, Input } from "@/components/ui";

type CsrfResponse = { csrfToken: string };

export default function LoginPage() {
  const router = useRouter();
  const [email, setEmail] = useState("admin@local.moveops");
  const [password, setPassword] = useState("Admin12345!");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  async function onSubmit(event: FormEvent) {
    event.preventDefault();
    setLoading(true);
    setError("");

    try {
      await api.request("/auth/login", {
        method: "POST",
        body: JSON.stringify({ email, password } satisfies LoginRequest),
      });

      const csrf = await api.request<CsrfResponse>("/auth/csrf");
      sessionStorage.setItem("csrfToken", csrf.csrfToken);
      router.push("/");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Login failed");
    } finally {
      setLoading(false);
    }
  }

  return (
    <main className="mx-auto flex min-h-screen max-w-md items-center px-6 py-10">
      <Card className="w-full">
        <h1 className="text-2xl font-bold">Login</h1>
        <form className="mt-4 space-y-4" onSubmit={onSubmit}>
          <label className="block">
            <span className="mb-1 block text-sm font-medium">Email</span>
            <Input type="email" value={email} onChange={(e) => setEmail(e.target.value)} required />
          </label>

          <label className="block">
            <span className="mb-1 block text-sm font-medium">Password</span>
            <Input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
            />
          </label>

          {error ? <p className="text-sm text-red-600">{error}</p> : null}

          <Button type="submit" disabled={loading}>
            {loading ? "Signing in..." : "Sign in"}
          </Button>
        </form>
      </Card>
    </main>
  );
}
