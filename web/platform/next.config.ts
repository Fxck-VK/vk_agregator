import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Project-owned AGENTS.md already routes agents to the shared UI catalogue.
  agentRules: false,
  output: "standalone",
  productionBrowserSourceMaps: false,
  turbopack: {
    root: process.cwd(),
  },
};

export default nextConfig;
