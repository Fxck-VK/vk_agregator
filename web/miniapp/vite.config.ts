import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

const apiTarget = 'http://127.0.0.1:8080';
const tunnelHost = process.env.VITE_TUNNEL_HOST?.trim();

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
  },
  server: {
    host: true,
    port: 5173,
    allowedHosts: true,
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
