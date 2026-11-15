import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const programStylesheet = readFileSync(
  resolve(process.cwd(), "src/features/account/ProfileReferralProgram/ProfileReferralProgram.module.css"),
  "utf8",
);

describe("ProfileReferralProgram responsive styles", () => {
  it("uses the shared panel background for the launch card", () => {
    expect(programStylesheet).toMatch(
      /\.launchCard,\s*\.statistic,\s*\.steps li\s*\{[^}]*background:\s*var\(--color-panel\);/s,
    );
    expect(programStylesheet).not.toContain("radial-gradient");
    expect(programStylesheet).not.toContain(".launchCard::before");
  });

  it("keeps explainer and statistic cards in desktop grids that stack on narrow screens", () => {
    expect(programStylesheet).toMatch(
      /\.steps\s*\{[^}]*grid-template-columns:\s*repeat\(3,\s*minmax\(0,\s*1fr\)\);/s,
    );
    expect(programStylesheet).toMatch(
      /\.statistics\s*\{[^}]*grid-template-columns:\s*repeat\(2,\s*minmax\(0,\s*1fr\)\);/s,
    );
    expect(programStylesheet).toMatch(
      /@media \(width < 42rem\)\s*\{[\s\S]*?\.steps,[\s\S]*?\.statistics\s*\{[\s\S]*?grid-template-columns:\s*1fr;/s,
    );
  });

});
