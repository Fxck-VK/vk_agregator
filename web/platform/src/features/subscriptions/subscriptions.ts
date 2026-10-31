import { z } from "zod";
import { webBrowserFetch, webBrowserMutation } from "@/lib/web-api/browser";
import { paymentSchema, PaymentRequestError } from "@/features/payments/payments";
import economy from "@/config/economy.generated.json";

const positive = z.number().int().positive().safe();
const date = z.string().datetime({ offset: true });
export const subscriptionStateSchema = z.object({
  subscription: z.object({
    plan_code: z.string().min(1).max(64), plan_name: z.string().min(1).max(100),
    amount: positive, stars: positive, currency: z.enum(["RUB", "rub"]).transform(() => "RUB" as const),
    period_start: date, paid_until: date, renewal_at: date.optional(),
    next_plan_code: z.string().min(1).max(64), next_amount: positive,
    auto_renew: z.boolean(), status: z.enum(["active", "cancelled", "expired", "past_due"]),
  }).strict().nullable(),
  balances: z.array(z.object({
    kind: z.string().min(1).max(64), available: z.number().int().safe(),
    reserved: z.number().int().nonnegative().safe(), expires_at: date.optional(),
  }).strict()),
  checkout_available: z.boolean(), recurring_available: z.boolean(), trial_used: z.boolean(),
}).strict();
export type SubscriptionState = z.infer<typeof subscriptionStateSchema>;
const requestSchema = z.object({
  plan_code: z.string().min(1).max(64), auto_renew: z.boolean(),
  retry_policy_version: z.union([z.literal(0), z.literal(economy.renewal_retries.version)]).optional(),
}).strict().refine(request => request.auto_renew || request.retry_policy_version === undefined);
export type SubscriptionCheckoutRequest = z.infer<typeof requestSchema>;
async function payload(response: Response): Promise<unknown> {
  if (!response.ok) throw new PaymentRequestError(response.status);
  return response.json();
}
export async function loadSubscription(signal?: AbortSignal): Promise<SubscriptionState> {
  return subscriptionStateSchema.parse(await payload(await webBrowserFetch("/web/v1/subscription", { cache: "no-store", signal })));
}
export async function createSubscriptionCheckout(request: SubscriptionCheckoutRequest, key: string) {
  const safeRequest = requestSchema.parse(request);
  const safeKey = z.string().uuid().parse(key);
  return paymentSchema.parse(await payload(await webBrowserMutation("/web/v1/subscription/checkout", {
    method: "POST", headers: { "Content-Type": "application/json", "X-Idempotency-Key": safeKey }, body: JSON.stringify(safeRequest),
  })));
}
export async function cancelSubscription(): Promise<SubscriptionState> {
  return subscriptionStateSchema.parse(await payload(await webBrowserMutation("/web/v1/subscription/cancel", {
    method: "POST", headers: { "Content-Type": "application/json" }, body: "{}",
  })));
}
