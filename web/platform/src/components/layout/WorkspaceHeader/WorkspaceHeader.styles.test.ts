import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const headerStylesheet = readFileSync(
  resolve(process.cwd(), "src/components/layout/WorkspaceHeader/WorkspaceHeader.module.css"),
  "utf8",
);
const appShellStylesheet = readFileSync(
  resolve(process.cwd(), "src/components/layout/AppShell/AppShell.module.css"),
  "utf8",
);
const modelSelectorStylesheet = readFileSync(
  resolve(process.cwd(), "src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.module.css"),
  "utf8",
);
const plansDialogStylesheet = readFileSync(
  resolve(process.cwd(), "src/components/layout/WorkspaceHeader/SubscriptionPlansDialog.module.css"),
  "utf8",
);
const modalBackdropStylesheet = readFileSync(
  resolve(process.cwd(), "src/components/ui/ModalBackdrop/ModalBackdrop.module.css"),
  "utf8",
);

describe("WorkspaceHeader styles", () => {
  it("floats interactive controls over workspace content without drawing a header strip", () => {
    const headerZIndex = Number(headerStylesheet.match(/\.header\s*\{[^}]*z-index:\s*(\d+);/s)?.[1]);


    expect(headerStylesheet).toMatch(/\.header\s*\{[^}]*position:\s*absolute;/s);
    expect(headerStylesheet).toMatch(/\.header\s*\{[^}]*inset-block-start:\s*0;/s);
    expect(headerStylesheet).toMatch(/\.header\s*\{[^}]*inset-inline:\s*0;/s);
    expect(headerStylesheet).toMatch(/\.header\s*\{[^}]*min-block-size:\s*4\.25rem;/s);
    expect(headerStylesheet).toMatch(/\.header\s*\{[^}]*background:\s*transparent;/s);
    expect(headerStylesheet).toMatch(/\.header\s*\{[^}]*pointer-events:\s*none;/s);
    expect(headerStylesheet).toMatch(
      /\.leading,\s*\.trailing\s*\{[^}]*pointer-events:\s*auto;/s,
    );
    expect(headerStylesheet).not.toMatch(/\.leading,\s*\.trailing\s*\{[^}]*position:\s*absolute;/s);
    expect(headerZIndex).toBeGreaterThan(0);
    expect(appShellStylesheet).not.toMatch(/\.sidebar\s*\{[^}]*z-index:/s);
  });

  it("styles the tariff action with the NeiroHub brand gradient", () => {
    expect(headerStylesheet).toMatch(
      /\.trailing\s*\{[^}]*display:\s*flex;[^}]*gap:\s*var\(--space-3\);/s,
    );
    expect(headerStylesheet).toMatch(
      /\.tariffButton\s*\{[^}]*background:\s*var\(--gradient-brand\);[^}]*color:\s*#fff;/s,
    );
  });

  it("keeps every floating header control at the same height", () => {
    expect(modelSelectorStylesheet).toMatch(
      /\.trigger\s*\{[^}]*min-block-size:\s*2\.75rem;/s,
    );
    expect(headerStylesheet).toMatch(
      /\.balance\s*\{[^}]*min-block-size:\s*2\.75rem;/s,
    );
    expect(headerStylesheet).toMatch(
      /\.tariffButton\s*\{[^}]*min-block-size:\s*2\.75rem;/s,
    );
    expect(headerStylesheet).toMatch(
      /\.tariffButton\s*\{[^}]*border:\s*0\.0625rem solid transparent;/s,
    );
  });

  it("uses the small radius token for all three header controls", () => {
    expect(modelSelectorStylesheet).toMatch(
      /\.trigger\s*\{[^}]*border-radius:\s*var\(--radius-sm\);/s,
    );
    expect(headerStylesheet).toMatch(
      /\.balance\s*\{[^}]*border-radius:\s*var\(--radius-sm\);/s,
    );
    expect(headerStylesheet).toMatch(
      /\.tariffButton\s*\{[^}]*border-radius:\s*var\(--radius-sm\);/s,
    );
  });

  it("keeps the model selector and balance transparent inside", () => {
    const modelTriggerRule = modelSelectorStylesheet.match(/\.trigger\s*\{([^}]*)\}/s)?.[1] ?? "";
    const balanceRule = headerStylesheet.match(/\.balance\s*\{([^}]*)\}/s)?.[1] ?? "";

    expect(modelTriggerRule).toContain("background: transparent");
    expect(balanceRule).toContain("background: transparent");
  });

  it("keeps subscription plans in a responsive, internally scrollable modal", () => {
    expect(modalBackdropStylesheet).toMatch(
      /\.backdrop\s*\{[^}]*position:\s*fixed;[^}]*inset:\s*0;/s,
    );
    expect(plansDialogStylesheet).toMatch(
      /\.dialog\s*\{[^}]*max-block-size:\s*calc\(100dvh - 2rem\);[^}]*overflow:\s*hidden;/s,
    );
    expect(plansDialogStylesheet).toMatch(
      /\.scrollArea\s*\{[^}]*min-block-size:\s*0;/s,
    );
    expect(plansDialogStylesheet).not.toMatch(/overflow-y:\s*(?:auto|scroll)/);
    expect(plansDialogStylesheet).toMatch(
      /\.planGrid\s*\{[^}]*grid-template-columns:\s*repeat\(3, minmax\(0, 1fr\)\);/s,
    );
    expect(plansDialogStylesheet).toMatch(
      /@media \(width < 44rem\)[\s\S]*\.planGrid\s*\{[^}]*grid-template-columns:\s*minmax\(0, 1fr\);/s,
    );
  });
});
