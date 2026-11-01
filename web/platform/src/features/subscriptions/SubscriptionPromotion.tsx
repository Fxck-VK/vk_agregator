"use client";

import { createContext, type ReactNode, useCallback, useContext, useLayoutEffect, useRef, useState, useSyncExternalStore } from "react";
import { PaymentRequestError } from "@/features/payments/payments";
import { browserSessionSnapshot } from "@/lib/web-api/browser-session-state";
import { requestPromotionQuote, type PromotionQuote } from "./promotions";

type PromotionFailure = "invalid" | "ineligible" | "unavailable";
type PromotionState = {
  quote: PromotionQuote | null; pending: boolean; enabled: boolean; preview: boolean; error: PromotionFailure | null;
  apply: (code: string) => Promise<boolean>; clear: () => void; cancel: () => void;
};
const unavailable: PromotionState = { quote: null, pending: false, enabled: false, preview: false, error: null, apply: async () => false, clear: () => {}, cancel: () => {} };
const PromotionContext = createContext<PromotionState>(unavailable);
type Props = { accountId: string; preview?: boolean; children: ReactNode };

function subscribeSession(notify: () => void) {
  let active = true, signal: AbortSignal;
  const aborted = () => { notify(); queueMicrotask(() => { if (active) { attach(); notify(); } }); };
  const attach = () => { signal?.removeEventListener("abort", aborted); signal = browserSessionSnapshot().signal; signal.addEventListener("abort", aborted, { once: true }); };
  attach(); return () => { active = false; signal.removeEventListener("abort", aborted); };
}
const sessionRevision = () => browserSessionSnapshot().revision;
const serverRevision = () => -1;

export function SubscriptionPromotionProvider(props: Props) {
  const revision = useSyncExternalStore(subscribeSession, sessionRevision, serverRevision);
  return <AccountPromotion {...props} revision={revision} />;
}

function AccountPromotion({ accountId, preview = false, children, revision }: Props & { revision: number }) {
  const identity = `${accountId}:${preview}:${revision}`, latestIdentity = useRef(identity);
  const [stored, setStored] = useState<{ identity: string; quote: PromotionQuote | null; pending: boolean; error: PromotionFailure | null }>({ identity, quote: null, pending: false, error: null });
  const { quote, pending, error } = stored.identity === identity ? stored : { quote: null, pending: false, error: null };
  const request = useRef<AbortController | null>(null), alive = useRef(true);
  useLayoutEffect(() => { latestIdentity.current = identity; alive.current = true; return () => { alive.current = false; request.current?.abort(); }; }, [identity]);
  const cancel = useCallback(() => { request.current?.abort(); request.current = null; if (alive.current) setStored(previous => ({ identity, quote: previous.identity === identity ? previous.quote : null, pending: false, error: null })); }, [identity]);
  const clear = useCallback(() => { cancel(); setStored({ identity, quote: null, pending: false, error: null }); }, [cancel, identity]);
  const apply = useCallback(async (code: string) => {
    if (preview || !accountId) return false;
    const session = browserSessionSnapshot();
    if (session.accountId && session.accountId !== accountId) return false;
    request.current?.abort(); const controller = new AbortController(); request.current = controller;
    const current = () => alive.current && latestIdentity.current === identity && request.current === controller && !controller.signal.aborted && browserSessionSnapshot().revision === session.revision;
    setStored({ identity, pending: true, error: null, quote: null });
    try {
      const value = await requestPromotionQuote(code, AbortSignal.any([controller.signal, session.signal]));
      if (!current()) return false;
      setStored({ identity, quote: value, pending: false, error: null }); return true;
    } catch (cause) {
      if (current()) setStored({ identity, quote: null, pending: false, error: cause instanceof PaymentRequestError ? cause.status === 400 ? "invalid" : cause.status === 409 ? "ineligible" : "unavailable" : "unavailable" });
      return false;
    } finally { if (current()) request.current = null; }
  }, [accountId, preview, identity]);
  return <PromotionContext.Provider value={{ quote, pending, error, enabled: !preview && Boolean(accountId), preview, apply, clear, cancel }}>{children}</PromotionContext.Provider>;
}

export function useSubscriptionPromotion() { return useContext(PromotionContext); }
