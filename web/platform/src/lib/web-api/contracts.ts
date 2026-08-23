import { z } from "zod";

const safeIdentityRefSchema = z
  .object({
    id: z.string().uuid(),
    account_id: z.string().uuid(),
    provider: z.string().trim().min(1),
    label: z.string().trim().min(1),
    verified: z.boolean(),
    last_used_at: z.string().datetime({ offset: true }).optional(),
    created_at: z.string().datetime({ offset: true }),
  })
  .strict();

export const accountProfileSchema = z
  .object({
    account_id: z.string().uuid(),
    identity_refs: z.array(safeIdentityRefSchema),
  })
  .strict();

export type AccountProfile = z.infer<typeof accountProfileSchema>;

export const accountBalanceSchema = z
  .object({
    balance: z.number().int().nonnegative(),
  })
  .strict();

export type AccountBalance = z.infer<typeof accountBalanceSchema>;

export const conversationItemSchema = z
  .object({
    id: z.string().uuid(),
    title: z.string(),
    created_at: z.string().datetime({ offset: true }),
    updated_at: z.string().datetime({ offset: true }),
  })
  .strict();

export const conversationListSchema = z
  .object({
    items: z.array(conversationItemSchema),
  })
  .strict();

export type ConversationItem = z.infer<typeof conversationItemSchema>;
export type ConversationList = z.infer<typeof conversationListSchema>;

export const conversationMessageSchema = z
  .object({
    id: z.string().uuid(),
    seq: z.number().int().positive(),
    role: z.enum(["user", "assistant"]),
    text: z.string(),
    rating: z.enum(["like", "dislike"]).nullable().optional().default(null),
    created_at: z.string().datetime({ offset: true }),
  })
  .strict();

export const conversationMessageListSchema = z
  .object({
    items: z.array(conversationMessageSchema),
    has_more_before: z.boolean().optional().default(false),
  })
  .strict();

export type ConversationMessage = z.infer<typeof conversationMessageSchema>;
export type ConversationMessageList = z.infer<typeof conversationMessageListSchema>;
export type ConversationMessageRating = ConversationMessage["rating"];

export const conversationMessageRatingResponseSchema = z
  .object({
    rating: z.enum(["like", "dislike"]).nullable(),
  })
  .strict();

export type ConversationMessageRatingResponse = z.infer<typeof conversationMessageRatingResponseSchema>;

export const imageModelSchema = z
  .object({
    id: z.string().trim().min(1),
    name: z.string().trim().min(1),
    quality_options: z.array(z.string().trim().min(1)),
    price_by_quality: z.record(z.string().trim().min(1), z.number().int().positive()).optional(),
    default_quality: z.string().trim().min(1),
    supports_reference_image: z.boolean(),
    max_reference_images: z.number().int().nonnegative(),
    max_output_count: z.number().int().positive().optional(),
  })
  .strict();

export const imageModelListSchema = z
  .object({
    items: z.array(imageModelSchema),
  })
  .strict();

export const imageJobStatusSchema = z.enum([
  "prepared",
  "received",
  "validated",
  "rejected",
  "awaiting_payment",
  "credits_reserved",
  "queued",
  "dispatching_provider",
  "provider_submitted",
  "provider_pending",
  "provider_processing",
  "provider_succeeded",
  "provider_failed",
  "postprocessing",
  "result_ready",
  "delivering",
  "succeeded",
  "failed_retryable",
  "failed_terminal",
  "cancelled",
  "expired",
  "refunded",
]);

export const webChatJobSchema = z
  .object({
    job_id: z.string().uuid(),
    status: imageJobStatusSchema,
  })
  .strict();

export const imageJobSchema = z
  .object({
    id: z.string().uuid(),
    status: imageJobStatusSchema,
    prompt: z.string().trim().min(1),
    model_id: z.string().trim().min(1),
    model_name: z.string().trim().min(1),
    image_quality: z.string().trim().min(1),
    cost_estimate: z.number().int().positive(),
    created_at: z.string().datetime({ offset: true }),
    updated_at: z.string().datetime({ offset: true }),
  })
  .strict();

export const imageJobPreparationSchema = z
  .object({
    job: imageJobSchema,
    balance: z.number().int().nonnegative(),
    can_afford: z.boolean(),
  })
  .strict();

export const imageJobActivationSchema = z
  .object({
    job: imageJobSchema,
  })
  .strict();

export const imageJobListSchema = z
  .object({
    items: z.array(imageJobSchema),
    has_more: z.boolean(),
    next_cursor: z.string().trim().min(1).nullable(),
  })
  .refine((page) => page.has_more === (page.next_cursor !== null), {
    message: "Image job history cursor must match has_more.",
  })
  .strict();

export const imageArtifactMetadataSchema = z
  .object({
    id: z.string().uuid(),
    mime_type: z.string().trim().min(1),
    size_bytes: z.number().int().positive(),
    width: z.number().int().nonnegative(),
    height: z.number().int().nonnegative(),
  })
  .strict();

export const imageJobResultSchema = z
  .object({
    job_id: z.string().uuid(),
    status: z.literal("succeeded"),
    artifacts: z.array(imageArtifactMetadataSchema).min(1),
  })
  .strict();

export type ImageModel = z.infer<typeof imageModelSchema>;
export type ImageModelList = z.infer<typeof imageModelListSchema>;
export type ImageJob = z.infer<typeof imageJobSchema>;
export type ImageJobPreparation = z.infer<typeof imageJobPreparationSchema>;
export type ImageJobActivation = z.infer<typeof imageJobActivationSchema>;
export type ImageJobList = z.infer<typeof imageJobListSchema>;
export type ImageArtifactMetadata = z.infer<typeof imageArtifactMetadataSchema>;
export type ImageJobResult = z.infer<typeof imageJobResultSchema>;
export type WebChatJob = z.infer<typeof webChatJobSchema>;

const safeWebChatReplayStatuses = new Set<WebChatJob["status"]>([
  "received",
  "validated",
  "credits_reserved",
  "dispatching_provider",
  "provider_submitted",
  "provider_pending",
  "provider_processing",
  "provider_succeeded",
  "postprocessing",
  "result_ready",
  "delivering",
  "failed_retryable",
  "succeeded",
]);

export const publicApiErrorSchema = z
  .object({
    error: z.string().trim().min(1),
  })
  .strict();

export type PublicApiError = z.infer<typeof publicApiErrorSchema>;

export function parseAccountProfile(payload: unknown): AccountProfile {
  return accountProfileSchema.parse(payload);
}

export function parseAccountBalance(payload: unknown): AccountBalance {
  return accountBalanceSchema.parse(payload);
}

export function parseConversationList(payload: unknown): ConversationList {
  return conversationListSchema.parse(payload);
}

export function parseConversationItem(payload: unknown): ConversationItem {
  return conversationItemSchema.parse(payload);
}

export function parseConversationMessageList(payload: unknown): ConversationMessageList {
  return conversationMessageListSchema.parse(payload);
}

export function parseConversationMessageRatingResponse(payload: unknown): ConversationMessageRatingResponse {
  return conversationMessageRatingResponseSchema.parse(payload);
}

export function parseWebChatJob(payload: unknown): WebChatJob {
  return webChatJobSchema.parse(payload);
}

export function isSafeWebChatAcceptedResponse(status: number, job: WebChatJob): boolean {
  if (status === 201) {
    return job.status === "queued";
  }
  return status === 200 && safeWebChatReplayStatuses.has(job.status);
}

export function parseImageModelList(payload: unknown): ImageModelList {
  return imageModelListSchema.parse(payload);
}

export function parseImageJobPreparation(payload: unknown): ImageJobPreparation {
  return imageJobPreparationSchema.parse(payload);
}

export function parseImageJobActivation(payload: unknown): ImageJobActivation {
  return imageJobActivationSchema.parse(payload);
}

export function parseImageJobList(payload: unknown): ImageJobList {
  return imageJobListSchema.parse(payload);
}

export function parseImageJobResult(payload: unknown): ImageJobResult {
  return imageJobResultSchema.parse(payload);
}
