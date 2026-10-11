import { browserSessionSnapshot } from "@/lib/web-api/browser-session-state";

type Session = ReturnType<typeof browserSessionSnapshot>;
let cached: { accountId: string; revision: number; code: string } | null = null;
let detachAbort: (() => void) | undefined;
const listeners = new Set<() => void>();

export function subscribeReferralCode(listener: () => void) {
  listeners.add(listener);
  return () => { listeners.delete(listener); };
}

function clearReferralCode() {
  detachAbort?.(); detachAbort = undefined;
  cached = null;
  listeners.forEach(listener => listener());
}

export function readReferralCode(accountId: string): string | null {
  if (typeof window === "undefined") return null;
  const session = browserSessionSnapshot();
  return session.accountId === accountId && cached?.accountId === accountId && cached.revision === session.revision
    ? cached.code : null;
}

/** Only a validated response from the still-current, confirmed session can write. */
export function rememberReferralCode(accountId: string, code: string, expected: Session): boolean {
  const current = browserSessionSnapshot();
  if (typeof window === "undefined" || expected.signal.aborted || expected.accountId !== accountId ||
      current.accountId !== accountId || current.revision !== expected.revision || !/^[A-Za-z0-9_-]{4,64}$/.test(code)) return false;
  detachAbort?.();
  cached = { accountId, revision: expected.revision, code };
  expected.signal.addEventListener("abort", clearReferralCode, { once: true });
  detachAbort = () => expected.signal.removeEventListener("abort", clearReferralCode);
  listeners.forEach(listener => listener());
  return true;
}
