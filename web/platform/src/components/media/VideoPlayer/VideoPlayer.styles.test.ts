import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const stylesheet = readFileSync(
  resolve(process.cwd(), "src/components/media/VideoPlayer/VideoPlayer.module.css"),
  "utf8",
);

describe("VideoPlayer layout", () => {
  it("fills the existing frame with the configured video", () => {
    const frameRule = stylesheet.match(/\.frame\s*\{[^}]*\}/s)?.[0] ?? "";
    const videoRule = stylesheet.match(/\.video\s*\{[^}]*\}/s)?.[0] ?? "";

    expect(frameRule).toContain("position: relative");
    expect(videoRule).toContain("position: absolute");
    expect(videoRule).toContain("inset: 0");
    expect(videoRule).toContain("inline-size: 100%");
    expect(videoRule).toContain("block-size: 100%");
    expect(videoRule).toContain("object-fit: contain");
  });
});
