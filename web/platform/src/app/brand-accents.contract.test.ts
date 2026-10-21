import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const read = (path: string) => readFileSync(resolve(process.cwd(), path), "utf8");

describe("restrained brand accents", () => {
  it.each([
    "src/components/public/PublicHeader/PublicHeader.module.css",
    "src/components/public/PublicFooter/PublicFooter.module.css",
  ])("sizes the shared logo without painting over its artwork in %s", (path) => {
    const rule = read(path).match(/\.brandMark\s*\{[^}]*\}/s)?.[0] ?? "";
    const size = rule.match(/inline-size:\s*([\d.]+rem)/)?.[1];
    expect(size).toBeDefined();
    expect(rule).toContain(`block-size: ${size}`);
    expect(rule).not.toMatch(/background:|color:/);
  });

  it("uses gradient text only for the landing hero accent and balance", () => {
    expect(read("src/features/workspace/WorkspaceLanding/WorkspaceLanding.module.css")).toMatch(
      /\.heroCopy h1 span\s*\{[^}]*background:\s*var\(--gradient-text, var\(--gradient-brand\)\)[^}]*background-clip:\s*text[^}]*color:\s*transparent/s,
    );
    expect(read("src/features/account/ProfileBalanceCard/ProfileBalanceCard.module.css")).toMatch(
      /\.balance strong\s*\{[^}]*background:\s*var\(--gradient-text, var\(--gradient-brand\)\)[^}]*background-clip:\s*text[^}]*color:\s*transparent/s,
    );
  });

  it("does not put the gradient on the primary landing button", () => {
    const css = read("src/features/workspace/WorkspaceLanding/WorkspaceLanding.module.css");
    const primary = css.match(/\.primaryButton\s*\{([\s\S]*?)\n\}/)?.[1] ?? "";

    expect(primary).not.toContain("--gradient-brand");
  });
});
