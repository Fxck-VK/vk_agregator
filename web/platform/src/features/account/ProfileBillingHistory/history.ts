import { z } from "zod";
import { webBrowserFetch } from "@/lib/web-api/browser";

const safeInteger = z.number().int().min(-Number.MAX_SAFE_INTEGER).max(Number.MAX_SAFE_INTEGER);
const timestamp = z.iso.datetime({ offset: true }).refine(value => {
  const date = new Date(value);
  // Date.parse normalizes impossible calendar dates; reject those too.
  const day = value.slice(0, 10);
  const calendarDate = new Date(`${day}T00:00:00Z`);
  return Number.isFinite(date.getTime()) && calendarDate.toISOString().slice(0, 10) === day;
});
export const historyItemSchema = z.object({
  id: z.string().regex(/^(ledger|payment|refund|external_refund):[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i),
  kind: z.enum(["subscription", "topup", "generation", "generation_refund", "bonus", "expiry", "refund_adjustment", "refund_compensation", "referral_bonus", "referral_reversal", "adjustment", "refund_money"]),
  created_at: timestamp,
  stars_delta: safeInteger,
  status: z.enum(["succeeded", "pending", "canceled", "failed", "refunded", "partially_refunded"]),
  money: z.object({ amount_minor: safeInteger.positive(), currency: z.literal("rub") }).optional(),
  plan_code: z.enum(["lite", "start", "pro", "ultima", "elite"]).optional(),
  modality: z.enum(["text", "image", "video", "audio"]).optional(),
  operation: z.enum(["text_generate", "image_generate", "image_edit", "video_generate", "video_image_to_video", "video_extend", "audio_tts", "audio_stt", "audio_music", "image_upscale"]).optional(),
  test_mode: z.boolean().optional(),
});
export const historyPageSchema = z.object({
  items: z.array(historyItemSchema).max(20),
  next_cursor: z.string().min(1).max(512).regex(/^[A-Za-z0-9_-]+$/).nullable(),
}).refine(page => page.next_cursor === null || page.items.length > 0);
export type HistoryItem = z.infer<typeof historyItemSchema>;
export type HistoryPage = z.infer<typeof historyPageSchema>;

export async function loadBillingHistory(cursor: string | null, signal: AbortSignal): Promise<HistoryPage> {
  const query = new URLSearchParams({ limit: "20" });
  if (cursor !== null) query.set("cursor", cursor);
  const response = await webBrowserFetch(`/web/v1/billing/history?${query}`, { cache: "no-store", signal });
  if (!response.ok) throw new Error("Billing history unavailable");
  return historyPageSchema.parse(await response.json());
}
