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

export const publicApiErrorSchema = z
  .object({
    error: z.string().trim().min(1),
  })
  .strict();

export type PublicApiError = z.infer<typeof publicApiErrorSchema>;

export function parseAccountProfile(payload: unknown): AccountProfile {
  return accountProfileSchema.parse(payload);
}

export function parseConversationList(payload: unknown): ConversationList {
  return conversationListSchema.parse(payload);
}
