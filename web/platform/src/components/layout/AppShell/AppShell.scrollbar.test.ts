import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const stylesheet = readFileSync(resolve(process.cwd(), "src/components/layout/AppShell/AppShell.module.css"), "utf8");
const component = readFileSync(resolve(process.cwd(), "src/components/layout/AppShell/AppShell.tsx"), "utf8");

const shellRule = stylesheet.match(/\.shell \{([\s\S]*?)\n\}/)?.[1];
const sidebarRule = stylesheet.match(/\.sidebar \{([\s\S]*?)\n\}/)?.[1];
const workspaceRule = stylesheet.match(/\.workspace \{([\s\S]*?)\n\}/)?.[1];
const workspaceScrollerRule = stylesheet.match(/\.workspaceScroller \{([\s\S]*?)\n\}/)?.[1];

describe("AppShell workspace scrollbar", () => {
  it("locks document scrolling only while the app shell is mounted", () => {
    expect(component).toContain("data-app-shell");
    expect(stylesheet).toContain(":global(html:has(body > [data-app-shell]))");
    expect(stylesheet).toContain(":global(body:has(> [data-app-shell]))");
    expect(stylesheet).toMatch(
      /:global\(html:has\(body > \[data-app-shell\]\)\),[\s\S]*:global\(body:has\(> \[data-app-shell\]\)\)\s*\{[^}]*block-size:\s*100%;[^}]*overflow:\s*hidden;/s,
    );
  });

  it("anchors the app shell to the viewport when the root document is restored to a stale scroll position", () => {
    expect(shellRule).toContain("position: fixed");
    expect(shellRule).toContain("inset: 0");
    expect(shellRule).toContain("inline-size: 100%");
  });

  it("keeps the shared workspace scroller as the only scroll owner", () => {
    expect(component).toContain('from "@/components/ui/ScrollArea/ScrollArea"');
    expect(component).toContain("<ScrollArea");
    expect(component).toContain("className={styles.workspaceScroller}");
    expect(component).toContain('viewportAs="main"');
    expect(workspaceRule).toContain("position: relative");
    expect(workspaceRule).toContain("overflow: hidden");
    expect(workspaceScrollerRule).toContain("background: var(--color-workspace)");
    expect(workspaceScrollerRule).not.toContain("margin-inline-end");
  });

  it("does not duplicate scrollbar rendering in the shell stylesheet", () => {
    expect(stylesheet).not.toContain("scrollbar-width:");
    expect(stylesheet).not.toContain("scrollbar-color:");
    expect(stylesheet).not.toContain("::-webkit-scrollbar");
  });
});

describe("AppShell workspace surface", () => {
  it("uses the panel surface for the sidebar layer and graphite behind the workspace", () => {
    expect(shellRule).toContain("--app-shell-canvas: var(--color-background)");
    expect(shellRule).toContain("background: var(--app-shell-canvas)");
    expect(sidebarRule).toContain("background: var(--color-sidebar, var(--color-panel))");
  });

  it("renders the desktop workspace flush with the vertical viewport edges", () => {
    expect(shellRule).toContain("--app-shell-edge-gap: var(--app-workspace-edge-gap)");
    expect(workspaceRule).toContain("block-size: 100dvh");
    expect(workspaceRule).toContain("margin-block: 0");
    expect(workspaceRule).toContain("margin-inline-end: var(--app-shell-edge-gap)");
    expect(workspaceRule).toContain("border-start-start-radius: var(--radius-lg)");
    expect(workspaceRule).toContain("border-end-start-radius: var(--radius-lg)");
    expect(workspaceRule).toContain("background: var(--color-workspace)");
    expect(workspaceRule).not.toContain("box-shadow:");
  });

  it("removes the floating-panel spacing and rounding on mobile", () => {
    expect(stylesheet).toMatch(
      /@media \(width < 48rem\) \{[\s\S]*?\.workspace \{[\s\S]*?block-size: 100dvh;[\s\S]*?margin: 0;[\s\S]*?border-radius: 0;/,
    );
  });
});
