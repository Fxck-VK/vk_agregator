import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  base: './',
  build: {
    outDir: 'dist',
    sourcemap: false,
  },
  server: {
    host: true,
    port: 5173,
    // Accept the rotating tunnel domain (e.g. *.trycloudflare.com). Not
    // hardcoded — the URL changes every run.
    allowedHosts: true,
    // The page is served over HTTPS through the tunnel; force the HMR client
    // websocket onto wss:443 so live reload works behind it.
    hmr: { clientPort: 443, protocol: 'wss' },
    // Proxy BFF calls to the local API on the same origin so an HTTPS page
    // never has to call http://localhost (mixed content).
    proxy: {
      '/miniapp': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/webhooks': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
});
