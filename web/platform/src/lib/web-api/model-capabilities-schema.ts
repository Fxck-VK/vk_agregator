import { z } from "zod";

const strings = z.array(z.string().max(160)).max(64).nullable();
const integers = z.array(z.number().int().nonnegative()).max(120).nullable();
const maximum = z.number().int().nonnegative().nullable();
const input = z.object({ support: z.enum(["supported", "unsupported", "unknown"]), extensions: strings, max_count: maximum }).strict();
const duration = z.object({
  mode: z.enum(["selected", "automatic", "reference_video", "unknown"]),
  min_seconds: maximum, max_seconds: maximum, allowed_seconds: integers,
  by_resolution: z.record(z.string(), z.array(z.number().int().nonnegative())).optional(),
  max_by_orientation: z.record(z.string(), z.number().int().nonnegative()).optional(),
}).strict();
const frame = z.enum(["required", "optional", "unsupported", "unknown"]);
const profile = z.object({
  text: z.object({ images: input, videos: input, files: input }).strict().optional(),
  image: z.object({ images: input, aspect_ratios: strings, resolutions: strings, quality_modes: strings, speed_modes: strings, max_output_count: maximum, max_combined_images: maximum }).strict().optional(),
  video: z.object({ images: input, videos: input, allowed_image_counts: integers, duration, resolutions: strings, quality_modes: strings, aspect_ratios: strings,
    audio: z.object({ mode: z.enum(["optional", "generated", "silent", "preserve_source", "unknown"]), selectable: z.boolean() }).strict(), start_frame: frame, end_frame: frame }).strict().optional(),
  audio: z.object({ audio: input, videos: input, output: z.string().max(100) }).strict().optional(),
  notes: z.array(z.string().max(2000)).max(20).optional(),
}).strict().refine((p) => [p.text, p.image, p.video, p.audio].filter(Boolean).length === 1, { message: "Exactly one model purpose is required" });
export const modelCapabilitiesSchema = z.object({ schema_version: z.literal(1), api: profile, application: profile }).strict()
  .refine((c) => (["text", "image", "video", "audio"] as const).every((kind) => Boolean(c.api[kind]) === Boolean(c.application[kind])), { message: "API and application must describe the same model purpose" });
