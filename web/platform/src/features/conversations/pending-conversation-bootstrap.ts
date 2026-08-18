import { z } from "zod";

const pendingConversationBootstrapPrefix = "neirohub.pending-conversation-bootstrap:";

const pendingConversationBootstrapSchema = z
  .object({
    conversationKey: z.string().uuid(),
    conversationId: z.string().uuid().optional(),
    messageKey: z.string().uuid(),
    prompt: z.string().trim().min(1),
  })
  .strict();

export type PendingConversationBootstrapIntent = z.infer<typeof pendingConversationBootstrapSchema>;

function getSessionStorage(): Storage | null {
  if (typeof window === "undefined") return null;
  try {
    return window.sessionStorage;
  } catch {
    return null;
  }
}

function keyFor(conversationKey: string): string {
  return `${pendingConversationBootstrapPrefix}${conversationKey}`;
}

export function savePendingConversationBootstrap(intent: PendingConversationBootstrapIntent): void {
  const parsed = pendingConversationBootstrapSchema.safeParse(intent);
  if (!parsed.success) return;
  try {
    getSessionStorage()?.setItem(keyFor(parsed.data.conversationKey), JSON.stringify(parsed.data));
  } catch {
    // A private-storage failure is handled by the temporary route's unavailable state.
  }
}

export function readPendingConversationBootstrap(conversationKey: string): PendingConversationBootstrapIntent | null {
  const storage = getSessionStorage();
  if (storage === null) return null;

  const key = keyFor(conversationKey);
  try {
    const raw = storage.getItem(key);
    if (raw === null) return null;
    const parsed = pendingConversationBootstrapSchema.safeParse(JSON.parse(raw));
    if (!parsed.success || parsed.data.conversationKey !== conversationKey) {
      storage.removeItem(key);
      return null;
    }
    return parsed.data;
  } catch {
    try {
      storage.removeItem(key);
    } catch {
      // Ignore cleanup failure after invalid private browser data.
    }
    return null;
  }
}

export function updatePendingConversationBootstrap(
  conversationKey: string,
  patch: Pick<PendingConversationBootstrapIntent, "conversationId">,
): void {
  const current = readPendingConversationBootstrap(conversationKey);
  if (current === null) return;
  savePendingConversationBootstrap({ ...current, ...patch });
}

export function clearPendingConversationBootstrap(conversationKey: string): void {
  try {
    getSessionStorage()?.removeItem(keyFor(conversationKey));
  } catch {
    // Cleanup must not interrupt navigation.
  }
}

export function clearPendingConversationBootstraps(): void {
  const storage = getSessionStorage();
  if (storage === null) return;
  try {
    const keys = Array.from({ length: storage.length }, (_, index) => storage.key(index))
      .filter((key): key is string => key?.startsWith(pendingConversationBootstrapPrefix) === true);
    keys.forEach((key) => storage.removeItem(key));
  } catch {
    // Logout must continue when browser storage is unavailable.
  }
}
