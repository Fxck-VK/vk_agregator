import { defineConfig, devices } from "@playwright/test";

const shouldStartWebServer = process.env.PLAYWRIGHT_SKIP_WEBSERVER !== "1";

export default defineConfig({
  testDir: "./e2e",
  timeout: 30_000,
  expect: {
    timeout: 8_000,
  },
  fullyParallel: false,
  reporter: [["list"]],
  use: {
    ...devices["Pixel 5"],
    baseURL: "http://127.0.0.1:5174",
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
    video: "off",
  },
  webServer: shouldStartWebServer
    ? {
        command: "node ./node_modules/vite/bin/vite.js --host 127.0.0.1 --port 5174",
        url: "http://127.0.0.1:5174",
        reuseExistingServer: !process.env.CI,
        timeout: 120_000,
        env: {
          VITE_DISABLE_HMR: "1",
          VITE_DEV_HOST: "127.0.0.1",
          VITE_FRONTEND_TELEMETRY_ENABLED: "true",
        },
      }
    : undefined,
});
