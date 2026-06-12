import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

const apiTarget = 'http://127.0.0.1:8080';
const tunnelHost = process.env.VITE_TUNNEL_HOST?.trim();
const devHost = process.env.VITE_DEV_HOST?.trim() || '127.0.0.1';

function resolveHmr():
  | boolean
  | {
      host: string;
      protocol: 'wss';
      clientPort: number;
    } {
  if (process.env.VITE_DISABLE_HMR === '1') {
    return false;
  }
  if (tunnelHost) {
    return {
      host: tunnelHost,
      protocol: 'wss',
      clientPort: 443,
    };
  }
  return true;
}

export default defineConfig({
  plugins: [react()],
  base: './',
  build: {
    outDir: 'dist',
    sourcemap: false,
    cssMinify: 'esbuild',
  },
  server: {
    host: devHost,
    port: 5173,
    allowedHosts: tunnelHost ? [tunnelHost] : undefined,
    hmr: resolveHmr(),
    proxy: {
      '/miniapp': {
        target: apiTarget,
        changeOrigin: true,
      },
      '/api': {
        target: apiTarget,
        changeOrigin: true,
      },
      '/webhooks': {
        target: apiTarget,
        changeOrigin: true,
      },
    },
  },
});
