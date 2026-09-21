import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const stylesheet = readFileSync(
  resolve(process.cwd(), "src/features/account/AccountMenu/AccountMenu.module.css"),
  "utf8",
);
const component = readFileSync(
  resolve(process.cwd(), "src/features/account/AccountMenu/AccountMenu.tsx"),
  "utf8",
);

const menuRule = stylesheet.match(/\.menu \{([\s\S]*?)\n\}/)?.[1];
const triggerRule = stylesheet.match(/\.trigger \{([\s\S]*?)\n\}/)?.[1];

describe("AccountMenu styles", () => {
  it("keeps an upward-opening menu reachable on short viewports", () => {
    expect(menuRule).toContain("inset-block-end: calc(100% + var(--space-2))");
    expect(menuRule).toContain("max-block-size: calc(100dvh - 6rem)");
    expect(component).toContain('from "@/components/ui/PopoverPanel/PopoverPanel"');
    expect(component).toContain("<PopoverSurface");
  });

  it("uses the same translucent overlay surface as conversation action panels", () => {
    expect(component).toContain("<PopoverSurface");
    expect(menuRule).not.toMatch(/background:|box-shadow:|backdrop-filter:|border-radius:/);
    expect(stylesheet).toMatch(
      /\.themeSection\s*\{[^}]*border-block:\s*0\.0625rem solid color-mix\(in srgb, var\(--color-border\) 60%, transparent\);/s,
    );
  });

  it("delegates scrollbar visuals to the shared component", () => {
    expect(stylesheet).not.toContain("scrollbar-width:");
    expect(stylesheet).not.toContain("scrollbar-color:");
    expect(stylesheet).not.toContain("::-webkit-scrollbar");
  });

  it("keeps the account trigger transparent with an outline on interaction", () => {
    expect(triggerRule).toContain("grid-template-columns: 2.5rem minmax(0, 1fr) 1.5rem");
    expect(triggerRule).toContain("background: transparent");
    expect(stylesheet).toMatch(
      /\.trigger:hover,\s*\.trigger:focus-visible,\s*\.trigger\[data-open="true"\]\s*\{[^}]*background:\s*transparent;[^}]*box-shadow:\s*inset 0 0 0 0\.0625rem var\(--color-accent\);/s,
    );
  });

  it("renders a fixed rounded-square account icon without turning the trigger into a pill", () => {
    expect(stylesheet).toMatch(/\.avatar\s*\{[^}]*inline-size:\s*2\.5rem;/s);
    expect(stylesheet).toMatch(/\.avatar\s*\{[^}]*block-size:\s*2\.5rem;/s);
    expect(stylesheet).toMatch(/\.avatar\s*\{[^}]*border-radius:\s*var\(--radius-sm\);/s);
    expect(triggerRule).toContain("border-radius: var(--radius-sm)");
  });

  it("highlights every account-menu row on hover without shifting its layout", () => {
    expect(component).toContain('import selectableStyles from "@/components/ui/selectable-control.module.css"');
    expect(component).toContain("${selectableStyles.control} ${styles.menuAction}");
    expect(component).toContain("${selectableStyles.control} ${styles.logoutAction}");
    expect(stylesheet).not.toMatch(/\.(menuAction|logoutAction):hover/);
  });

  it("delegates theme icons and the moving outline to the shared mode switch panel", () => {
    expect(component).toMatch(/<ModeSwitchPanel\s[^>]*iconOnly\s[^>]*items=\{themeOptions\}/s);
    expect(stylesheet).not.toContain(".themeSwitcher::before");
    expect(stylesheet).not.toContain(".themeOption");
  });
});
