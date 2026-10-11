import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { startGenerationPolling } from "./generation-polling";

function connection(online: boolean) {
  Object.defineProperty(navigator, "onLine", { configurable: true, value: online });
  window.dispatchEvent(new Event(online ? "online" : "offline"));
}

describe("generation polling connectivity", () => {
  const cleanups: Array<() => void> = [];
  beforeEach(() => { vi.useFakeTimers(); connection(true); });
  afterEach(() => {
    cleanups.splice(0).forEach(stop => stop());
    connection(true);
    Object.defineProperty(document, "visibilityState", { configurable: true, value: "visible" });
    vi.useRealTimers();
  });

  function start(options: Parameters<typeof startGenerationPolling>[0]) {
    const polling = startGenerationPolling(options);
    cleanups.push(polling.stop);
    return polling;
  }

  it("preserves the initial delay and online interval without overlapping requests", async () => {
    const poll = vi.fn().mockResolvedValue(true);
    start({ poll, delayMs: 4000, initialDelayMs: 1500 });
    await vi.advanceTimersByTimeAsync(1499);
    expect(poll).not.toHaveBeenCalled();
    await vi.advanceTimersByTimeAsync(1);
    expect(poll).toHaveBeenCalledTimes(1);
    await vi.advanceTimersByTimeAsync(3999);
    expect(poll).toHaveBeenCalledTimes(1);
    await vi.advanceTimersByTimeAsync(1);
    expect(poll).toHaveBeenCalledTimes(2);
  });

  it("waits offline, resumes immediately once, and returns to the old cadence", async () => {
    connection(false);
    const poll = vi.fn().mockResolvedValue(true);
    start({ poll, delayMs: 2500 });
    await vi.advanceTimersByTimeAsync(60000);
    expect(poll).not.toHaveBeenCalled();
    connection(true);
    connection(true);
    expect(poll).toHaveBeenCalledTimes(1);
    await vi.advanceTimersByTimeAsync(2499);
    expect(poll).toHaveBeenCalledTimes(1);
    await vi.advanceTimersByTimeAsync(1);
    expect(poll).toHaveBeenCalledTimes(2);
  });

  it("aborts a disconnected read and ignores its late completion after reconnect", async () => {
    let finishOld!: (repeat: boolean) => void;
    const poll = vi.fn().mockImplementationOnce(() => new Promise<boolean>(resolve => { finishOld = resolve; })).mockResolvedValue(true);
    start({ poll, delayMs: 2000 });
    const oldSignal = poll.mock.calls[0][0] as AbortSignal;
    await vi.advanceTimersByTimeAsync(5000);
    expect(poll).toHaveBeenCalledTimes(1);
    connection(false);
    expect(oldSignal.aborted).toBe(true);
    await vi.advanceTimersByTimeAsync(60000);
    expect(poll).toHaveBeenCalledTimes(1);
    connection(true);
    expect(poll).toHaveBeenCalledTimes(2);
    finishOld(false);
    await vi.advanceTimersByTimeAsync(2000);
    expect(poll).toHaveBeenCalledTimes(3);
  });

  it("pauses the deadline offline without losing the active job or spending attempts", async () => {
    const poll = vi.fn().mockResolvedValue(true);
    const onExhausted = vi.fn();
    start({ poll, delayMs: 2000, timeoutMs: 30000, maxAttempts: 15, onExhausted });
    await vi.advanceTimersByTimeAsync(1000);
    connection(false);
    await vi.advanceTimersByTimeAsync(900000);
    expect(poll).toHaveBeenCalledTimes(1);
    expect(onExhausted).not.toHaveBeenCalled();
    connection(true);
    await vi.advanceTimersByTimeAsync(26000);
    expect(poll).toHaveBeenCalledTimes(15);
    expect(onExhausted).toHaveBeenCalledTimes(1);
    connection(false); connection(true);
    await vi.advanceTimersByTimeAsync(30000);
    expect(poll).toHaveBeenCalledTimes(15);
  });

  it("still aborts a hung read at the original connected-time deadline", async () => {
    const onExhausted = vi.fn();
    const poll = vi.fn().mockImplementation(() => new Promise<boolean>(() => {}));
    start({ poll, delayMs: 2000, timeoutMs: 30000, onExhausted });
    await vi.advanceTimersByTimeAsync(5000);
    connection(false);
    await vi.advanceTimersByTimeAsync(60000);
    connection(true);
    await vi.advanceTimersByTimeAsync(24999);
    expect(onExhausted).not.toHaveBeenCalled();
    await vi.advanceTimersByTimeAsync(1);
    expect(onExhausted).toHaveBeenCalledTimes(1);
    expect((poll.mock.calls[1][0] as AbortSignal).aborted).toBe(true);
  });

  it("keeps existing hidden-tab pauses and reconnects only when visible", async () => {
    Object.defineProperty(document, "visibilityState", { configurable: true, value: "hidden" });
    const poll = vi.fn().mockResolvedValue(true);
    start({ poll, delayMs: 1500, pauseWhenHidden: true });
    connection(false); connection(true);
    await vi.advanceTimersByTimeAsync(30000);
    expect(poll).not.toHaveBeenCalled();
    Object.defineProperty(document, "visibilityState", { configurable: true, value: "visible" });
    document.dispatchEvent(new Event("visibilitychange"));
    await vi.advanceTimersByTimeAsync(0);
    expect(poll).toHaveBeenCalledTimes(1);
  });

  it("retains the existing first image delay after a visibility refresh", async () => {
    const poll = vi.fn().mockResolvedValue(true);
    start({ poll, delayMs: attempt => [1500, 3000, 6000][Math.min(attempt, 2)], initialDelayMs: 1500, pauseWhenHidden: true });
    Object.defineProperty(document, "visibilityState", { configurable: true, value: "hidden" });
    document.dispatchEvent(new Event("visibilitychange"));
    Object.defineProperty(document, "visibilityState", { configurable: true, value: "visible" });
    document.dispatchEvent(new Event("visibilitychange"));
    await vi.advanceTimersByTimeAsync(1499);
    expect(poll).toHaveBeenCalledTimes(1);
    await vi.advanceTimersByTimeAsync(1);
    expect(poll).toHaveBeenCalledTimes(2);
  });

  it("cleanup prevents reconnect/manual refresh and aborts the current read", async () => {
    const poll = vi.fn().mockImplementation(() => new Promise<boolean>(() => {}));
    const polling = start({ poll, delayMs: 4000 });
    const signal = poll.mock.calls[0][0] as AbortSignal;
    polling.stop();
    expect(signal.aborted).toBe(true);
    connection(false); connection(true); polling.refresh();
    await vi.advanceTimersByTimeAsync(30000);
    expect(poll).toHaveBeenCalledTimes(1);
  });
});
