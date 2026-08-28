import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const lightThemePlaceholder = readFileSync(
  resolve(process.cwd(), "public/assets/images/models/chip-silhouette-dark.svg"),
  "utf8",
);

describe("light-theme model placeholder asset", () => {
  it("preserves the supplied masked chip geometry and color", () => {
    expect(lightThemePlaceholder).toContain('<mask id="face-cutout">');
    expect(lightThemePlaceholder).toContain('<g fill="#15161C">');
    expect(lightThemePlaceholder).toMatch(
      /x="180"\s+y="180"\s+width="664"\s+height="664"\s+rx="154"/s,
    );
    expect(lightThemePlaceholder).toMatch(
      /M405 585\s+C435 625 473 645 512 645\s+C551 645 589 625 619 585/s,
    );
    expect(lightThemePlaceholder).toContain('stroke-width="46"');
    expect(lightThemePlaceholder).not.toContain("#0C0C0F");
  });
});
