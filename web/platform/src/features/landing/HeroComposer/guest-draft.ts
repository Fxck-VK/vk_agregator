export const guestDraftStorageKey = "neirohub.guest-draft";

const guestDraftMaxLength = 4_000;
const guestDraftLifetimeMs = 15 * 60_000;

type StoredGuestDraft = {
  createdAt: number;
  target: GuestDraftTarget;
  text: string;
};

export type GuestDraftTarget = "chat" | "image";

function browserSessionStorage(): Storage | null {
  return typeof window === "undefined" ? null : window.sessionStorage;
}

export function saveGuestDraft(
  text: string,
  target: GuestDraftTarget,
  storage: Storage | null = browserSessionStorage(),
  now = Date.now(),
): void {
  const normalizedText = text.trim().slice(0, guestDraftMaxLength);
  if (!normalizedText || !storage) return;

  try {
    storage.setItem(guestDraftStorageKey, JSON.stringify({ createdAt: now, target, text: normalizedText } satisfies StoredGuestDraft));
  } catch {
    // A draft is a convenience; restricted storage must not block navigation.
  }
}

export function loadGuestDraft(
  target: GuestDraftTarget,
  storage: Storage | null = browserSessionStorage(),
  now = Date.now(),
): string | null {
  if (!storage) return null;

  try {
    const rawValue = storage.getItem(guestDraftStorageKey);
    if (!rawValue) return null;
    const value = JSON.parse(rawValue) as Partial<StoredGuestDraft>;
    const text = value.text;
    const isValid = typeof value.createdAt === "number"
      && (value.target === "chat" || value.target === "image")
      && typeof text === "string"
      && text.length > 0
      && now >= value.createdAt
      && now - value.createdAt <= guestDraftLifetimeMs;

    if (!isValid) {
      storage.removeItem(guestDraftStorageKey);
      return null;
    }

    return value.target === target ? text ?? null : null;
  } catch {
    try { storage.removeItem(guestDraftStorageKey); } catch { /* Optional storage. */ }
    return null;
  }
}

export function consumeGuestDraft(
  target: GuestDraftTarget,
  storage: Storage | null = browserSessionStorage(),
  now = Date.now(),
): string | null {
  if (!storage) return null;
  const text = loadGuestDraft(target, storage, now);
  if (text !== null) {
    try { storage.removeItem(guestDraftStorageKey); } catch { /* Optional storage. */ }
  }
  return text;
}
