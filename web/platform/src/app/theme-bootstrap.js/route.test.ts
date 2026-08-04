import { describe, expect, it } from "vitest";

import { GET, dynamic } from "./route";

describe("theme bootstrap route", () => {
  it("serves the shared theme bootstrap as a cacheable static script", async () => {
    const response = GET();

    expect(dynamic).toBe("force-static");
    expect(response.headers.get("Content-Type")).toContain("text/javascript");
    expect(response.headers.get("Cache-Control")).toContain("stale-while-revalidate");
    expect(await response.text()).toContain("neirohub.theme");
  });
});
