import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const readWorkspaceFile = (path: string) => readFileSync(resolve(process.cwd(), path), "utf8");
const frameStylesPath = resolve(
  process.cwd(),
  "src/components/layout/WorkspacePageFrame/WorkspacePageFrame.module.css",
);
const frameStyles = readWorkspaceFile(
  "src/components/layout/WorkspacePageFrame/WorkspacePageFrame.module.css",
);
const filesStyles = readWorkspaceFile(
  "src/features/files/FilesWorkspace/FilesWorkspace.module.css",
);
const masonryGridStyles = readWorkspaceFile(
  "src/components/ui/MasonryGrid/MasonryGrid.module.css",
);
const filesSource = readWorkspaceFile(
  "src/features/files/FilesWorkspace/FilesWorkspace.tsx",
);
const modelsStyles = readWorkspaceFile(
  "src/features/models/ModelsCatalog/ModelsCatalog.module.css",
);
const inspirationStyles = readWorkspaceFile(
  "src/features/inspiration/InspirationGallery/InspirationGallery.module.css",
);

const globals = readWorkspaceFile("src/app/globals.css");
const landingStyles = readWorkspaceFile(
  "src/features/workspace/WorkspaceLanding/WorkspaceLanding.module.css",
);
const standalonePageStyles = [
  [
    readWorkspaceFile("src/features/workspace/WorkspaceHome/WorkspaceHome.module.css"),
    "content",
  ],
  [
    readWorkspaceFile("src/features/image-generation/ImageWorkspace/ImageWorkspace.module.css"),
    "workspace",
  ],
  [
    readWorkspaceFile("src/features/account/ProfileWorkspace/ProfileWorkspace.module.css"),
    "workspace",
  ],
] as const;
const conversationStyles = readWorkspaceFile(
  "src/features/conversations/ConversationHistory/ConversationHistory.module.css",
);
const pageSources = [
  filesSource,
  readWorkspaceFile("src/features/models/ModelsCatalog/ModelsCatalog.tsx"),
  readWorkspaceFile("src/features/inspiration/InspirationGallery/InspirationGallery.tsx"),
];

describe("WorkspacePageFrame styles", () => {
  it("defines one approved workspace content width and responsive side gutter", () => {
    expect(globals).toContain("--workspace-page-shell-width: 66rem");
    expect(globals).toContain("--workspace-content-frame-width: 100%");
    expect(globals).toContain(
      "--workspace-page-inline-gutter: clamp(var(--space-4), 4.5vw, 3.5rem)",
    );
    expect(globals).toMatch(
      /@media \(width < 48rem\) \{[\s\S]*?:root \{[\s\S]*?--workspace-page-inline-gutter:\s*var\(--space-4\)/,
    );
  });

  it("lets page content fill the inner width of the shared 66rem shell", () => {
    expect(existsSync(frameStylesPath)).toBe(true);
    if (!existsSync(frameStylesPath)) {
      return;
    }

    const frameStyles = readFileSync(frameStylesPath, "utf8");
    expect(frameStyles).toMatch(
      /\.frame\s*\{[^}]*inline-size:\s*min\(100%,\s*var\(--workspace-page-shell-width\)\);/s,
    );
    expect(frameStyles).toMatch(/\.frame\s*\{[^}]*margin-inline:\s*auto;/s);
    expect(frameStyles).toMatch(
      /\.frame\s*\{[^}]*padding-inline:\s*var\(--workspace-page-inline-gutter\);/s,
    );
    expect(frameStyles).toMatch(
      /\.content\s*\{[^}]*inline-size:\s*min\(100%,\s*var\(--workspace-content-frame-width\)\);[^}]*margin-inline:\s*auto;/s,
    );
    expect(frameStyles).toMatch(
      /@media \(48rem <= width < 82rem\) \{[\s\S]*?\.frame\s*\{[^}]*padding-inline-end:\s*var\(--space-4\);[^}]*\}[\s\S]*?\.content\s*\{[^}]*margin-inline-end:\s*0;/,
    );
  });

  it("is used by files, models, and inspiration pages", () => {
    for (const source of pageSources) {
      expect(source).toContain(
        'import { WorkspacePageFrame } from "@/components/layout/WorkspacePageFrame/WorkspacePageFrame";',
      );
      expect(source).toContain("<WorkspacePageFrame");
    }
  });

  it("lets masonry file cards fill the shared content width without a page-only override", () => {
    expect(filesSource).toContain("<WorkspacePageFrame>");
    expect(filesStyles).not.toContain("--workspace-content-frame-width");
    expect(frameStyles).toMatch(
      /\.frame\s*\{[^}]*padding-inline:\s*var\(--workspace-page-inline-gutter\);/s,
    );

    const masonryRule = masonryGridStyles.match(/\.grid\s*\{([^}]*)\}/s)?.[1] ?? "";
    const masonryItemRule = masonryGridStyles.match(/\.grid\s*>\s*li\s*\{([^}]*)\}/s)?.[1] ?? "";

    expect(masonryRule).toContain("column-width: 17rem");
    expect(masonryRule).toContain("column-gap: var(--space-4)");
    expect(masonryRule).not.toContain("grid-template-columns");
    expect(masonryItemRule).toContain("inline-size: 100%");
    expect(masonryItemRule).toContain("margin-block-end: var(--space-4)");
    expect(masonryItemRule).toContain("break-inside: avoid");
  });

  it("owns one 80px top gap while each page keeps only its own bottom spacing", () => {
    expect(frameStyles).toMatch(
      /\.frame\s*\{[^}]*padding-block-start:\s*5rem;/s,
    );
    expect(filesStyles).toMatch(
      /\.workspace\s*\{[^}]*padding-block-end:\s*clamp\(var\(--space-6\),\s*5vw,\s*4rem\);/s,
    );
    expect(modelsStyles).toMatch(
      /\.catalog\s*\{[^}]*padding-block-end:\s*var\(--space-8\);/s,
    );
    expect(inspirationStyles).toMatch(/\.gallery\s*\{[^}]*padding-block-end:\s*5rem;/s);

    for (const stylesheet of [filesStyles, modelsStyles, inspirationStyles]) {
      expect(stylesheet).not.toMatch(/padding-block-start:/);
      expect(stylesheet).not.toMatch(/padding-block:(?!-)/);
    }
  });

  it("keeps the landing page on the same shared content boundaries", () => {
    expect(landingStyles).toMatch(
      /\.main\s*\{[^}]*inline-size:\s*min\(100%,\s*var\(--workspace-page-shell-width\)\);/s,
    );
    expect(landingStyles).toMatch(
      /\.main\s*\{[^}]*padding:\s*0 var\(--workspace-page-inline-gutter\) var\(--space-8\);/s,
    );
    expect(landingStyles).toMatch(
      /\.contentFrame\s*\{[^}]*inline-size:\s*min\(100%,\s*var\(--workspace-content-frame-width\)\);/s,
    );
  });

  it("aligns standalone workspace sections to the same shell and side gutters", () => {
    for (const [stylesheet, className] of standalonePageStyles) {
      expect(stylesheet).toMatch(
        new RegExp(`\\.${className}\\s*\\{[^}]*inline-size:\\s*min\\(100%,\\s*var\\(--workspace-page-shell-width\\)\\);`, "s"),
      );
      expect(stylesheet).toMatch(
        new RegExp(`\\.${className}\\s*\\{[^}]*margin-inline:\\s*auto;`, "s"),
      );
      expect(stylesheet).toMatch(
        new RegExp(`\\.${className}\\s*\\{[^}]*padding-inline:\\s*var\\(--workspace-page-inline-gutter\\);`, "s"),
      );
    }

    expect(conversationStyles).toMatch(
      /\.(?:history|state)\s*\{[^}]*inline-size:\s*min\(100%,\s*var\(--workspace-page-shell-width\)\);/s,
    );
    expect(conversationStyles).toMatch(
      /\.(?:history|state)\s*\{[^}]*padding-inline:\s*var\(--workspace-page-inline-gutter\);/s,
    );
  });
});
