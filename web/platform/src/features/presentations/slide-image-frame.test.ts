import { describe, expect, it } from "vitest";
import type { ImageModel } from "@/lib/web-api/contracts";
import { defaultSlideImageFrame, dragSlideImageFrame, isSlideImageFrame, nearestSlideImageRatio, slideImageFrameCSS } from "./slide-image-frame";

describe("slide image geometry", () => {
  it("normalizes reverse drag and keeps resize and move gestures inside the slide", () => {
    expect(dragSlideImageFrame(defaultSlideImageFrame, "new", { x: 9000, y: 8000 }, { x: 2000, y: 3000 })).toEqual({
      x: 2000, y: 3000, width: 7000, height: 5000,
    });
    const frame = { x: 2000, y: 3000, width: 5000, height: 4000 };
    expect(dragSlideImageFrame(frame, "move", { x: 2000, y: 3000 }, { x: 9999, y: 0 })).toEqual({ ...frame, x: 5000, y: 0 });
    expect(dragSlideImageFrame(frame, "nw", { x: 2000, y: 3000 }, { x: 9999, y: 0 })).toEqual({ x: 6500, y: 0, width: 500, height: 7000 });
    expect(isSlideImageFrame({ ...frame, width: 9000 })).toBe(false);
    expect(isSlideImageFrame({ ...frame, height: Number.NaN })).toBe(false);
    expect(slideImageFrameCSS(frame)).toEqual({ left: "20%", top: "30%", width: "50%", height: "40%" });
  });

  it("uses the nearest priced ratio at an admitted default resolution, accounting for the slide aspect", () => {
    const model: ImageModel = {
      id: "example", name: "Example", quality_options: ["1K-low", "2K-low"], default_quality: "1K-low",
      allowed_aspect_ratios: ["1:1", "16:9", "4:3", "9:16"], default_aspect_ratio: "1:1",
      price_by_variant: { "1K-low:1:1": 8, "1K-low:4:3": 9, "2K-low:16:9": 18 },
      supports_reference_image: false, max_reference_images: 0,
    };
    const squareInCoordinates = { x: 0, y: 0, width: 5000, height: 5000 };
    expect(nearestSlideImageRatio(squareInCoordinates, model)).toBe("4:3");
    expect(nearestSlideImageRatio(squareInCoordinates, { ...model, price_by_variant: undefined, price_by_count: {
      "1K-low:1:1:1": 9, "1K-low:16:9:2": 25,
    } })).toBe("1:1");
  });
});
