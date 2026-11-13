import { z } from "zod";
export const generationOptionsSchema = z.object({
 reference_video_artifact_id:z.string().uuid().optional(),
 character_orientation:z.enum(["image","video"]).optional(),
 keep_original_sound:z.boolean().optional(),
 reference_artifact_ids:z.array(z.string().uuid()).max(16).optional(),
 image_quality:z.string().min(1).optional(), aspect_ratio:z.string().min(1).optional(),
 output_count:z.number().int().positive().optional(), resolution:z.string().min(1).optional(), duration_sec:z.number().int().positive().optional(),
}).strict();
export type GenerationOptions = z.infer<typeof generationOptionsSchema>;
