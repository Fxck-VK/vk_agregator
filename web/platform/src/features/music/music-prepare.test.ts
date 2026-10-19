import { afterEach, describe, expect, it, vi } from "vitest";

import { webBrowserMutation } from "@/lib/web-api/browser";
import { MusicApiError, prepareMusicJob, type MusicPrepareBody } from "./music-api";

vi.mock("@/lib/web-api/browser", () => ({ webBrowserFetch: vi.fn(), webBrowserMutation: vi.fn() }));

const body: MusicPrepareBody = {
  audio_artifact_ids: [], model_id: "suno_v6",
  music: { action: "extend", continue_at_sec: 45, prompt: "synthetic continuation" },
  sources: [{ job_id: "11111111-1111-4111-8111-111111111111", audio_index: 1 }],
};
const key = "22222222-2222-4222-8222-222222222222";
const preparation = {
  balance: 100, can_afford: true,
  job: { id: "33333333-3333-4333-8333-333333333333", action: "extend", model_id: "suno_v6", cost_estimate: 30, created_at: "2026-10-19T10:00:00Z", status: "prepared" },
};

afterEach(() => { vi.useRealTimers(); vi.resetAllMocks(); });

describe("music preparation deadline", () => {
  it("aborts a hung price request without automatically retrying or activating it", async () => {
    vi.useFakeTimers();
    let signal: AbortSignal | undefined;
    vi.mocked(webBrowserMutation).mockImplementation((_path, init) => new Promise((_resolve, reject) => {
      signal = init.signal ?? undefined;
      signal?.addEventListener("abort", () => reject(signal?.reason), { once: true });
    }));
    const result = prepareMusicJob(body, key).catch(error => error);
    expect(signal).toBeInstanceOf(AbortSignal);
    await vi.advanceTimersByTimeAsync(20_000);
    expect(signal?.aborted).toBe(true);
    const error = await result;
    expect(error).toBeInstanceOf(MusicApiError);
    expect(error.status).toBe(408);
    expect(webBrowserMutation).toHaveBeenCalledTimes(1);
    expect(vi.mocked(webBrowserMutation).mock.calls[0][0]).toBe("/web/v1/music-jobs/prepare");
  });

  it("keeps the idempotency key for a manual retry after an unknown preparation result", async () => {
    vi.useFakeTimers();
    vi.mocked(webBrowserMutation).mockImplementationOnce((_path, init) => new Promise((_resolve, reject) => {
      init.signal?.addEventListener("abort", () => reject(init.signal?.reason), { once: true });
    })).mockResolvedValueOnce(new Response(JSON.stringify(preparation), { status: 201 }));
    const first = prepareMusicJob(body, key).catch(error => error);
    await vi.advanceTimersByTimeAsync(20_000);
    expect(await first).toBeInstanceOf(MusicApiError);
    expect(await prepareMusicJob(body, key)).toEqual(preparation);
    expect(webBrowserMutation).toHaveBeenCalledTimes(2);
    for (const [path, init] of vi.mocked(webBrowserMutation).mock.calls) {
      expect(path).toBe("/web/v1/music-jobs/prepare");
      expect(init.headers).toMatchObject({ "X-Idempotency-Key": key });
    }
    expect(vi.getTimerCount()).toBe(0);
  });
});
