const pendingConversationTitleSyncPrefix = "neirohub.pending-conversation-title-sync:";
const maxFallbackTitleRunes = 80;
const maxPendingTitleSyncAgeMs = 2 * 60 * 1_000;

type PendingConversationTitleSync = {
  fallbackTitle: string;
  createdAt: number;
};

function getSessionStorage(): Storage | null {
  if (typeof window === "undefined") {
    return null;
  }

  try {
    return window.sessionStorage;
  } catch {
    return null;
  }
}

function keyFor(conversationID: string) {
  return `${pendingConversationTitleSyncPrefix}${conversationID}`;
}

export function fallbackConversationTitle(prompt: string): string {
  const normalized = prompt.trim().replace(/\s+/g, " ");
  const runes = Array.from(normalized);
  if (runes.length <= maxFallbackTitleRunes) {
    return normalized;
  }
  return `${runes.slice(0, maxFallbackTitleRunes - 3).join("").trim()}...`;
}

export function savePendingConversationTitleSync(conversationID: string, fallbackTitle: string) {
  const normalizedTitle = fallbackConversationTitle(fallbackTitle);
  if (normalizedTitle === "") {
    return;
  }

  try {
    getSessionStorage()?.setItem(keyFor(conversationID), JSON.stringify({ fallbackTitle: normalizedTitle, createdAt: Date.now() }));
  } catch {
    // Browser storage is an optimisation, not a dependency of sending a message.
  }
}

export function readPendingConversationTitleSync(conversationID: string): string | null {
  const sessionStorage = getSessionStorage();
  if (sessionStorage === null) {
    return null;
  }

  try {
    const raw = sessionStorage.getItem(keyFor(conversationID));
    if (raw === null) {
      return null;
    }
    const value = JSON.parse(raw) as Partial<PendingConversationTitleSync>;
    const title = typeof value.fallbackTitle === "string" ? fallbackConversationTitle(value.fallbackTitle) : "";
    if (title === "" || typeof value.createdAt !== "number" || Date.now() - value.createdAt > maxPendingTitleSyncAgeMs) {
      sessionStorage.removeItem(keyFor(conversationID));
      return null;
    }
    return title;
  } catch {
    return null;
  }
}

export function clearPendingConversationTitleSync(conversationID: string) {
  try {
    getSessionStorage()?.removeItem(keyFor(conversationID));
  } catch {
    // A disabled browser storage must not affect the visible workspace.
  }
}
