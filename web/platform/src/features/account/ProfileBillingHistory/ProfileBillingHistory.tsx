"use client";

import { useCallback, useEffect, useRef, useState, useSyncExternalStore } from "react";
import { StateNotice } from "@/components/ui/AsyncState/AsyncState";
import { Button } from "@/components/ui/Button/Button";
import { CreditAmount } from "@/components/ui/CreditAmount/CreditAmount";
import { formatPaymentNumber, formatPaymentPrice } from "@/features/payments/payments";
import { useDictionary, useLocale } from "@/i18n/LocaleProvider";
import { browserSessionSnapshot } from "@/lib/web-api/browser-session-state";
import { loadBillingHistory, type HistoryItem } from "./history";
import styles from "./ProfileBillingHistory.module.css";

type Props = { accountId: string; preview?: boolean };
type HistoryState = { items: HistoryItem[]; cursor: string | null; phase: "loading" | "ready" | "error"; loaded: boolean };

// Session state exposes cancellation rather than a subscription API. Rearm after
// abort has finished replacing its controller, including a same-account login.
function subscribeSession(notify: () => void) {
  let active = true;
  let signal: AbortSignal;
  const aborted = () => {
    notify();
    queueMicrotask(() => { if (active) { attach(); notify(); } });
  };
  const attach = () => {
    signal?.removeEventListener("abort", aborted);
    signal = browserSessionSnapshot().signal;
    signal.addEventListener("abort", aborted, { once: true });
  };
  attach();
  return () => { active = false; signal.removeEventListener("abort", aborted); };
}
const sessionRevision = () => browserSessionSnapshot().revision;
const serverRevision = () => -1;

export function ProfileBillingHistory(props: Props) {
  const revision = useSyncExternalStore(subscribeSession, sessionRevision, serverRevision);
  return <AccountBillingHistory key={`${props.accountId}:${Boolean(props.preview)}:${revision}`} {...props} revision={revision} />;
}

function AccountBillingHistory({ accountId, preview = false, revision }: Props & { revision: number }) {
  const t = useDictionary().profile.billingHistory;
  const locale = useLocale();
  const [state, setState] = useState<HistoryState>({ items: [], cursor: null, phase: "loading", loaded: false });
  const controller = useRef<AbortController | null>(null);
  const requestId = useRef(0);
  const busy = useRef(false);
  const seenCursors = useRef(new Set<string>());
  const session = browserSessionSnapshot();
  const ownsSession = session.accountId === accountId && session.revision === revision && !session.signal.aborted;

  const load = useCallback(async (cursor: string | null) => {
    const session = browserSessionSnapshot();
    if (preview || busy.current || !controller.current || session.accountId !== accountId || session.revision !== revision) return;
    busy.current = true;
    const id = ++requestId.current;
    const signal = AbortSignal.any([session.signal, controller.current.signal]);
    const current = () => !signal.aborted && requestId.current === id && browserSessionSnapshot().revision === session.revision && browserSessionSnapshot().accountId === accountId;
    try {
      const page = await loadBillingHistory(cursor, signal);
      if (!current()) return;
      if (page.next_cursor !== null && (page.next_cursor === cursor || seenCursors.current.has(page.next_cursor))) throw new Error("Invalid history cursor");
      if (cursor) seenCursors.current.add(cursor);
      setState(previous => {
        const items = cursor === null ? [] : previous.items;
        const ids = new Set(items.map(item => item.id));
        const added = page.items.filter(item => { if (ids.has(item.id)) return false; ids.add(item.id); return true; });
        return { items: [...items, ...added], cursor: page.next_cursor, phase: "ready", loaded: true };
      });
    } catch {
      if (current()) setState(previous => ({ ...previous, phase: "error" }));
    } finally {
      if (current()) busy.current = false;
    }
  }, [accountId, preview, revision]);

  useEffect(() => {
    const lifetime = new AbortController();
    controller.current = lifetime;
    void load(null);
    return () => { lifetime.abort(); busy.current = false; };
  }, [load]);

  const requestPage = (cursor: string | null) => {
    if (busy.current) return;
    setState(previous => ({ ...previous, phase: "loading" }));
    void load(cursor);
  };

  if (preview) return <div className={styles.history}><StateNotice kind="info">{t.preview}</StateNotice></div>;
  if (revision === -1) return <div className={styles.history}><StateNotice kind="loading">{t.loading}</StateNotice></div>;
  if (!ownsSession) return <div className={styles.history}><StateNotice kind="info">{t.sessionUnavailable}</StateNotice></div>;
  const title = (item: HistoryItem) => item.kind === "generation" && item.operation ? t.operations[item.operation] : t.kinds[item.kind];
  // Format the whole and fractional parts separately: dividing a large safe
  // minor-unit integer by 100 otherwise loses a stored kopek to floating point.
  const money = (minor: number) => formatPaymentPrice(minor % 100, locale).replace(/^0/, formatPaymentNumber(Math.floor(minor / 100), locale));
  return <div className={styles.history}>
    {state.loaded && state.items.length === 0 ? <StateNotice>{t.empty}</StateNotice> : null}
    {state.items.length ? <ul className={styles.rows} aria-label={t.listLabel}>
      {state.items.map(item => <li key={item.id} className={styles.row}>
        <div className={styles.description}>
          <strong>{title(item)}{item.kind === "subscription" && item.plan_code ? ` · ${t.plans[item.plan_code]}` : ""}</strong>
          <div className={styles.details}><span>{t.statuses[item.status]}</span>{item.test_mode === true ? <span className={styles.badge}>{t.testMode}</span> : null}</div>
          <time dateTime={item.created_at}>{new Intl.DateTimeFormat(locale, { dateStyle: "medium", timeStyle: "short" }).format(new Date(item.created_at))}</time>
        </div>
        <div className={styles.amounts}>
          <CreditAmount value={item.stars_delta} prefix={item.stars_delta > 0 ? "+" : undefined} />
          {item.money ? <span>{money(item.money.amount_minor)} ₽</span> : null}
        </div>
      </li>)}
    </ul> : null}
    {state.phase === "loading" ? <StateNotice kind="loading">{state.loaded ? t.loadingMore : t.loading}</StateNotice> : null}
    {state.phase === "error" ? <StateNotice kind="error" action={{ label: t.retry, onClick: () => requestPage(state.loaded ? state.cursor : null) }}>{state.loaded ? t.pageError : t.error}</StateNotice> : null}
    {state.phase === "ready" && state.cursor ? <Button variant="outline" onClick={() => requestPage(state.cursor)}>{t.more}</Button> : null}
  </div>;
}
