import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

function stylesheet(path: string): string {
  return readFileSync(resolve(process.cwd(), path), "utf8");
}

function escapeRegExp(value: string): string {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function rule(path: string, selector: string): string {
  return Array.from(
    stylesheet(path).matchAll(new RegExp(`${escapeRegExp(selector)}\\s*\\{[^}]*\\}`, "gs")),
    ([matchingRule]) => matchingRule,
  ).join("\n");
}

describe("primary interface typography", () => {
  it.each([
    ["src/features/workspace/WorkspaceLanding/WorkspaceLanding.module.css", ".heroCopy h1"],
    ["src/features/workspace/WorkspaceHome/WorkspaceHome.module.css", ".content h1"],
    ["src/features/models/ModelsCatalog/ModelsCatalog.module.css", ".header h1"],
    ["src/features/files/FilesWorkspace/FilesWorkspace.module.css", ".header h1"],
    ["src/features/inspiration/InspirationGallery/InspirationGallery.module.css", ".heading h1"],
    ["src/components/public/SectionHeading/SectionHeading.module.css", ".copy > h1.title"],
  ])("uses the display role in %s", (path, selector) => {
    const headingRule = rule(path, selector);

    expect(headingRule).toContain("font-size: var(--font-size-display)");
    expect(headingRule).toContain("line-height: var(--line-height-display)");
    expect(headingRule).toContain("font-weight: var(--font-weight-semibold)");
    expect(headingRule).toContain("letter-spacing: var(--letter-spacing-display)");
  });

  it("uses the section role for workspace, catalogue, and public section headings", () => {
    const workspace = stylesheet("src/features/workspace/WorkspaceLanding/WorkspaceLanding.module.css");

    expect(workspace).toContain("font-size: var(--font-size-section)");
    expect(workspace).toContain("line-height: var(--line-height-section)");
    expect(workspace).toContain("letter-spacing: var(--letter-spacing-section)");

    for (const [path, selector] of [
      ["src/features/models/ModelsCatalog/ModelsCatalog.module.css", ".sectionTitle"],
      ["src/components/public/SectionHeading/SectionHeading.module.css", ".title"],
    ]) {
      const headingRule = rule(path, selector);
      expect(headingRule).toContain("font-size: var(--font-size-section)");
      expect(headingRule).toContain("line-height: var(--line-height-section)");
      expect(headingRule).toContain("font-weight: var(--font-weight-semibold)");
      expect(headingRule).toContain("letter-spacing: var(--letter-spacing-section)");
    }
  });

  it.each([
    ["src/features/workspace/WorkspaceLanding/WorkspaceLanding.module.css", ".heroCopy p"],
    ["src/features/workspace/WorkspaceHome/WorkspaceHome.module.css", ".description"],
    ["src/features/models/ModelsCatalog/ModelsCatalog.module.css", ".header p"],
    ["src/features/inspiration/InspirationGallery/InspirationGallery.module.css", ".heading > p:last-child"],
    ["src/components/public/SectionHeading/SectionHeading.module.css", ".description"],
  ])("uses the supporting role in %s", (path, selector) => {
    const supportingRule = rule(path, selector);

    expect(supportingRule).toContain("font-size: var(--font-size-supporting)");
    expect(supportingRule).toContain("line-height: var(--line-height-supporting)");
    expect(supportingRule).toContain("font-weight: var(--font-weight-regular)");
  });
});
