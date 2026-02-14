import { NextRequest } from "next/server";

export const runtime = "nodejs";

function joinURL(base: string, path: string) {
  const normalizedBase = base.replace(/\/+$/, "");
  const normalizedPath = path.replace(/^\/+/, "");
  return `${normalizedBase}/${normalizedPath}`;
}

function upstreamBase() {
  // API_INTERNAL_URL is required in Azure because API has internal ingress only.
  // In local dev, it can be omitted (falls back to direct API at localhost).
  const base = process.env.API_INTERNAL_URL ?? process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080/api";
  // Guard against accidental self-referential config like `/api`, which would recurse.
  if (base.startsWith("/")) {
    if (process.env.NODE_ENV === "production") {
      throw new Error("Invalid upstream base URL: must be absolute (set API_INTERNAL_URL).");
    }
    return "http://localhost:8080/api";
  }
  return base;
}

async function proxy(req: NextRequest, pathParts: string[]) {
  const upstreamPath = pathParts.join("/");
  const upstreamURL = new URL(joinURL(upstreamBase(), upstreamPath));
  upstreamURL.search = req.nextUrl.search;

  const headers = new Headers(req.headers);
  // Ensure upstream gets the original Host for cookie + logging sanity.
  headers.set("X-Forwarded-Host", req.headers.get("host") ?? "");
  headers.set("X-Forwarded-Proto", req.nextUrl.protocol.replace(":", ""));
  headers.delete("host");

  const init: RequestInit = {
    method: req.method,
    headers,
    // Follow internal redirects (e.g. http -> https) so we don't leak internal ACA URLs
    // back to the browser via Location headers.
    redirect: "follow",
    body: req.method === "GET" || req.method === "HEAD" ? undefined : req.body,
  };

  // Node fetch requires `duplex: 'half'` when streaming a request body.
  // NextRequest.body is a ReadableStream for non-GET/HEAD, so set it explicitly.
  if (init.body) {
    (init as any).duplex = "half";
  }

  const upstreamResp = await fetch(upstreamURL, init);

  // Copy headers including Set-Cookie so API sessions work through the proxy.
  const respHeaders = new Headers(upstreamResp.headers);
  respHeaders.delete("content-encoding");
  respHeaders.delete("transfer-encoding");

  return new Response(upstreamResp.body, {
    status: upstreamResp.status,
    headers: respHeaders,
  });
}

export async function GET(req: NextRequest, ctx: { params: Promise<{ path: string[] }> }) {
  const { path } = await ctx.params;
  return proxy(req, path);
}

export async function POST(req: NextRequest, ctx: { params: Promise<{ path: string[] }> }) {
  const { path } = await ctx.params;
  return proxy(req, path);
}

export async function PUT(req: NextRequest, ctx: { params: Promise<{ path: string[] }> }) {
  const { path } = await ctx.params;
  return proxy(req, path);
}

export async function PATCH(req: NextRequest, ctx: { params: Promise<{ path: string[] }> }) {
  const { path } = await ctx.params;
  return proxy(req, path);
}

export async function DELETE(req: NextRequest, ctx: { params: Promise<{ path: string[] }> }) {
  const { path } = await ctx.params;
  return proxy(req, path);
}

export async function OPTIONS(req: NextRequest, ctx: { params: Promise<{ path: string[] }> }) {
  const { path } = await ctx.params;
  return proxy(req, path);
}
