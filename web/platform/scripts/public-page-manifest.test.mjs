import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { mkdtemp, mkdir, readFile, rm, stat, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";

const script = fileURLToPath(new URL("./public-page-manifest.mjs", import.meta.url));

async function fixture(t, paths) {
  const root = await mkdtemp(join(tmpdir(), "epifera-publication-"));
  t.after(() => rm(root, { recursive: true, force: true }));
  for (const path of paths) {
    const target = join(root, "src/app", path);
    await mkdir(dirname(target), { recursive: true });
    await writeFile(target, "export const publications = [];\n");
  }
  return root;
}

function generate(root) {
  return spawnSync(process.execPath, [script, root], { encoding: "utf8", windowsHide: true });
}

test("discovers new static and dynamic public routes, never private or unimplemented content", async t => {
  const root = await fixture(t, [
    "(public)/page.tsx", "(public)/publication.ts",
    "(public)/(editorial)/guides/[slug]/page.tsx", "(public)/(editorial)/guides/[slug]/publication.ts",
    "app/private/page.tsx", "app/private/publication.ts",
    "(public)/_internal/page.tsx",
  ]);
  const result = generate(root);
  assert.equal(result.status, 0, result.stderr);
  const manifest = await readFile(join(root, "src/content/publication/generated.ts"), "utf8");
  assert.match(manifest, /route: "\/"/);
  assert.match(manifest, /route: "\/guides\/\[slug\]"/);
  assert.match(manifest, /\(editorial\)\/guides\/\[slug\]\/publication/);
  assert.doesNotMatch(manifest, /private|_internal|data\/models/);
});

test("fails before build if a public page has no explicit publication policy", async t => {
  const root = await fixture(t, ["(public)/new-page/page.tsx"]);
  const result = generate(root);
  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /publication\.ts/);
});

test("rejects orphan declarations rather than advertising nonexistent pages", async t => {
  const root = await fixture(t, ["(public)/missing/publication.ts"]);
  const result = generate(root);
  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /page\.tsx/);
});

test("does not rewrite the manifest when nothing changed", async t => {
  const root = await fixture(t, ["(public)/page.tsx", "(public)/publication.ts"]);
  assert.equal(generate(root).status, 0);
  const target = join(root, "src/content/publication/generated.ts");
  const before = await stat(target);
  assert.equal(generate(root).status, 0);
  assert.equal((await stat(target)).mtimeMs, before.mtimeMs);
});
