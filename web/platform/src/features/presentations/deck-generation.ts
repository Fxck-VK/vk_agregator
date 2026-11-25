import { z } from "zod";
import { presentationDeckCopy } from "@/i18n/presentation-deck";
import { webBrowserFetch, webBrowserMutation } from "@/lib/web-api/browser";
import { conversationItemSchema, isSafeWebChatAcceptedResponse, parseWebChatJob, webChatJobSchema } from "@/lib/web-api/contracts";

const quoteSchema = z.object({
  model_id: z.string().min(1),
  model_name: z.string().min(1),
  credits: z.number().int().positive(),
});
export type DeckQuote = z.infer<typeof quoteSchema>;
export type DeckAttempt = { conversationKey: string; generationKey: string; conversation?: string; job?: string };

// Keep only retry keys for this tab/account. The brief and the model's answer stay in memory.
export async function recoverDeckAttempt(accountId: string, prompt: string, quote: DeckQuote) {
  const raw = JSON.stringify([accountId, prompt, quote.model_id, quote.credits]);
  const digest = new Uint8Array(await crypto.subtle.digest("SHA-256", new TextEncoder().encode(raw)));
  const key = "epifera:deck-retry:" + Array.from(digest, n => n.toString(16).padStart(2, "0")).join("");
  const schema = z.object({ conversationKey: z.uuid(), generationKey: z.uuid() });
  let attempt: DeckAttempt = { conversationKey: crypto.randomUUID(), generationKey: crypto.randomUUID() };
  try {
    const saved = schema.safeParse(JSON.parse(sessionStorage.getItem(key) ?? "null"));
    if (saved.success) attempt = saved.data;
    else sessionStorage.setItem(key, JSON.stringify(attempt));
  } catch { /* The in-memory keys still protect retries in the open form. */ }
  return { attempt, clear: () => { try { sessionStorage.removeItem(key); } catch { /* Optional storage. */ } } };
}

export class DeckRequestError extends Error {
  constructor(public status: number) {
    super((presentationDeckCopy.requestErrors.byStatus as Record<number, string>)[status] ?? presentationDeckCopy.requestErrors.unavailable);
  }
}

export async function fetchDeckQuote(signal: AbortSignal) {
  const response = await webBrowserFetch("/web/v1/presentations/quote", { signal, cache: "no-store" });
  if (!response.ok) throw new DeckRequestError(response.status);
  return quoteSchema.parse(await response.json());
}

export async function submitDeck(attempt: DeckAttempt, quote: DeckQuote, prompt: string, signal: AbortSignal, onPrepared?: (conversation: string) => void) {
  if (!attempt.conversation) {
    const response = await webBrowserMutation("/web/v1/conversations", {
      method: "POST", headers: { "X-Idempotency-Key": attempt.conversationKey }, signal,
    });
    if (!response.ok) throw new DeckRequestError(response.status);
    attempt.conversation = conversationItemSchema.parse(await response.json()).id;
  }
  signal.throwIfAborted();
  onPrepared?.(attempt.conversation);
  const response = await webBrowserMutation(`/web/v1/presentations/${attempt.conversation}/generate`, {
    method: "POST", signal,
    headers: { "Content-Type": "application/json", "X-Idempotency-Key": attempt.generationKey },
    body: JSON.stringify({ model_id: quote.model_id, expected_presentation_credits: quote.credits, prompt }),
  });
  if (!response.ok) throw new DeckRequestError(response.status);
  const job = parseWebChatJob(await response.json());
  if (!isSafeWebChatAcceptedResponse(response.status, job)) throw new DeckRequestError(503);
  attempt.job = job.job_id;
  return { conversation: attempt.conversation, job: job.job_id };
}

const deckJobSchema = webChatJobSchema.extend({
  prompt: z.string().max(32_000),
  answer: z.string().optional(),
  failure_code: z.string().max(80).optional(),
  model_name: z.string().max(160).optional(),
});

const failedStatuses = new Set(["failed_terminal", "cancelled", "expired", "refunded", "rejected", "awaiting_payment"]);

export async function readDeckJob(run: { conversation: string; job?: string }, signal: AbortSignal) {
  const query = run.job ? `?job_id=${run.job}` : "";
  const response = await webBrowserFetch(`/web/v1/presentations/${run.conversation}${query}`, { signal, cache: "no-store" });
  if (!response.ok) throw new DeckRequestError(response.status);
  const { job_id, status, prompt, answer, failure_code, model_name } = deckJobSchema.parse(await response.json());
  if (run.job && job_id !== run.job) throw new DeckRequestError(503);
  const state = { job: job_id, status, prompt, failureCode: failure_code, modelName: model_name };
  if (failedStatuses.has(status)) return { ...state, kind: "failed" } as const;
  if (status !== "succeeded") return { ...state, kind: "pending" } as const;
  if (!answer?.trim()) return { ...state, kind: "missing" } as const;
  return { ...state, kind: "ready", text: answer } as const;
}
