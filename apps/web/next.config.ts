import type { NextConfig } from "next";

const isProd = process.env.NODE_ENV === "production";

const cspDirectives = isProd
  ? [
      "default-src 'self'",
      "base-uri 'self'",
      "frame-ancestors 'none'",
      "object-src 'none'",
      "img-src 'self' data:",
      "style-src 'self' 'unsafe-inline'",
      // Next.js runtime currently emits inline bootstrap scripts; keep this minimal exception.
      "script-src 'self' 'unsafe-inline'",
      "font-src 'self' data:",
      "connect-src 'self'",
      "form-action 'self'",
    ]
  : [
      "default-src 'self'",
      "base-uri 'self'",
      "frame-ancestors 'none'",
      "object-src 'none'",
      "img-src 'self' data: blob:",
      "style-src 'self' 'unsafe-inline'",
      "script-src 'self' 'unsafe-inline' 'unsafe-eval'",
      "font-src 'self' data:",
      "connect-src 'self' http://localhost:3000 http://127.0.0.1:3000 http://localhost:8080 http://127.0.0.1:8080 ws://localhost:3000 ws://127.0.0.1:3000 ws://localhost:8080 ws://127.0.0.1:8080",
      "form-action 'self'",
    ];

const nextConfig: NextConfig = {
  experimental: {
    optimizePackageImports: ["@moveops/client"],
  },
  async headers() {
    return [
      {
        source: "/:path*",
        headers: [
          { key: "Content-Security-Policy", value: cspDirectives.join("; ") },
          { key: "X-Frame-Options", value: "DENY" },
          { key: "X-Content-Type-Options", value: "nosniff" },
          { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
          { key: "Permissions-Policy", value: "camera=(), microphone=(), geolocation=(), payment=(), usb=()" },
        ],
      },
    ];
  },
};

export default nextConfig;
