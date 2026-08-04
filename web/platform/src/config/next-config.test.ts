import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

import nextConfig from "../../next.config";

describe("Next.js workspace configuration", () => {
  it("pins tracing and Turbopack to the platform package instead of a parent lockfile", () => {
    const platformRoot = resolve(".");

    expect(nextConfig.outputFileTracingRoot).toBe(platformRoot);
    expect(nextConfig.turbopack?.root).toBe(platformRoot);
  });
});
