import { api } from "@/lib/api";

export type SessionPayload = {
  user: { id: string; email: string; fullName: string };
  tenant: { id: string; slug: string; name: string };
};

export async function getMe() {
  return api.request<SessionPayload>("/auth/me");
}

export async function getCsrf() {
  return api.request<{ csrfToken: string }>("/auth/csrf");
}

export async function logout() {
  let csrfToken = sessionStorage.getItem("csrfToken");
  if (!csrfToken) {
    const csrf = await getCsrf();
    csrfToken = csrf.csrfToken;
    sessionStorage.setItem("csrfToken", csrfToken);
  }

  await api.request<void>("/auth/logout", {
    method: "POST",
    headers: {
      "X-CSRF-Token": csrfToken,
    },
  });
  sessionStorage.removeItem("csrfToken");
}
