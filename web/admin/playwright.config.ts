const playwright = await import("@playwright/test").catch(() => import("../miniapp/node_modules/@playwright/test/index.mjs"));
const { defineConfig, devices } = playwright;

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
    ...devices["Desktop Chrome"],
    baseURL: "http://127.0.0.1:5175",
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
    video: "off",
  },
  webServer: shouldStartWebServer
    ? {
        command: "node ./node_modules/vite/bin/vite.js --host 127.0.0.1 --port 5175",
        url: "http://127.0.0.1:5175",
        reuseExistingServer: !process.env.CI,
        timeout: 120_000,
        env: {
          VITE_ADMIN_DEV_HOST: "127.0.0.1",
          VITE_DISABLE_HMR: "1",
        },
      }
    : undefined,
});
