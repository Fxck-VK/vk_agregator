import assert from "node:assert/strict";
import {readFileSync, readdirSync} from "node:fs";
import {resolve} from "node:path";
import test from "node:test";
import {collectPageInventory, platformRoot, renderHtml, renderMarkdown} from "./release-page-inventory.mjs";

const inventory = collectPageInventory();

test("keeps the platform, authentication and legal routes in PROD and unfinished pages in DEV", () => {
  for (const path of ["/", "/app", "/app/files", "/app/models", "/app/profile", "/login/register", "/legal", "/legal/marketing-consent"]) {
    assert.ok(inventory.prod.some(page => page.path === path), path);
  }
  for (const path of ["/app/music", "/app/speech", "/app/ui-states", "/ai", "/ai/docs", "/landing-variants"]) {
    assert.ok(inventory.dev.some(page => page.path === path), path);
    assert.ok(!inventory.prod.some(page => page.path === path), path);
  }
  assert.equal(inventory.dev.filter(page => page.path.startsWith("/academy/courses/epifera-essentials/")).length, 14);
  assert.equal(inventory.dev.filter(page => page.path.startsWith("/ai/docs/")).length, 7);
});

test("accounts for every implemented page source and keeps the lists disjoint", () => {
  const sources = [...inventory.prod, ...inventory.dev, ...inventory.redirects, ...inventory.excluded];
  const covered = new Set(sources.map(page => page.source));
  const files = readdirSync(resolve(platformRoot, "src/app"), {recursive: true});
  const pages = files.filter(file => /(?:^|[/\\])page\.[jt]sx?$/.test(file)
    && !file.split(/[/\\]/).some(part => part.startsWith("_") || part.startsWith("@")));
  for (const file of pages) {
    assert.ok(covered.has("src/app/" + file.replaceAll("\\", "/")), file);
  }
  const prod = new Set(inventory.prod.map(page => page.path));
  assert.equal(prod.size, inventory.prod.length);
  assert.equal(new Set(inventory.dev.map(page => page.path)).size, inventory.dev.length);
  assert.ok(inventory.dev.every(page => !prod.has(page.path)));
  assert.ok(inventory.prod.find(page => page.path === "/app/chat/[conversationId]").parameter);
  assert.ok(inventory.prod.find(page => page.path === "/invite/[code]").parameter);
});

test("committed lists match current release admission", () => {
  for (const view of ["prod", "dev"]) {
    const stored = readFileSync(resolve(platformRoot, "docs/pages-" + view + ".md"), "utf8").replaceAll("\r\n", "\n");
    assert.equal(stored, renderMarkdown(inventory, view));
  }
});

test("browser reports escape labels and keep parameterized routes as text", () => {
  const copy = structuredClone(inventory);
  copy.prod[0].title = '<script>alert("label")</script>';
  const html = renderHtml(copy, "prod");
  assert.ok(html.includes("&lt;script&gt;"));
  assert.ok(!html.includes('<script>alert("label")'));
  assert.ok(!/href="[^"]*\[conversationId\]/.test(html));
  assert.ok(html.includes('href="dev.html"'));
  assert.ok(html.includes("http://localhost:7159/ru/app"));
});
