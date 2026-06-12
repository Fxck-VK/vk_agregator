import { spawn } from "node:child_process";
import http from "node:http";
import { fileURLToPath } from "node:url";
import path from "node:path";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const baseURL = "http://127.0.0.1:5174";
const startupTimeoutMs = 120_000;
const testTimeoutMs = 120_000;

function spawnNode(args, env = {}) {
  return spawn(process.execPath, args, {
    cwd: root,
    env: { ...process.env, ...env },
    stdio: "inherit",
    windowsHide: true,
  });
}

function isReachable() {
  return new Promise((resolve) => {
    const req = http.get(baseURL, (res) => {
      res.resume();
      resolve(true);
    });
    req.on("error", () => resolve(false));
    req.setTimeout(1000, () => {
      req.destroy();
      resolve(false);
    });
  });
}

async function waitForServer(child) {
  const started = Date.now();
  while (Date.now() - started < startupTimeoutMs) {
    if (await isReachable()) return;
    if (child?.exitCode !== null) {
      throw new Error(`Vite exited before becoming ready with code ${child.exitCode}`);
    }
    await new Promise((resolve) => setTimeout(resolve, 500));
  }
  throw new Error(`Timed out waiting for ${baseURL}`);
}

function waitForExit(child, timeoutMs) {
  return new Promise((resolve) => {
    const timer = setTimeout(() => {
      child.kill("SIGKILL");
      resolve({ code: 1, signal: "timeout" });
    }, timeoutMs);
    child.on("exit", (code, signal) => {
      clearTimeout(timer);
      resolve({ code, signal });
    });
  });
}

async function stop(child) {
  if (!child || child.exitCode !== null || child.killed) return;
  child.kill("SIGTERM");
  const result = await waitForExit(child, 5000);
  if (result.signal === "timeout") {
    child.kill("SIGKILL");
  }
}

let server;
let ownsServer = false;

try {
  if (!(await isReachable())) {
    ownsServer = true;
    server = spawnNode(["./node_modules/vite/bin/vite.js", "--host", "127.0.0.1", "--port", "5174"], {
      VITE_DISABLE_HMR: "1",
      VITE_DEV_HOST: "127.0.0.1",
      VITE_FRONTEND_TELEMETRY_ENABLED: "true",
    });
  }

  await waitForServer(server);

  const testRun = spawnNode(["./node_modules/@playwright/test/cli.js", "test", "--config=playwright.config.ts"], {
    PLAYWRIGHT_SKIP_WEBSERVER: "1",
  });
  const result = await waitForExit(testRun, testTimeoutMs);
  if (result.code !== 0) {
    process.exitCode = result.code ?? 1;
  }
} catch (error) {
  console.error(error instanceof Error ? error.message : String(error));
  process.exitCode = 1;
} finally {
  if (ownsServer) {
    await stop(server);
  }
}
