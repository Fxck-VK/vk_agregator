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

    expect(pageRule).toContain("background: var(--color-workspace)");
    expect(pageRule).not.toContain("radial-gradient");
  });
});

describe("WorkspaceLanding hero", () => {
  it("keeps home sections narrow while giving the footer a wider centered frame", () => {
    const contentFrameRule = stylesheet.match(/\.contentFrame\s*\{[^}]*\}/s)?.[0] ?? "";
    const footerInnerRule = stylesheet.match(/\.footerInner\s*\{[^}]*\}/s)?.[0] ?? "";

    expect(contentFrameRule).toContain(
      "inline-size: min(100%, var(--workspace-content-frame-width))",
    );
    expect(contentFrameRule).toContain("margin-inline: auto");
    expect(componentSource.match(/styles\.contentFrame/g)).toHaveLength(8);
    expect(footerInnerRule).toContain("inline-size: min(100%, 76rem)");
    expect(stylesheet).toMatch(
      /@media \(48rem <= width < 82rem\)\s*\{[\s\S]*?\.main\s*\{[^}]*padding-inline-end:\s*var\(--space-4\);[\s\S]*?\.contentFrame\s*\{[^}]*margin-inline-end:\s*0;/,
    );
  });

  it("keeps the desktop heading compact and on one line", () => {
    const headingRule = stylesheet.match(/\.heroCopy h1\s*\{[^}]*\}/s)?.[0] ?? "";

    expect(headingRule).toContain("font-size: var(--font-size-display)");
    expect(headingRule).toContain("white-space: nowrap");
  });

  it("fills the available viewport and restores heading wrapping on narrow screens", () => {
    const heroRule = stylesheet.match(/\.hero\s*\{[^}]*\}/s)?.[0] ?? "";

    expect(heroRule).toContain("gap: clamp(var(--space-4), 2vw, var(--space-6))");
    expect(heroRule).toContain(
      "min-block-size: calc(100dvh - var(--header-height))",
    );
    expect(stylesheet).toMatch(
      /@media \(width < 48rem\)[\s\S]*?\.hero\s*\{[^}]*min-block-size:\s*calc\(100dvh - var\(--header-height-mobile\)\);[\s\S]*?\.heroCopy h1\s*\{[^}]*white-space:\s*normal;/s,
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
    expect(featuredModelsSource).toContain('<CatalogActionContent label={msg("featuredModels.showMore")} />');
    expect(featuredModelsSource).toContain('<CatalogActionContent label={msg("featuredModels.allAiModels")} />');
    expect(actionRule).toContain("position: relative");
    expect(actionRule).toContain("background: transparent");
    expect(actionRule).toContain("min-block-size: 3.25rem");
    expect(actionRule).toContain("padding: var(--space-3) var(--space-8)");
    expect(backgroundRule).toContain("object-fit: fill");
    expect(backgroundRule).toContain("pointer-events: none");
    expect(featuredModelsStylesheet).toMatch(
      /\.catalogActionLabel\s*\{[^}]*translate:\s*0 -0\.1rem;/s,
    );
    expect(featuredModelsStylesheet).toMatch(
      /\.catalogAction:hover \.catalogActionBackground\s*\{[^}]*filter:\s*brightness\(/s,
    );
  });

  it("reveals the additional model cards and catalogue action with reduced-motion support", () => {
    expect(featuredModelsSource).toContain('revealed={isNewlyRevealed}');
    expect(featuredModelsSource).toContain('data-revealed={expanded || undefined}');
    expect(featuredModelsStylesheet).toMatch(
      /\.revealedCard\s*\{[^}]*animation:\s*featured-model-card-reveal[^;]*both;/s,
    );
    expect(featuredModelsStylesheet).toMatch(
      /\.revealedCard:nth-child\(6\)\s*\{[^}]*animation-delay:/s,
    );
    expect(featuredModelsStylesheet).toMatch(
      /\.revealedCatalogAction\s*\{[^}]*animation:\s*featured-model-action-reveal[^;]*both;/s,
    );
    expect(featuredModelsStylesheet).toMatch(
      /@media \(prefers-reduced-motion: reduce\)[\s\S]*\.revealedCard,[\s\S]*\.revealedCatalogAction\s*\{[^}]*animation:\s*none;/s,
    );
  });

  it("keeps the workspace prompt card narrow on desktop and fluid below 36rem", () => {
    const promptExampleRule = stylesheet.match(/\.promptExample\s*\{[^}]*\}/s)?.[0] ?? "";

    expect(promptExampleRule).toContain("inline-size: min(100%, 20rem)");
    expect(stylesheet).toMatch(
      /@media \(width < 36rem\)[\s\S]*\.promptExample\s*\{[^}]*inline-size:\s*100%;/s,
    );
    expect(componentSource).toContain("<InspirationExampleCard");
    expect(componentSource).not.toContain('sizes="(max-width: 36rem) 100vw, 20rem"');
    expect(stylesheet).not.toContain(".promptFeature");
    expect(stylesheet).not.toContain(".promptImage");
  });

  it("lays out capability previews as one tall tile beside two stacked tiles with captions below", () => {
    const mosaicRule = stylesheet.match(/\.capabilityMosaic\s*\{[^}]*\}/s)?.[0] ?? "";
    const cardRule = stylesheet.match(/\.capabilityCard\s*\{[^}]*\}/s)?.[0] ?? "";
    const largeRule = stylesheet.match(/\.capabilityLarge\s*\{[^}]*\}/s)?.[0] ?? "";
    const visualRule = stylesheet.match(/\.capabilityVisual\s*\{[^}]*\}/s)?.[0] ?? "";

    expect(componentSource.match(/styles\.capabilityVisual/g)).toHaveLength(3);
    expect(componentSource.match(/styles\.capabilityCopy/g)).toHaveLength(3);
    expect(mosaicRule).toContain("grid-template-columns: repeat(2, minmax(0, 1fr))");
    expect(largeRule).toContain("grid-row: 1 / 3");
    expect(cardRule).toContain("background: transparent");
    expect(cardRule).toContain("border: 0");
    expect(visualRule).toContain("overflow: hidden");
    expect(visualRule).toContain("border-radius: 1.25rem");
  });

  it("styles the Lite tariff like the approved split card in NeiroHub colors", () => {
    const cardRule = stylesheet.match(/\.planCard\s*\{[^}]*\}/s)?.[0] ?? "";
    const offerRule = stylesheet.match(/\.planOffer\s*\{[^}]*\}/s)?.[0] ?? "";
    const priceRule = stylesheet.match(/\.planPrice\s*\{[^}]*\}/s)?.[0] ?? "";

    expect(cardRule).toContain("grid-template-columns: minmax(18rem, 0.82fr) 1.38fr");
    expect(cardRule).toContain("background: var(--color-panel)");
    expect(componentSource).toContain("assetPaths.images.workspace.litePlanBackground");
    expect(offerRule).toContain("background-size: cover");
    expect(priceRule).toContain("font-size: clamp(2.75rem, 6vw, 4rem)");
    expect(componentSource).toContain("styles.planBalance");
    expect(componentSource).toContain('<CreditAmount className={styles.planBalance} value={400} />');
    expect(componentSource.match(/styles\.planBenefitIcon/g)).toHaveLength(3);
  });

  it("builds the footer on the shared workspace canvas with three navigation levels", () => {
    const footerRule = stylesheet.match(/\.footer\s*\{[^}]*\}/s)?.[0] ?? "";
    const topRule = stylesheet.match(/\.footerTop\s*\{[^}]*\}/s)?.[0] ?? "";
    const columnsRule = stylesheet.match(/\.footerColumns\s*\{[^}]*\}/s)?.[0] ?? "";
    const metaRule = stylesheet.match(/\.footerMeta\s*\{[^}]*\}/s)?.[0] ?? "";

    expect(footerRule).toContain("background: var(--color-workspace)");
    expect(footerRule).not.toContain("background: var(--color-surface)");
    expect(topRule).toContain("grid-template-columns: 1fr auto 1fr");
    expect(columnsRule).toContain("grid-template-columns: repeat(3, minmax(0, 1fr))");
    expect(metaRule).toContain("border-block-start: 0.0625rem solid var(--color-border)");
    expect(componentSource).toContain("styles.footerTop");
    expect(componentSource.match(/styles\.footerColumn\}/g)).toHaveLength(3);
    expect(componentSource).toContain("styles.footerMeta");
    expect(componentSource).toContain("https://vk.me/neirohub_help");
    expect(componentSource).toContain('msg("workspaceLanding.2026NeirohubAllRightsReserved")');
  });

  it("keeps the gap between the final section and footer compact", () => {
    const mainRule = stylesheet.match(/\.main\s*\{[^}]*\}/s)?.[0] ?? "";
    const communitySectionRule = stylesheet.match(/\.communitySection\s*\{[^}]*\}/s)?.[0] ?? "";
    const footerInnerRule = stylesheet.match(/\.footerInner\s*\{[^}]*\}/s)?.[0] ?? "";

    expect(communitySectionRule).toContain("padding-block-end: 0");
    expect(mainRule).toContain(
      "padding: 0 var(--workspace-page-inline-gutter) var(--space-8)",
    );
    expect(footerInnerRule).toContain(
      "padding: var(--space-8) clamp(var(--space-4), 4.5vw, 3.5rem) var(--space-6)",
    );
  });

  it("renders the social card as a dark outlined panel with layered previews", () => {
    const cardRule = stylesheet.match(/(?:^|\n)\.communityCard\s*\{\s*position:[^}]*\}/s)?.[0] ?? "";
    const visualRule = stylesheet.match(/\.communityVisual\s*\{[^}]*\}/s)?.[0] ?? "";
    const buttonRule = stylesheet.match(/\.communityButton\s*\{[^}]*\}/s)?.[0] ?? "";
    const phoneRule = stylesheet.match(/\.socialPhone\s*\{[^}]*\}/s)?.[0] ?? "";

    expect(cardRule).toContain("background: var(--color-panel)");
    expect(cardRule).toContain("border: 0.0625rem solid var(--color-card-border)");
    expect(cardRule).toContain("overflow: hidden");
    expect(visualRule).toContain("position: relative");
    expect(buttonRule).toContain("background: var(--color-surface-light)");
    expect(buttonRule).toContain("color: var(--color-text-on-light)");
    expect(phoneRule).toContain("position: absolute");
    expect(componentSource).toContain("styles.communityPreviewLeft");
    expect(componentSource).toContain("styles.communityPreviewRight");
    expect(componentSource).toContain("styles.qrPlaceholder");
  });
});
