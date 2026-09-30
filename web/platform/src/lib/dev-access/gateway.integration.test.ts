// @vitest-environment node
// Run with NGINX_BINARY set to a local Nginx executable. No live credentials or
// backend are used: the upstream calls the real route handler from this project.
import { spawn, spawnSync, type ChildProcess } from "node:child_process";
import { createHash } from "node:crypto";
import { mkdtempSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { createServer, request as httpRequest, type Server } from "node:http";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { afterAll, beforeAll, describe, expect, it, vi } from "vitest";

vi.mock("server-only", () => ({}));
import { GET } from "@/app/web/dev-access/[action]/route";

const binary = process.env.NGINX_BINARY;
const credential = "dev-user:$apr1$testsalt$MzvSpeHc5QSY4QdwuDl1x/";
const basic = "Basic " + Buffer.from("dev-user:fixture-password").toString("base64");

async function listen(server: Server): Promise<number> {
  await new Promise<void>(resolve => server.listen(0, "127.0.0.1", resolve));
  return (server.address() as { port: number }).port;
}

describe.skipIf(!binary)("real DEV Nginx gateway", () => {
  let upstream: Server;
  let nginx: ChildProcess;
  let dir: string;
  let base: string;

  // Native HTTP preserves Sec-Fetch-Mode; Node fetch overwrites it with "cors".
  const request = (path: string, headers: Record<string, string> = {}, method = "GET") => new Promise<Response>((resolve, reject) => {
    const req = httpRequest(base + path, { method, headers: { host: "dev-web.neiirohub.ru", ...headers } }, res => {
      const chunks: Buffer[] = [];
      res.on("data", chunk => chunks.push(chunk));
      res.on("end", () => {
        const responseHeaders = new Headers();
        for (const [key, value] of Object.entries(res.headers)) if (value) responseHeaders.set(key, String(value));
        resolve(new Response(Buffer.concat(chunks), { status: res.statusCode!, headers: responseHeaders }));
      });
    });
    req.on("error", reject);
    req.end();
  });

  beforeAll(async () => {
    vi.stubEnv("WEB_ORIGIN", "https://dev-web.neiirohub.ru");
    vi.stubEnv("DEV_WEB_BASIC_AUTH_HTPASSWD", credential);
    upstream = createServer(async (req, res) => {
      try {
        const url = new URL(req.url!, "https://dev-web.neiirohub.ru");
        if (url.pathname.startsWith("/web/dev-access/")) {
          const headers = new Headers();
          for (const [key, value] of Object.entries(req.headers)) if (value) headers.set(key, String(value));
          const response = await GET(new Request(url, { headers }), { params: Promise.resolve({ action: url.pathname.split("/").at(-1)! }) });
          res.writeHead(response.status, Object.fromEntries(response.headers));
          res.end(Buffer.from(await response.arrayBuffer()));
        } else if (url.pathname === "/web/v1/me") {
          res.writeHead(401, { "WWW-Authenticate": 'Basic realm="account-error"', "Content-Type": "application/json" });
          res.end('{"error":"account-session-required"}');
        } else {
          res.writeHead(200, { "Content-Type": "application/json" });
          res.end(JSON.stringify({ path: url.pathname, proof: req.headers["x-dev-web-proof"] ?? null, user: req.headers["x-dev-web-user"] ?? null, authorization: req.headers.authorization ?? null }));
        }
      } catch {
        res.writeHead(500); res.end();
      }
    });
    const upstreamPort = await listen(upstream);
    const reservation = createServer();
    const port = await listen(reservation);
    await new Promise<void>(resolve => reservation.close(() => resolve()));
    base = `http://127.0.0.1:${port}`;
    dir = mkdtempSync(join(tmpdir(), "nh-dev-gate-test-"));
    mkdirSync(join(dir, "logs"));
    mkdirSync(join(dir, "temp"));
    writeFileSync(join(dir, "htpasswd"), credential + "\n");
    const proof = createHash("sha256").update(`neirohub-dev-web-issuer-v1:${credential}`).digest("hex");
    writeFileSync(join(dir, "issuer.conf"), `proxy_set_header X-Dev-Web-Proof "${proof}";\n`);
    // Preserve the production location rules; replace only environment paths,
    // upstream address and listener. All ports bind only to loopback.
    const fragment = readFileSync(resolve("../../deployments/nginx/dev-web.conf"), "utf8")
      .replace("server platform:3000;", `server 127.0.0.1:${upstreamPort};`)
      .replace("listen 80;", `listen 127.0.0.1:${port};`)
      .replace("/tmp/dev-web.htpasswd", "htpasswd")
      .replace("/tmp/dev-web-issuer.conf", "issuer.conf");
    writeFileSync(join(dir, "nginx.conf"), `worker_processes 1;\npid nginx.pid;\nerror_log logs/error.log;\nevents { worker_connections 64; }\nhttp { access_log off; proxy_temp_path temp; map $http_x_forwarded_proto $forwarded_proto { default https; }\n${fragment}\n}`);
    const prefix = dir.replaceAll("\\", "/") + "/";
    const checked = spawnSync(binary!, ["-p", prefix, "-c", "nginx.conf", "-t"], { cwd: dir, encoding: "utf8", windowsHide: true });
    expect(checked.status, checked.stderr).toBe(0);
    nginx = spawn(binary!, ["-p", prefix, "-c", "nginx.conf", "-g", "daemon off;"], { cwd: dir, stdio: "ignore", windowsHide: true });
    await vi.waitFor(async () => expect((await request("/")).status).toBe(401), { timeout: 10000, interval: 100 });
  }, 15000);

  afterAll(async () => {
    if (nginx && dir) {
      spawnSync(binary!, ["-p", dir.replaceAll("\\", "/") + "/", "-c", "nginx.conf", "-s", "quit"], { cwd: dir, stdio: "ignore", windowsHide: true });
      await vi.waitFor(() => expect(nginx.exitCode).not.toBeNull(), { timeout: 10000 });
    }
    upstream?.closeAllConnections();
    if (upstream) await new Promise<void>(resolve => upstream.close(() => resolve()));
    vi.unstubAllEnvs();
    // Keep the unique temp folder's synthetic-only config/error log for diagnosis.
  });

  it("protects pages, API and static assets without repeated password challenges", async () => {
    for (const path of ["/", "/ru/app", "/web/v1/me", "/assets/image.jpg", "/_next/static/test.js"]) {
      const response = await request(path, { authorization: basic });
      expect(response.status).toBe(401);
      expect(response.headers.get("www-authenticate")).toBeNull();
      expect(response.headers.get("location")).toBeNull();
    }
  });

  it("challenges only at login, rejects wrong passwords and remembers valid access", async () => {
    const navigate = await request("/en/app?model=test", { "sec-fetch-mode": "navigate", "sec-fetch-dest": "document" });
    expect(navigate.status).toBe(303);
    expect(navigate.headers.get("location")).toBe("/__dev/login?returnTo=%2Fen%2Fapp%3Fmodel%3Dtest");
    const challenge = await request("/__dev/login");
    expect(challenge.status).toBe(401);
    expect(challenge.headers.get("www-authenticate")).toContain("NeiroHub development");
    expect((await request("/__dev/login", { authorization: "Basic " + Buffer.from("dev-user:wrong").toString("base64") })).status).toBe(401);
    const login = await request("/__dev/login?returnTo=%2Fen%2Fapp", { authorization: basic });
    expect(login.status).toBe(303);
    expect(login.headers.get("location")).toBe("/en/app");
    const cookie = login.headers.get("set-cookie")!.split(";")[0];
    for (const path of ["/en/app", "/assets/image.jpg", "/_next/static/test.js"]) {
      const allowed = await request(path, { cookie, "x-dev-web-proof": "forged", "x-dev-web-user": "forged", authorization: basic });
      expect(allowed.status).toBe(200);
      expect(await allowed.json()).toEqual({ path, proof: null, user: null, authorization: null });
      expect(allowed.headers.get("set-cookie")).toBeNull();
      expect(allowed.headers.get("cache-control")).toBe("private, no-store");
    }
    const account = await request("/web/v1/me", { cookie });
    expect(account.status).toBe(401);
    expect(await account.json()).toEqual({ error: "account-session-required" });
    expect(account.headers.get("www-authenticate")).toBeNull();
    expect(account.headers.get("location")).toBeNull();
    expect((await request("/ru/app", { cookie: cookie + "tampered" })).status).toBe(401);
  });

  it("blocks public internal routes and does not replay a rejected write", async () => {
    for (const path of ["/web/dev-access/issue", "/web/dev-access/check", "/_dev_access_check", "/__dev/unknown", "/admin", "/metrics"]) {
      expect((await request(path, { "x-dev-web-user": "dev-user" })).status).toBe(404);
    }
    const write = await request("/web/v1/image-jobs", { "sec-fetch-mode": "navigate", "sec-fetch-dest": "document" }, "POST");
    expect(write.status).toBe(401);
    expect(write.headers.get("location")).toBeNull();
    expect((await request("/__dev/login", { authorization: basic }, "POST")).status).toBe(403);
  });

  it("fails closed when the session checker loses its DEV configuration", async () => {
    vi.stubEnv("DEV_WEB_BASIC_AUTH_HTPASSWD", "");
    try {
      const unavailable = await request("/ru/app");
      expect(unavailable.status).toBe(500);
      expect(unavailable.headers.get("www-authenticate")).toBeNull();
    } finally {
      vi.stubEnv("DEV_WEB_BASIC_AUTH_HTPASSWD", credential);
    }
  });
});
