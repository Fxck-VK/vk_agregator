import type { MusicJob, MusicJobResult } from "./music-api";

export const maxCachedMusicResults = 60;

/** Private result metadata owned by one account workspace; never persisted. */
export function createMusicResultCache() {
  const results = new Map<string, { createdAt: string; result: MusicJobResult }>();
  return {
    get(jobID: string) {
      const entry = results.get(jobID);
      if (entry) {
        results.delete(jobID);
        results.set(jobID, entry);
      }
      return entry?.result;
    },
    getAll() {
      return [...results.values()]
        .sort((left, right) => Date.parse(right.createdAt) - Date.parse(left.createdAt))
        .map(entry => entry.result);
    },
    remember(job: MusicJob, result: MusicJobResult) {
      if (job.status !== "succeeded" || result.job_id !== job.id) throw new Error("Music result does not match a completed job.");
      results.delete(job.id);
      results.set(job.id, { createdAt: job.created_at, result });
      if (results.size > maxCachedMusicResults) results.delete(results.keys().next().value!);
    },
  };
}

export type MusicResultCache = ReturnType<typeof createMusicResultCache>;

/** Share reads within one mounted controller; an aborted owner cannot fill the cache. */
export function createMusicResultReader(
  cache: MusicResultCache,
  loader: (jobID: string, signal: AbortSignal) => Promise<MusicJobResult>,
  signal: AbortSignal,
) {
  const pending = new Map<string, { signal: AbortSignal; promise: Promise<MusicJobResult> }>();
  return (job: MusicJob, readSignal?: AbortSignal): Promise<MusicJobResult> => {
    const requestSignal = readSignal ? AbortSignal.any([signal, readSignal]) : signal;
    if (requestSignal.aborted) return Promise.reject(new DOMException("Music read cancelled.", "AbortError"));
    if (job.status !== "succeeded") return Promise.reject(new Error("Music result requires a completed job."));
    const cached = cache.get(job.id);
    if (cached) return Promise.resolve(cached);
    const existing = pending.get(job.id);
    if (existing && !existing.signal.aborted) return existing.promise;
    const request = Promise.resolve().then(() => {
      if (requestSignal.aborted) throw new DOMException("Music read cancelled.", "AbortError");
      return loader(job.id, requestSignal);
    }).then(result => {
      if (requestSignal.aborted) throw new DOMException("Music read cancelled.", "AbortError");
      cache.remember(job, result);
      return result;
    }).finally(() => { if (pending.get(job.id)?.promise === request) pending.delete(job.id); });
    pending.set(job.id, { signal: requestSignal, promise: request });
    return request;
  };
}
