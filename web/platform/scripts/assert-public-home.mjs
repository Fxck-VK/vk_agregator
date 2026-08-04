import assert from "node:assert/strict";
import { readdir, readFile, stat } from "node:fs/promises";
import { resolve } from "node:path";
import { pathToFileURL } from "node:url";

const root = resolve(import.meta.dirname, "..");
const landingRoot = resolve(root, "src", "features", "landing");

async function walk(directory) {
  const entries = await readdir(directory, { withFileTypes: true });
  return (await Promise.all(entries.map(async (entry) => {
    const path = resolve(directory, entry.name);
    return entry.isDirectory() ? walk(path) : [path];
  }))).flat();
}

const sourceFiles = (await walk(landingRoot)).filter((path) => /\.(?:ts|tsx)$/.test(path) && !path.endsWith(".test.ts") && !path.endsWith(".test.tsx"));
const landingSource = (await Promise.all(sourceFiles.map((path) => readFile(path, "utf8")))).join("\n");

for (const forbidden of ["WorkspaceFrame", "SidebarConversations", "SessionProvider", "AccountProvider", "@/features/session", "@/lib/web-api"]) {
  assert.doesNotMatch(landingSource, new RegExp(forbidden), `public landing must not import private module: ${forbidden}`);
}

const pageSource = await readFile(resolve(root, "src", "app", "page.tsx"), "utf8");
assert.match(pageSource, /canonical:\s*"\/"/, "homepage canonical metadata is required");
assert.match(pageSource, /index:\s*true/, "homepage must remain indexable");
assert.match(pageSource, /application\/ld\+json/, "homepage JSON-LD is required");

const prerenderManifest = JSON.parse(await readFile(resolve(root, ".next", "prerender-manifest.json"), "utf8"));
assert.ok(prerenderManifest.routes?.["/"], "homepage must be emitted as a static prerendered route");

const publicHomeSource = await readFile(resolve(landingRoot, "PublicHome", "PublicHome.tsx"), "utf8");
for (const block of ["hero", "quick-tools", "trust-strip", "news", "models", "how-it-works", "capabilities", "use-cases", "prompt-library", "faq", "social", "footer"]) {
  assert.match(publicHomeSource, new RegExp(`data-landing-block=\\"${block}\\"`), `missing landing block: ${block}`);
}

let clientBytes = 0;
try {
  const manifestPath = resolve(root, ".next", "server", "app", "page_client-reference-manifest.js");
  await import(`${pathToFileURL(manifestPath).href}?assert-public-home`);
  const manifest = globalThis.__RSC_MANIFEST?.["/page"];
  const chunks = manifest?.entryJSFiles?.["[project]/src/app/page"];
  assert.ok(Array.isArray(chunks) && chunks.length > 0, "public page client entry chunks are required");
  for (const chunk of new Set(chunks)) clientBytes += (await stat(resolve(root, ".next", chunk))).size;
  assert.ok(clientBytes <= 350 * 1024, `public page client entry is ${Math.round(clientBytes / 1024)} KiB; expected at most 350 KiB`);
} catch (error) {
  if (error?.code === "ENOENT") throw new Error("Run `npm run build` before `npm run test:public-home`.", { cause: error });
  throw error;
}

console.log(`public homepage assertions passed; page client entry ${Math.round(clientBytes / 1024)} KiB`);
