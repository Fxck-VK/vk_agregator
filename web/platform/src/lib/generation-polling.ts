type GenerationPollingOptions = {
  /** Return true to check again. Publish results only while signal is not aborted. */
  poll: (signal: AbortSignal) => Promise<boolean>;
  delayMs: number | ((attempt: number) => number);
  initialDelayMs?: number;
  autoStart?: boolean;
  pauseWhenHidden?: boolean;
  maxAttempts?: number;
  timeoutMs?: number;
  onExhausted?: () => void;
};

/** Only coordinates reads. It never replays generation submissions or other writes. */
export function startGenerationPolling({
  poll, delayMs, initialDelayMs, autoStart = true, pauseWhenHidden = false,
  maxAttempts = Infinity, timeoutMs, onExhausted,
}: GenerationPollingOptions) {
  const browserOnline = () => navigator.onLine !== false;
  let disposed = false;
  let finished = !autoStart;
  let connected = browserOnline();
  let available = connected && (!pauseWhenHidden || document.visibilityState !== "hidden");
  let attempts = 0;
  let delayAttempt = 0;
  let timer: ReturnType<typeof setTimeout> | undefined;
  let deadline: ReturnType<typeof setTimeout> | undefined;
  let remainingMs = timeoutMs;
  let budgetStartedAt: number | undefined;
  let request: AbortController | undefined;

  const clearTimer = () => { clearTimeout(timer); timer = undefined; };
  const pauseBudget = () => {
    clearTimeout(deadline);
    deadline = undefined;
    if (remainingMs !== undefined && budgetStartedAt !== undefined) {
      remainingMs = Math.max(0, remainingMs - (Date.now() - budgetStartedAt));
    }
    budgetStartedAt = undefined;
  };
  const abortRead = () => {
    const oldRequest = request;
    request = undefined;
    oldRequest?.abort();
  };
  const finish = (exhausted = false) => {
    if (disposed || finished) return;
    finished = true;
    clearTimer(); pauseBudget(); abortRead();
    if (exhausted) onExhausted?.();
  };
  const resumeBudget = () => {
    if (remainingMs === undefined || budgetStartedAt !== undefined) return;
    budgetStartedAt = Date.now();
    deadline = setTimeout(() => finish(true), remainingMs);
  };
  const schedule = (delay: number) => {
    clearTimer();
    if (!disposed && !finished && available) timer = setTimeout(() => void run(), delay);
  };
  const run = async () => {
    if (disposed || finished || !available || request) return;
    if (!browserOnline()) { offline(); return; }
    clearTimer();
    if (attempts >= maxAttempts) { finish(true); return; }
    attempts += 1;
    const controller = new AbortController();
    request = controller;
    let repeat = true;
    try {
      repeat = await poll(controller.signal);
    } catch {
      // Callers handle actionable errors. An interrupted read is never a terminal job.
    } finally {
      if (!browserOnline() && available) offline();
      if (disposed || finished || request !== controller || controller.signal.aborted) return;
      request = undefined;
      if (!repeat) { finish(); return; }
      if (attempts >= maxAttempts) { finish(true); return; }
      delayAttempt += 1;
      schedule(typeof delayMs === "function" ? delayMs(delayAttempt) : delayMs);
    }
  };
  const updateAvailability = (resetDelay: boolean) => {
    const next = connected && (!pauseWhenHidden || document.visibilityState !== "hidden");
    if (disposed || next === available) return;
    available = next;
    if (finished) return;
    if (!available) {
      clearTimer(); pauseBudget();
      if (request) attempts -= 1; // A disconnected read does not consume a retry.
      abortRead();
    } else {
      // Existing image trackers start their delay sequence after the visible check.
      if (resetDelay) delayAttempt = -1;
      resumeBudget();
      void run();
    }
  };
  const offline = () => { connected = false; updateAvailability(false); };
  const online = () => { connected = browserOnline(); updateAvailability(false); };
  const visibility = () => updateAvailability(true);
  window.addEventListener("offline", offline);
  window.addEventListener("online", online);
  if (pauseWhenHidden) document.addEventListener("visibilitychange", visibility);
  if (available && !finished) {
    resumeBudget();
    if (initialDelayMs === undefined) void run();
    else schedule(initialDelayMs);
  }

  return {
    refresh() {
      if (disposed || !available || request) return;
      if (finished) { finished = false; attempts = 0; delayAttempt = 0; }
      resumeBudget();
      void run();
    },
    stop() {
      if (disposed) return;
      disposed = true;
      clearTimer(); pauseBudget(); abortRead();
      window.removeEventListener("offline", offline);
      window.removeEventListener("online", online);
      if (pauseWhenHidden) document.removeEventListener("visibilitychange", visibility);
    },
  };
}
