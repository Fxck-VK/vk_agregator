import { conversationListSchema, type ConversationItem } from "@/lib/web-api/contracts";

const key = "neirohub.conversation-list.v1";
const maxAgeMs = 24 * 60 * 60 * 1000;
let activeAccount: string | null = null;

/** Only call after the server has confirmed the workspace account. */
export function activateConversationCache(accountId: string) {
  if (activeAccount && activeAccount !== accountId) clearConversationCache();
  activeAccount = accountId;
}
export function clearConversationCache() {
  activeAccount = null;
  try { localStorage.removeItem(key); } catch { /* Storage is optional. */ }
}
export function readConversationCache(accountId: string): ConversationItem[] | null {
  if (accountId !== activeAccount) return null;
  try {
    const raw = localStorage.getItem(key);
    if (!raw) return null;
    if (raw.length > 100_000) { localStorage.removeItem(key); return null; }
    const value = JSON.parse(raw);
    if (value.accountId !== accountId || !Number.isFinite(value.savedAt) || Date.now() - value.savedAt > maxAgeMs || value.savedAt > Date.now()) {
      localStorage.removeItem(key); return null;
    }
    const parsed = conversationListSchema.safeParse({ items: value.items });
    if (!parsed.success || parsed.data.items.length > 20) { localStorage.removeItem(key); return null; }
    return parsed.data.items;
  } catch { return null; }
}
export function writeConversationCache(accountId: string, items: ConversationItem[]) {
  if (accountId !== activeAccount) return;
  // Explicit projection: no prompt bodies, credentials, attachments or pending drafts.
  const metadata = items.slice(0, 20).map(({ id, title, created_at, updated_at }) => ({ id, title: title.slice(0, 240), created_at, updated_at }));
  if (!conversationListSchema.safeParse({ items: metadata }).success) return;
  try { localStorage.setItem(key, JSON.stringify({ accountId, savedAt: Date.now(), items: metadata })); } catch { /* Quota/disabled storage must not prevent reading chats. */ }
}
