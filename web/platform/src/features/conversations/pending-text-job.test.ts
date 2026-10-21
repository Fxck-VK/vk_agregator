import { afterEach, describe, expect, it, vi } from "vitest";

import { clearPendingTextJob, readPendingTextJob, savePendingTextJob } from "./pending-text-job";

const conversationID = "d7c979f5-24e5-4f88-924b-a592d6e5a906";
const otherConversationID = "62d33e7f-7b0e-4a26-975b-41080b55d78d";
const jobID = "a2a006fc-4457-4bb5-bc4d-4f553d51766b";
const key = `neirohub:conversation-pending-text:${conversationID}`;

afterEach(() => {
  vi.restoreAllMocks();
  vi.useRealTimers();
  window.sessionStorage.clear();
});

describe("pending text job", () => {
  it("stores only an opaque job reference, sequence and timestamp within its conversation", () => {
    savePendingTextJob(conversationID, jobID, 12);
    expect(JSON.parse(window.sessionStorage.getItem(key)!)).toEqual({ jobID, baselineSeq: 12, startedAt: expect.any(Number) });
    expect(readPendingTextJob(conversationID)).toEqual({ jobID, baselineSeq: 12, startedAt: expect.any(Number) });
    expect(readPendingTextJob(otherConversationID)).toBeNull();
  });

  it("removes expired metadata without extending its original retention on reads", () => {
    vi.useFakeTimers();
    savePendingTextJob(conversationID, jobID, 12);
    vi.advanceTimersByTime(30_000);
    expect(readPendingTextJob(conversationID)).not.toBeNull();
    vi.advanceTimersByTime(3_570_000);
    expect(readPendingTextJob(conversationID)).toBeNull();
    expect(window.sessionStorage.getItem(key)).toBeNull();
  });

  it.each([
    "invalid-json",
    JSON.stringify({ jobID: "invalid", baselineSeq: 0, startedAt: 1 }),
    JSON.stringify({ jobID, baselineSeq: -1, startedAt: 1 }),
    JSON.stringify({ jobID, baselineSeq: 0, startedAt: Number.MAX_SAFE_INTEGER }),
    JSON.stringify({ jobID, baselineSeq: 0, startedAt: 1, unexpected: "discard" }),
  ])("removes invalid stored metadata %#", raw => {
    window.sessionStorage.setItem(key, raw);
    expect(readPendingTextJob(conversationID)).toBeNull();
    expect(window.sessionStorage.getItem(key)).toBeNull();
  });

  it("does not save invalid references or sequence values", () => {
    savePendingTextJob("invalid", jobID, 0);
    savePendingTextJob(conversationID, "invalid", 0);
    savePendingTextJob(conversationID, jobID, -1);
    expect(window.sessionStorage.length).toBe(0);
  });

  it("clears only the requested conversation", () => {
    savePendingTextJob(conversationID, jobID, 12);
    savePendingTextJob(otherConversationID, jobID, 0);
    clearPendingTextJob(conversationID);
    expect(readPendingTextJob(conversationID)).toBeNull();
    expect(readPendingTextJob(otherConversationID)).not.toBeNull();
  });

  it("keeps optional storage errors outside message submission", () => {
    vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => { throw new Error("unavailable"); });
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => { throw new Error("unavailable"); });
    vi.spyOn(Storage.prototype, "removeItem").mockImplementation(() => { throw new Error("unavailable"); });
    expect(readPendingTextJob(conversationID)).toBeNull();
    expect(() => savePendingTextJob(conversationID, jobID, 0)).not.toThrow();
    expect(() => clearPendingTextJob(conversationID)).not.toThrow();
  });
});
