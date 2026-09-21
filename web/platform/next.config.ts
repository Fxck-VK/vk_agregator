import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Project-owned AGENTS.md already routes agents to the shared UI catalogue.
  agentRules: false,
  output: "standalone",
  // Keep prefetch headers visible to the locale proxy: speculative requests must
  // never overwrite the user's language or authentication return destination.
  skipProxyUrlNormalize: true,
  productionBrowserSourceMaps: false,
  turbopack: {
    root: process.cwd(),
  },
};

export default nextConfig;
