import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const programStylesheet = readFileSync(
  resolve(process.cwd(), "src/features/account/ProfileReferralProgram/ProfileReferralProgram.module.css"),
  "utf8",
);
const faqStylesheet = readFileSync(
  resolve(process.cwd(), "src/features/account/ProfileReferralFaq/ProfileReferralFaq.module.css"),
  "utf8",
);

describe("ProfileReferralProgram responsive styles", () => {
  it("uses a decorative blue accent treatment for the launch card", () => {
    expect(programStylesheet).toMatch(
      /\.launchCard\s*\{[^}]*position:\s*relative;[^}]*overflow:\s*hidden;[^}]*background:\s*radial-gradient\(/s,
    );
    const launchAccentRule = programStylesheet.match(/\.launchCard::before\s*\{([\s\S]*?)\n\}/)?.[1];

    expect(launchAccentRule).toContain('content: ""');
    expect(launchAccentRule).toContain("background: var(--color-accent)");
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

  it("keeps FAQ rows full-width with a distinct disclosure affordance", () => {
    expect(faqStylesheet).toMatch(
      /\.item\s*\{[^}]*inline-size:\s*100%;[^}]*background:\s*var\(--color-surface-raised\);/s,
    );
    expect(faqStylesheet).toMatch(
      /\.indicator\s*\{[^}]*border:\s*0\.0625rem solid var\(--color-border\);[^}]*border-radius:\s*999px;/s,
    );
  });
});
