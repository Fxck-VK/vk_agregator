import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const stylesheet = readFileSync(
  resolve(process.cwd(), "src/features/workspace/WorkspaceLanding/WorkspaceLanding.module.css"),
  "utf8",
);

const componentSource = readFileSync(
  resolve(process.cwd(), "src/features/workspace/WorkspaceLanding/WorkspaceLanding.tsx"),
  "utf8",
);

const featuredModelsStylesheet = readFileSync(
  resolve(process.cwd(), "src/features/workspace/FeaturedModels/FeaturedModels.module.css"),
  "utf8",
);

const featuredModelsSource = readFileSync(
  resolve(process.cwd(), "src/features/workspace/FeaturedModels/FeaturedModels.tsx"),
  "utf8",
);

describe("WorkspaceLanding background", () => {
  it("uses a flat workspace background without an accent glow", () => {
    const pageRule = stylesheet.match(/\.page\s*\{[^}]*\}/s)?.[0] ?? "";

    expect(pageRule).toContain("background: var(--color-background)");
    expect(pageRule).not.toContain("radial-gradient");
  });
});

describe("WorkspaceLanding hero", () => {
  it("aligns every home section and the footer to one centered content frame", () => {
    const contentFrameRule = stylesheet.match(/\.contentFrame\s*\{[^}]*\}/s)?.[0] ?? "";
    const footerInnerRule = stylesheet.match(/\.footerInner\s*\{[^}]*\}/s)?.[0] ?? "";

    expect(contentFrameRule).toContain("inline-size: min(100%, 46rem)");
    expect(contentFrameRule).toContain("margin-inline: auto");
    expect(componentSource.match(/styles\.contentFrame/g)).toHaveLength(8);
    expect(footerInnerRule).toContain("inline-size: min(100%, 46rem)");
    expect(stylesheet).toMatch(
      /@media \(48rem <= width < 82rem\)\s*\{[\s\S]*?\.main\s*\{[^}]*padding-inline-end:\s*var\(--space-4\);[\s\S]*?\.contentFrame\s*\{[^}]*margin-inline-end:\s*0;/,
    );
  });

  it("keeps the desktop heading compact and on one line", () => {
    const headingRule = stylesheet.match(/\.heroCopy h1\s*\{[^}]*\}/s)?.[0] ?? "";

    expect(headingRule).toContain("font-size: var(--font-size-display)");
    expect(headingRule).toContain("white-space: nowrap");
  });

  it("uses a compact desktop hero and restores heading wrapping on narrow screens", () => {
    const heroRule = stylesheet.match(/\.hero\s*\{[^}]*\}/s)?.[0] ?? "";

    expect(heroRule).toContain("gap: clamp(var(--space-4), 2vw, var(--space-6))");
    expect(heroRule).toContain("min-block-size: min(40rem, calc(100dvh - 4.5rem))");
    expect(stylesheet).toMatch(
      /@media \(width < 48rem\)[\s\S]*\.heroCopy h1\s*\{[^}]*white-space:\s*normal;/s,
    );
  });

  it("keeps the all-models arrow transparent until hover or keyboard focus", () => {
    expect(stylesheet).toMatch(
      /\.arrowIcon\s*\{[^}]*border-color:\s*transparent;[^}]*background:\s*transparent;[^}]*color:\s*var\(--color-text\);/s,
    );
    expect(stylesheet).toMatch(
      /\.allToolsShortcut:hover \.arrowIcon,\s*\.allToolsShortcut:focus-visible \.arrowIcon\s*\{[^}]*border-color:\s*var\(--color-accent\);/s,
    );
  });

  it("pins the supplied 90+ artwork above the all-models arrow", () => {
    const arrowRule = stylesheet.match(/(?:^|\n)\.arrowIcon\s*\{[^}]*\}/s)?.[0] ?? "";
    const badgeRule = stylesheet.match(/\.modelCountBadge\s*\{[^}]*\}/s)?.[0] ?? "";

    expect(componentSource).toContain("assetPaths.images.workspace.allModelsBadge");
    expect(componentSource).toMatch(
      /<Image[^>]*alt=""[^>]*className=\{styles\.modelCountBadge\}[^>]*src=\{assetPaths\.images\.workspace\.allModelsBadge\}/s,
    );
    expect(arrowRule).toContain("position: relative");
    expect(badgeRule).toContain("position: absolute");
    expect(badgeRule).toContain("inset-block-start: -1.1rem");
    expect(badgeRule).toMatch(/inset-inline-start:\s*-/);
    expect(badgeRule).toContain("pointer-events: none");
  });

  it("uses the supplied artwork for the centered popular-model actions", () => {
    const actionRule = featuredModelsStylesheet.match(/\.catalogAction\s*\{[^}]*\}/s)?.[0] ?? "";
    const backgroundRule = featuredModelsStylesheet.match(/\.catalogActionBackground\s*\{[^}]*\}/s)?.[0] ?? "";

    expect(componentSource).not.toContain("styles.primaryButton");
    expect(featuredModelsSource).toContain("assetPaths.images.workspace.allModelsButtonBackground");
    expect(featuredModelsSource).toMatch(
      /<Image[^>]*alt=""[^>]*className=\{styles\.catalogActionBackground\}[^>]*fill[^>]*sizes="12rem"[^>]*src=\{assetPaths\.images\.workspace\.allModelsButtonBackground\}/s,
    );
    expect(featuredModelsSource).toContain('<CatalogActionContent label="Показать ещё" />');
    expect(featuredModelsSource).toContain('<CatalogActionContent label="Все нейросети" />');
    expect(actionRule).toContain("position: relative");
    expect(actionRule).toContain("background: transparent");
    expect(backgroundRule).toContain("object-fit: fill");
    expect(backgroundRule).toContain("pointer-events: none");
    expect(featuredModelsStylesheet).toMatch(
      /\.catalogAction:hover \.catalogActionBackground\s*\{[^}]*filter:\s*brightness\(/s,
    );
  });
});
