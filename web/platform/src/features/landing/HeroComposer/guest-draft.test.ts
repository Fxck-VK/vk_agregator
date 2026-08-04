import { describe, expect, it } from "vitest";

import { consumeGuestDraft, guestDraftStorageKey, loadGuestDraft, saveGuestDraft } from "./guest-draft";

function createStorage(): Storage {
  const values = new Map<string, string>();

  return {
    get length() { return values.size; },
    clear: () => values.clear(),
    getItem: (key) => values.get(key) ?? null,
    key: (index) => Array.from(values.keys())[index] ?? null,
    removeItem: (key) => { values.delete(key); },
    setItem: (key, value) => { values.set(key, value); },
  };
}

describe("guest landing draft", () => {
  it("stores a trimmed bounded text draft without files", () => {
    const storage = createStorage();

    saveGuestDraft(`  ${"а".repeat(4500)}  `, "image", storage, 1_000);

    expect(JSON.parse(storage.getItem(guestDraftStorageKey) ?? "null")).toEqual({
      createdAt: 1_000,
      text: "а".repeat(4_000),
      target: "image",
    });
  });

  it("returns a recent draft and removes an expired one", () => {
    const storage = createStorage();
    saveGuestDraft("Новый запрос", "chat", storage, 1_000);

    expect(loadGuestDraft("chat", storage, 1_000 + 14 * 60_000)).toBe("Новый запрос");
    expect(loadGuestDraft("chat", storage, 1_000 + 16 * 60_000)).toBeNull();
    expect(storage.getItem(guestDraftStorageKey)).toBeNull();
  });

  it("consumes only a draft for the requested destination", () => {
    const storage = createStorage();
    saveGuestDraft("Нарисуй город", "image", storage, 1_000);

    expect(consumeGuestDraft("chat", storage, 1_001)).toBeNull();
    expect(storage.getItem(guestDraftStorageKey)).not.toBeNull();
    expect(consumeGuestDraft("image", storage, 1_001)).toBe("Нарисуй город");
    expect(storage.getItem(guestDraftStorageKey)).toBeNull();
  });
});
