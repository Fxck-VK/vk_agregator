const pendingConversationPromptPrefix = "neirohub.pending-conversation-prompt:";

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

function getPendingConversationPromptKey(conversationID: string) {
  return `${pendingConversationPromptPrefix}${conversationID}`;
}

export function savePendingConversationPrompt(conversationID: string, prompt: string) {
  const normalizedPrompt = prompt.trim();
  if (normalizedPrompt === "") {
    return;
  }

  try {
    getSessionStorage()?.setItem(getPendingConversationPromptKey(conversationID), normalizedPrompt);
  } catch {
    // A disabled browser storage must not prevent an accepted chat from opening.
  }
}

export function readPendingConversationPrompt(conversationID: string): string | null {
  const sessionStorage = getSessionStorage();
  if (sessionStorage === null) {
    return null;
  }

  let storedPrompt: string | null;
  try {
    storedPrompt = sessionStorage.getItem(getPendingConversationPromptKey(conversationID));
  } catch {
    return null;
  }

  const normalizedPrompt = storedPrompt?.trim() ?? "";
  return normalizedPrompt === "" ? null : normalizedPrompt;
}

export function clearPendingConversationPrompt(conversationID: string) {
  try {
    getSessionStorage()?.removeItem(getPendingConversationPromptKey(conversationID));
  } catch {
    // A browser storage failure must not interrupt the first chat request.
  }
}
