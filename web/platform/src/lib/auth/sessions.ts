import { z } from "zod";
export const accountSessionsSchema = z.object({ items: z.array(z.object({
  id: z.string().uuid(), account_id: z.string().uuid(), identity_id: z.string().uuid().optional(),
  created_at: z.string().datetime({ offset: true }), updated_at: z.string().datetime({ offset: true }), expires_at: z.string().datetime({ offset: true }),
  revoked: z.boolean(), current: z.boolean(),
}).strict()) }).strict();
export type AccountSession = z.infer<typeof accountSessionsSchema>["items"][number];
