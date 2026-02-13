import { clearCsrfToken, requestJSON } from "@/lib/api";

export type SessionPayload = {
  user: { id: string; email: string; fullName: string };
  tenant: { id: string; slug: string; name: string };
};

export async function getMe() {
  return requestJSON<SessionPayload>("/auth/me", undefined, { suppressAuthRedirect: true });
}

export async function getCsrf() {
  return requestJSON<{ csrfToken: string }>("/auth/csrf", undefined, { suppressAuthRedirect: true, withCsrf: false });
}

export async function logout() {
  await requestJSON<void>("/auth/logout", { method: "POST" });
  clearCsrfToken();
}
