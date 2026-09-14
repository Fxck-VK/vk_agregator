import { z } from "zod";
import { webBrowserFetch, webBrowserMutation } from "@/lib/web-api/browser";

export function isCheckoutURL(value: string): boolean {
  try {
    const url = new URL(value);
    return url.protocol === "https:" && !url.username && !url.password && !url.port
      && ["yoomoney.ru", "yookassa.ru", "checkout.yookassa.ru"].includes(url.hostname);
  } catch { return false; }
}

const productSchema = z.object({
  code: z.string().min(1).max(64),
  amount: z.number().int().positive().safe(),
  currency: z.literal("RUB"),
  credits: z.number().int().positive().safe(),
}).strict();
const catalogSchema = z.object({ items: z.array(productSchema), checkout_available: z.boolean() }).strict();
const paymentSchema = z.object({
  id: z.string().uuid(),
  status: z.enum(["created", "provider_pending", "waiting_for_user", "succeeded", "canceled", "failed", "expired", "refunded", "partially_refunded"]),
  amount: z.number().int().positive().safe(),
  currency: z.literal("RUB"),
  credits: z.number().int().positive().safe(),
  confirmation_url: z.string().refine(isCheckoutURL).optional(),
}).strict();
export type PaymentProduct = z.infer<typeof productSchema>;
export type PaymentCatalog = z.infer<typeof catalogSchema>;
export type Payment = z.infer<typeof paymentSchema>;
export type PaymentRequest = { product_code: string };

export class PaymentRequestError extends Error {
  constructor(public readonly status: number) { super("Payment request failed"); }
}
async function payload(response: Response): Promise<unknown> {
  if (!response.ok) throw new PaymentRequestError(response.status);
  return response.json();
}
export async function loadPaymentProducts(signal?: AbortSignal): Promise<PaymentCatalog> {
  return catalogSchema.parse(await payload(await webBrowserFetch("/web/v1/payment-products", { cache: "no-store", signal })));
}
export async function createPayment(request: PaymentRequest, key: string): Promise<Payment> {
  return paymentSchema.parse(await payload(await webBrowserMutation("/web/v1/payments/intents", {
    method: "POST", headers: { "Content-Type": "application/json", "X-Idempotency-Key": key }, body: JSON.stringify(request),
  })));
}
export async function getPayment(id: string, signal?: AbortSignal): Promise<Payment> {
  const safeID = z.string().uuid().parse(id);
  return paymentSchema.parse(await payload(await webBrowserFetch(`/web/v1/payments/${safeID}`, { cache: "no-store", signal })));
}

const pendingKey = "neirohub:pending-payment";
export function readPendingPayment(): string | null {
  try { const parsed = z.string().uuid().safeParse(sessionStorage.getItem(pendingKey)); return parsed.success ? parsed.data : null; }
  catch { return null; }
}
export function savePendingPayment(id: string): void {
  try { sessionStorage.setItem(pendingKey, z.string().uuid().parse(id)); } catch { /* Storage can be unavailable in private mode. */ }
}
export function clearPendingPayment(id: string): void {
  try { if (readPendingPayment() === id) sessionStorage.removeItem(pendingKey); } catch { /* Optional navigation state only. */ }
}
const attemptKey = "neirohub:payment-attempt";
export function paymentAttemptKey(code: string): string {
  try {
    const previous = JSON.parse(sessionStorage.getItem(attemptKey) ?? "null") as { code?: string; key?: string } | null;
    if (previous?.code === code && z.string().uuid().safeParse(previous.key).success) return previous.key!;
  } catch { /* Start a new operation when no valid attempt exists. */ }
  const key = crypto.randomUUID();
  try { sessionStorage.setItem(attemptKey, JSON.stringify({ code, key })); } catch { /* In-memory retry still keeps its key. */ }
  return key;
}
export function clearPaymentAttempt(): void {
  try { sessionStorage.removeItem(attemptKey); } catch { /* No billing state lives here. */ }
}
export const formatPaymentNumber = (value: number) => new Intl.NumberFormat("ru-RU").format(value).replace(/\u00a0/g, " ");
export const formatPaymentPrice = (minorUnits: number) => formatPaymentNumber(minorUnits / 100);
