import { spawn } from "node:child_process";
import { fileURLToPath } from "node:url";
import { generatePublicPageManifest, watchPublicPages } from "./public-page-manifest.mjs";

generatePublicPageManifest();
const stopPublicPageWatch = watchPublicPages();

const child = spawn(process.execPath, [
  // Keep the rewrite origin as localhost while binding its IPv4 loopback address.
  "--dns-result-order=ipv4first",
  fileURLToPath(new URL("../node_modules/next/dist/bin/next", import.meta.url)),
  "dev", "--hostname", "localhost", "--port", "7159", ...process.argv.slice(2),
], {
  cwd: fileURLToPath(new URL("..", import.meta.url)),
  env: { ...process.env, NEIROHUB_LOCAL_WORKSPACE_PREVIEW: "1" },
  stdio: "inherit",
  windowsHide: true,
});
child.on("error", error => { stopPublicPageWatch(); console.error(error.message); process.exitCode = 1; });
child.on("exit", code => { stopPublicPageWatch(); process.exitCode = code ?? 1; });
for (const signal of ["SIGINT", "SIGTERM"]) process.on(signal, () => { child.kill(signal); });
