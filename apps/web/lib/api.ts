import type { components } from "@moveops/client";

const apiBase = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080/api";

type ErrorEnvelope = components["schemas"]["ErrorEnvelope"];

export class ApiError extends Error {
  status: number;
  code?: string;
  details?: unknown;

  constructor(status: number, message: string, code?: string, details?: unknown) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
    this.details = details;
  }
}

let csrfTokenCache: string | null = null;
let csrfFetchPromise: Promise<string> | null = null;

function isMutatingMethod(method?: string) {
  const normalized = (method ?? "GET").toUpperCase();
  return normalized !== "GET" && normalized !== "HEAD" && normalized !== "OPTIONS";
}

async function fetchCsrfToken(force = false): Promise<string> {
  if (!force && csrfTokenCache) return csrfTokenCache;
  if (!force && csrfFetchPromise) return csrfFetchPromise;

  csrfFetchPromise = (async () => {
    const response = await fetch(`${apiBase}/auth/csrf`, {
      method: "GET",
      credentials: "include",
    });
    if (!response.ok) {
      const fallback = `HTTP ${response.status}`;
      throw new ApiError(response.status, fallback);
    }
    const payload = (await response.json()) as { csrfToken: string };
    csrfTokenCache = payload.csrfToken;
    if (typeof window !== "undefined") {
      sessionStorage.setItem("csrfToken", payload.csrfToken);
    }
    return payload.csrfToken;
  })();

  try {
    return await csrfFetchPromise;
  } finally {
    csrfFetchPromise = null;
  }
}

export async function primeCsrfToken() {
  return fetchCsrfToken();
}

export function clearCsrfToken() {
  csrfTokenCache = null;
  if (typeof window !== "undefined") {
    sessionStorage.removeItem("csrfToken");
  }
}

export function hydrateCsrfFromSessionStorage() {
  if (csrfTokenCache || typeof window === "undefined") return;
  const cached = sessionStorage.getItem("csrfToken");
  if (cached) {
    csrfTokenCache = cached;
  }
}

async function toApiError(response: Response): Promise<ApiError> {
  const fallback = `HTTP ${response.status}`;
  let message = fallback;
  let code: string | undefined;
  let details: unknown;
  try {
    const payload = (await response.json()) as ErrorEnvelope;
    if (payload?.error?.message) message = payload.error.message;
    if (payload?.error?.code) code = payload.error.code;
    details = payload?.error?.details;
  } catch {
    message = fallback;
  }

  if (response.status === 403 && code === "CSRF_INVALID") {
    message = "Session expired, please refresh and sign in again.";
  }

  return new ApiError(response.status, message, code, details);
}

type RequestOptions = {
  withCsrf?: boolean;
  suppressAuthRedirect?: boolean;
};

export async function requestJSON<T>(path: string, init?: RequestInit, options?: RequestOptions): Promise<T> {
  const method = init?.method ?? "GET";
  const withCsrf = options?.withCsrf ?? (isMutatingMethod(method) && path !== "/auth/login");
  const headers = new Headers(init?.headers ?? {});

  if (!headers.has("Content-Type") && init?.body && !(init.body instanceof FormData)) {
    headers.set("Content-Type", "application/json");
  }
  if (withCsrf && !headers.has("X-CSRF-Token")) {
    hydrateCsrfFromSessionStorage();
    const token = await fetchCsrfToken();
    headers.set("X-CSRF-Token", token);
  }

  const response = await fetch(`${apiBase}${path}`, {
    ...init,
    method,
    credentials: "include",
    headers,
  });

  if (!response.ok) {
    const apiError = await toApiError(response);
    if (response.status === 401 && !options?.suppressAuthRedirect && typeof window !== "undefined") {
      window.location.replace("/login");
    }
    throw apiError;
  }

  if (response.status === 204) {
    return undefined as T;
  }
  return (await response.json()) as T;
}

export async function requestBlob(path: string, init?: RequestInit, options?: RequestOptions): Promise<Response> {
  const method = init?.method ?? "GET";
  const withCsrf = options?.withCsrf ?? (isMutatingMethod(method) && path !== "/auth/login");
  const headers = new Headers(init?.headers ?? {});
  if (withCsrf && !headers.has("X-CSRF-Token")) {
    hydrateCsrfFromSessionStorage();
    const token = await fetchCsrfToken();
    headers.set("X-CSRF-Token", token);
  }

  const response = await fetch(`${apiBase}${path}`, {
    ...init,
    method,
    credentials: "include",
    headers,
  });
  if (!response.ok) {
    const apiError = await toApiError(response);
    if (response.status === 401 && !options?.suppressAuthRedirect && typeof window !== "undefined") {
      window.location.replace("/login");
    }
    throw apiError;
  }
  return response;
}

export function isUnauthorizedError(error: unknown) {
  return error instanceof ApiError && error.status === 401;
}

export function isForbiddenError(error: unknown) {
  return error instanceof ApiError && error.status === 403;
}

export const api = {
  request: requestJSON,
};
