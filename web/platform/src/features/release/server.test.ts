import { afterEach, describe, expect, it, vi } from "vitest";
vi.mock("server-only", () => ({}));
vi.mock("next/headers", () => ({ headers: vi.fn() }));
vi.mock("@/lib/dev-access/session", () => ({ getDevAccessConfig: vi.fn(), hasDevAccess: vi.fn() }));
import { getDevAccessConfig, hasDevAccess } from "@/lib/dev-access/session";
import { releaseStateForRequest } from "./server";

afterEach(() => { vi.unstubAllEnvs(); vi.resetAllMocks(); });
describe("release view server authority", () => {
  it("ignores cookies, headers and query overrides on production", () => {
    vi.stubEnv("NODE_ENV", "production"); vi.stubEnv("WEB_ORIGIN", "https://epifera.com");
    expect(releaseStateForRequest(new Headers({ cookie: "epifera_release_view=dev", "X-Epifera-Release-View": "dev" }))).toEqual({ view: "prod", canSwitch: false, localSimulation: false });
  });
  it("allows local preview preference without changing environment", () => {
    vi.stubEnv("NODE_ENV", "development"); vi.stubEnv("NEIROHUB_LOCAL_WORKSPACE_PREVIEW", "1");
    expect(releaseStateForRequest(new Headers()).view).toBe("dev");
    expect(releaseStateForRequest(new Headers({ cookie: "epifera_release_view=prod" }))).toEqual({ view: "prod", canSwitch: true, localSimulation: true });
  });
  it("requires the existing DEV gate and fails closed for invalid preference", () => {
    vi.stubEnv("NODE_ENV", "production"); vi.stubEnv("WEB_ORIGIN", "https://dev-web.epifera.com");
    vi.mocked(getDevAccessConfig).mockReturnValue({} as ReturnType<typeof getDevAccessConfig>);
    vi.mocked(hasDevAccess).mockReturnValue(false);
    expect(releaseStateForRequest(new Headers()).canSwitch).toBe(false);
    vi.mocked(hasDevAccess).mockReturnValue(true);
    expect(releaseStateForRequest(new Headers()).view).toBe("dev");
    expect(releaseStateForRequest(new Headers({ cookie: "epifera_release_view=garbage" })).view).toBe("prod");
    expect(releaseStateForRequest(new Headers({ cookie: "epifera_release_view=prod; epifera_release_view=dev" })).view).toBe("prod");
  });
});
