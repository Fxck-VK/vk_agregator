import { describe, expect, it } from "vitest";

import nextConfig from "../../next.config";

describe("Next.js workspace boundary", () => {
  it("keeps Turbopack inside the platform package", () => {
    expect(nextConfig.turbopack?.root).toBe(process.cwd());
  });
});
