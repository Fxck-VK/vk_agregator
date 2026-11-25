import { resolveImageResolution } from "@/features/image-generation/image-resolution-options";
import { imagePrice } from "@/features/models/image-price";
import type { ImageModel } from "@/lib/web-api/contracts";

export type SlideImageFrame = { x: number; y: number; width: number; height: number };
export type SlideImageFrameGesture = "new" | "move" | "nw" | "ne" | "sw" | "se";
type Point = Pick<SlideImageFrame, "x" | "y">;

export const slideImageFrameScale = 10_000;
const minEdge = 500;
export const defaultSlideImageFrame: SlideImageFrame = { x: 5300, y: 0, width: 4700, height: slideImageFrameScale };

const clamp = (value: number, min: number, max: number) => Math.max(min, Math.min(max, Math.round(value)));

export function isSlideImageFrame({ x, y, width, height }: SlideImageFrame): boolean {
  return [x, y, width, height].every(Number.isInteger) && x >= 0 && y >= 0 && width >= minEdge && height >= minEdge
    && x + width <= slideImageFrameScale && y + height <= slideImageFrameScale;
}

export function slideImageFramePoint(x: number, y: number, bounds: { left: number; top: number; width: number; height: number }): Point {
  return {
    x: clamp((x - bounds.left) / (bounds.width || 1) * slideImageFrameScale, 0, slideImageFrameScale),
    y: clamp((y - bounds.top) / (bounds.height || 1) * slideImageFrameScale, 0, slideImageFrameScale),
  };
}

export function setSlideImageFrameField(frame: SlideImageFrame, field: keyof SlideImageFrame, value: number): SlideImageFrame {
  if (!Number.isFinite(value)) return frame;
  switch (field) {
    case "x": return { ...frame, x: clamp(value, 0, slideImageFrameScale - frame.width) };
    case "y": return { ...frame, y: clamp(value, 0, slideImageFrameScale - frame.height) };
    case "width": return { ...frame, width: clamp(value, minEdge, slideImageFrameScale - frame.x) };
    case "height": return { ...frame, height: clamp(value, minEdge, slideImageFrameScale - frame.y) };
  }
}

export function dragSlideImageFrame(frame: SlideImageFrame, mode: SlideImageFrameGesture, start: Point, end: Point): SlideImageFrame {
  const dx = end.x - start.x;
  const dy = end.y - start.y;
  if (mode === "new") {
    const width = Math.max(minEdge, Math.abs(dx));
    const height = Math.max(minEdge, Math.abs(dy));
    return {
      x: clamp(Math.min(start.x, end.x), 0, slideImageFrameScale - width),
      y: clamp(Math.min(start.y, end.y), 0, slideImageFrameScale - height),
      width, height,
    };
  }
  if (mode === "move") {
    return setSlideImageFrameField(setSlideImageFrameField(frame, "x", frame.x + dx), "y", frame.y + dy);
  }
  const fromLeft = mode.endsWith("w");
  const fromTop = mode.startsWith("n");
  const right = frame.x + frame.width;
  const bottom = frame.y + frame.height;
  const x = fromLeft ? clamp(frame.x + dx, 0, right - minEdge) : frame.x;
  const y = fromTop ? clamp(frame.y + dy, 0, bottom - minEdge) : frame.y;
  return {
    x, y,
    width: (fromLeft ? right : clamp(right + dx, x + minEdge, slideImageFrameScale)) - x,
    height: (fromTop ? bottom : clamp(bottom + dy, y + minEdge, slideImageFrameScale)) - y,
  };
}

export function slideImageFrameCSS(frame: SlideImageFrame) {
  const pct = (value: number) => `${value * 100 / slideImageFrameScale}%`;
  return { left: pct(frame.x), top: pct(frame.y), width: pct(frame.width), height: pct(frame.height) };
}

export function slideImageFrameAspectRatio(frame: SlideImageFrame): number {
  return 16 * frame.width / (9 * frame.height);
}

export function nearestSlideImageRatio(frame: SlideImageFrame, model: ImageModel): string | undefined {
  const quality = resolveImageResolution(model.quality_options, "", model.default_quality);
  if (!quality) return undefined;
  const target = slideImageFrameAspectRatio(frame);
  let ratio: string | undefined;
  let distance = Infinity;
  for (const option of model.allowed_aspect_ratios ?? []) {
    const [width, height] = option.split(":").map(Number);
    if (!(width > 0 && height > 0) || model.price_by_variant && !model.price_by_variant[`${quality}:${option}`]
      || !imagePrice(model, quality, option, 1)) continue;
    const next = Math.abs(Math.log((width / height) / target));
    if (next < distance) {
      ratio = option;
      distance = next;
    }
  }
  return ratio;
}
