import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

import { inspectAssetEntries } from "./validate-assets.mjs";

test("accepts named raster assets and safe SVG", () => {
  assert.deepEqual(
    inspectAssetEntries([
      { relativePath: "public/assets/images/example-card.webp" },
      {
        relativePath: "public/assets/icons/models/model-mark.svg",
        content: '<svg xmlns="http://www.w3.org/2000/svg"><path d="M0 0" /></svg>',
      },
    ]),
    [],
  );
});

test("accepts a kebab-case MP4 video asset", () => {
  assert.deepEqual(
    inspectAssetEntries([
      { relativePath: "public/assets/videos/workspace/example-video.mp4" },
    ]),
    [],
  );
});

test("accepts a local editorial WebM clip", () => {
  assert.deepEqual(inspectAssetEntries([
    { relativePath: "public/assets/videos/marketing/creative-suite-image.webm" },
  ]), []);
});

test("accepts local WOFF2 fonts with their license and source metadata", () => {
  assert.deepEqual(inspectAssetEntries([
    { relativePath: "public/assets/fonts/ibm-plex-sans/ibm-plex-sans-semibold.woff2", fontSignature: "wOF2" },
    { relativePath: "public/assets/fonts/ibm-plex-sans/LICENSE.txt" },
    { relativePath: "public/assets/fonts/ibm-plex-sans/SOURCE.txt" },
  ]), []);
});

test("rejects disguised font files and keeps font metadata scoped to fonts", () => {
  const errors = inspectAssetEntries([
    { relativePath: "public/assets/fonts/fake.woff2", fontSignature: "<htm" },
    { relativePath: "public/assets/images/font.woff2", fontSignature: "wOF2" },
    { relativePath: "public/assets/images/LICENSE.txt" },
  ]);
  assert.equal(errors.length, 3);
  assert.equal(errors.filter(error => error.includes("valid font signature")).length, 2);
  assert.ok(errors.some(error => error.includes("unsupported extension .txt")));
});

test("rejects invalid names, extensions, duplicates and unsafe SVG", () => {
  const errors = inspectAssetEntries([
    { relativePath: "public/assets/images/Icon 1.png" },
    { relativePath: "public/assets/images/photo.jpg" },
    { relativePath: "public/assets/images/card.webp" },
    { relativePath: "PUBLIC/ASSETS/IMAGES/CARD.WEBP" },
    {
      relativePath: "public/assets/icons/models/unsafe.svg",
      content: '<svg onload="alert(1)"><script>alert(1)</script></svg>',
    },
  ]);

  assert.ok(errors.some((error) => error.includes("kebab-case")));
  assert.ok(errors.some((error) => error.includes("unsupported extension")));
  assert.ok(errors.some((error) => error.includes("duplicate normalized path")));
  assert.ok(errors.some((error) => error.includes("unsafe SVG")));
});

test("keeps the upload-media icon transparent and uses the approved artwork", () => {
  const icon = readFileSync(new URL("../public/assets/icons/ui/upload-media.svg", import.meta.url), "utf8");

  assert.doesNotMatch(icon, /<rect\b/);
  assert.match(icon, /stroke-width="6\.5"/);
  assert.match(icon, /M111 35V65/);
});

test("uses the approved left-aligned play mark for the animate icon", () => {
  const icon = readFileSync(new URL("../public/assets/icons/ui/animate-white.svg", import.meta.url), "utf8");

  assert.match(icon, /M8\.83 8\.8/);
  assert.doesNotMatch(icon, /M10 8\.8/);
});

// Browser-icon dimensions are checked against config/brand.ts in app/layout.test.tsx.
