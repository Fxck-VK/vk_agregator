import type { ImageModel } from "@/lib/web-api/contracts";
import { imagePrice } from "@/features/models/image-price";
import referencePricesPreview from "@/features/session/model-reference-prices.preview.json";

const referencePrices: Readonly<Record<string, Readonly<Record<string, number>>>> = referencePricesPreview;

type LocalImageSelection = {
  model_id?: string;
  image_quality?: string;
  aspect_ratio?: string;
  output_count?: number;
  reference_count?: number;
};

// The zero-reference catalogue and the reference matrix are generated together
// by Go. This dev-only adapter never reproduces provider cost arithmetic.
export function localImagePrice(models: readonly ImageModel[] | undefined, selection: LocalImageSelection): number | undefined {
  const model = models?.find(item => item.id === selection.model_id);
  if (!model) return undefined;
  const quality = selection.image_quality ?? model.default_quality;
  const ratio = selection.aspect_ratio ?? model.default_aspect_ratio ?? "1:1";
  const count = selection.output_count ?? 1, refs = selection.reference_count ?? 0;
  if (!Number.isSafeInteger(count) || count < 1 || count > (model.max_output_count ?? 1)
    || !Number.isSafeInteger(refs) || refs < 0 || refs > (model.max_reference_images ?? 0)
    || (refs > 0 && !model.supports_reference_image)
    || !model.quality_options.includes(quality) || !model.allowed_aspect_ratios?.includes(ratio)) return undefined;
  const price = refs === 0
    ? imagePrice(model, quality, ratio, count)
    : referencePrices[model.id]?.[`${quality}:${ratio}:${refs}:${count}`];
  return price !== undefined && Number.isSafeInteger(price) && price > 0 ? price : undefined;
}
