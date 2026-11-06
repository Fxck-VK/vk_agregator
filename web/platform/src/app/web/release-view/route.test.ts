import { afterEach, describe, expect, it, vi } from "vitest";
vi.mock("server-only", () => ({}));
vi.mock("@/features/release/server", () => ({ releaseStateForRequest: vi.fn() }));
import { releaseStateForRequest } from "@/features/release/server";
import { GET, POST } from "./route";
afterEach(() => vi.resetAllMocks());
const state = { view: "dev" as const, canSwitch: true, localSimulation: true };
function request(body: string, origin = "http://localhost:7159") { return new Request("http://localhost:7159/web/release-view", { method: "POST", headers: { Origin: origin, "Content-Type": "application/json" }, body }); }
describe("release preference only", () => {
  it("is absent on production and unauthorized DEV", async () => {
    vi.mocked(releaseStateForRequest).mockReturnValue({ view: "prod", canSwitch: false, localSimulation: false });
    expect((await GET(new Request("https://epifera.com/web/release-view"))).status).toBe(404);
    const result = await POST(request('{"view":"dev"}'));
    expect(result.status).toBe(404); expect(result.headers.has("set-cookie")).toBe(false);
  });
  it("sets only an HttpOnly preference and keeps all account/payment cookies untouched", async () => {
    vi.mocked(releaseStateForRequest).mockReturnValue(state);
    const result = await POST(request('{"view":"prod"}'));
    expect(result.status).toBe(200);
    expect(result.headers.get("set-cookie")).toMatch(/^epifera_release_view=prod;/);
    expect(result.headers.get("set-cookie")).toContain("HttpOnly");
    expect(await result.json()).toMatchObject({ view: "prod", canSwitch: true, localSimulation: true });
    expect(result.headers.get("cache-control")).toContain("no-store");
  });
  it.each(['{"view":"invalid"}', '{"view":"dev","account_id":"fake"}', 'x'.repeat(2050)])("rejects invalid or oversized bodies", async body => {
    vi.mocked(releaseStateForRequest).mockReturnValue(state);
    const result = await POST(request(body));
    expect([400, 413]).toContain(result.status); expect(result.headers.has("set-cookie")).toBe(false);
  });
  it("rejects cross-origin requests", async () => {
    vi.mocked(releaseStateForRequest).mockReturnValue(state);
    expect((await POST(request('{"view":"dev"}', "https://attacker.invalid"))).status).toBe(403);
  });
});
