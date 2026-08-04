import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

const read = (path: string) => readFileSync(resolve(process.cwd(), path), "utf8");

describe("public homepage responsive style contract", () => {
  it("keeps the desktop shell fixed and removes its offset on mobile", () => {
    const sidebar = read("src/features/landing/PublicSidebar/PublicSidebar.module.css");
    const shell = read("src/features/landing/PublicShell/PublicShell.module.css");
    const header = read("src/features/landing/PublicHeader/PublicHeader.module.css");

    expect(sidebar).toContain("position: fixed");
    expect(shell).toContain("margin-inline-start: var(--sidebar-width)");
    expect(shell).toMatch(/@media \(max-width: 47\.99rem\)[\s\S]*margin-inline-start: 0/);
    expect(header).toContain("position: sticky");
  });

  it("uses horizontal scroll-snap for dense mobile rows and respects reduced motion", () => {
    const tools = read("src/features/landing/QuickTools/QuickTools.module.css");
    const capabilities = read("src/features/landing/CapabilitiesCarousel/CapabilitiesCarousel.module.css");
    const globals = read("src/app/globals.css");

    expect(tools).toContain("scroll-snap-type: x mandatory");
    expect(capabilities).toContain("scroll-snap-type: x mandatory");
    expect(globals).toContain("prefers-reduced-motion: reduce");
  });

  it("defines dark, light and system-aware color contracts", () => {
    const globals = read("src/app/globals.css");
    expect(globals).toContain(':root[data-theme="dark"]');
    expect(globals).toContain(':root[data-theme="light"]');
    expect(globals).toContain(':root[data-theme="system"]');
  });

  it("keeps a visible keyboard focus indicator on the primary composer", () => {
    const composer = read("src/features/landing/HeroComposer/HeroComposer.module.css");

    expect(composer).toMatch(/\.promptField textarea:focus-visible[\s\S]*outline:/);
  });
});
