import { z } from "zod";
import { webBrowserMutation } from "@/lib/web-api/browser";
import { assertBrowserSession, browserSessionSnapshot } from "@/lib/web-api/browser-session-state";
import { PaymentRequestError } from "@/features/payments/payments";
import { subscriptionPlanPreview } from "@/features/pricing/subscription-preview";

const positive = z.number().int().positive().safe();
const planCode = z.enum(["lite", "start", "pro", "ultima", "elite"]);
const quoteSchema = z.object({
  code: z.string().regex(/^[A-Z0-9_-]{3,64}$/), revision: positive,
  percent: positive.max(99),
  plans: z.array(z.object({
    plan_code: planCode, base_amount: positive, amount: positive,
    currency: z.literal("RUB"), stars: positive,
  }).strict()).min(1).max(5),
}).strict().superRefine((value, context) => {
  const seen = new Set<string>();
  for (const plan of value.plans) {
    const canonical = subscriptionPlanPreview(plan.plan_code);
    if (seen.has(plan.plan_code) || plan.base_amount !== canonical.amount || plan.stars !== canonical.stars
      || !Number.isSafeInteger(plan.base_amount * value.percent)
      || plan.amount !== plan.base_amount - Math.floor(plan.base_amount * value.percent / 100)) {
      context.addIssue({ code: "custom", message: "Invalid promotion price" });
    }
    seen.add(plan.plan_code);
  }
});
export type PromotionQuote = z.infer<typeof quoteSchema>;
export function normalizePromotionCode(code: string) {
  return z.string().regex(/^[A-Za-z0-9_-]{3,64}$/).parse(code.trim()).toUpperCase();
}
export function parsePromotionQuote(value: unknown): PromotionQuote { return quoteSchema.parse(value); }
export async function requestPromotionQuote(code: string, signal?: AbortSignal): Promise<PromotionQuote> {
  const normalized = normalizePromotionCode(code), session = browserSessionSnapshot();
  const response = await webBrowserMutation("/web/v1/subscription/promo", {
    method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ code: normalized }),
    cache: "no-store", signal: AbortSignal.any([session.signal, AbortSignal.timeout(15_000), ...(signal ? [signal] : [])]),
  });
  assertBrowserSession(response, session);
  if (!response.ok) throw new PaymentRequestError(response.status);
  const quote = parsePromotionQuote(await response.json());
  assertBrowserSession(response, session);
  if (quote.code !== normalized) throw new Error("Promotion quote mismatch");
  return quote;
}
