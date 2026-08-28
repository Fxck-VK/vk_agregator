import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const read = (path: string) => readFileSync(resolve(process.cwd(), path), "utf8");

describe("restrained brand accents", () => {
  it.each([
    "src/components/public/PublicHeader/PublicHeader.module.css",
    "src/components/public/PublicFooter/PublicFooter.module.css",
  ])("uses the brand gradient for the compact mark in %s", (path) => {
    expect(read(path)).toMatch(/\.brandMark\s*\{[^}]*background:\s*var\(--gradient-brand\)/s);
  });

  it("uses gradient text only for the landing hero accent and balance", () => {
    expect(read("src/features/workspace/WorkspaceLanding/WorkspaceLanding.module.css")).toMatch(
      /\.heroCopy h1 span\s*\{[^}]*background:\s*var\(--gradient-brand\)[^}]*background-clip:\s*text[^}]*color:\s*transparent/s,
    );
    expect(read("src/features/account/ProfileBalanceCard/ProfileBalanceCard.module.css")).toMatch(
      /\.balance strong\s*\{[^}]*background:\s*var\(--gradient-brand\)[^}]*background-clip:\s*text[^}]*color:\s*transparent/s,
    );
  });

  it("does not put the gradient on the primary landing button", () => {
    const css = read("src/features/workspace/WorkspaceLanding/WorkspaceLanding.module.css");
    const primary = css.match(/\.primaryButton\s*\{([\s\S]*?)\n\}/)?.[1] ?? "";

    expect(primary).not.toContain("--gradient-brand");
  });
});
