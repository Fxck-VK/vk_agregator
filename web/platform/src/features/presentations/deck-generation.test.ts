import { beforeEach, describe, expect, it, vi } from "vitest";
import { readDeck, presentationPrompt, emptyBrief } from "./deck-content";
import { fetchDeckQuote, readDeckJob, submitDeck, type DeckAttempt } from "./deck-generation";

const api = vi.hoisted(() => ({ read: vi.fn(), write: vi.fn() }));
vi.mock("@/lib/web-api/browser", () => ({ webBrowserFetch: api.read, webBrowserMutation: api.write }));
const conversation = "12345678-0000-4000-8000-012345678901", job = "12345678-0000-4000-8000-012345678902";
const quote = { model_id: "fixture", model_name: "Модель", credits: 5 };
const signal = new AbortController().signal;
const when = "2026-01-01T00:00:00Z";
const input = presentationPrompt({ ...emptyBrief, topic: "Тестовая тема" });
const source = Array.from({ length: 5 }, (_, i) => `## Слайд ${i + 1}\n- Тезис ${i + 1}`).join("\n\n");
beforeEach(() => { api.read.mockReset(); api.write.mockReset(); });

describe("presentation generation", () => {
  it("uses the quoted model and total, and reuses both keys after an uncertain submission", async () => {
    api.read.mockResolvedValueOnce(Response.json(quote));
    expect(await fetchDeckQuote(signal)).toEqual(quote);
    api.write.mockResolvedValueOnce(Response.json({ id: conversation, title: "", created_at: when, updated_at: when }))
      .mockRejectedValueOnce(new Error("network"))
      .mockResolvedValueOnce(Response.json({ job_id: job, status: "queued" }, { status: 201 }));
    const attempt: DeckAttempt = { conversationKey: crypto.randomUUID(), generationKey: crypto.randomUUID() };
    await expect(submitDeck(attempt, quote, input, signal)).rejects.toThrow();
    await expect(submitDeck(attempt, quote, input, signal)).resolves.toEqual({ conversation, job });
    expect(api.write.mock.calls[2]).toEqual(api.write.mock.calls[1]);
    expect(JSON.parse(api.write.mock.calls[2][1].body)).toEqual({ model_id: quote.model_id, expected_presentation_credits: 5, prompt: input });
  });
  it("reads a finished deck from the dedicated conversation", async () => {
    api.read.mockResolvedValueOnce(Response.json({ job_id: job, status: "succeeded" }))
      .mockResolvedValueOnce(Response.json({ items: [
        { id: conversation, role: "user", text: input, seq: 1, created_at: when },
        { id: job, role: "assistant", text: source, seq: 2, created_at: when },
      ] }));
    expect(await readDeckJob(conversation, job, input, signal)).toEqual({ kind: "ready", text: source });
    expect(readDeck(source, 5)).toHaveLength(5);
  });
  it("does not present content while the worker is still processing or after failure", async () => {
    for (const [status, kind] of [["provider_processing", "pending"], ["failed_terminal", "failed"]]) {
      api.read.mockResolvedValueOnce(Response.json({ job_id: job, status }));
      expect(await readDeckJob(conversation, job, input, signal)).toEqual({ kind });
    }
    expect(api.read).toHaveBeenCalledTimes(2);
  });
  it("requires correction instead of fabricating incomplete slides", () => {
    expect(() => readDeck(source, 10)).toThrow();
    expect(() => readDeck("## Пустой", 1)).toThrow();
    expect(readDeck("\u0060\u0060\u0060markdown\n" + source + "\n\u0060\u0060\u0060", 5)).toEqual(readDeck(source, 5));
  });
});
