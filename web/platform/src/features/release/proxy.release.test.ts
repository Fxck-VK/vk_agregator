import { NextRequest } from "next/server";
import { afterEach, expect, it, vi } from "vitest";
vi.mock("server-only", () => ({}));
import { proxy } from "@/proxy";
afterEach(() => vi.unstubAllEnvs());
it("blocks direct PROD routes despite forged DEV cookies and headers", () => {
  vi.stubEnv("NODE_ENV", "production"); vi.stubEnv("WEB_ORIGIN", "https://epifera.com");
  for (const path of ["/ru/app/music", "/ru/mcp", "/ru/ai/image-generator"]) {
    const result = proxy(new NextRequest("https://epifera.com" + path, { headers: { cookie: "epifera_release_view=dev", "X-Epifera-Release-View": "dev" } }));
    expect(result.status).toBe(404); expect(result.headers.get("x-middleware-rewrite")).toContain("/release-unavailable");
  }
  expect(proxy(new NextRequest("https://epifera.com/ru/legal/user-agreement")).status).toBe(200);
  expect(proxy(new NextRequest("https://epifera.com/ru/app/profile")).status).toBe(200);
});
it("keeps local DEV routes and restricts a confirmed PROD preference", () => {
  vi.stubEnv("NODE_ENV", "development"); vi.stubEnv("NEIROHUB_LOCAL_WORKSPACE_PREVIEW", "1");
  expect(proxy(new NextRequest("http://localhost:7159/ru/mcp")).status).toBe(200);
  expect(proxy(new NextRequest("http://localhost:7159/ru/mcp", { headers: { cookie: "epifera_release_view=prod" } })).status).toBe(404);
});
