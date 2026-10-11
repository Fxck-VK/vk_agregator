import { describe, expect, it, vi } from "vitest";
import type { MusicJob, MusicJobResult } from "./music-api";
import { createMusicResultCache, createMusicResultReader } from "./music-result-cache";

const job: MusicJob = {
  action: "generate", cost_estimate: 5, created_at: "2026-09-16T10:00:00Z",
  id: "33333333-3333-4333-8333-333333333333", model_id: "suno_v6", status: "succeeded",
};
const result: MusicJobResult = { artifacts: [], job_id: job.id, tracks: [] };

describe("music result cache", () => {
  it("cancels an offline result read and retries without an old promise evicting the new request", async () => {
    const cache = createMusicResultCache();
    let finishOld!: (result: MusicJobResult) => void;
    let finishNew!: (result: MusicJobResult) => void;
    const loader = vi.fn()
      .mockImplementationOnce(() => new Promise<MusicJobResult>(resolve => { finishOld = resolve; }))
      .mockImplementationOnce(() => new Promise<MusicJobResult>(resolve => { finishNew = resolve; }));
    const read = createMusicResultReader(cache, loader, new AbortController().signal);
    const offlineRead = new AbortController();
    const first = read(job, offlineRead.signal);
    const cancelled = expect(first).rejects.toMatchObject({ name: "AbortError" });
    await Promise.resolve();
    offlineRead.abort();
    expect((loader.mock.calls[0][1] as AbortSignal).aborted).toBe(true);
    const second = read(job);
    await Promise.resolve();
    expect(loader).toHaveBeenCalledTimes(2);
    finishOld(result);
    await cancelled;
    expect(cache.getAll()).toEqual([]);
    const joined = read(job);
    finishNew(result);
    expect(await second).toBe(result);
    expect(await joined).toBe(result);
    expect(loader).toHaveBeenCalledTimes(2);
  });
  it("coalesces concurrent completed-result reads and reuses successful results", async () => {
    const cache = createMusicResultCache();
    let finish!: (value: MusicJobResult) => void;
    const loader = vi.fn(() => new Promise<MusicJobResult>(resolve => { finish = resolve; }));
    const read = createMusicResultReader(cache, loader, new AbortController().signal);
    const first = read(job);
    const second = read(job);
    await Promise.resolve();
    expect(loader).toHaveBeenCalledTimes(1);
    finish(result);
    expect(await first).toBe(result);
    expect(await second).toBe(result);
    expect(await read(job)).toBe(result);
    expect(loader).toHaveBeenCalledTimes(1);
  });

  it("does not cache a failed read and permits a later retry", async () => {
    const loader = vi.fn().mockRejectedValueOnce(new Error("unavailable")).mockResolvedValueOnce(result);
    const cache = createMusicResultCache();
    const read = createMusicResultReader(cache, loader, new AbortController().signal);
    await expect(read(job)).rejects.toThrow("unavailable");
    expect(cache.getAll()).toEqual([]);
    expect(await read(job)).toBe(result);
    expect(loader).toHaveBeenCalledTimes(2);
  });

  it("rejects late cancelled responses and a result belonging to a different job", async () => {
    const cache = createMusicResultCache();
    const request = new AbortController();
    let finish!: (value: MusicJobResult) => void;
    const read = createMusicResultReader(cache, () => new Promise(resolve => { finish = resolve; }), request.signal);
    const pending = read(job);
    const rejected = expect(pending).rejects.toMatchObject({ name: "AbortError" });
    await Promise.resolve();
    request.abort();
    finish(result);
    await rejected;
    expect(cache.getAll()).toEqual([]);
    const wrongResult = createMusicResultReader(cache, async () => ({ ...result, job_id: "44444444-4444-4444-8444-444444444444" }), new AbortController().signal);
    await expect(wrongResult(job)).rejects.toThrow("does not match");
    expect(cache.getAll()).toEqual([]);
  });

  it("keeps results bounded, sorted by job creation, and independent between owners", async () => {
    const cache = createMusicResultCache();
    const read = createMusicResultReader(cache, async id => ({ ...result, job_id: id }), new AbortController().signal);
    for (let index = 0; index < 61; index += 1) {
      await read({ ...job, id: `job-${index}`, created_at: new Date(Date.UTC(2026, 8, 16, 10, index)).toISOString() });
    }
    expect(cache.getAll()).toHaveLength(60);
    expect(cache.get(job.id)).toBeUndefined();
    expect(cache.get("job-0")).toBeUndefined();
    expect(cache.getAll()[0]?.job_id).toBe("job-60");
    expect(createMusicResultCache().getAll()).toEqual([]);
  });

  it("does not treat an unfinished job as a cacheable result", async () => {
    const cache = createMusicResultCache();
    const loader = vi.fn().mockResolvedValue(result);
    const read = createMusicResultReader(cache, loader, new AbortController().signal);
    await expect(read({ ...job, status: "result_ready" })).rejects.toThrow("completed");
    expect(loader).not.toHaveBeenCalled();
    expect(cache.getAll()).toEqual([]);
  });
});
