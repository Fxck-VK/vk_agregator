import type { ChatModel, ImageModel } from "@/lib/web-api/contracts";
import type { VideoModel } from "@/features/models/video-model-catalog";

// Test fixture adapter only. Production model facts come from /web/v1/models.
export function imageModelFixture(model: ImageModel): ImageModel {
  const ratios = model.allowed_aspect_ratios ?? ["16:9", "1:1", "9:16"];
  const prices = model.price_by_quality ?? Object.fromEntries(model.quality_options.map(quality => [quality, 1]));
  return {
    ...model,
    categories: model.categories ?? ["popular", "images"],
    description: model.description ?? `${model.name}: описание из каталога`,
    allowed_aspect_ratios: ratios,
    default_aspect_ratio: model.default_aspect_ratio ?? ratios[0],
    max_output_count: model.max_output_count ?? 1,
    price_by_quality: prices,
    price_by_variant: model.price_by_variant ?? Object.fromEntries(model.quality_options.flatMap(quality => ratios.map(ratio => [`${quality}:${ratio}`, prices[quality]]))),
    quality_label: model.quality_label ?? "Разрешение",
    show_output_count: model.show_output_count ?? true,
  };
}

export function makeModelCatalogFixture({ images = [], text = [], video = [], defaultModelId }: {
  images?: ImageModel[];
  text?: ChatModel[];
  video?: VideoModel[];
  defaultModelId?: string;
} = {}) {
  const inputs = () => ({ images: { support: "unknown", enabled: false }, video: { support: "unknown", enabled: false }, audio: { support: "unknown", enabled: false }, documents: { support: "unknown", enabled: false }, max_total_bytes: 0 });
  const items = [
    ...images.map(source => {
      const { id, name, description, categories, ...image } = imageModelFixture(source);
      return { id, name, description, categories, kind: "image", verification: "legacy-unverified", operations: [{ id: "generate", kind: "image", enabled: true, inputs: inputs(), image }] };
    }),
    ...text.map(({ id, name, description, categories, estimate_credits = 0, max_prompt_bytes, max_output_tokens }) => ({ id, name, description: description ?? `${name}: описание из каталога`, categories: categories ?? ["popular", "text", "study-work", ...(estimate_credits === 0 ? ["free"] : [])], kind: "text", verification: "legacy-unverified", operations: [{ id: "reply", kind: "text", enabled: true, inputs: inputs(), text: { estimate_credits, max_prompt_bytes, max_output_tokens } }] })),
    ...video.map(({ id, name, description, categories, variants, start_image, end_image, ...options }) => ({ id, name, description, categories: categories ?? ["popular", "video-audio"], kind: "video", verification: "legacy-unverified", operations: [{ id: "generate", kind: "video", enabled: true, inputs: inputs(), video: { ...options, start_image: start_image ?? "unsupported", end_image: end_image ?? "unsupported", variants: variants ?? options.allowed_resolutions.flatMap(resolution => options.allowed_durations_sec.filter(duration_sec => options.price_by_option[`${resolution}:${duration_sec}`] > 0).flatMap(duration_sec => options.allowed_aspect_ratios.map(aspect_ratio => ({ resolution, duration_sec, aspect_ratio, fps: 0, audio: false })))) } }] })),
  ];
  return { schema_version: 1, default_model_id: defaultModelId ?? text[0]?.id ?? items[0]?.id ?? "", items };
}
