// Real browser tests against a loopback fixture. No real accounts/providers.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { createServer } from "node:http";
import { test } from "node:test";
import ts from "typescript";
import { chromium } from "@playwright/test";

for (const fallback of [false, true]) {
  test(`two tabs share cookie rotation using ${fallback ? "IndexedDB" : "Web Locks"}`, async () => {
    let rotations = 0;
    const allowed = new Set(["browser-session", "browser-session-state", "session-lock", "read-error"]);
    const server = createServer(async (req, res) => {
      const path = new URL(req.url, "http://localhost").pathname;
      if (path.startsWith("/modules/")) {
        const name = path.slice(9).replace(/\.js$/, "");
        if (!allowed.has(name)) { res.writeHead(404).end(); return; }
        const source = await readFile(new URL(`../src/lib/web-api/${name}.ts`, import.meta.url), "utf8");
        res.writeHead(200, { "Content-Type": "text/javascript" }).end(ts.transpileModule(source, { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ES2022 } }).outputText);
        return;
      }
      if (path === "/web/v1/me") {
        res.writeHead(req.headers.cookie?.includes("nh_access=valid") ? 200 : 401, { "Cache-Control": "no-store" }).end(); return;
      }
      if (path === "/web/v1/auth/refresh") {
        rotations++;
        assert.equal(req.headers["x-csrf-token"], "synthetic");
        await new Promise(resolve => setTimeout(resolve, 50));
        res.writeHead(200, { "Set-Cookie": "nh_access=valid; HttpOnly; SameSite=Lax; Path=/", "Cache-Control": "no-store" }).end(); return;
      }
      res.writeHead(200, { "Content-Type": "text/html", "Set-Cookie": "nh_csrf=synthetic; SameSite=Lax; Path=/" }).end("<!doctype html><title>Session test</title>");
    });
    await new Promise(resolve => server.listen(0, "127.0.0.1", resolve));
    let browser;
    try {
      browser = await chromium.launch({ headless: true }).catch(error => {
        if (process.platform !== "win32") throw error;
        return chromium.launch({ channel: "msedge", headless: true });
      });
      const context = await browser.newContext();
      const pages = await Promise.all([context.newPage(), context.newPage()]);
      const origin = `http://127.0.0.1:${server.address().port}`;
      await Promise.all(pages.map(page => page.goto(origin)));
      if (fallback) await Promise.all(pages.map(page => page.evaluate(() => Object.defineProperty(navigator, "locks", { value: undefined }))));
      const statuses = await Promise.all(pages.map(page => page.evaluate(async () => {
        const { refreshBrowserSession } = await import("/modules/browser-session.js");
        return (await refreshBrowserSession()).status;
      })));
      assert.deepEqual(statuses, [200, 200]);
      assert.equal(rotations, 1);
      // A late 401 from an earlier read must not rotate the already updated session again.
      assert.equal(await pages[1].evaluate(async () => (await (await import("/modules/browser-session.js")).refreshBrowserSession()).status), 200);
      assert.equal(rotations, 1);
      await context.close();
    } finally {
      await browser?.close(); server.closeAllConnections(); await new Promise(resolve => server.close(resolve));
    }
  });
}
