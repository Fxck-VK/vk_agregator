import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const stylesheet = readFileSync(
  resolve(process.cwd(), "src/components/chat/ChatTextInput/ChatTextInput.module.css"),
  "utf8",
);

describe("ChatTextInput scroll layout", () => {
  it("reserves an inline gutter so text never overlaps the vertical scrollbar", () => {
    expect(stylesheet).toMatch(
      /\.root\s*\{[^}]*--chat-text-input-scrollbar-gutter:\s*1\.25rem;[^}]*inline-size:\s*100%;/s,
    );
    expect(stylesheet).toMatch(
      /\.input\s*\{[^}]*padding-inline-end:\s*calc\(var\(--space-3\) \+ var\(--chat-text-input-scrollbar-gutter\) \+ var\(--chat-text-input-action-gutter\)\);/s,
    );
    expect(stylesheet).toMatch(
      /\.composer\s*\{[^}]*padding-inline-end:\s*calc\(var\(--chat-text-input-scrollbar-gutter\) \+ var\(--chat-text-input-action-gutter\)\);/s,
    );
    expect(stylesheet).toMatch(
      /\.scrollArea\s*\{[^}]*--scroll-area-vertical-track-inline-end:\s*calc\(var\(--chat-text-input-expand-control-size\) \+ 0\.75rem\);/s,
    );
  });

  it("grows through nine rows and reserves room for the expand action", () => {
    expect(stylesheet).toContain("--chat-text-input-max-auto-rows: 9;");
    expect(stylesheet).toContain("--chat-text-input-action-gutter: 2.75rem;");
    expect(stylesheet).toMatch(
      /\.root\[data-manually-expanded="true"\]\s+\.scrollArea\s*\{[^}]*block-size:\s*clamp\(18rem,\s*60vh,\s*40rem\);/s,
    );
    expect(stylesheet).toMatch(
      /\.expandButton\s*\{[^}]*position:\s*absolute;[^}]*inset-block-start:\s*0\.5rem;[^}]*inset-inline-end:\s*calc\(\(var\(--input-control-size, 2\.5rem\) - var\(--chat-text-input-expand-control-size\)\) \/ 2\);/s,
    );
    expect(stylesheet).toContain("--chat-text-input-expand-control-size: 2rem;");
  });

  it("reduces every initial input height by forty percent", () => {
    expect(stylesheet).toMatch(
      /\.compact\s*\{[^}]*--chat-text-input-min-height:\s*3rem;/s,
    );
    expect(stylesheet).toMatch(
      /\.expanded\s*\{[^}]*--chat-text-input-min-height:\s*4\.8rem;/s,
    );
    expect(stylesheet).toMatch(
      /\.scrollArea\[data-appearance="composer"\]\s*\{[^}]*--chat-text-input-min-height:\s*1\.95rem;/s,
    );
    expect(stylesheet).toMatch(
      /\.scrollArea\[data-appearance="composer"\]\.expanded\s*\{[^}]*--chat-text-input-min-height:\s*3\.6rem;/s,
    );
  });

  it("animates only manual opening and closing and crossfades the icon", () => {
    expect(stylesheet).toMatch(
      /\.scrollArea\s*\{[^}]*transition:\s*block-size var\(--chat-text-input-manual-transition-duration, 0ms\) cubic-bezier\(0\.22, 1, 0\.36, 1\);/s,
    );
    expect(stylesheet).toMatch(
      /\.root\[data-manual-transitioning="true"\]\s*\{[^}]*--chat-text-input-manual-transition-duration:\s*360ms;/s,
    );
    expect(stylesheet).toMatch(
      /\.expandIcon path\s*\{[^}]*transition:[^}]*opacity 220ms ease,[^}]*transform 360ms cubic-bezier\(0\.22, 1, 0\.36, 1\);/s,
    );
    expect(stylesheet).toMatch(
      /\.expandButton\[data-state="expanded"\]\s+\.expandGlyph\s*\{[^}]*opacity:\s*0;/s,
    );
    expect(stylesheet).toMatch(
      /\.expandButton\[data-state="expanded"\]\s+\.collapseGlyph\s*\{[^}]*opacity:\s*1;/s,
    );
    expect(stylesheet).toMatch(
      /@media \(prefers-reduced-motion: reduce\)\s*\{[^}]*\.scrollArea,[^}]*\.expandIcon path\s*\{[^}]*transition:\s*none;/s,
    );
  });
});
