import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

function readSource(path: string): string {
  const absolutePath = resolve(process.cwd(), path);
  return existsSync(absolutePath) ? readFileSync(absolutePath, "utf8") : "";
}

describe("shared Tooltip contract", () => {
  it("owns the branded tooltip surface and lightweight hover behavior", () => {
    const component = readSource("src/components/ui/Tooltip/Tooltip.tsx");
    const stylesheet = readSource("src/components/ui/Tooltip/Tooltip.module.css");

    expect(component).toContain('data-ui="tooltip-bubble"');
    expect(component).toContain('role="tooltip"');
    expect(component).toContain("data-placement={placement}");
    expect(stylesheet).toMatch(
      /\.bubble\s*\{[^}]*border-radius:\s*var\(--radius-sm\);[^}]*background:\s*var\(--panel-surface-background\);[^}]*box-shadow:\s*var\(--panel-surface-shadow\);/s,
    );
    expect(stylesheet).toMatch(/\.anchor:hover \.anchoredBubble,/s);
    expect(stylesheet).toMatch(/\.anchor:focus-within \.anchoredBubble\s*\{/s);
  });

  it("is the single tooltip surface used by the composer and sidebar", () => {
    const composer = readSource("src/components/chat/ChatComposer/ChatComposer.tsx");
    const submitButton = readSource("src/components/chat/ChatSubmitButton/ChatSubmitButton.tsx");
    const sidebar = readSource("src/components/layout/Sidebar/Sidebar.tsx");
    const sidebarStyles = readSource("src/components/layout/Sidebar/Sidebar.module.css");

    expect(composer).toContain("<ChatSubmitButton");
    expect(submitButton).toContain("<Tooltip label={label}>");
    expect(composer).toContain("label={submitLabel}");
    expect(composer).not.toContain("title={submitLabel}");
    expect(sidebar).toContain("<TooltipBubble");
    expect(sidebarStyles).not.toMatch(/\.railTooltip\s*\{[^}]*background:/s);
  });
});
