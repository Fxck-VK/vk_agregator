import type { CSSProperties } from "react";

type Pixels = Pick<ImageData, "data" | "width" | "height">;
type Bounds = { x: number; y: number; width: number; height: number };

// Only paired, thin, opaque black bands count as padding. This deliberately
// leaves isolated dark edges, transparent artwork and large dark areas alone.
export function findPreviewContentBounds({ data, width, height }: Pixels): Bounds | null {
  if (width < 3 || height < 3 || data.length !== width * height * 4) return null;
  const black = (x: number, y: number) => {
    const offset = (y * width + x) * 4;
    return data[offset + 3] >= 250 && data[offset] <= 16 && data[offset + 1] <= 16 && data[offset + 2] <= 16;
  };
  const row = (y: number) => {
    for (let x = 0; x < width; x++) if (!black(x, y)) return false;
    return true;
  };
  const maxY = Math.floor(height * 0.2);
  let top = 0, bottom = 0;
  while (top <= maxY && row(top)) top++;
  while (bottom <= maxY && row(height - 1 - bottom)) bottom++;
  if (!top || !bottom || top > maxY || bottom > maxY) top = bottom = 0;

  const column = (x: number) => {
    for (let y = top; y < height - bottom; y++) if (!black(x, y)) return false;
    return true;
  };
  const maxX = Math.floor(width * 0.2);
  let left = 0, right = 0;
  while (left <= maxX && column(left)) left++;
  while (right <= maxX && column(width - 1 - right)) right++;
  if (!left || !right || left > maxX || right > maxX) left = right = 0;
  if (!top && !bottom && !left && !right) return null;
  return { x: left, y: top, width: width - left - right, height: height - top - bottom };
}

export function previewCropStyle(width: number, height: number, bounds: Bounds): CSSProperties {
  const side = Math.min(bounds.width, bounds.height);
  return {
    position: "absolute",
    width: `${width / side * 100}%`,
    height: `${height / side * 100}%`,
    left: `${-(bounds.x + (bounds.width - side) / 2) / side * 100}%`,
    top: `${-(bounds.y + (bounds.height - side) / 2) / side * 100}%`,
    maxWidth: "none",
  };
}

export function measurePreviewCrop(image: HTMLImageElement): CSSProperties | undefined {
  if (!image.naturalWidth || !image.naturalHeight) return;
  try {
    // Sampling is bounded regardless of the original file's resolution. Canvas
    // is only read for detection; the original image URL and upload stay intact.
    const scale = Math.min(1, 512 / Math.max(image.naturalWidth, image.naturalHeight));
    const canvas = document.createElement("canvas");
    canvas.width = Math.max(1, Math.round(image.naturalWidth * scale));
    canvas.height = Math.max(1, Math.round(image.naturalHeight * scale));
    const context = canvas.getContext("2d", { willReadFrequently: true });
    if (!context) return;
    context.drawImage(image, 0, 0, canvas.width, canvas.height);
    const bounds = findPreviewContentBounds(context.getImageData(0, 0, canvas.width, canvas.height));
    if (bounds) return previewCropStyle(canvas.width, canvas.height, bounds);
  } catch {
    // If pixels are unreadable, retain the normal image preview.
  }
}
