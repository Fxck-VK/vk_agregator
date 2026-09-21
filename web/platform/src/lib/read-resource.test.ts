import { afterEach, expect, it, vi } from "vitest";
import { createReadResource } from "./read-resource";
afterEach(() => vi.useRealTimers());
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
