import { describe, expect, it } from "vitest";

import { assetPaths } from "./asset-paths";

describe("assetPaths", () => {
  it("exposes a stable inspiration image URL without eager imports", () => {
    expect(assetPaths.images.inspiration.paperCraneCloud).toBe(
      "/assets/images/inspiration/paper-crane-cloud.png",
    );
  });
});
