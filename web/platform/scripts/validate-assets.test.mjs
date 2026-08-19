import assert from "node:assert/strict";
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
