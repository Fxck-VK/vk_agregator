import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const scriptDirectory = dirname(fileURLToPath(import.meta.url));
const platformDirectory = resolve(scriptDirectory, "..");
const repositoryDirectory = resolve(platformDirectory, "..", "..");
const dockerfilePath = resolve(repositoryDirectory, "Dockerfile.platform");
const dockerignorePath = resolve(repositoryDirectory, ".dockerignore");
const ciWorkflowPath = resolve(repositoryDirectory, ".github", "workflows", "ci.yml");

let dockerfile;
try {
  dockerfile = await readFile(dockerfilePath, "utf8");
} catch (error) {
  throw new Error(`Platform Dockerfile is required at ${dockerfilePath}`, { cause: error });
}

const dockerignore = await readFile(dockerignorePath, "utf8");
const ciWorkflow = (await readFile(ciWorkflowPath, "utf8")).replace(/\r\n/g, "\n");

assert.doesNotMatch(
  dockerfile,
  /^#\s*syntax=/m,
  "Dockerfile must not use a mutable BuildKit syntax frontend",
);
for (const environmentPattern of ["**/.env*", "**/dev.env*", "**/_env*"]) {
  assert.match(
    dockerignore,
    new RegExp(`^${environmentPattern.replace(/[-/\\^$*+?.()|[\]{}]/g, "\\$&")}$`, "m"),
    `Docker build context must exclude nested ${environmentPattern} files`,
  );
}

const platformJob = ciWorkflow.match(/^  platform:\n(?<body>[\s\S]*?)(?=^  admin:)/m)?.[0];
assert.ok(platformJob, "CI must define the dedicated web platform job");
assert.match(platformJob, /set -euo pipefail/, "platform audits must preserve audit failures through tee");
assert.match(
  platformJob,
  /npm --prefix web\/platform audit --omit=dev --audit-level=high --json \| tee "\$RUNNER_TEMP\/platform-runtime-audit\.json"/,
  "CI must emit and enforce the production dependency audit",
);
assert.match(
  platformJob,
  /npm --prefix web\/platform audit --audit-level=high --json \| tee "\$RUNNER_TEMP\/platform-full-audit\.json"/,
  "CI must emit and enforce the full dependency audit",
);

assert.match(
  dockerfile,
  /^FROM node:24-alpine@sha256:[a-f0-9]{64} AS runtime$/m,
  "runtime must use a pinned Node Alpine base image",
);

const runtimeStage = dockerfile.split(/^FROM node:24-alpine@sha256:[a-f0-9]{64} AS runtime$/m)[1];
assert.ok(runtimeStage, "runtime stage must be present");
assert.match(runtimeStage, /^USER node$/m, "runtime must not run as root");
assert.match(
  runtimeStage,
  /^COPY --from=build --chown=node:node \/app\/.next\/standalone \.\/$/m,
  "runtime must copy only the standalone server output",
);
assert.match(
  runtimeStage,
  /^COPY --from=build --chown=node:node \/app\/.next\/static \.\/\.next\/static$/m,
  "runtime must copy the compiled static assets",
);
assert.match(
  runtimeStage,
  /^COPY --from=build --chown=node:node \/app\/public \.\/public$/m,
  "runtime must copy public assets only",
);
assert.doesNotMatch(
  runtimeStage,
  /^COPY --from=build .*\/(?:src|node_modules)(?:\s|$)/m,
  "runtime must not copy source code or the build dependency tree",
);

console.log("platform packaging assertions passed");
