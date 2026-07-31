import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("server-only", () => ({}));

import { getWebApiInternalOrigin } from "./internal-origin";

describe("getWebApiInternalOrigin", () => {
  afterEach(() => {
    vi.unstubAllEnvs();
  });

  it("returns a plain HTTP(S) origin without exposing the configured value", () => {
    vi.stubEnv("WEB_API_INTERNAL_ORIGIN", "https://backend.internal:8443/");

    expect(getWebApiInternalOrigin()).toBe("https://backend.internal:8443");
  });

  it.each([
    "http://backend.internal:8080/web/v1",
    "http://backend.internal:8080?token=secret",
    "http://backend.internal:8080#fragment",
    "ftp://backend.internal:8080",
    "https://user:password@backend.internal",
  ])("rejects a non-origin internal API value", (value) => {
    vi.stubEnv("WEB_API_INTERNAL_ORIGIN", value);

    expect(() => getWebApiInternalOrigin()).toThrow("must be a plain HTTP(S) origin");
  });
});
