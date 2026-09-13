import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const fileStylesheet = readFileSync(
  resolve(process.cwd(), "src/features/files/FilePreviewDialog/FilePreviewDialog.module.css"),
  "utf8",
);
const modeSwitchPanelStylesheet = readFileSync(
  resolve(process.cwd(), "src/components/ui/ModeSwitchPanel/ModeSwitchPanel.module.css"),
  "utf8",
);
const selectableStylesheet = readFileSync(
  resolve(process.cwd(), "src/components/ui/selectable-control.module.css"),
  "utf8",
);
const templateStylesheet = readFileSync(
  resolve(process.cwd(), "src/components/media/MediaPreviewDialogTemplate/MediaPreviewDialogTemplate.module.css"),
  "utf8",
);
const dialogSource = readFileSync(
  resolve(process.cwd(), "src/features/files/FilePreviewDialog/FilePreviewDialog.tsx"),
  "utf8",
);
const editorStylesheet = readFileSync(
  resolve(process.cwd(), "src/features/files/FilePreviewDialog/FileEditorPanel.module.css"),
  "utf8",
);
const editorSource = readFileSync(
  resolve(process.cwd(), "src/features/files/FilePreviewDialog/FileEditorPanel.tsx"),
  "utf8",
);
const animationStylesheet = readFileSync(
  resolve(process.cwd(), "src/features/files/FilePreviewDialog/FileAnimationPanel.module.css"),
  "utf8",
);
describe("FilePreviewDialog shared-template composition", () => {
  it("uses the shared preview template instead of a file-specific modal shell", () => {
    expect(dialogSource).toContain(
      'from "@/components/media/MediaPreviewDialogTemplate/MediaPreviewDialogTemplate"',
    );
    expect(dialogSource).not.toContain('from "@/components/ui/ModalBackdrop/ModalBackdrop"');
    expect(dialogSource).not.toContain("useSyncExternalStore");
    expect(dialogSource).not.toContain("<ModalBackdrop");
    expect(dialogSource).not.toContain("styles.dialog");
    expect(dialogSource).not.toContain("styles.previewPane");
    expect(dialogSource).not.toContain("styles.previewNavigation");
  });

  it("removes structural viewer rules from the file stylesheet", () => {
    expect(fileStylesheet).not.toMatch(/\.dialog\s*\{/);
    expect(fileStylesheet).not.toMatch(/\.thumbnailRail\s*\{/);
    expect(fileStylesheet).not.toMatch(/\.previewPane\s*\{/);
    expect(fileStylesheet).not.toMatch(/\.previewNavigation\s*\{/);
    expect(fileStylesheet).not.toMatch(/\.closeButton\s*\{/);
    expect(fileStylesheet).not.toMatch(/\.infoPanel\s*\{/);
  });

  it("inherits the approved desktop and narrow layouts from the shared template", () => {
    expect(templateStylesheet).toMatch(
      /\.dialog\s*\{[^}]*grid-template-columns:\s*6rem minmax\(0, 1fr\) clamp\(25rem, 30vw, 32rem\) 3rem;/s,
    );
    expect(templateStylesheet).toMatch(/\.thumbnailRail\s*\{[^}]*grid-column:\s*1;/s);
    expect(templateStylesheet).toMatch(/\.preview\s*\{[^}]*grid-column:\s*2;/s);
    expect(templateStylesheet).toMatch(/\.infoPanel\s*\{[^}]*grid-column:\s*3;/s);
    expect(templateStylesheet).toMatch(
      /@media\s*\(width < 60rem\)[\s\S]*\.dialog\s*\{[^}]*grid-template-columns:\s*minmax\(0, 1fr\);/s,
    );
  });

  it("composes the file toolbar from the reusable mode-switch panel", () => {
    const previewFooterRule = templateStylesheet.match(/\.previewFooter\s*\{([^}]*)\}/s)?.[1] ?? "";
    const toolRailRule = fileStylesheet.match(/\.toolRail\s*\{([^}]*)\}/s)?.[1] ?? "";
    const panelRootRule = modeSwitchPanelStylesheet.match(/\.root\s*\{([^}]*)\}/s)?.[1] ?? "";
    const panelViewportRule = modeSwitchPanelStylesheet.match(/\.viewport\s*\{([^}]*)\}/s)?.[1] ?? "";
    const panelButtonRule = modeSwitchPanelStylesheet.match(/\.viewport button\s*\{([^}]*)\}/s)?.[1] ?? "";

    expect(templateStylesheet).toMatch(
      /\.previewWithFooter\s*\{[^}]*--preview-stage-bottom:\s*5\.25rem;/s,
    );
    expect(previewFooterRule).toContain("position: absolute");
    expect(previewFooterRule).toContain("inset-block-end: 0");
    expect(previewFooterRule).toContain("inset-inline: 0.25rem");
    expect(templateStylesheet.match(/\.previewFooter\s*\{/g)).toHaveLength(1);
    expect(templateStylesheet).toMatch(
      /\.previewWithFooter\s*\{[^}]*overflow:\s*visible;/s,
    );
    expect(templateStylesheet).toMatch(/\.previewStage\s*\{[^}]*overflow:\s*hidden;/s);
    expect(toolRailRule).toMatch(/(?:^|\n)\s*inline-size:\s*100%;/);
    expect(panelRootRule).toContain("inline-size: fit-content");
    expect(panelRootRule).toContain("max-inline-size: 100%");
    expect(panelViewportRule).toContain("display: flex");
    expect(panelViewportRule).toContain("justify-content: safe center");
    expect(toolRailRule).not.toContain("background:");
    expect(toolRailRule).not.toContain("border:");
    expect(panelViewportRule).toContain("background: var(--panel-surface-background)");
    expect(panelViewportRule).toContain("border: var(--panel-surface-border)");
    expect(panelButtonRule).toContain("flex: 0 0 auto");
    expect(panelButtonRule).toContain("justify-content: center");
    expect(panelButtonRule).toContain("text-align: center");
    expect(dialogSource).toContain('from "@/components/ui/ModeSwitchPanel/ModeSwitchPanel"');
    expect(dialogSource).toContain("<ModeSwitchPanel");
    expect(dialogSource).not.toContain('from "@/components/ui/ScrollArea/ScrollArea"');
    expect(fileStylesheet).not.toContain("scrollbar-width:");
    expect(fileStylesheet).not.toContain("::-webkit-scrollbar");
    expect(fileStylesheet).not.toMatch(/\.(?:toolRailViewport|toolIndicator|activeTool|toolIcon)\b/);
    const toolIndicatorRule = modeSwitchPanelStylesheet.match(/\.indicator\s*\{([^}]*)\}/s)?.[1] ?? "";
    const inactiveHoverRule = selectableStylesheet.match(
      /\.control:where\([^}]*?\)\s*\{([^}]*)\}/s,
    )?.[1] ?? "";
    expect(toolIndicatorRule).toMatch(/border:[^;]*var\(--color-accent\);/);
    expect(toolIndicatorRule).toContain("background: transparent");
    expect(toolIndicatorRule).toContain("transform-origin: left center");
    expect(toolIndicatorRule).toContain("will-change: transform");
    expect(toolIndicatorRule).toContain(
      "transform 520ms cubic-bezier(0.45, 0, 0.55, 1)",
    );
    expect(toolIndicatorRule).toContain(
      "inline-size 520ms cubic-bezier(0.45, 0, 0.55, 1)",
    );
    expect(inactiveHoverRule).toContain("border-color: var(--color-accent)");
    expect(inactiveHoverRule).not.toMatch(/(?:^|\n)\s*color:/);
    expect(inactiveHoverRule).not.toMatch(/(?:^|\n)\s*background:/);
  });

  it("keeps file prompt and metadata styling in the adapter while inheriting shared actions", () => {
    expect(fileStylesheet).toMatch(/\.promptCollapsed\s*\{[^}]*-webkit-line-clamp:\s*5;/s);
    expect(fileStylesheet).toMatch(/\.promptToggle\s*\{[^}]*color:\s*var\(--color-accent\)/s);
    expect(fileStylesheet).toMatch(/\.metadata\s*\{[^}]*display:\s*grid;/s);
    expect(fileStylesheet).not.toMatch(/\.(?:panelFooter|panelActions)\b/);
    expect(dialogSource).toContain("getActions={(item) =>");
    expect(templateStylesheet).toMatch(
      /\.secondaryActions\s*\{[^}]*grid-template-columns:\s*repeat\(2, minmax\(0, 1fr\)\);/s,
    );
    expect(templateStylesheet).toMatch(
      /\.secondaryAction\s*\{[^}]*border-radius:\s*var\(--radius-sm\);/s,
    );
  });

  it("inherits the prompt heading and rectangular copy control from the shared component", () => {
    expect(dialogSource).toContain("<MediaPreviewPromptHeader");
    expect(fileStylesheet).not.toMatch(/\.(?:promptHeader|copyPromptButton|actionIcon)\b/);
    expect(templateStylesheet).toMatch(/\.promptHeader\s*\{[^}]*display:\s*flex;/s);
    expect(templateStylesheet).toMatch(
      /\.copyButton\s*\{[^}]*border-radius:\s*var\(--radius-sm\);/s,
    );
  });

  it("uses the small radius token for the animation action button", () => {
    const actionButtonRule = animationStylesheet.match(
      /\.actionButton\s*\{([^}]*)\}/s,
    )?.[1] ?? "";

    expect(actionButtonRule).toContain("border-radius: var(--radius-sm)");
  });

  it("reserves a clear gutter between editor controls and the floating scrollbar", () => {
    const bodyScrollRule = editorStylesheet.match(/\.bodyScroll\s*\{([^}]*)\}/s)?.[1] ?? "";
    const bodyViewportRule = editorStylesheet.match(/\.bodyViewport\s*\{([^}]*)\}/s)?.[1] ?? "";

    expect(bodyScrollRule).toContain("margin-inline-end: calc(-1 * var(--space-4))");
    expect(bodyViewportRule).toContain("padding-inline-end: var(--space-4)");
  });

  it("keeps the editor explanation card at its natural size inside the scrollable panel", () => {
    const howCardRule = editorStylesheet.match(/\.howCard\s*\{([^}]*)\}/s)?.[1] ?? "";
    const howCardImageRule = editorStylesheet.match(/\.howCard img\s*\{([^}]*)\}/s)?.[1] ?? "";

    expect(howCardRule).toContain("display: grid");
    expect(howCardImageRule).toContain("aspect-ratio: 16 / 9");
    expect(howCardImageRule).toContain("block-size: auto");
    expect(howCardImageRule).toContain("border-radius: var(--radius-lg)");
    expect(howCardImageRule).not.toContain("block-size: 7rem");
    expect(editorStylesheet).not.toContain(".howCard::after");
  });

  it("renders the editor tool cursor as a brand outline without a fill", () => {
    const toolCursorRule = editorStylesheet.match(/\.toolCursor\s*\{([^}]*)\}/s)?.[1] ?? "";

    expect(toolCursorRule).toContain("border: 0.125rem solid var(--color-accent)");
    expect(toolCursorRule).toContain("background: transparent");
    expect(toolCursorRule).toContain("box-shadow: none");
  });

  it("keeps the zoom controls centered inside a narrow portrait edit surface", () => {
    const editSurfaceRule = editorStylesheet.match(/\.editSurface\s*\{([^}]*)\}/s)?.[1] ?? "";
    const zoomControlsRule = [...editorStylesheet.matchAll(/\.zoomControls\s*\{([^}]*)\}/gs)]
      .map((match) => match[1] ?? "")
      .find((rule) => rule.includes("position: absolute")) ?? "";

    expect(editSurfaceRule).toContain("container-name: edit-preview");
    expect(editSurfaceRule).toContain("container-type: inline-size");
    expect(zoomControlsRule).toContain("inset-inline-start: 50%");
    expect(zoomControlsRule).toContain("inset-inline-end: auto");
    expect(zoomControlsRule).toContain("max-inline-size: calc(100% - var(--space-2))");
    expect(zoomControlsRule).toContain("transform: translateX(-50%)");
    expect(editorStylesheet).toMatch(
      /@container edit-preview \(inline-size < 10rem\)[\s\S]*\.zoomControls button\s*\{[^}]*inline-size:\s*1\.5rem;[^}]*block-size:\s*1\.5rem;/s,
    );
  });

  it("uses the small radius token for every editor control button", () => {
    const controlButtonRule = editorStylesheet.match(
      /\.iconButton,\s*\.clearButton,\s*\.selectionModes button,\s*\.modelSelector\s*\{([^}]*)\}/s,
    )?.[1] ?? "";

    expect(controlButtonRule).toContain("border-radius: var(--radius-sm)");
    expect(editorStylesheet).not.toMatch(/\.iconButton\s*\{[^}]*border-radius:/s);
    expect(editorStylesheet).not.toMatch(/\.clearButton\s*\{[^}]*border-radius:/s);
    expect(editorStylesheet).not.toMatch(/\.selectionModes button\s*\{[^}]*border-radius:/s);
    expect(editorStylesheet).toMatch(
      /\.generationSettings \[data-ui="input-control-chip"\]\s*\{[^}]*border-radius: var\(--radius-sm\)/s,
    );
    const modelSelectorRadiusRules = [...editorStylesheet.matchAll(
      /\.modelSelector\s*\{([^}]*)\}/gs,
    )].filter((match) => match[1]?.includes("border-radius:"));
    expect(modelSelectorRadiusRules).toHaveLength(1);
    expect(modelSelectorRadiusRules[0]?.[1]).toContain("border-radius: var(--radius-sm)");
  });

  it("keeps a toggled-off edit tool visually inactive while the pointer remains over it", () => {
    const inactiveToolRule = editorStylesheet.match(
      /\.iconButton\[data-active="false"\],\s*\.selectionModes button\[data-active="false"\]\s*\{([^}]*)\}/s,
    )?.[1] ?? "";
    const inactiveToolHoverRule = editorStylesheet.match(
      /\.iconButton\[data-active="false"\]:hover,\s*\.selectionModes button\[data-active="false"\]:hover\s*\{([^}]*)\}/s,
    )?.[1] ?? "";

    expect(inactiveToolRule).toContain("color: var(--color-text-muted)");
    expect(inactiveToolHoverRule).toContain("border-color: rgb(255 255 255 / 16%)");
    expect(inactiveToolHoverRule).toContain("color: #fff");
    expect(inactiveToolHoverRule).toContain("box-shadow: none");
  });

  it("uses the supplied brush and lasso assets without losing button state colors", () => {
    const selectionToolIconRule = editorStylesheet.match(
      /\.selectionToolIcon\s*\{([^}]*)\}/s,
    )?.[1] ?? "";

    expect(editorSource).not.toContain("function BrushIcon");
    expect(editorSource).not.toContain("function LassoIcon");
    expect(editorSource).toContain("styles.brushIcon");
    expect(editorSource).toContain("styles.lassoIcon");
    expect(selectionToolIconRule).toContain("inline-size: 1.2rem");
    expect(selectionToolIconRule).toContain("block-size: 1.2rem");
    expect(selectionToolIconRule).toContain("background: currentColor");
    expect(selectionToolIconRule).toContain(
      "-webkit-mask: var(--selection-tool-icon) center / contain no-repeat",
    );
    expect(selectionToolIconRule).toContain(
      "mask: var(--selection-tool-icon) center / contain no-repeat",
    );
    expect(editorStylesheet).toContain('url("/assets/icons/ui/brush-white.svg")');
    expect(editorStylesheet).toContain('url("/assets/icons/ui/lasso-white.svg")');
  });

  it("wraps the prompt field in the shared transparent input surface", () => {
    const promptFieldRule = editorStylesheet.match(/\.field textarea\s*\{([^}]*)\}/s)?.[1] ?? "";
    const promptSurfaceRule = editorStylesheet.match(/\.promptSurface\s*\{([^}]*)\}/s)?.[1] ?? "";

    expect(editorSource).toContain('from "@/components/ui/InputSurface/InputSurface"');
    expect(editorSource).toContain("<InputSurface className={styles.promptSurface}>");
    expect(promptSurfaceRule).toContain("background: transparent");
    expect(promptFieldRule).toContain("border: 0");
    expect(promptFieldRule).toContain("background: transparent");
    expect(promptFieldRule).toContain("resize: none");
  });

  it("uses the shared range-slider component for brush size", () => {
    expect(editorSource).toContain('from "@/components/ui/RangeSlider/RangeSlider"');
    expect(editorSource).toContain("<RangeSlider");
    expect(editorSource).not.toContain("brushSizeThumbWidth");
    expect(editorStylesheet).not.toMatch(/\.brushSize(?:Control|Range|Fill|Value)\b/);
  });

  it("floats the editor model list above or below without expanding the panel", () => {
    const modelFieldRule = editorStylesheet.match(/\.modelField\s*\{([^}]*)\}/s)?.[1] ?? "";
    const modelOptionRule = editorStylesheet.match(/\.modelOption\s*\{([^}]*)\}/s)?.[1] ?? "";
    const topRule = editorStylesheet.match(
      /\.modelOption\[data-placement="top"\]\s*\{([^}]*)\}/s,
    )?.[1] ?? "";
    const bottomRule = editorStylesheet.match(
      /\.modelOption\[data-placement="bottom"\]\s*\{([^}]*)\}/s,
    )?.[1] ?? "";

    expect(modelFieldRule).toContain("position: relative");
    expect(modelOptionRule).toContain("position: absolute");
    expect(modelOptionRule).toContain("inset-inline: 0");
    expect(modelOptionRule).toContain("inline-size: 100%");
    expect(topRule).toContain("inset-block-end: calc(100% + var(--space-2))");
    expect(bottomRule).toContain("inset-block-start: calc(100% + var(--space-2))");
  });
});
