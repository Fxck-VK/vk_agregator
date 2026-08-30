import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const stylesheet = readFileSync(
  resolve(process.cwd(), "src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.module.css"),
  "utf8",
);
const headerStylesheet = readFileSync(
  resolve(process.cwd(), "src/components/layout/WorkspaceHeader/WorkspaceHeader.module.css"),
  "utf8",
);

describe("WorkspaceModelSelector layout", () => {
  it("keeps the floating trigger compact beside the landing content", () => {
    const triggerRule = stylesheet.match(/\.trigger\s*\{[^}]*\}/s)?.[0] ?? "";
    const modelIconRule = stylesheet.match(/\.modelIcon\s*\{[^}]*\}/s)?.[0] ?? "";

    expect(triggerRule).toContain("gap: var(--space-1)");
    expect(triggerRule).toContain("max-inline-size: min(11.5rem, 42vw)");
    expect(triggerRule).toContain("padding: var(--space-2)");
    expect(modelIconRule).toContain("inline-size: 1.5rem");
    expect(modelIconRule).toContain("block-size: 1.5rem");
  });

  it("keeps search and footer fixed while only the model list scrolls", () => {
    expect(stylesheet).toMatch(
      /\.popover\s*\{[^}]*grid-template-rows:\s*auto minmax\(0,\s*1fr\) auto;[^}]*overflow:\s*hidden;/s,
    );
    expect(stylesheet).toMatch(/\.scrollArea\s*\{[^}]*min-block-size:\s*0;[^}]*overflow-y:\s*auto;/s);
  });

  it("constrains the popover to the mobile viewport", () => {
    expect(stylesheet).toMatch(
      /@media \(width < 48rem\) \{[\s\S]*?\.popover\s*\{[^}]*inline-size:\s*calc\(100vw - 5\.75rem\);[^}]*max-block-size:\s*calc\(100dvh - 5\.5rem\);/,
    );
  });

  it("uses the workspace header container width at the tablet breakpoint", () => {
    expect(headerStylesheet).toMatch(/\.header\s*\{[^}]*container-type:\s*inline-size;/s);
    expect(stylesheet).toMatch(
      /\.popover\s*\{[^}]*inline-size:\s*min\(32rem,\s*calc\(100cqi - 3rem\)\);/s,
    );
  });

  it("keeps a visible keyboard focus indicator on the search", () => {
    expect(stylesheet).toMatch(
      /\.search:focus-visible\s*\{[^}]*outline:\s*0\.125rem solid var\(--color-focus\);/s,
    );
  });
});
