import { describe, expect, it } from "vitest";
import { findPreviewContentBounds, previewCropStyle } from "./preview-crop";

function pixels(width = 100, height = 100, left = 0, top = 6, right = 0, bottom = 4) {
  const data = new Uint8ClampedArray(width * height * 4);
  for (let y = 0; y < height; y++) for (let x = 0; x < width; x++) {
    const offset = (y * width + x) * 4;
    const inside = x >= left && x < width - right && y >= top && y < height - bottom;
    data[offset] = data[offset + 1] = data[offset + 2] = inside ? 245 : 0;
    data[offset + 3] = 255;
  }
  return { data, width, height };
}

describe("attachment preview content bounds", () => {
  it("hides paired black bars while retaining the original pixel data", () => {
    const image = pixels(); const original = image.data.slice();
    expect(findPreviewContentBounds(image)).toEqual({ x: 0, y: 6, width: 100, height: 90 });
    expect(image.data).toEqual(original);
  });

  it("finds the inner content of a black frame", () => {
    expect(findPreviewContentBounds(pixels(100, 100, 4, 6, 5, 4))).toEqual({ x: 4, y: 6, width: 91, height: 90 });
  });

  it("preserves ordinary content, isolated dark edges and entirely dark images", () => {
    expect(findPreviewContentBounds(pixels(100, 100, 0, 0, 0, 0))).toBeNull();
    expect(findPreviewContentBounds(pixels(100, 100, 0, 8, 0, 0))).toBeNull();
    expect(findPreviewContentBounds(pixels(100, 100, 0, 50, 0, 50))).toBeNull();
    expect(findPreviewContentBounds(pixels(100, 100, 0, 25, 0, 25))).toBeNull();
  });

  it("does not interpret textured dark content as a flat black margin", () => {
    const image = pixels();
    for (let y = 0; y < 100; y++) image.data[(y * 100 + 50) * 4] = 48;
    expect(findPreviewContentBounds(image)).toBeNull();
  });

  it("does not treat transparency as a black band", () => {
    const image = pixels();
    for (let x = 0; x < 100; x++) image.data[x * 4 + 3] = 0;
    expect(findPreviewContentBounds(image)).toBeNull();
  });

  it("covers the square with the inner area, keeping the source aspect ratio", () => {
    const style = previewCropStyle(200, 220, { x: 0, y: 10, width: 200, height: 200 });
    expect(style).toMatchObject({ position: "absolute", width: "100%", left: "0%", top: "-5%" });
    expect(parseFloat(style.height as string)).toBeCloseTo(110);
  });
});
