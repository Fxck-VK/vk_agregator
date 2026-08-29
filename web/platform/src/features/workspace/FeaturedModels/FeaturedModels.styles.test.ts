import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const stylesheet = readFileSync(
  resolve(process.cwd(), "src/features/workspace/FeaturedModels/FeaturedModels.module.css"),
  "utf8",
);

describe("FeaturedModels card geometry", () => {
  it("centers a narrower grid and keeps the cards compact", () => {
    expect(stylesheet).toMatch(
      /\.grid\s*\{[^}]*inline-size:\s*min\(100%,\s*58rem\)[^}]*margin-inline:\s*auto/s,
    );
    expect(stylesheet).toMatch(
      /\.card\s*\{[^}]*grid-template-rows:\s*auto\s+1fr[^}]*gap:\s*var\(--space-3\)[^}]*min-block-size:\s*11\.5rem/s,
    );
  });
});
