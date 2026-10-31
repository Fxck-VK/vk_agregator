"use client";

import { useState } from "react";
import { Button } from "@/components/ui/Button/Button";
import { StateNotice } from "@/components/ui/AsyncState/AsyncState";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog/ConfirmDialog";
import { CreditAmount } from "@/components/ui/CreditAmount/CreditAmount";
import { formatPaymentPrice } from "@/features/payments/payments";
import { useReadResource } from "@/lib/use-read-resource";
import { useLocale } from "@/i18n/LocaleProvider";
import { cancelSubscription, loadSubscription, type SubscriptionState } from "./subscriptions";
import { subscriptionCopy } from "./copy";
import styles from "./subscriptions.module.css";

export function ProfileSubscriptionCard({ preview = false }: { preview?: boolean }) {
  const locale = useLocale(), t = subscriptionCopy[locale];
  const resource = useReadResource(loadSubscription, null, !preview);
  const [confirm, setConfirm] = useState(false), [busy, setBusy] = useState(false), [error, setError] = useState("");
  const [mutation, setMutation] = useState<{ before: SubscriptionState | null; after: SubscriptionState } | null>(null);
  const state = mutation?.before === resource.data ? mutation.after : resource.data, plan = state?.subscription;
  const date = (value: string) => new Intl.DateTimeFormat(locale, { dateStyle: "medium", timeStyle: "short" }).format(new Date(value));
  async function cancel() {
    if (busy || preview) return;
    setBusy(true); setError("");
    try { const result = await cancelSubscription(); setMutation({ before: resource.data, after: result }); setConfirm(false); }
    catch { setError(t.cancelError); }
    finally { setBusy(false); }
  }
  if (preview) return <StateNotice inline>{t.preview}</StateNotice>;
  return <section className={styles.profile} aria-label={t.title}>
    {resource.pending ? <StateNotice inline kind="loading">{t.loading}</StateNotice> : null}
    {resource.failed ? <StateNotice inline kind="error" action={{ label: t.retry, onClick: resource.retry }}>{t.error}</StateNotice> : null}
    {state ? <>
      <h3>{plan ? plan.plan_name : t.noPlan}</h3>
      {plan ? <>
        <p>{t.status[plan.status]}</p>
        <dl className={styles.details}>
          <div><dt>{t.paidUntil}</dt><dd>{date(plan.paid_until)}</dd></div>
          <div><dt>{t.renewal}</dt><dd>{plan.auto_renew ? t.on : t.off}</dd></div>
          {plan.auto_renew ? <div><dt>{t.nextCharge}</dt><dd>{formatPaymentPrice(plan.next_amount, locale)} ₽ · {date(plan.renewal_at ?? plan.paid_until)}</dd></div> : null}
        </dl>
        {!plan.auto_renew ? <p>{t.renewOff}</p> : <Button variant="outline" onClick={() => setConfirm(true)}>{t.cancel}</Button>}
      </> : null}
      <h3>{t.balances}</h3>
      {state.balances.length ? <ul className={styles.lots}>{state.balances.map((lot, index) => <li key={`${lot.kind}:${lot.expires_at ?? ""}:${index}`}>
        <strong>{t.lot[lot.kind as keyof typeof t.lot] ?? t.otherGrant}</strong>
        <span>{lot.available < 0 ? t.adjustedBalance : t.available}: <CreditAmount value={lot.available}/> · {t.reserved}: <CreditAmount value={lot.reserved}/></span>
        <span>{lot.expires_at ? `${t.expires}: ${date(lot.expires_at)}` : t.noExpiry}</span>
      </li>)}</ul> : <p>{t.empty}</p>}
    </> : null}
    {confirm ? <ConfirmDialog title={t.cancelTitle} description={t.cancelDescription} cancelLabel={t.keep} confirmLabel={t.cancel} pendingLabel={t.cancelling} isPending={busy} errorMessage={error} onCancel={() => { setConfirm(false); setError(""); }} onConfirm={() => void cancel()}/> : null}
  </section>;
}
