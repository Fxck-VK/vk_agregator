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

test("provides square favicon assets at the declared browser sizes", () => {
  const assets = [
    ["../public/assets/brand/favicons/neirohub-favicon-32.png", 32],
    ["../public/assets/brand/favicons/neirohub-favicon-48.png", 48],
    ["../public/assets/brand/favicons/neirohub-apple-touch-icon-180.png", 180],
  ];

  for (const [relativePath, expectedSize] of assets) {
    const png = readFileSync(new URL(relativePath, import.meta.url));

    assert.deepEqual(
      [...png.subarray(0, 8)],
      [137, 80, 78, 71, 13, 10, 26, 10],
      `${relativePath} must be a PNG file`,
    );
    assert.equal(png.readUInt32BE(16), expectedSize, `${relativePath} width`);
    assert.equal(png.readUInt32BE(20), expectedSize, `${relativePath} height`);
  }
});
