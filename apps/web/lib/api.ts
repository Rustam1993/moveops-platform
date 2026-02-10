import { FetchClient } from "@moveops/client";

const apiBase = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080/api";

export const api = new FetchClient({
  baseUrl: apiBase,
  credentials: "include",
});
