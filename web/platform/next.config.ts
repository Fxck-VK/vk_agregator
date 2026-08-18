import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "standalone",
  productionBrowserSourceMaps: false,
  turbopack: {
    root: process.cwd(),
  },
};

export default nextConfig;
