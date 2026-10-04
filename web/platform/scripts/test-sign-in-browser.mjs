// The built platform runs against a loopback fixture; no real accounts or mail.
import assert from "node:assert/strict";
import { randomUUID } from "node:crypto";
import { cp, readFile } from "node:fs/promises";
import { createServer } from "node:http";
import { once } from "node:events";
import { spawn } from "node:child_process";
import { test } from "node:test";
import { chromium } from "@playwright/test";

const listen = server => new Promise(resolve => server.listen(0, "localhost", resolve));
const close = server => { server.closeAllConnections(); return new Promise(resolve => server.close(resolve)); };
const account = "10000000-0000-4000-8000-000000000001";
const profile = { account_id: account, identity_refs: [], password_set: true };
const methods = { registration: false, password: true, recovery: false, email_link: false, phone_link: false, providers: [] };
const delay = milliseconds => new Promise(resolve => setTimeout(resolve, milliseconds));

test("password sign-in replaces cached guest content and explains failures", { timeout: 120_000 }, async t => {
  // Standalone packaging requires these alongside the generated server, just
  // as Dockerfile.platform copies them into the deployed runtime.
  await cp(new URL("../.next/static", import.meta.url), new URL("../.next/standalone/.next/static", import.meta.url), { recursive: true });
  await cp(new URL("../public", import.meta.url), new URL("../.next/standalone/public", import.meta.url), { recursive: true });
  const models = JSON.parse(await readFile(new URL("../src/features/session/model-catalog.preview.json", import.meta.url), "utf8"));
  let mode = "success"; let loginRequests = 0; let loginId; let sessionId;
  const api = createServer(async (request, response) => {
    const path = new URL(request.url, "http://localhost").pathname;
    const id = request.headers["x-request-id"] ?? randomUUID();
    response.setHeader("X-Request-ID", id); response.setHeader("Cache-Control", "no-store");
    const json = (status, payload) => response.writeHead(status, { "Content-Type": "application/json" }).end(JSON.stringify(payload));
    if (path === "/web/v1/auth/methods") return json(200, methods);
    if (path === "/web/v1/models") return json(200, models);
    if (path === "/web/v1/me") {
      sessionId = id;
      const signedIn = mode !== "cookies-rejected" && request.headers.cookie?.includes("nh_access=synthetic");
      if (signedIn) { response.setHeader("X-NeiroHub-Account-ID", account); await delay(150); }
      return json(signedIn ? 200 : 401, signedIn ? profile : { error: "unauthorized" });
    }
    if (path === "/web/v1/auth/password/login") {
      loginRequests++; loginId = id;
      let body = ""; for await (const chunk of request) body += chunk;
      assert.deepEqual(JSON.parse(body), { email: "member@example.test", password: "synthetic-password" });
      assert.ok(request.headers.origin);
      if (mode === "unavailable") return json(503, { error: "authentication unavailable" });
      if (mode !== "cookies-rejected") response.setHeader("Set-Cookie", "nh_access=synthetic; Secure; HttpOnly; SameSite=Lax; Path=/");
      response.setHeader("X-NeiroHub-Account-ID", account);
      return json(201, { session: { account_id: account } });
    }
    if (path === "/web/v1/balance") return json(200, { balance: 1000 });
    if (path === "/web/v1/conversations") return json(200, { items: [] });
    return json(404, { error: "fixture route unavailable" });
  });
  await listen(api);
  const reservation = createServer(); await listen(reservation);
  const port = reservation.address().port; await close(reservation);
  const origin = `http://localhost:${port}`;
  const child = spawn(process.execPath, [".next/standalone/server.js"], {
    env: { ...process.env, NODE_ENV: "production", HOSTNAME: "localhost", PORT: String(port), WEB_API_INTERNAL_ORIGIN: `http://localhost:${api.address().port}`, NEIROHUB_LOCAL_WORKSPACE_PREVIEW: "0" },
    stdio: "ignore", windowsHide: true,
  });
  let browser;
  try {
    let ready = false;
    for (let attempt = 0; attempt < 80; attempt++) {
      try { ready = (await fetch(`${origin}/health`)).ok; } catch { /* startup */ }
      if (ready || child.exitCode !== null) break;
      await delay(250);
    }
    assert.ok(ready, "isolated platform did not become ready");
    browser = await chromium.launch({ headless: true }).catch(error => {
      if (process.platform !== "win32") throw error;
      return chromium.launch({ channel: "msedge", headless: true });
    });
    for (const scenario of ["success", "cookies-rejected", "unavailable"]) {
      await t.test(scenario, async () => {
        mode = scenario; loginRequests = 0;
        const context = await browser.newContext(); const page = await context.newPage();
        let documents = 0;
        page.on("request", request => { if (request.isNavigationRequest() && new URL(request.url()).pathname === "/ru/app") documents++; });
        try {
          // Cache a real guest workspace before moving to its login page.
          const landing = await page.goto(`${origin}/ru/app`);
          assert.equal(landing.status(), 200);
          await page.getByRole("link", { name: "Войти", exact: true }).first().click();
          await page.getByLabel("Электронная почта", { exact: true }).fill("member@example.test");
          await page.getByLabel("Пароль", { exact: true }).fill("synthetic-password");
          await page.evaluate(() => { window.signInOldDocument = true; });
          await page.getByRole("button", { name: "Войти", exact: true }).click();
          if (scenario === "success") {
            await page.waitForURL(`${origin}/ru/app`);
            await page.locator('[data-sidebar-account-trigger="true"]').waitFor();
            assert.equal(await page.getByRole("button", { name: "Войти", exact: true }).count(), 0);
            assert.equal(await page.getByRole("link", { name: "Войти", exact: true }).count(), 0);
            assert.equal(await page.evaluate(() => window.signInOldDocument), undefined);
            assert.equal(documents, 2, "sign-in must fetch a fresh document instead of reusing guest RSC");
          } else {
            const reason = scenario === "cookies-rejected" ? /Не удалось подтвердить вход/ : /Сервис входа временно недоступен/;
            const alert = page.getByRole("alert").filter({ hasText: reason }); await alert.waitFor();
            const text = await alert.innerText();
            assert.match(text, reason);
            assert.ok(text.includes(scenario === "cookies-rejected" ? sessionId : loginId));
            assert.equal(documents, 1);
            assert.equal(await page.evaluate(() => window.signInOldDocument), true);
            assert.equal(await page.getByRole("button", { name: scenario === "cookies-rejected" ? "Повторить вход" : "Войти", exact: true }).isEnabled(), true);
            const password = page.locator('input[type="password"]');
            assert.equal(await password.count() ? await password.inputValue() : "", "");
          }
          assert.equal(loginRequests, 1);
        } finally { await context.close(); }
      });
    }
  } finally {
    await browser?.close();
    if (child.exitCode === null) { const stopped = once(child, "exit"); child.kill(); await stopped; }
    await close(api);
  }
});
