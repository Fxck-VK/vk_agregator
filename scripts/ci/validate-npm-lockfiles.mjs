#!/usr/bin/env node

import { readFile } from "node:fs/promises";

const defaultLockfiles = [
  "web/miniapp/package-lock.json",
  "web/admin/package-lock.json",
  "web/platform/package-lock.json",
];
const lockfiles = process.argv.slice(2);
const targets = lockfiles.length > 0 ? lockfiles : defaultLockfiles;
let invalid = false;

for (const path of targets) {
  let lock;
  try {
    lock = JSON.parse(await readFile(path, "utf8"));
  } catch (error) {
    console.error(`${path}: unreadable lockfile (${error.name})`);
    invalid = true;
    continue;
  }

  if (!lock.packages || typeof lock.packages !== "object") {
    console.error(`${path}: missing packages inventory`);
    invalid = true;
    continue;
  }

  const missing = [];
  let packageCount = 0;
  for (const [packagePath, metadata] of Object.entries(lock.packages)) {
    if (!packagePath || metadata?.link === true) {
      continue;
    }
    packageCount += 1;
    const fields = [];
    if (typeof metadata?.resolved !== "string" || metadata.resolved.length === 0) {
      fields.push("resolved");
    }
    if (typeof metadata?.integrity !== "string" || metadata.integrity.length === 0) {
      fields.push("integrity");
    }
    if (fields.length > 0) {
      missing.push(`${packagePath} (${fields.join(",")})`);
    }
  }

  if (packageCount === 0) {
    console.error(`${path}: package inventory is empty`);
    invalid = true;
    continue;
  }
  if (missing.length > 0) {
    console.error(`${path}: ${missing.length} package entries lack immutable source metadata`);
    for (const item of missing) {
      console.error(`- ${item}`);
    }
    invalid = true;
    continue;
  }

  console.log(`${path}: ${packageCount} package entries have resolved URLs and integrity hashes`);
}

if (invalid) {
  process.exitCode = 1;
}
