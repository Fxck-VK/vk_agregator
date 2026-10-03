import assert from "node:assert/strict";
import { existsSync, readFileSync, readdirSync, statSync } from "node:fs";
import { createRequire } from "node:module";
import { dirname, resolve, sep } from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";

import { ESLint } from "eslint";

const scriptDirectory = dirname(fileURLToPath(import.meta.url));
const platformDirectory = resolve(scriptDirectory, "..");
const eslintConfigPath = resolve(platformDirectory, "eslint.config.mjs");
const scriptRequire = createRequire(import.meta.url);
const pluginPackagePath = scriptRequire.resolve("@next/eslint-plugin-next/package.json");
const pluginDirectory = dirname(pluginPackagePath);
const pluginRequire = createRequire(pluginPackagePath);
const getRootDirsPath = resolve(pluginDirectory, "dist", "utils", "get-root-dirs.js");

function toPosixPath(path) {
  return path.split(sep).join("/");
}

function normalizePath(path) {
  const normalized = resolve(path).split(sep).join("/");
  return process.platform === "win32" ? normalized.toLowerCase() : normalized;
}

function findPackageJson(startPath) {
  let current = dirname(startPath);

  while (current !== dirname(current)) {
    const packageJsonPath = resolve(current, "package.json");

    if (existsSync(packageJsonPath)) {
      const packageJson = readJson(packageJsonPath);

      if (packageJson.name && packageJson.version) {
        return packageJsonPath;
      }
    }

    current = dirname(current);
  }

  throw new Error(`Could not find package.json for ${startPath}`);
}

function collectJavaScriptFiles(directory) {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const entryPath = resolve(directory, entry.name);

    if (entry.isDirectory()) {
      return collectJavaScriptFiles(entryPath);
    }

    return entry.isFile() && entry.name.endsWith(".js") ? [entryPath] : [];
  });
}

function readJson(path) {
  return JSON.parse(readFileSync(path, "utf8"));
}

async function lintText(source) {
  const eslint = new ESLint({
    cwd: platformDirectory,
    overrideConfigFile: eslintConfigPath,
  });

  const [result] = await eslint.lintText(source, {
    filePath: resolve(platformDirectory, "src", "app", "lint-contract-fixture.tsx"),
  });

  return result.messages.filter((message) => message.ruleId === "@next/next/no-html-link-for-pages");
}

test("keeps the Next lint root-dir dependency scoped to the current caller contract", () => {
  const pluginFiles = collectJavaScriptFiles(resolve(pluginDirectory, "dist"));
  const fastGlobCallers = pluginFiles
    .filter((filePath) => readFileSync(filePath, "utf8").includes("fast-glob"))
    .map((filePath) => toPosixPath(filePath.slice(pluginDirectory.length + 1)));

  assert.deepEqual(fastGlobCallers, ["dist/utils/get-root-dirs.js"]);

  const getRootDirsSource = readFileSync(getRootDirsPath, "utf8");

  assert.ok(
    getRootDirsSource.includes('require("fast-glob")'),
    "get-root-dirs must still be the only fast-glob import",
  );
  assert.ok(
    getRootDirsSource.includes("(0, _fastglob.globSync)(rootDir.replace(/\\\\/g, '/'), {"),
    "Next must still call globSync with the rootDir pattern only",
  );
  assert.match(
    getRootDirsSource,
    /onlyDirectories:\s*true/,
    "Next must still ask for directories only; broaden this override if the caller changes",
  );

  const fastGlobAlias = pluginRequire("fast-glob");
  const aliasPackagePath = findPackageJson(pluginRequire.resolve("fast-glob"));
  const aliasPackage = readJson(aliasPackagePath);

  assert.equal(aliasPackage.name, "glob");
  assert.equal(aliasPackage.version, "12.0.0");
  assert.equal(typeof fastGlobAlias.globSync, "function");
  assert.deepEqual(
    fastGlobAlias.globSync(toPosixPath(platformDirectory), { onlyDirectories: true }).map(normalizePath),
    [normalizePath(platformDirectory)],
  );
});

test("keeps the Next no-html-link-for-pages rule active for app routes", async () => {
  assert.ok(
    statSync(resolve(platformDirectory, "src", "app", "login", "page.tsx")).isFile(),
    "the lint contract fixture depends on the real /login app route",
  );

  const anchorMessages = await lintText(`
    export default function Fixture() {
      return <a href="/login">Login</a>;
    }
  `);

  assert.equal(anchorMessages.length, 1);
  assert.match(anchorMessages[0].message, /Use `<Link \/>`/);

  const linkMessages = await lintText(`
    import Link from "next/link";

    export default function Fixture() {
      return <Link href="/login">Login</Link>;
    }
  `);

  assert.deepEqual(linkMessages, []);
});
