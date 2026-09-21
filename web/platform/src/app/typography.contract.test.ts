import { readFileSync, readdirSync } from "node:fs";
import { join, relative, resolve } from "node:path";

import { describe, expect, it } from "vitest";

function stylesheet(path: string): string {
  return readFileSync(resolve(process.cwd(), path), "utf8");
}

function cssFiles(directory: string): string[] {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const path = join(directory, entry.name);

    if (entry.isDirectory()) {
      return cssFiles(path);
    }

    return entry.isFile() && entry.name.endsWith(".css") ? [path] : [];
  });
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
  it("uses shared typography tokens instead of local interface weights", () => {
    const sourceDirectory = resolve(process.cwd(), "src");
    const hardcodedWeights = cssFiles(sourceDirectory).flatMap((path) => {
      if (path.endsWith("globals.css")) {
        return [];
      }

      const matches = Array.from(
        stylesheet(relative(process.cwd(), path)).matchAll(/font-weight:\s*(500|600|650|700|750)\s*;/g),
      );

      return matches.map((match) => `${relative(process.cwd(), path)}: ${match[0]}`);
    });

    expect(hardcodedWeights).toEqual([]);
  });

  it("maps semantic bold text to the shared semibold weight", () => {
    const semanticBoldRule = rule(
      "src/app/globals.css",
      ":where(h1, h2, h3, h4, h5, h6, strong, b)",
    );

    expect(semanticBoldRule).toContain("font-weight: var(--font-weight-semibold)");
  });

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

  it("uses the subsection role for assistant-message H3 headings", () => {
    const headingRule = rule(
      "src/components/chat/AssistantMessageContent/AssistantMessageContent.module.css",
      ".content h3",
    );

    expect(headingRule).toContain("font-size: var(--font-size-subsection)");
    expect(headingRule).toContain("line-height: var(--line-height-subsection)");
    expect(headingRule).toContain("font-weight: var(--font-weight-semibold)");
    expect(headingRule).toContain("letter-spacing: var(--letter-spacing-subsection)");
  });

  it("uses the approved tracking across assistant-message heading levels", () => {
    const path = "src/components/chat/AssistantMessageContent/AssistantMessageContent.module.css";

    expect(rule(path, ".content h1")).toContain("letter-spacing: var(--letter-spacing-display)");
    expect(rule(path, ".content h2")).toContain("letter-spacing: var(--letter-spacing-section)");
    expect(rule(path, ".content h3")).toContain("letter-spacing: var(--letter-spacing-subsection)");
    expect(rule(path, ".content h4")).toContain("letter-spacing: var(--letter-spacing-interface)");
    expect(stylesheet(path)).not.toContain("letter-spacing: -0.015em");
  });
});

describe("workspace control typography", () => {
  it("uses the medium weight for shared input-control chips", () => {
    const chipRule = rule(
      "src/components/ui/InputControlChip/InputControlChip.module.css",
      ".chip",
    );

    expect(chipRule).toContain("font-weight: var(--font-weight-medium)");
    expect(chipRule).not.toContain("font-weight: 700");
  });

  it.each([
    [
      "src/components/layout/Sidebar/Sidebar.module.css",
      ".brand",
      "--font-size-supporting",
      "--line-height-body",
      "--font-weight-semibold",
    ],
    [
      "src/components/layout/Sidebar/Sidebar.module.css",
      ".navigationList a",
      "--font-size-navigation",
      "--line-height-navigation",
      "--font-weight-medium",
    ],
    [
      "src/components/layout/WorkspaceHeader/WorkspaceHeader.module.css",
      ".title",
      "--font-size-navigation",
      "--line-height-navigation",
      "--font-weight-medium",
    ],
    [
      "src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.module.css",
      ".trigger",
      "--font-size-navigation",
      "--line-height-navigation",
      "--font-weight-medium",
    ],
    [
      "src/components/chat/ChatTextInput/ChatTextInput.module.css",
      ".input",
      "--font-size-body",
      "--line-height-body",
      "--font-weight-regular",
    ],
  ])("maps %s %s to its semantic role", (path, selector, size, lineHeight, weight) => {
    const controlRule = rule(path, selector);

    expect(controlRule).toContain(`font-size: var(${size})`);
    expect(controlRule).toContain(`line-height: var(${lineHeight})`);
    expect(controlRule).toContain(`font-weight: var(${weight})`);
  });

  it.each([
    ["src/components/layout/WorkspaceHeader/WorkspaceHeader.module.css", ".balance"],
    ["src/features/conversations/ConversationRow/ConversationRow.module.css", ".link"],
    ["src/features/account/AccountMenu/AccountMenu.module.css", ".identity"],
  ])("uses the compact UI role in %s", (path, selector) => {
    const uiRule = rule(path, selector);

    expect(uiRule).toContain("font-size: var(--font-size-ui)");
    expect(uiRule).toContain("line-height: var(--line-height-ui)");
  });

  it.each([
    ["src/features/conversations/SidebarConversations/SidebarConversations.module.css", ".conversations h2"],
    ["src/components/chat/ChatComposer/ChatComposer.module.css", ".note"],
  ])("uses the service-label role in %s", (path, selector) => {
    const captionRule = rule(path, selector);

    expect(captionRule).toContain("font-size: var(--font-size-caption)");
    expect(captionRule).toContain("line-height: var(--line-height-caption)");
  });
});

describe("card and secondary surface typography", () => {
  it("keeps plan-benefit emphasis at the same medium weight as its line", () => {
    const path = "src/features/workspace/WorkspaceLanding/WorkspaceLanding.module.css";

    expect(rule(path, ".planBenefits li")).toContain("font-weight: var(--font-weight-medium)");
    expect(rule(path, ".planBenefits strong")).toContain("font-weight: inherit");
  });

  it.each([
    ["src/features/models/ModelCard/ModelCard.module.css", ".copy h3"],
    ["src/features/workspace/FeaturedModelShortcuts/FeaturedModelShortcuts.module.css", ".shortcut"],
    ["src/features/files/FileCard/FileCard.module.css", ".content h2"],
    ["src/components/public/ModelPreviewCard/ModelPreviewCard.module.css", ".copy h3"],
  ])("uses compact model and file names in %s", (path, selector) => {
    const nameRule = rule(path, selector);

    expect(nameRule).toContain("font-size: var(--font-size-ui)");
    expect(nameRule).toContain("line-height: var(--line-height-caption)");
    expect(nameRule).toContain("font-weight: var(--font-weight-semibold)");
  });

  it.each([
    ["src/features/files/FilesEmptyState/FilesEmptyState.module.css", ".emptyState h2"],
    ["src/features/image-generation/ImageTemplatePicker/ImageTemplatePicker.module.css", ".header h2"],
    ["src/components/public/EmptyState/EmptyState.module.css", ".root h2"],
  ])("uses the section role for standalone secondary headings in %s", (path, selector) => {
    const headingRule = rule(path, selector);

    expect(headingRule).toContain("font-size: var(--font-size-section)");
    expect(headingRule).toContain("line-height: var(--line-height-section)");
    expect(headingRule).toContain("font-weight: var(--font-weight-semibold)");
    expect(headingRule).toContain("letter-spacing: var(--letter-spacing-section)");
  });

  it.each([
    ["src/features/account/ProfileWorkspace/ProfileWorkspace.module.css", ".section h2"],
    ["src/features/account/ProfileLoginMethods/ProfileLoginMethods.module.css", ".section h2"],
    ["src/features/account/ProfileReferralFaq/ProfileReferralFaq.module.css", ".section h2"],
  ])("uses the supporting role for profile subsection headings in %s", (path, selector) => {
    const headingRule = rule(path, selector);

    expect(headingRule).toContain("font-size: var(--font-size-supporting)");
    expect(headingRule).toContain("line-height: var(--line-height-body)");
    expect(headingRule).toContain("font-weight: var(--font-weight-semibold)");
  });

  it("uses navigation, supporting, and body roles in image-generation guidance", () => {
    const path = "src/features/image-generation/ImageGenerationGuide/ImageGenerationGuide.module.css";

    expect(rule(path, ".tab")).toContain("font-size: var(--font-size-navigation)");
    expect(rule(path, ".tab")).toContain("line-height: var(--line-height-navigation)");
    expect(rule(path, ".step h3")).toContain("font-size: var(--font-size-supporting)");
    expect(rule(path, ".step h3")).toContain("line-height: var(--line-height-body)");
    expect(rule(path, ".step p")).toContain("font-size: var(--font-size-body)");
    expect(rule(path, ".step p")).toContain("line-height: var(--line-height-body)");
  });
});
