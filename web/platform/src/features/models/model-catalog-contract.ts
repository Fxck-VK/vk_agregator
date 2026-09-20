import { z } from "zod";
import { modelCapabilitiesSchema } from "@/lib/web-api/model-capabilities-schema";

import {
  parseChatModelList,
  parseImageModelList,
  type ChatModelList,
  type ImageModelList,
} from "@/lib/web-api/contracts";

const nonEmptyString = z.string().trim().min(1);
const positiveInt = z.number().int().positive();
const nonNegativeInt = z.number().int().nonnegative();
const positivePriceByKey = z.record(nonEmptyString, positiveInt);

function addUniqueStringIssues(values: readonly string[], ctx: z.RefinementCtx, label: string) {
  if (new Set(values).size !== values.length) {
    ctx.addIssue({ code: "custom", message: `${label} must be unique.` });
  }
}

function uniqueStringArray(label: string) {
  return z.array(nonEmptyString).min(1).superRefine((values, ctx) => addUniqueStringIssues(values, ctx, label));
}

const publicCategorySchema = z.enum(["popular", "images", "text", "video-audio", "free", "study-work"]);
const publicKindSchema = z.enum(["text", "image", "video", "audio"]);
const supportSchema = z.enum(["unknown", "unsupported", "supported"]);

const fileFormatSchema = z.object({
  extension: nonEmptyString,
  mime: nonEmptyString,
}).strict();

const publicInputSchema = z.object({
  support: supportSchema,
  enabled: z.boolean(),
  required: z.boolean().optional(),
  processing: nonEmptyString.optional(),
  formats: z.array(fileFormatSchema).optional(),
  max_count: nonNegativeInt.optional(),
  allowed_counts: z.array(nonNegativeInt).optional(),
  max_bytes: nonNegativeInt.optional(),
  max_pages: nonNegativeInt.optional(),
  max_duration_sec: nonNegativeInt.optional(),
  max_width: nonNegativeInt.optional(),
  max_height: nonNegativeInt.optional(),
}).strict();

const publicInputsSchema = z.object({
  images: publicInputSchema,
  video: publicInputSchema,
  audio: publicInputSchema,
  documents: publicInputSchema,
  max_total_bytes: nonNegativeInt,
}).strict();

const textControlsSchema = z.object({
  estimate_credits: z.number().int().nonnegative(),
  max_prompt_bytes: positiveInt.optional(),
  max_output_tokens: positiveInt.optional(),
  context_tokens: positiveInt.optional(),
}).strict();

const imageControlsSchema = z.object({
  quality_options: uniqueStringArray("Image qualities"),
  default_quality: nonEmptyString,
  allowed_aspect_ratios: uniqueStringArray("Image aspect ratios"),
  default_aspect_ratio: nonEmptyString,
  max_output_count: positiveInt,
  supports_reference_image: z.boolean(),
  max_reference_images: nonNegativeInt,
  price_by_quality: positivePriceByKey,
  price_by_variant: positivePriceByKey,
  quality_label: nonEmptyString,
  show_output_count: z.boolean(),
  max_prompt_bytes: positiveInt.optional(),
}).strict().superRefine((image, ctx) => {
  if (!image.quality_options.includes(image.default_quality)) {
    ctx.addIssue({ code: "custom", path: ["default_quality"], message: "Default image quality must be available." });
  }
  if (!image.allowed_aspect_ratios.includes(image.default_aspect_ratio)) {
    ctx.addIssue({ code: "custom", path: ["default_aspect_ratio"], message: "Default image aspect ratio must be available." });
  }
  for (const quality of Object.keys(image.price_by_quality)) {
    if (!image.quality_options.includes(quality)) {
      ctx.addIssue({ code: "custom", path: ["price_by_quality", quality], message: "Image quality price must use a known quality." });
    }
  }
  const pricedQualities = new Set<string>();
  const pricedRatios = new Set<string>();
  for (const variant of Object.keys(image.price_by_variant)) {
    const [quality, ...ratioParts] = variant.split(":");
    const ratio = ratioParts.join(":");
    if (!quality || !ratio) {
      ctx.addIssue({ code: "custom", path: ["price_by_variant", variant], message: "Image variant price keys must be quality:ratio." });
      continue;
    }
    if (image.quality_options.includes(quality)) {
      pricedQualities.add(quality);
    } else {
      ctx.addIssue({ code: "custom", path: ["price_by_variant", variant], message: "Image variant price must use a known quality." });
    }
    if (image.allowed_aspect_ratios.includes(ratio)) {
      pricedRatios.add(ratio);
    } else {
      ctx.addIssue({ code: "custom", path: ["price_by_variant", variant], message: "Image variant price must use a known aspect ratio." });
    }
  }
  const defaultVariant = `${image.default_quality}:${image.default_aspect_ratio}`;
  if (image.price_by_variant[defaultVariant] === undefined) {
    ctx.addIssue({ code: "custom", path: ["price_by_variant", defaultVariant], message: "Default image quality and aspect ratio must have a priced variant." });
  }
  for (const quality of image.quality_options) {
    if (!pricedQualities.has(quality)) {
      ctx.addIssue({ code: "custom", path: ["price_by_variant", quality], message: "Every offered image quality must have at least one priced variant." });
    }
  }
  for (const ratio of image.allowed_aspect_ratios) {
    if (!pricedRatios.has(ratio)) {
      ctx.addIssue({ code: "custom", path: ["price_by_variant", ratio], message: "Every offered image aspect ratio must have at least one priced variant." });
    }
  }
});

const videoVariantSchema = z.object({
  duration_sec: positiveInt,
  resolution: nonEmptyString,
  aspect_ratio: nonEmptyString,
  fps: nonNegativeInt.nullable(),
  audio: z.boolean().nullable(),
}).strict();

const videoControlsSchema = z.object({
  allowed_resolutions: uniqueStringArray("Video resolutions"),
  allowed_durations_sec: z.array(positiveInt).min(1),
  allowed_aspect_ratios: uniqueStringArray("Video aspect ratios"),
  default_resolution: nonEmptyString,
  default_duration_sec: positiveInt,
  default_aspect_ratio: nonEmptyString,
  price_by_option: positivePriceByKey,
  variants: z.array(videoVariantSchema).min(1),
  start_image: nonEmptyString,
  end_image: nonEmptyString,
}).strict().superRefine((video, ctx) => {
  if (new Set(video.allowed_durations_sec).size !== video.allowed_durations_sec.length) {
    ctx.addIssue({ code: "custom", path: ["allowed_durations_sec"], message: "Video durations must be unique." });
  }
  if (!video.allowed_resolutions.includes(video.default_resolution)) {
    ctx.addIssue({ code: "custom", path: ["default_resolution"], message: "Default video resolution must be available." });
  }
  if (!video.allowed_durations_sec.includes(video.default_duration_sec)) {
    ctx.addIssue({ code: "custom", path: ["default_duration_sec"], message: "Default video duration must be available." });
  }
  if (!video.allowed_aspect_ratios.includes(video.default_aspect_ratio)) {
    ctx.addIssue({ code: "custom", path: ["default_aspect_ratio"], message: "Default video aspect ratio must be available." });
  }
  const seenVariants = new Set<string>();
  let hasDefaultVariant = false;
  for (const variant of video.variants) {
    const variantKey = `${variant.resolution}:${variant.duration_sec}:${variant.aspect_ratio}`;
    if (seenVariants.has(variantKey)) {
      ctx.addIssue({ code: "custom", path: ["variants"], message: "Video variants must be unique." });
    }
    seenVariants.add(variantKey);
    if (!video.allowed_resolutions.includes(variant.resolution)) {
      ctx.addIssue({ code: "custom", path: ["variants"], message: "Video variant must use a known resolution." });
    }
    if (!video.allowed_durations_sec.includes(variant.duration_sec)) {
      ctx.addIssue({ code: "custom", path: ["variants"], message: "Video variant must use a known duration." });
    }
    if (!video.allowed_aspect_ratios.includes(variant.aspect_ratio)) {
      ctx.addIssue({ code: "custom", path: ["variants"], message: "Video variant must use a known aspect ratio." });
    }
    if (video.price_by_option[`${variant.resolution}:${variant.duration_sec}`] === undefined) {
      ctx.addIssue({ code: "custom", path: ["price_by_option"], message: "Every video variant must have a matching price option." });
    }
    hasDefaultVariant ||= (
      variant.resolution === video.default_resolution
      && variant.duration_sec === video.default_duration_sec
      && variant.aspect_ratio === video.default_aspect_ratio
    );
  }
  if (!hasDefaultVariant) {
    ctx.addIssue({ code: "custom", path: ["variants"], message: "Video variants must include the default option." });
  }
  for (const option of Object.keys(video.price_by_option)) {
    const [resolution, durationText] = option.split(":");
    const duration = Number(durationText);
    if (!resolution || !Number.isInteger(duration)) {
      ctx.addIssue({ code: "custom", path: ["price_by_option", option], message: "Video price keys must be resolution:duration." });
      continue;
    }
    if (!video.variants.some((variant) => variant.resolution === resolution && variant.duration_sec === duration)) {
      ctx.addIssue({ code: "custom", path: ["price_by_option", option], message: "Video price must correspond to at least one variant." });
    }
  }
});

const audioControlsSchema = z.object({
  tasks: z.array(nonEmptyString).optional(),
  languages: z.array(nonEmptyString).optional(),
  voices: z.array(nonEmptyString).optional(),
  formats: z.array(nonEmptyString).optional(),
  max_duration_sec: positiveInt.optional(),
}).strict();

const musicControlsSchema = z.object({
  title: nonEmptyString,
  group: nonEmptyString,
  output_kind: nonEmptyString,
  supports_max: z.boolean(),
  supports_custom_model: z.boolean(),
  supports_persona: z.boolean().default(false),
  supports_audio_format: z.boolean(),
  min_sources: nonNegativeInt,
  max_sources: nonNegativeInt,
  min_uploads: nonNegativeInt,
  max_uploads: nonNegativeInt,
  estimate_credits: nonNegativeInt,
  max_estimate_credits: nonNegativeInt.optional(),
  unavailable_reason: nonEmptyString.optional(),
}).strict().superRefine((music, ctx) => {
  if (music.max_sources < music.min_sources) {
    ctx.addIssue({ code: "custom", path: ["max_sources"], message: "Music max sources must be >= min sources." });
  }
  if (music.max_uploads < music.min_uploads) {
    ctx.addIssue({ code: "custom", path: ["max_uploads"], message: "Music max uploads must be >= min uploads." });
  }
  if (music.max_estimate_credits !== undefined && music.max_estimate_credits < music.estimate_credits) {
    ctx.addIssue({ code: "custom", path: ["max_estimate_credits"], message: "Music max estimate must be >= estimate." });
  }
});

const publicOperationSchema = z.object({
  id: nonEmptyString,
  kind: publicKindSchema,
  enabled: z.boolean(),
  inputs: publicInputsSchema,
  text: textControlsSchema.optional(),
  image: imageControlsSchema.optional(),
  video: videoControlsSchema.optional(),
  audio: audioControlsSchema.optional(),
  music: musicControlsSchema.optional(),
}).strict().superRefine((operation, ctx) => {
  const outputKeys = (["text", "image", "video", "audio"] as const).filter((key) => operation[key] !== undefined);
  if (outputKeys.length !== 1) {
    ctx.addIssue({ code: "custom", message: "Operation must expose exactly one output contract." });
    return;
  }
  if (outputKeys[0] !== operation.kind) {
    ctx.addIssue({ code: "custom", path: [outputKeys[0]], message: "Operation output must match operation kind." });
  }
});

const publicCatalogModelSchema = z.object({
  capabilities: modelCapabilitiesSchema.optional(),
  id: nonEmptyString,
  name: nonEmptyString,
  description: nonEmptyString,
  kind: publicKindSchema,
  categories: z.array(publicCategorySchema).min(1),
  verification: z.enum(["legacy-unverified", "pending-verification", "verified-contract"]),
  version: nonEmptyString.optional(),
  operations: z.array(publicOperationSchema).min(1),
}).strict().superRefine((model, ctx) => {
  if (model.capabilities && (!model.capabilities.api[model.kind] || !model.capabilities.application[model.kind])) {
    ctx.addIssue({ code: "custom", path: ["capabilities"], message: "Capabilities must match the model purpose." });
  }
  addUniqueStringIssues(model.categories, ctx, "Model categories");
  const operationIDs = model.operations.map((operation) => operation.id);
  addUniqueStringIssues(operationIDs, ctx, "Model operations");
  if (model.verification !== "pending-verification" && !model.operations.some((operation) => operation.enabled && operation.kind === model.kind)) {
    ctx.addIssue({ code: "custom", path: ["operations"], message: "Model kind must have a matching enabled operation." });
  }
});

export const modelCatalogSchema = z.object({
  schema_version: z.literal(1),
  default_model_id: nonEmptyString,
  items: z.array(publicCatalogModelSchema).min(1),
}).strict().superRefine((catalog, ctx) => {
  const modelIDs = catalog.items.map((model) => model.id);
  addUniqueStringIssues(modelIDs, ctx, "Catalog model IDs");
  if (!modelIDs.includes(catalog.default_model_id)) {
    ctx.addIssue({ code: "custom", path: ["default_model_id"], message: "Model catalog default must be present." });
  }
});

export type PublicCatalog = z.infer<typeof modelCatalogSchema>;
export type PublicCatalogModel = PublicCatalog["items"][number];
export type PublicOperation = PublicCatalogModel["operations"][number];
export type PublicCatalogCategory = PublicCatalogModel["categories"][number];

export const videoModelSchema = z.object({
  capabilities: modelCapabilitiesSchema.optional(),
  id: nonEmptyString,
  name: nonEmptyString,
  description: nonEmptyString,
  categories: z.array(publicCategorySchema).optional(),
  operations: z.array(publicOperationSchema).optional(),
  allowed_resolutions: uniqueStringArray("Video model resolutions"),
  allowed_durations_sec: z.array(positiveInt).min(1),
  allowed_aspect_ratios: uniqueStringArray("Video model aspect ratios"),
  default_resolution: nonEmptyString,
  default_duration_sec: positiveInt,
  default_aspect_ratio: nonEmptyString,
  price_by_option: positivePriceByKey,
  variants: z.array(videoVariantSchema).min(1).optional(),
  start_image: nonEmptyString.optional(),
  end_image: nonEmptyString.optional(),
}).strict();

export const videoModelListSchema = z.object({
  items: z.array(videoModelSchema),
}).strict();

export type VideoModelList = z.infer<typeof videoModelListSchema>;
export type VideoModel = VideoModelList["items"][number];

export function parseModelCatalog(payload: unknown): PublicCatalog {
  return modelCatalogSchema.parse(payload);
}

function enabledOperation(model: PublicCatalogModel, kind: PublicOperation["kind"]) {
  return model.operations.find((operation) => operation.enabled && operation.kind === kind);
}

export function projectImageModelCatalog(catalog: PublicCatalog): ImageModelList {
  return parseImageModelList({
    items: catalog.items.flatMap((model) => {
      const operation = enabledOperation(model, "image");
      if (operation?.image === undefined) return [];
      return [{
        id: model.id,
        name: model.name,
        description: model.description,
        categories: model.categories,
        operations: model.operations,
        ...(model.capabilities ? { capabilities: model.capabilities } : {}),
        quality_options: operation.image.quality_options,
        price_by_quality: operation.image.price_by_quality,
        price_by_variant: operation.image.price_by_variant,
        default_quality: operation.image.default_quality,
        default_aspect_ratio: operation.image.default_aspect_ratio,
        supports_reference_image: operation.image.supports_reference_image,
        max_reference_images: operation.image.max_reference_images,
        max_output_count: operation.image.max_output_count,
        allowed_aspect_ratios: operation.image.allowed_aspect_ratios,
        quality_label: operation.image.quality_label,
        show_output_count: operation.image.show_output_count,
        max_prompt_bytes: operation.image.max_prompt_bytes,
      }];
    }),
  });
}

export function projectChatModelCatalog(catalog: PublicCatalog): ChatModelList {
  const items = catalog.items.flatMap((model) => {
    const operation = enabledOperation(model, "text");
    if (operation?.text === undefined) return [];
    return [{
      id: model.id,
      name: model.name,
      description: model.description,
      categories: model.categories,
      operations: model.operations,
        ...(model.capabilities ? { capabilities: model.capabilities } : {}),
      estimate_credits: operation.text.estimate_credits,
      max_prompt_bytes: operation.text.max_prompt_bytes,
      max_output_tokens: operation.text.max_output_tokens,
    }];
  });
  const defaultModelID = items.some((model) => model.id === catalog.default_model_id)
    ? catalog.default_model_id
    : items[0]?.id ?? "";
  return parseChatModelList({ default_model_id: defaultModelID, items });
}

export function projectVideoModelCatalog(catalog: PublicCatalog): VideoModelList {
  return videoModelListSchema.parse({
    items: catalog.items.flatMap((model) => {
      const operation = enabledOperation(model, "video");
      if (operation?.video === undefined) return [];
      return [{
        id: model.id,
        name: model.name,
        description: model.description,
        categories: model.categories,
        operations: model.operations,
        ...(model.capabilities ? { capabilities: model.capabilities } : {}),
        allowed_resolutions: operation.video.allowed_resolutions,
        allowed_durations_sec: operation.video.allowed_durations_sec,
        allowed_aspect_ratios: operation.video.allowed_aspect_ratios,
        default_resolution: operation.video.default_resolution,
        default_duration_sec: operation.video.default_duration_sec,
        default_aspect_ratio: operation.video.default_aspect_ratio,
        price_by_option: operation.video.price_by_option,
        variants: operation.video.variants,
        start_image: operation.video.start_image,
        end_image: operation.video.end_image,
      }];
    }),
  });
}
