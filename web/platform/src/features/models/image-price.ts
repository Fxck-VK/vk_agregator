import type { ImageModel } from "@/lib/web-api/contracts";

type ImagePriceModel = Pick<ImageModel, "price_by_quality" | "price_by_variant" | "price_by_count">;

export function imagePrice(model: ImagePriceModel | null | undefined, quality: string, ratio: string, count: number): number | undefined {
  if (!model || !Number.isSafeInteger(count) || count < 1) return undefined;
  if (model.price_by_count) return model.price_by_count[`${quality}:${ratio}:${count}`];
  const unit = model.price_by_variant ? model.price_by_variant[`${quality}:${ratio}`] : model.price_by_quality?.[quality];
  const total = unit === undefined ? undefined : unit * count;
  return total !== undefined && Number.isSafeInteger(total) && total > 0 ? total : undefined;
}
