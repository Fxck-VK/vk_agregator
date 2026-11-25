import { z } from "zod";
import { conversationHistoryPageLimit } from "@/features/conversations/conversation-history-contract";
import { webBrowserFetch, webBrowserMutation } from "@/lib/web-api/browser";
import { isSafeWebChatAcceptedResponse, parseConversationItem, parseConversationMessageList, parseWebChatJob, webChatJobSchema } from "@/lib/web-api/contracts";
import { DeckRequestError } from "./deck-generation";

const route = "/web/v1/presentation-chats";
const price = z.number().int().positive();
const quoteSchema = z.object({ model_id: z.string().min(1), model_name: z.string().min(1), credits: price, turn_credits: price });
const deckSchema = webChatJobSchema.extend({
  conversation_id: z.uuid(),
  model_id: z.string().min(1).optional(),
  presentation_credits: price.optional(),
  unavailable: z.boolean().optional(),
});
const turnSchema = webChatJobSchema.extend({
  model_id: z.string().min(1),
  presentation_credits: price,
  turn_credits: price,
  reply: z.string().optional(),
  brief: z.unknown().optional(),
  missing: z.array(z.enum(["topic", "slide_count", "purpose", "audience", "style", "key_points"])).optional(),
  presentation: deckSchema.optional(),
});
const newConversationSchema = z.strictObject({}).transform(() => null);
const attemptSchema = z.object({ conversationKey: z.uuid(), messageKey: z.uuid() });
const attemptPrefix = "epifera:presentation-chat-retry:";

export const maxPresentationChatMessageBytes = 2800;
export type PresentationChatQuote = z.infer<typeof quoteSchema>;
export type PresentationDeckPrice = Pick<PresentationChatQuote, "model_id" | "credits">;
export type PresentationChatTurn = z.infer<typeof turnSchema>;

async function read(response: Response): Promise<unknown> {
  if (!response.ok) throw new DeckRequestError(response.status);
  return response.json();
}

// Keep idempotency keys for an uncertain submission without storing chat text.
export async function recoverPresentationChatAttempt(account: string, conversation: string | undefined, prompt: string, quote: PresentationChatQuote) {
  const input = JSON.stringify([account, conversation, prompt, quote.model_id, quote.credits, quote.turn_credits]);
  const digest = new Uint8Array(await crypto.subtle.digest("SHA-256", new TextEncoder().encode(input)));
  const key = attemptPrefix + Array.from(digest, byte => byte.toString(16).padStart(2, "0")).join("");
  let attempt = { conversationKey: crypto.randomUUID(), messageKey: crypto.randomUUID() };
  try {
    const saved = attemptSchema.safeParse(JSON.parse(sessionStorage.getItem(key) ?? "null"));
    if (saved.success) attempt = saved.data;
    else sessionStorage.setItem(key, JSON.stringify(attempt));
  } catch { /* The open chat can still retry with its in-memory keys. */ }
  return {
    ...attempt,
    clear: () => { try { sessionStorage.removeItem(key); } catch { /* Optional storage. */ } },
  };
}

export function clearPresentationChatAttempts() {
  try {
    for (let index = sessionStorage.length - 1; index >= 0; index--) {
      const key = sessionStorage.key(index);
      if (key?.startsWith(attemptPrefix)) sessionStorage.removeItem(key);
    }
  } catch { /* Optional storage. */ }
}

export async function fetchPresentationChatQuote(signal: AbortSignal) {
  return quoteSchema.parse(await read(await webBrowserFetch(`${route}/quote`, { signal, cache: "no-store" })));
}

export async function createPresentationChat(idempotencyKey: string, signal: AbortSignal) {
  const response = await webBrowserMutation(route, { method: "POST", signal, headers: { "X-Idempotency-Key": idempotencyKey } });
  const conversation = parseConversationItem(await read(response));
  if (conversation.kind !== "presentation") throw new DeckRequestError(503);
  return conversation;
}

export async function sendPresentationChatTurn(conversation: string, idempotencyKey: string, prompt: string, quote: PresentationChatQuote, signal: AbortSignal) {
  const response = await webBrowserMutation(`${route}/${conversation}/messages`, {
    method: "POST", signal, headers: { "Content-Type": "application/json", "X-Idempotency-Key": idempotencyKey },
    body: JSON.stringify({ prompt, model_id: quote.model_id, expected_presentation_credits: quote.credits, expected_turn_credits: quote.turn_credits }),
  });
  return readAcceptedJob(response);
}

export async function resumePresentationChatTurn(conversation: string, job: string, signal: AbortSignal) {
  const response = await webBrowserMutation(`${route}/${conversation}/turns/${job}/resume`, { method: "POST", signal });
  if (await readAcceptedJob(response) !== job) throw new DeckRequestError(503);
}

async function readAcceptedJob(response: Response) {
  const job = parseWebChatJob(await read(response));
  if (!isSafeWebChatAcceptedResponse(response.status, job)) throw new DeckRequestError(503);
  return job.job_id;
}

export async function readPresentationChatTurn(conversation: string, job: string | undefined, signal: AbortSignal) {
  const path = job ? `${route}/${conversation}/turns/${job}` as const : `${route}/${conversation}` as const;
  const payload = await read(await webBrowserFetch(path, { signal, cache: "no-store" }));
  const turn = job ? turnSchema.parse(payload) : z.union([turnSchema, newConversationSchema]).parse(payload);
  if (job && turn?.job_id !== job) throw new DeckRequestError(503);
  return turn;
}

export async function readPresentationChatHistory(conversation: string, signal: AbortSignal) {
  const response = await webBrowserFetch(`/web/v1/conversations/${conversation}/messages?limit=${conversationHistoryPageLimit}`, { signal, cache: "no-store" });
  const page = parseConversationMessageList(await read(response));
  if (page.kind !== "presentation") throw new DeckRequestError(503);
  return page.items;
}

export async function startPresentationFromChat(conversation: string, turn: PresentationChatTurn, signal: AbortSignal, approvedQuote?: PresentationDeckPrice) {
  const response = await webBrowserMutation(`${route}/${conversation}/turns/${turn.job_id}/generate`, {
    method: "POST", signal, headers: { "Content-Type": "application/json" },
    body: JSON.stringify(approvedQuote
      ? { model_id: approvedQuote.model_id, expected_presentation_credits: approvedQuote.credits, accept_updated_quote: true }
      : { model_id: turn.model_id, expected_presentation_credits: turn.presentation_credits }),
  });
  const deck = deckSchema.parse(await read(response));
  if (response.status !== 200 && response.status !== 201) throw new DeckRequestError(503);
  return deck;
}
