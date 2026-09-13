import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const stylesheet = readFileSync(
  resolve(process.cwd(), "src/components/ui/ScrollArea/ScrollArea.module.css"),
  "utf8",
);

describe("ScrollArea styles", () => {
  it("hides native scrollbar chrome without disabling native overflow", () => {
    expect(stylesheet).toMatch(/\.viewport\s*\{[^}]*overflow:\s*auto;[^}]*scrollbar-width:\s*none;/s);
    expect(stylesheet).toMatch(/\.viewport::-webkit-scrollbar\s*\{[^}]*display:\s*none;/s);
  });

  it("floats one branded thumb over the inline edge of every surface", () => {
    expect(stylesheet).toMatch(/\.frame\s*\{[^}]*position:\s*relative;[^}]*overflow:\s*hidden;/s);
    expect(stylesheet).toMatch(/\.track\s*\{[^}]*position:\s*absolute;[^}]*opacity:\s*0;/s);
    expect(stylesheet).toMatch(
      /\[data-orientation="vertical"\].*\.track\s*\{[^}]*inset-inline-end:\s*var\(--scroll-area-vertical-track-inline-end,\s*0\.25rem\);/s,
    );
    expect(stylesheet).toMatch(/\.thumb\s*\{[^}]*background:\s*color-mix\(in srgb, var\(--color-accent\)[^;]*;/s);
    expect(stylesheet).toMatch(/\[data-scrollable="true"\].*:hover.*\.track,[\s\S]*\[data-scrolling="true"\].*\.track,[\s\S]*\[data-dragging="true"\].*\.track\s*\{[^}]*opacity:\s*1;/s);
  });

  it("reserves a slim gutter below horizontal content", () => {
    expect(stylesheet).toMatch(
      /\[data-orientation="horizontal"\]\[data-scrollable="true"\].*\.frame\s*\{[^}]*padding-block-end:\s*0\.75rem;/s,
    );
    expect(stylesheet).toMatch(
      /\[data-orientation="horizontal"\].*\.track\s*\{[^}]*inset-block-end:\s*0\.125rem;[^}]*block-size:\s*0\.25rem;/s,
    );
    expect(stylesheet).toMatch(
      /\[data-orientation="horizontal"\].*\.thumb\s*\{[^}]*inline-size:\s*var\(--scroll-area-thumb-size,[^)]+\);[^}]*translateX\(var\(--scroll-area-thumb-offset,[^)]+\)\);/s,
    );
  });

  it("can place a horizontal track below the content without lifting it", () => {
    expect(stylesheet).toMatch(
      /\.root\[data-orientation="horizontal"\]\[data-track-placement="outside"\]\s*\{[^}]*overflow:\s*visible;/s,
    );
    expect(stylesheet).toMatch(
      /\.root\[data-orientation="horizontal"\]\[data-track-placement="outside"\] > \.frame\s*\{[^}]*overflow:\s*visible;[^}]*padding-block-end:\s*0;/s,
    );
    expect(stylesheet).toMatch(
      /\.root\[data-orientation="horizontal"\]\[data-track-placement="outside"\] > \.frame > \.track\s*\{[^}]*inset-block-end:\s*-0\.625rem;/s,
    );
  });

  it("can place a vertical track beside content without narrowing it", () => {
    expect(stylesheet).toMatch(
      /\.root\[data-orientation="vertical"\]\[data-track-placement="outside"\] > \.frame\s*\{[^}]*overflow:\s*visible;/s,
    );
    expect(stylesheet).toMatch(
      /\.root\[data-orientation="vertical"\]\[data-track-placement="outside"\] > \.frame > \.track\s*\{[^}]*inset-inline-end:\s*-0\.75rem;/s,
    );
  });

  it("scopes visibility state to its own track when scroll areas are nested", () => {
    expect(stylesheet).toMatch(/\.root\[data-scrollable="false"\] > \.frame > \.track/);
    expect(stylesheet).toMatch(/\.root\[data-scrollable="true"\]:hover > \.frame > \.track/);
    expect(stylesheet).not.toMatch(/\.root\[data-scrollable="false"\] \.track/);
  });

  it("removes the fade transition for reduced motion", () => {
    expect(stylesheet).toMatch(/@media \(prefers-reduced-motion: reduce\)\s*\{[^}]*\.track\s*\{[^}]*transition:\s*none;/s);
  });
});
