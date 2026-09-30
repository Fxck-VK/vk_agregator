import { afterEach, expect, it, vi } from "vitest";
import { createReadResource } from "./read-resource";
import { ReadError } from "./web-api/read-error";
afterEach(() => vi.useRealTimers());
it("automatically recovers a transient read while retaining the visible snapshot", async () => {
  vi.useFakeTimers();
  const loader = vi.fn().mockRejectedValueOnce(new ReadError("unavailable", 503)).mockResolvedValue([2]);
  const resource = createReadResource(loader, [1]);
  await resource.load(true);
  expect(resource.getSnapshot()).toMatchObject({ data: [1], failed: true, error: { kind: "unavailable" } });
  await vi.advanceTimersByTimeAsync(2500);
  expect(resource.getSnapshot()).toMatchObject({ data: [2], failed: false, error: null });
  resource.dispose();
});
it("does not retry terminal errors or resurrect a disposed request", async () => {
  vi.useFakeTimers();
  const terminal = vi.fn().mockRejectedValue(new ReadError("forbidden", 403));
  const resource = createReadResource(terminal);
  await resource.load(); await vi.advanceTimersByTimeAsync(120000);
  expect(terminal).toHaveBeenCalledTimes(1);
  resource.dispose();
  const transient = vi.fn().mockRejectedValue(new ReadError("network"));
  const abandoned = createReadResource(transient);
  await abandoned.load(); abandoned.dispose(); await vi.advanceTimersByTimeAsync(120000);
  expect(transient).toHaveBeenCalledTimes(1);
});
it("pauses automatic retries offline and resumes without losing cached data", async () => {
  vi.useFakeTimers();
  const online = vi.spyOn(navigator, "onLine", "get").mockReturnValue(false);
  const loader = vi.fn().mockRejectedValueOnce(new ReadError("network")).mockResolvedValue([3]);
  const resource = createReadResource(loader, [1]);
  await resource.load(true); await vi.advanceTimersByTimeAsync(25_000);
  expect(loader).toHaveBeenCalledTimes(1);
  online.mockReturnValue(true); await resource.resume();
  expect(resource.getSnapshot().data).toEqual([3]);
  resource.dispose(); online.mockRestore();
});
it("respects Retry-After even when focus events request a refresh", async () => {
  vi.useFakeTimers();
  const loader = vi.fn().mockRejectedValueOnce(new ReadError("rate_limit", 429, 20_000)).mockResolvedValue([2]);
  const resource = createReadResource(loader);
  await resource.load(); await resource.resume(); await vi.advanceTimersByTimeAsync(19_999);
  expect(loader).toHaveBeenCalledTimes(1);
  await vi.advanceTimersByTimeAsync(1000); expect(resource.getSnapshot().data).toEqual([2]);
  resource.dispose();
});
it("deduplicates reads, retains a good snapshot on failure and recovers", async () => {
  const loader = vi.fn().mockResolvedValueOnce([1]).mockRejectedValueOnce(new Error("offline")).mockResolvedValueOnce([2]);
  const resource = createReadResource(loader);
  await Promise.all([resource.load(), resource.load()]);
  expect(loader).toHaveBeenCalledTimes(1);
  await resource.load(true);
  expect(resource.getSnapshot()).toMatchObject({ data: [1], failed: true, pending: false });
  await resource.load(true);
  expect(resource.getSnapshot()).toMatchObject({ data: [2], failed: false });
});
it("serves the seed without a request and rejects late results after owner disposal", async () => {
  vi.useFakeTimers();
  let complete!: (value: number) => void;
  const loader = vi.fn(() => new Promise<number>(resolve => { complete = resolve; }));
  const resource = createReadResource(loader, 1);
  await resource.load();
  expect(loader).not.toHaveBeenCalled();
  vi.advanceTimersByTime(60_001);
  const pending = resource.load();
  await Promise.resolve(); resource.dispose(); complete(2); await pending;
  expect(resource.getSnapshot().data).toBe(1);
});
it("invalidates an older in-flight balance read after an accepted update", async () => {
  let finishOld!: (value: number) => void;
  const loader = vi.fn().mockImplementationOnce(() => new Promise<number>(resolve => { finishOld = resolve; })).mockResolvedValueOnce(120);
  const resource = createReadResource<number>(loader, 100);
  const old = resource.load(true);
  await Promise.resolve();
  await resource.invalidate();
  finishOld(100); await old;
  expect(resource.getSnapshot().data).toBe(120);
});
