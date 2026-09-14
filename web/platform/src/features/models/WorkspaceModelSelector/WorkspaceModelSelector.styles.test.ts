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
const modelCardStylesheet = readFileSync(
  resolve(process.cwd(), "src/features/models/ModelCard/ModelCard.module.css"),
  "utf8",
);
const component = readFileSync(
  resolve(process.cwd(), "src/features/models/WorkspaceModelSelector/ModelSelector.tsx"),
  "utf8",
);

describe("WorkspaceModelSelector layout", () => {
  it("gives the panel variant a full-width outline and a wrapping model name", () => {
    const panelRootRule = stylesheet.match(
      /\.root\[data-variant="panel"\]\s*\{[^}]*\}/s,
    )?.[0] ?? "";
    const panelTriggerRule = stylesheet.match(
      /\.root\[data-variant="panel"\] \.trigger\s*\{[^}]*\}/s,
    )?.[0] ?? "";
    const panelTextRule = stylesheet.match(
      /\.root\[data-variant="panel"\] \.triggerText\s*\{[^}]*\}/s,
    )?.[0] ?? "";
    const panelHoverRule = stylesheet.match(
      /\.root\[data-variant="panel"\] \.trigger:hover\s*\{[^}]*\}/s,
    )?.[0] ?? "";

    expect(panelRootRule).toContain("inline-size: 100%");
    expect(panelTriggerRule).toContain("display: grid");
    expect(panelTriggerRule).toContain("grid-template-columns: auto minmax(0, 1fr) auto");
    expect(panelTriggerRule).toContain("inline-size: 100%");
    expect(panelTriggerRule).toContain("max-inline-size: none");
    expect(panelTriggerRule).toContain("border-color: rgb(255 255 255 / 18%)");
    expect(panelTriggerRule).toContain("border-radius: var(--radius-sm)");
    expect(panelTriggerRule).toContain("background: transparent");
    expect(panelTextRule).toContain("overflow: visible");
    expect(panelTextRule).toContain("overflow-wrap: anywhere");
    expect(panelTextRule).toContain("text-overflow: clip");
    expect(panelTextRule).toContain("white-space: normal");
    expect(panelHoverRule).toContain("border-color: rgb(255 255 255 / 18%)");
    expect(panelHoverRule).toContain("background: transparent");
    expect(panelHoverRule).toContain("box-shadow: none");
  });

  it("sizes the floating trigger to the full model name and wraps only when space is limited", () => {
    const triggerRule = stylesheet.match(/\.trigger\s*\{[^}]*\}/s)?.[0] ?? "";
    const triggerTextRule = stylesheet.match(/\.triggerText\s*\{[^}]*\}/s)?.[0] ?? "";
    const modelIconRule = stylesheet.match(/\.modelIcon\s*\{[^}]*\}/s)?.[0] ?? "";

    expect(triggerRule).toContain("gap: var(--space-1)");
    expect(triggerRule).toContain("inline-size: max-content");
    expect(triggerRule).toContain("max-inline-size: 100%");
    expect(triggerTextRule).toContain("white-space: normal");
    expect(triggerTextRule).toContain("overflow-wrap: anywhere");
    expect(triggerTextRule).not.toContain("overflow: hidden");
    expect(triggerTextRule).not.toContain("text-overflow: ellipsis");
    expect(stylesheet).not.toContain("max-inline-size: min(11.5rem");
    expect(triggerRule).toContain("padding: var(--space-2)");
    expect(modelIconRule).toContain("inline-size: 1.5rem");
    expect(modelIconRule).toContain("block-size: 1.5rem");
    expect(modelIconRule).toContain("transform: translateY(-0.0625rem)");
  });

  it("keeps the wide chevron compact and separated from the model name", () => {
    const chevronRule = stylesheet.match(/\.chevron\s*\{[^}]*\}/s)?.[0] ?? "";
    const chevronImageRule = stylesheet.match(/\.chevron img\s*\{[^}]*\}/s)?.[0] ?? "";

    expect(chevronRule).toContain("inline-size: 0.75rem");
    expect(chevronRule).toContain("block-size: 0.75rem");
    expect(chevronRule).toContain("margin-inline-start: var(--space-1)");
    expect(chevronImageRule).toContain("inline-size: 100%");
    expect(chevronImageRule).toContain("block-size: auto");
    expect(chevronImageRule).not.toContain("translateY");
  });

  it("keeps search and footer fixed while only the model list scrolls", () => {
    expect(stylesheet).toMatch(
      /\.popover\s*\{[^}]*grid-template-rows:\s*auto auto minmax\(0,\s*1fr\) auto;[^}]*overflow:\s*hidden;/s,
    );
    expect(stylesheet).toMatch(/\.scrollArea\s*\{[^}]*min-block-size:\s*0;/s);
    expect(component).toContain('from "@/components/ui/ScrollArea/ScrollArea"');
    expect(component).toMatch(/<ScrollArea\s+className=\{styles.scrollArea\}\s+viewportClassName=\{styles.scrollViewport\}/);
    expect(stylesheet).not.toMatch(/overflow-y:\s*(?:auto|scroll)/);
  });

  it("reserves room for the floating scrollbar outside model cards", () => {
    expect(stylesheet).toMatch(
      /\.scrollViewport\s*\{[^}]*padding-inline-end:\s*var\(--space-4\);/s,
    );
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

  it("uses the shared purple focus treatment on the search surface", () => {
    expect(stylesheet).toMatch(
      /\.searchRow:focus-within\s*\{[^}]*border-color:\s*var\(--input-focus-border-color\);[^}]*box-shadow:\s*var\(--input-focus-ring\);/s,
    );
  });

  it("insets the search surface so the complete focus ring remains visible", () => {
    expect(stylesheet).toMatch(
      /\.searchRow\s*\{[^}]*margin:\s*var\(--space-3\) var\(--space-3\) 0;[^}]*border:\s*0\.0625rem solid var\(--color-border\);[^}]*border-radius:\s*var\(--radius-md\);/s,
    );
  });

  it("leaves selection visuals to the shared control style without a separate mark or fill", () => {
    const optionRule = modelCardStylesheet.match(/\.selectorCard\s*\{[^}]*\}/s)?.[0] ?? "";
    expect(optionRule).not.toMatch(/(?:background|border|border-radius|transition):/);
    expect(modelCardStylesheet).not.toMatch(/\.(?:selectionMark|selectorSelected)\b/);
    expect(modelCardStylesheet).not.toMatch(/\.selectorCard:(?:hover|focus-visible)/);
  });

  it("uses price-free model rows with only an icon and description", () => {
    const optionRule = modelCardStylesheet.match(/\.selectorCard\s*\{[^}]*\}/s)?.[0] ?? "";

    expect(optionRule).toContain("grid-template-columns: auto minmax(0, 1fr);");
    expect(optionRule).not.toContain("price");
    expect(component).not.toContain("getMinimumPrice");
    expect(component).not.toContain("styles.price");
    expect(stylesheet).not.toMatch(/\.(?:option|optionSelected|optionIcon|optionCopy|optionTitle|optionDescription|selectionMark)\s*\{/);
  });

  it("reveals downward and closes upward without scaling its contents", () => {
    expect(component).toContain('type PopoverState = "closed" | "open" | "closing"');
    expect(component).toContain("data-state={popoverState}");
    expect(component).toContain('popover.addEventListener("animationend", closeAfterAnimation)');
    expect(stylesheet).toMatch(
      /\.popover\[data-state="open"\]\s*\{[^}]*animation:\s*workspaceModelSelectorOpen var\(--motion-normal\) both;/s,
    );
    expect(stylesheet).toMatch(
      /\.popover\[data-state="closing"\]\s*\{[^}]*pointer-events:\s*none;[^}]*animation:\s*workspaceModelSelectorClose var\(--motion-normal\) both;/s,
    );
    expect(stylesheet).toMatch(
      /@keyframes workspaceModelSelectorOpen\s*\{\s*from\s*\{[^}]*clip-path:\s*inset\(0 0 100% 0\);[^}]*opacity:\s*0;/s,
    );
    expect(stylesheet).toMatch(
      /@keyframes workspaceModelSelectorClose\s*\{[\s\S]*?to\s*\{[^}]*clip-path:\s*inset\(0 0 100% 0\);[^}]*opacity:\s*0;/,
    );
    expect(stylesheet).not.toMatch(/scaleY\(/);
    expect(stylesheet).toMatch(
      /@media \(prefers-reduced-motion: reduce\)\s*\{[^}]*\.popover\s*\{[^}]*animation-duration:\s*1ms;/s,
    );
  });

});
