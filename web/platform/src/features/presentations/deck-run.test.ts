import { afterEach, describe, expect, it } from "vitest";
import { emptyBrief, presentationPrompt } from "./deck-content";
import { getDeckDraft, parseDeckRun, readDeckBrief, saveDeckDraft } from "./deck-run";

const conversation = "12345678-0000-4000-8000-012345678901";
const job = "12345678-0000-4000-8000-012345678902";

afterEach(() => sessionStorage.clear());

describe("presentation recovery", () => {
  it("accepts only a valid presentation and optional job from the URL", () => {
    expect(parseDeckRun({ run: conversation, job })).toEqual({ conversation, job });
    expect(parseDeckRun({ run: conversation })).toEqual({ conversation });
    expect(parseDeckRun({ run: conversation, pending: "1" })).toEqual({ conversation, pending: true });
    expect(parseDeckRun({ run: [conversation] })).toBeUndefined();
    expect(parseDeckRun({ run: conversation, job: "other" })).toBeUndefined();
  });

  it("restores old briefs including punctuation and multiline requirements", () => {
    const brief = {
      ...emptyBrief, topic: "Alfa Romeo. Стиль и техника",
      audience: "Друзья", count: 10 as const, style: "Деловой" as const,
      notes: "История\n\nФинал",
    };
    expect(readDeckBrief(presentationPrompt(brief))).toEqual(brief);
    expect(readDeckBrief("обычное сообщение")).toBeUndefined();
  });

  it("restores a bounded slide image for the same account and job", () => {
    const slides = [{ title: "Фото", points: ["Подпись"], image: "data:image/jpeg;base64,cHJldmlldw==" }];
    saveDeckDraft("account", job, slides);
    expect(getDeckDraft("account", job)).toEqual(slides);
    expect(getDeckDraft("other", job)).toBeUndefined();
    saveDeckDraft("account", job, [{ ...slides[0], image: "https://example.test/photo.jpg" }]);
    saveDeckDraft("account", job, [{ ...slides[0], image: `data:image/jpeg;base64,${"a".repeat(70_000)}` }]);
    expect(getDeckDraft("account", job)).toEqual(slides);
  });

  it("keeps edits scoped to the account and generation", () => {
    const slides = [{ title: "Доработанная обложка", points: ["Заметка"] }];
    saveDeckDraft("account", conversation, slides);
    expect(getDeckDraft("account", conversation)).toEqual(slides);
    expect(getDeckDraft("other", conversation)).toBeUndefined();
    expect(getDeckDraft("account", job)).toBeUndefined();
    saveDeckDraft("account", conversation, [{ title: "", points: [""] }]);
    expect(getDeckDraft("account", conversation)).toEqual(slides);
  });
});
