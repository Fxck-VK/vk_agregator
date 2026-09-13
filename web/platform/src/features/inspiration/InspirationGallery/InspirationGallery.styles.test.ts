import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const inspirationStylesheet = readFileSync(
  resolve(process.cwd(), "src/features/inspiration/InspirationExampleCard/InspirationExampleCard.module.css"),
  "utf8",
);
const templateStylesheet = readFileSync(
  resolve(process.cwd(), "src/components/media/MediaPreviewDialogTemplate/MediaPreviewDialogTemplate.module.css"),
  "utf8",
);
const modalCloseButtonStylesheet = readFileSync(
  resolve(process.cwd(), "src/components/ui/ModalCloseButton/ModalCloseButton.module.css"),
  "utf8",
);
const stylesheet = `${templateStylesheet}\n${inspirationStylesheet}`;
const masonryGridStylesheet = readFileSync(
  resolve(process.cwd(), "src/components/ui/MasonryGrid/MasonryGrid.module.css"),
  "utf8",
);
const gallerySource = readFileSync(
  resolve(process.cwd(), "src/features/inspiration/InspirationGallery/InspirationGallery.tsx"),
  "utf8",
);
const cardSource = readFileSync(
  resolve(process.cwd(), "src/features/inspiration/InspirationExampleCard/InspirationExampleCard.tsx"),
  "utf8",
);
const modalBackdropStylesheet = readFileSync(
  resolve(process.cwd(), "src/components/ui/ModalBackdrop/ModalBackdrop.module.css"),
  "utf8",
);
const actionIconSvgs = [
  "copy-white.svg",
  "download-white.svg",
  "repost-white.svg",
  "star-white.svg",
].map((filename) =>
  readFileSync(resolve(process.cwd(), "public/assets/icons/ui", filename), "utf8"),
);

describe("InspirationGallery styles", () => {
  it("uses the extracted preview template on the active gallery path", () => {
    expect(gallerySource).toContain(
      'from "../InspirationExampleCard/InspirationExampleDialogTemplate"',
    );
  });

  it("uses the shared dialog for standalone cards without retaining the legacy shell", () => {
    expect(cardSource).toContain(
      'from "./InspirationExampleDialogTemplate"',
    );
    expect(cardSource).not.toContain("ModalBackdrop");
    expect(cardSource).not.toContain("ScrollArea");
    expect(cardSource).not.toContain("export function InspirationExampleDialog(");
    expect(inspirationStylesheet).not.toMatch(/\.dialog\s*\{/);
    expect(inspirationStylesheet).not.toMatch(/\.closeButton\s*\{/);
    expect(inspirationStylesheet).not.toMatch(/\.thumbnailRail\s*\{/);
    expect(inspirationStylesheet).not.toMatch(/\.previewNavigation\s*\{/);
    expect(inspirationStylesheet).not.toMatch(/\.infoPanel\s*\{/);
  });

  it("uses a fixed viewport dialog and a single-column mobile layout", () => {
    expect(modalBackdropStylesheet).toMatch(/\.backdrop\s*\{[^}]*position:\s*fixed;/s);
    expect(modalBackdropStylesheet).toMatch(/\.backdrop\s*\{[^}]*inset:\s*0;/s);
    expect(stylesheet).toMatch(/@media\s*\(width\s*<\s*60rem\)/);
    expect(stylesheet).toMatch(/@media\s*\(width\s*<\s*60rem\)[\s\S]*\.dialog\s*\{[^}]*grid-template-columns:\s*minmax\(0, 1fr\);/s);
  });

  it("constrains the desktop thumbnail rail so it scrolls inside the dialog", () => {
    const dialogRule = stylesheet.match(/\.dialog\s*\{([^}]*)\}/s)?.[1] ?? "";
    const thumbnailRailRule = stylesheet.match(/\.thumbnailRail\s*\{([^}]*)\}/s)?.[1] ?? "";
    const thumbnailRailViewportRule = stylesheet.match(
      /\.thumbnailRailViewport\s*\{([^}]*)\}/s,
    )?.[1] ?? "";
    const thumbnailRule = stylesheet.match(/\.thumbnail\s*\{([^}]*)\}/s)?.[1] ?? "";

    expect(dialogRule).toContain("grid-template-columns: 6rem");
    expect(dialogRule).toContain("grid-template-rows: minmax(0, 1fr)");
    expect(thumbnailRailRule).toContain("block-size: 100%");
    expect(thumbnailRailRule).toContain("min-block-size: 0");
    expect(thumbnailRailViewportRule).toContain("padding-inline-end: 1rem");
    expect(thumbnailRule).toContain("flex: 0 0 auto");
  });

  it("keeps the desktop dialog and its three regions at stable viewport-based sizes", () => {
    const dialogRule = stylesheet.match(/\.dialog\s*\{([^}]*)\}/s)?.[1] ?? "";
    const previewRule = stylesheet.match(/\.preview\s*\{([^}]*)\}/s)?.[1] ?? "";
    const previewStageRule = stylesheet.match(/\.previewStage\s*\{([^}]*)\}/s)?.[1] ?? "";
    const previewSurfaceRule = stylesheet.match(/\.previewSurface\s*\{([^}]*)\}/s)?.[1] ?? "";
    const previewMediaRule = stylesheet.match(/\.previewMedia\s*\{([^}]*)\}/s)?.[1] ?? "";
    const infoPanelRule = stylesheet.match(/\.infoPanel\s*\{([^}]*)\}/s)?.[1] ?? "";
    const infoPanelViewportRule = stylesheet.match(
      /\.infoPanelViewport\s*\{([^}]*)\}/s,
    )?.[1] ?? "";

    expect(dialogRule).toContain("inline-size: min(100rem, calc(100vw - 3rem))");
    expect(dialogRule).toContain("block-size: min(60rem, calc(100dvh - 3rem))");
    expect(dialogRule).toContain(
      "grid-template-columns: 6rem minmax(0, 1fr) clamp(25rem, 30vw, 32rem) 3rem",
    );
    expect(previewRule).toContain("inline-size: 100%");
    expect(previewRule).toContain("block-size: 100%");
    expect(previewRule).toContain("overflow: hidden");
    expect(previewStageRule).toContain("position: absolute");
    expect(previewRule).toContain("--preview-stage-bottom: 1rem");
    expect(previewStageRule).toContain("inset-block: 0 var(--preview-stage-bottom)");
    expect(previewStageRule).toContain("inset-inline: 3.5rem");
    expect(previewSurfaceRule).toContain("display: grid");
    expect(previewSurfaceRule).toContain("place-items: center");
    expect(previewMediaRule).toMatch(/(?:^|\n)\s*inline-size:\s*auto;/);
    expect(previewMediaRule).toMatch(/(?:^|\n)\s*block-size:\s*auto;/);
    expect(previewMediaRule).toContain("max-inline-size: 100%");
    expect(previewMediaRule).toContain("max-block-size: 100%");
    expect(previewMediaRule).toContain("object-fit: contain");
    expect(previewMediaRule).not.toContain("box-shadow");
    expect(infoPanelRule).toContain("block-size: 100%");
    expect(infoPanelRule).toContain("overflow: hidden");
    expect(infoPanelRule).not.toContain("overflow: auto");
    expect(infoPanelViewportRule).toContain("display: flex");
    expect(infoPanelViewportRule).toContain("block-size: 100%");
    expect(infoPanelViewportRule).toContain("padding: clamp(1.25rem, 2vw, 1.75rem)");
  });

  it("rounds the visible preview media without cropping it", () => {
    const previewStageRule = stylesheet.match(/\.previewStage\s*\{([^}]*)\}/s)?.[1] ?? "";
    const previewSurfaceRule = stylesheet.match(/\.previewSurface\s*\{([^}]*)\}/s)?.[1] ?? "";
    const previewMediaRule = stylesheet.match(/\.previewMedia\s*\{([^}]*)\}/s)?.[1] ?? "";

    expect(previewStageRule).toContain("container-type: size");
    expect(previewSurfaceRule).toContain(
      "inline-size: min(100%, calc(100cqh * var(--media-aspect)))",
    );
    expect(previewSurfaceRule).toContain("aspect-ratio: var(--media-aspect)");
    expect(previewSurfaceRule).toContain("max-block-size: 100%");
    expect(previewSurfaceRule).toContain("overflow: hidden");
    expect(previewSurfaceRule).toContain("border-radius: 1rem");
    expect(previewMediaRule).toContain("object-fit: contain");
    expect(previewMediaRule).toContain("border-radius: inherit");
  });

  it("places the close control in its own column to the right of the desktop info panel", () => {
    const closeButtonPlacementRule = stylesheet.match(
      /\.closeButtonPlacement\s*\{([^}]*)\}/s,
    )?.[1] ?? "";
    const closeButtonRule = modalCloseButtonStylesheet.match(/\.button\s*\{([^}]*)\}/s)?.[1] ?? "";
    const thumbnailRailRule = stylesheet.match(/\.thumbnailRail\s*\{([^}]*)\}/s)?.[1] ?? "";
    const previewRule = stylesheet.match(/\.preview\s*\{([^}]*)\}/s)?.[1] ?? "";
    const infoPanelRule = stylesheet.match(/\.infoPanel\s*\{([^}]*)\}/s)?.[1] ?? "";

    expect(closeButtonPlacementRule).toContain("position: relative");
    expect(closeButtonPlacementRule).toContain("grid-column: 4");
    expect(closeButtonPlacementRule).toContain("grid-row: 1");
    expect(closeButtonPlacementRule).toContain("align-self: start");
    expect(closeButtonPlacementRule).toContain("justify-self: end");
    expect(closeButtonRule).toContain("border-radius: 0.875rem");
    expect(thumbnailRailRule).toContain("grid-column: 1");
    expect(previewRule).toContain("grid-column: 2");
    expect(infoPanelRule).toContain("grid-column: 3");
  });

  it("uses one border-only hover and focus treatment for modal navigation controls", () => {
    const navigationRule = stylesheet.match(/\.previewNavigation\s*\{([^}]*)\}/s)?.[1] ?? "";
    const navigationHoverRule = stylesheet.match(
      /\.previewNavigation:hover,\s*\.previewNavigation:focus-visible\s*\{([^}]*)\}/s,
    )?.[1] ?? "";
    const closeButtonHoverRule = modalCloseButtonStylesheet.match(
      /\.button:hover,\s*\.button:focus-visible\s*\{([^}]*)\}/s,
    )?.[1] ?? "";

    expect(navigationRule).toContain("border-radius: 0.5rem");
    for (const hoverRule of [navigationHoverRule, closeButtonHoverRule]) {
      expect(hoverRule).toContain("border-color: var(--input-focus-border-color)");
      expect(hoverRule).toContain("outline: none");
      expect(hoverRule).not.toContain("box-shadow:");
      expect(hoverRule).not.toContain("background:");
    }
  });

  it("uses one border-only selection and focus treatment for modal thumbnails", () => {
    const selectedThumbnailRule = stylesheet.match(
      /\.thumbnail\[aria-current="true"\]\s*\{([^}]*)\}/s,
    )?.[1] ?? "";
    const focusedThumbnailRule = stylesheet.match(
      /\.thumbnail:focus-visible\s*\{([^}]*)\}/s,
    )?.[1] ?? "";

    expect(selectedThumbnailRule).toContain("border-color: var(--color-accent)");
    expect(selectedThumbnailRule).not.toContain("box-shadow:");
    expect(focusedThumbnailRule).toContain("border-color: var(--input-focus-border-color)");
    expect(focusedThumbnailRule).toContain("outline: none");
    expect(focusedThumbnailRule).not.toContain("box-shadow:");
  });

  it("uses thumbnails, a compact preview and a roomy details panel on narrow screens", () => {
    const templateTabletSection = templateStylesheet.slice(
      templateStylesheet.indexOf("@media (width < 60rem)"),
      templateStylesheet.indexOf("@media (width < 40rem)"),
    );
    const inspirationTabletSection = inspirationStylesheet.slice(
      inspirationStylesheet.indexOf("@media (width < 60rem)"),
      inspirationStylesheet.indexOf("@media (width < 40rem)"),
    );
    const tabletSection = `${templateTabletSection}\n${inspirationTabletSection}`;
    const phoneSection = `${templateStylesheet.slice(templateStylesheet.indexOf("@media (width < 40rem)"))}\n${inspirationStylesheet.slice(inspirationStylesheet.indexOf("@media (width < 40rem)"))}`;

    expect(tabletSection).toMatch(/\.dialog\s*\{[^}]*grid-template-columns:\s*minmax\(0, 1fr\)/s);
    expect(tabletSection).toMatch(/\.dialog\s*\{[^}]*grid-template-rows:\s*5\.5rem clamp\(12rem, 30dvh, 19rem\) minmax\(0, 1fr\)/s);
    expect(tabletSection).toMatch(/\.dialog\s*\{[^}]*inline-size:\s*calc\(100vw - 3rem\)/s);
    expect(tabletSection).toMatch(/\.dialog\s*\{[^}]*block-size:\s*calc\(100dvh - 3rem\)/s);
    expect(tabletSection).toMatch(/\.thumbnailRailViewport\s*\{[^}]*flex-direction:\s*row/s);
    expect(tabletSection).toMatch(/\.thumbnailRail\s*\{[^}]*grid-row:\s*1/s);
    expect(tabletSection).toMatch(/\.preview\s*\{[^}]*grid-row:\s*2/s);
    expect(tabletSection).toMatch(/\.infoPanel\s*\{[^}]*grid-row:\s*3/s);
    expect(tabletSection).not.toMatch(/\.infoPanel\s*\{[^}]*overflow:\s*auto/s);
    expect(phoneSection).toMatch(/\.dialog\s*\{[^}]*inline-size:\s*100vw/s);
    expect(phoneSection).toMatch(/\.dialog\s*\{[^}]*block-size:\s*100dvh/s);
    expect(phoneSection).toMatch(/\.dialog\s*\{[^}]*grid-template-rows:\s*5\.25rem clamp\(11rem, 28dvh, 17rem\) minmax\(0, 1fr\)/s);
    expect(phoneSection).not.toMatch(/\.promptHeader\s*\{[^}]*flex-direction:\s*column/s);
  });

  it("places the close control and actions at the top of narrow-screen details", () => {
    const tabletSection = `${templateStylesheet.slice(
      templateStylesheet.indexOf("@media (width < 60rem)"),
      templateStylesheet.indexOf("@media (width < 40rem)"),
    )}\n${inspirationStylesheet.slice(
      inspirationStylesheet.indexOf("@media (width < 60rem)"),
      inspirationStylesheet.indexOf("@media (width < 40rem)"),
    )}`;

    expect(tabletSection).toMatch(/\.closeButtonPlacement\s*\{[^}]*position:\s*relative/s);
    expect(tabletSection).toMatch(/\.closeButtonPlacement\s*\{[^}]*grid-row:\s*3/s);
    expect(tabletSection).toMatch(/\.closeButtonPlacement\s*\{[^}]*margin-block-start:\s*0\.5rem/s);
    expect(tabletSection).toMatch(/\.closeButtonPlacement\s*\{[^}]*margin-inline-end:\s*var\(--space-4\)/s);
    expect(tabletSection).toMatch(/\.modelTitle\s*\{[^}]*order:\s*1/s);
    expect(tabletSection).toMatch(/\.previewActions\s*\{[^}]*order:\s*2/s);
    expect(tabletSection).toMatch(/\.promptHeaderSlot\s*\{[^}]*order:\s*3/s);
    expect(tabletSection).toMatch(/\.promptBlock\s*\{[^}]*order:\s*4/s);
  });

  it("reserves space beside the preview for previous and next controls", () => {
    const previewRule = stylesheet.match(/\.preview\s*\{([^}]*)\}/s)?.[1] ?? "";
    const previewStageRule = stylesheet.match(/\.previewStage\s*\{([^}]*)\}/s)?.[1] ?? "";
    const navigationRule = stylesheet.match(/\.previewNavigation\s*\{([^}]*)\}/s)?.[1] ?? "";

    expect(previewRule).toContain("position: relative");
    expect(previewStageRule).toContain("inset-inline: 3.5rem");
    expect(navigationRule).toContain("position: absolute");
  });

  it("insets both navigation controls so their hover rings stay inside the preview", () => {
    const previousButtonRule = stylesheet.match(/\.previousButton\s*\{([^}]*)\}/s)?.[1] ?? "";
    const nextButtonRule = stylesheet.match(/\.nextButton\s*\{([^}]*)\}/s)?.[1] ?? "";

    expect(previousButtonRule).toContain("inset-inline-start: 0.25rem");
    expect(nextButtonRule).toContain("inset-inline-end: 0.25rem");
  });

  it("uses the same masonry flow as My Files", () => {
    const gridRule = masonryGridStylesheet.match(/\.grid\s*\{([^}]*)\}/s)?.[1] ?? "";
    const itemRule = masonryGridStylesheet.match(/\.grid\s*>\s*li\s*\{([^}]*)\}/s)?.[1] ?? "";

    expect(gridRule).toContain("column-width: 17rem");
    expect(gridRule).toContain("column-gap: var(--space-4)");
    expect(gridRule).not.toContain("grid-template-columns");
    expect(itemRule).toContain("inline-size: 100%");
    expect(itemRule).toContain("margin-block-end: var(--space-4)");
    expect(itemRule).toContain("break-inside: avoid");
  });

  it("keeps gallery media at its natural aspect ratio without cropping", () => {
    const cardRule = stylesheet.match(/\.card\s*\{([^}]*)\}/s)?.[1] ?? "";
    const mediaRule = stylesheet.match(/\.cardMedia\s*\{([^}]*)\}/s)?.[1] ?? "";

    expect(cardRule).not.toContain("aspect-ratio");
    expect(mediaRule).toContain("block-size: auto");
    expect(mediaRule).toContain("object-fit: contain");
  });

  it("does not crop gallery media on hover", () => {
    const hoverMediaRule = stylesheet.match(
      /\.card:hover\s+\.cardMedia,[\s\S]*?\.card:focus-visible\s+\.cardMedia\s*\{([^}]*)\}/s,
    )?.[1] ?? "";

    expect(hoverMediaRule).not.toContain("transform:");
  });

  it("reveals the model label only when the card is hovered or keyboard-focused", () => {
    const metaRule = stylesheet.match(/\.cardMeta\s*\{([^}]*)\}/s)?.[1] ?? "";
    const visibleMetaRule = stylesheet.match(
      /\.card:hover\s+\.cardMeta,[\s\S]*?\.card:focus-visible\s+\.cardMeta\s*\{([^}]*)\}/s,
    )?.[1] ?? "";

    expect(metaRule).toContain("opacity: 0");
    expect(visibleMetaRule).toContain("opacity: 1");
  });

  it("clamps collapsed prompts to five lines", () => {
    const collapsedPromptRule = stylesheet.match(/\.promptCollapsed\s*\{([^}]*)\}/s)?.[1] ?? "";

    expect(collapsedPromptRule).toContain("-webkit-line-clamp: 5");
    expect(collapsedPromptRule).toContain("overflow: hidden");
  });

  it("embeds the prompt toggle into the last visible line with a soft fade", () => {
    const promptBlockRule = stylesheet.match(/\.promptBlock\s*\{([^}]*)\}/s)?.[1] ?? "";
    const collapsedToggleRule = stylesheet.match(
      /\.promptToggleCollapsed\s*\{([^}]*)\}/s,
    )?.[1] ?? "";
    const expandedToggleRule = stylesheet.match(
      /\.promptToggleExpanded\s*\{([^}]*)\}/s,
    )?.[1] ?? "";

    expect(promptBlockRule).toContain("position: relative");
    expect(collapsedToggleRule).toContain("position: absolute");
    expect(collapsedToggleRule).toContain("inset-block-end: 0");
    expect(collapsedToggleRule).toContain("inset-inline-end: 0");
    expect(collapsedToggleRule).toContain("linear-gradient");
    expect(expandedToggleRule).toContain("position: static");
  });

  it("centers the close icon geometrically with two CSS lines", () => {
    const closeIconRule = modalCloseButtonStylesheet.match(/\.icon\s*\{([^}]*)\}/s)?.[1] ?? "";
    const closeIconLinesRule = modalCloseButtonStylesheet.match(
      /\.icon::before,[\s\S]*?\.icon::after\s*\{([^}]*)\}/s,
    )?.[1] ?? "";

    expect(closeIconRule).toContain("position: relative");
    expect(closeIconLinesRule).toContain("inset-block-start: 50%");
    expect(closeIconLinesRule).toContain("inset-inline-start: 50%");
    expect(closeIconLinesRule).toContain("block-size: 0.125rem");
    expect(modalCloseButtonStylesheet).toMatch(
      /\.icon::before\s*\{[^}]*transform:\s*translate\(-50%, -50%\) rotate\(45deg\)/s,
    );
    expect(modalCloseButtonStylesheet).toMatch(
      /\.icon::after\s*\{[^}]*transform:\s*translate\(-50%, -50%\) rotate\(-45deg\)/s,
    );
  });

  it("keeps the prompt title and copy action together on one line", () => {
    const promptHeaderRule = stylesheet.match(/\.promptHeader\s*\{([^}]*)\}/s)?.[1] ?? "";
    const promptTitleRule = stylesheet.match(/\.promptHeader h2\s*\{([^}]*)\}/s)?.[1] ?? "";
    const copyButtonRule = [
      ...stylesheet.matchAll(/\.copyButton\s*\{([^}]*)\}/gs),
    ].at(-1)?.[1] ?? "";

    expect(promptHeaderRule).toContain("flex-wrap: nowrap");
    expect(promptHeaderRule).toContain("gap: var(--space-2)");
    expect(promptTitleRule).toContain("white-space: nowrap");
    expect(copyButtonRule).toContain("flex: 0 0 auto");
    expect(copyButtonRule).toContain("padding: 0.5rem 0.625rem");
    expect(copyButtonRule).toContain("white-space: nowrap");
  });

  it("uses the reference typography hierarchy in the preview panel", () => {
    const modelNameRule = stylesheet.match(/\.modelTitle strong\s*\{([^}]*)\}/s)?.[1] ?? "";
    const promptTitleRule = stylesheet.match(/\.promptHeader h2\s*\{([^}]*)\}/s)?.[1] ?? "";
    const promptRule = stylesheet.match(/\.prompt\s*\{([^}]*)\}/s)?.[1] ?? "";
    const copyButtonRule = [
      ...stylesheet.matchAll(/\.copyButton\s*\{([^}]*)\}/gs),
    ].at(-1)?.[1] ?? "";
    const sharedActionRule = stylesheet.match(
      /\.primaryAction,[\s\S]*?\.secondaryAction\s*\{([^}]*)\}/s,
    )?.[1] ?? "";

    expect(modelNameRule).toContain("font-weight: var(--font-weight-semibold)");
    expect(promptTitleRule).toContain("font-weight: var(--font-weight-semibold)");
    expect(promptRule).toContain("font-weight: var(--font-weight-medium)");
    expect(copyButtonRule).toContain("font-weight: var(--font-weight-semibold)");
    expect(sharedActionRule).toContain("font-weight: var(--font-weight-semibold)");
  });

  it("uses moderately rounded rectangular action buttons in the preview panel", () => {
    const sharedActionRule = stylesheet.match(
      /\.primaryAction,[\s\S]*?\.secondaryAction\s*\{([^}]*)\}/s,
    )?.[1] ?? "";

    expect(sharedActionRule).toContain("border-radius: var(--radius-sm)");
    expect(inspirationStylesheet).not.toMatch(/\.(?:actions|recreateButton|secondaryActions|secondaryButton)\b/);
  });

  it("renders action icons at cap height without the original empty SVG canvas", () => {
    const copyIconRule = stylesheet.match(/\.copyIcon\s*\{([^}]*)\}/s)?.[1] ?? "";
    const actionIconRule = stylesheet.match(/\.previewActionIcon\s*\{([^}]*)\}/s)?.[1] ?? "";

    expect(copyIconRule).toContain("inline-size: 1.5rem");
    expect(copyIconRule).toContain("block-size: 1.5rem");
    expect(actionIconRule).toContain("inline-size: 1.5rem");
    expect(actionIconRule).toContain("block-size: 1.5rem");
    actionIconSvgs.forEach((svg) => {
      expect(svg).not.toContain('viewBox="0 0 1254 1254"');
    });
  });
});
