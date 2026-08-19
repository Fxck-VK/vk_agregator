import { access, readdir, readFile } from "node:fs/promises";
import { dirname, extname, relative, resolve, sep } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

const allowedExtensions = new Set([".avif", ".png", ".svg", ".webp"]);
const allowedMetadataNames = new Set([".gitkeep", "README.md"]);
const kebabCaseAssetName = /^[a-z0-9]+(?:-[a-z0-9]+)*\.(?:avif|png|svg|webp)$/;
const unsafeSvgPatterns = [
  /<script\b/i,
  /<foreignObject\b/i,
  /<(?:embed|iframe|object)\b/i,
  /<!DOCTYPE\b/i,
  /<!ENTITY\b/i,
  /\son[a-z]+\s*=/i,
  /(?:href|xlink:href)\s*=\s*["']\s*(?:[a-z][a-z0-9+.-]*:|\/\/)/i,
];

function normalizePath(value) {
  return value.split("\\").join("/");
}

export function inspectAssetEntries(entries) {
  const errors = [];
  const seenPaths = new Map();

  for (const entry of [...entries].sort((left, right) =>
    left.relativePath.localeCompare(right.relativePath))) {
    const relativePath = normalizePath(entry.relativePath);
    const normalizedPath = relativePath.toLowerCase();
    const previousPath = seenPaths.get(normalizedPath);
    if (previousPath) {
      errors.push(`${relativePath}: duplicate normalized path of ${previousPath}`);
    } else {
      seenPaths.set(normalizedPath, relativePath);
    }

    const fileName = relativePath.split("/").at(-1) ?? relativePath;
    if (allowedMetadataNames.has(fileName)) {
      continue;
    }

    const extension = extname(fileName).toLowerCase();
    if (!allowedExtensions.has(extension)) {
      errors.push(`${relativePath}: unsupported extension ${extension || "<none>"}`);
      continue;
    }
    if (!kebabCaseAssetName.test(fileName)) {
      errors.push(`${relativePath}: asset filename must use lowercase kebab-case`);
    }
    if (
      extension === ".svg" &&
      unsafeSvgPatterns.some((pattern) => pattern.test(entry.content ?? ""))
    ) {
      errors.push(`${relativePath}: unsafe SVG content`);
    }
  }

  return errors;
}

async function collectFiles(root, prefix, include) {
  const records = [];

  async function visit(directory) {
    const entries = await readdir(directory, { withFileTypes: true });
    entries.sort((left, right) => left.name.localeCompare(right.name));
    for (const entry of entries) {
      const absolutePath = resolve(directory, entry.name);
      if (entry.isDirectory()) {
        await visit(absolutePath);
        continue;
      }
      if (!entry.isFile() || !include(absolutePath)) {
        continue;
      }

      const relativePath = normalizePath(relative(root, absolutePath));
      const extension = extname(entry.name).toLowerCase();
      records.push({
        relativePath: `${prefix}/${relativePath}`,
        content: extension === ".svg" ? await readFile(absolutePath, "utf8") : undefined,
      });
    }
  }

  await visit(root);
  return records;
}

export async function validateAssetLibrary(projectRoot) {
  const publicRoot = resolve(projectRoot, "public", "assets");
  try {
    await access(publicRoot);
  } catch {
    return ["public/assets: missing required asset root"];
  }

  const publicRecords = await collectFiles(publicRoot, "public/assets", () => true);
  const featuresRoot = resolve(projectRoot, "src", "features");
  const featureRecords = await collectFiles(featuresRoot, "src/features", (absolutePath) => {
    const relativePath = relative(featuresRoot, absolutePath).split(sep);
    return relativePath.includes("assets");
  });
  return inspectAssetEntries([...publicRecords, ...featureRecords]);
}

const scriptPath = fileURLToPath(import.meta.url);
const isCli = process.argv[1] && pathToFileURL(resolve(process.argv[1])).href === import.meta.url;
if (isCli) {
  const projectRoot = resolve(dirname(scriptPath), "..");
  const errors = await validateAssetLibrary(projectRoot);
  if (errors.length > 0) {
    for (const error of errors) {
      console.error(error);
    }
    process.exitCode = 1;
  } else {
    console.log("Asset validation passed.");
  }
}
