export type FetchClientOptions = {
  baseUrl: string;
  credentials?: RequestCredentials;
};

export class FetchClient {
  private readonly baseUrl: string;
  private readonly credentials: RequestCredentials;

  constructor(options: FetchClientOptions) {
    this.baseUrl = options.baseUrl;
    this.credentials = options.credentials ?? "include";
  }

  async request<T>(path: string, init?: RequestInit): Promise<T> {
    const response = await fetch(`${this.baseUrl}${path}`, {
      ...init,
      credentials: this.credentials,
      headers: {
        "Content-Type": "application/json",
        ...(init?.headers ?? {}),
      },
    });

    if (!response.ok) {
      const body = await response.json().catch(() => ({}));
      const message = body?.error?.message ?? `HTTP ${response.status}`;
      throw new Error(message);
    }

    if (response.status === 204) {
      return undefined as T;
    }

    return (await response.json()) as T;
  }
}
