import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

const apiTarget = "http://127.0.0.1:8080";
const devHost = process.env.VITE_ADMIN_DEV_HOST?.trim() || "127.0.0.1";

export default defineConfig({
  plugins: [react()],
  base: "./",
  build: {
    outDir: "dist",
    sourcemap: false,
    cssMinify: "esbuild",
  },
  server: {
    host: devHost,
    port: 5175,
    proxy: {
      "/admin": {
        target: apiTarget,
        changeOrigin: true,
      },
      "/billing": {
        target: apiTarget,
        changeOrigin: true,
      },
      "/health": {
        target: apiTarget,
        changeOrigin: true,
      },
      "/healthz": {
        target: apiTarget,
        changeOrigin: true,
      },
    },
  },
});
