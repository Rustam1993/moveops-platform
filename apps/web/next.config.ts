import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  experimental: {
    optimizePackageImports: ["@moveops/client"],
  },
};

export default nextConfig;
