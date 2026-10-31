"use client";

import { useEffect, useRef, useState } from "react";
import { useOptionalWorkspaceAccount } from "@/features/account/WorkspaceAccount/WorkspaceAccount";
import { Button } from "@/components/ui/Button/Button";
import { StateNotice } from "@/components/ui/AsyncState/AsyncState";
import { CreditAmount } from "@/components/ui/CreditAmount/CreditAmount";
import { LegalPaymentNotice } from "@/features/legal/LegalLinks";
import { PaymentStatus } from "@/features/payments/PaymentStatus";
import { checkoutNavigation } from "@/features/payments/checkout-navigation";
import { clearPaymentAttempt, formatPaymentPrice, paymentAttemptKey, PaymentRequestError, savePendingPayment } from "@/features/payments/payments";
import { subscriptionPlanPreview, type SubscriptionPreviewId } from "@/features/pricing/subscription-preview";
import { useReadResource } from "@/lib/use-read-resource";
import { useLocale } from "@/i18n/LocaleProvider";
import economy from "@/config/economy.generated.json";
import Link from "@/i18n/Link";
import { createSubscriptionCheckout, loadSubscription, type SubscriptionCheckoutRequest } from "./subscriptions";
import { subscriptionCopy } from "./copy";
import styles from "./subscriptions.module.css";

export function SubscriptionCheckout({ planID, preview = false }: { planID: SubscriptionPreviewId; preview?: boolean }) {
  const locale = useLocale(), t = subscriptionCopy[locale], account = useOptionalWorkspaceAccount();
  const plan = subscriptionPlanPreview(planID), next = subscriptionPlanPreview(plan.next_plan_code);
  const state = useReadResource(loadSubscription, null, Boolean(account) && !preview);
  const [consent, setConsent] = useState(false), [busy, setBusy] = useState(false), [error, setError] = useState("");
  const [paymentID, setPaymentID] = useState<string | null>(null);
  const attempt = useRef<{ request: SubscriptionCheckoutRequest; key: string } | null>(null), submitting = useRef(false), alive = useRef(true);
  useEffect(() => { alive.current = true; return () => { alive.current = false; }; }, []);
  const trialUsed = plan.trial && state.data?.trial_used === true;
  const available = !preview && Boolean(account) && !state.failed && !state.pending && state.data?.checkout_available === true && (!plan.trial || state.data?.trial_used === false);
  const autoRenew = consent && state.data?.recurring_available === true;
  async function purchase() {
    if (!available || submitting.current) return;
    submitting.current = true; setBusy(true); setError("");
    attempt.current ??= {
      request: { plan_code: plan.id, auto_renew: autoRenew, ...(autoRenew ? { retry_policy_version: economy.renewal_retries.version } : {}) },
      key: paymentAttemptKey(`subscription:${plan.id}:${autoRenew}${autoRenew ? `:retry:${economy.renewal_retries.version}` : ""}`),
    };
    try {
      const payment = await createSubscriptionCheckout(attempt.current.request, attempt.current.key);
      if (!alive.current) return;
      if (payment.amount !== plan.amount || payment.credits !== plan.stars) { setError(t.conflict); return; }
      savePendingPayment(payment.id); clearPaymentAttempt();
      if (payment.status === "waiting_for_user" && payment.confirmation_url) checkoutNavigation.open(payment.confirmation_url);
      else setPaymentID(payment.id);
    } catch (cause) {
      if (!alive.current) return;
      setError(cause instanceof PaymentRequestError && cause.status === 409 ? t.conflict : t.purchaseError);
    } finally { submitting.current = false; if (alive.current) setBusy(false); }
  }
  if (paymentID) return <PaymentStatus paymentID={paymentID} onDone={() => { setPaymentID(null); attempt.current = null; state.retry(); }}/>;
  return <div className={styles.checkout} aria-label={t.offer}>
    <p className={styles.summary}><strong>{plan.name} · {formatPaymentPrice(plan.amount, locale)} ₽ / {plan.period_days} {t.days}</strong><CreditAmount value={plan.stars}/></p>
    {plan.trial ? <p>{t.trial}</p> : null}
    <p>{t.next}: {formatPaymentPrice(next.amount, locale)} ₽ · {next.name} · {next.stars.toLocaleString(locale)} {t.stars} / {next.period_days} {t.days}, {t.after} {plan.period_days} {t.days}.</p>
    <p>{t.expiry}</p>
    <label className={styles.consent}><input type="checkbox" checked={consent} disabled={busy || !state.data?.recurring_available || preview} onChange={event => { setConsent(event.target.checked); attempt.current = null; clearPaymentAttempt(); setError(""); }}/><span>{t.consent}</span></label>
    {!autoRenew ? <p>{t.renewOff}</p> : null}
    {state.data && !state.data.recurring_available ? <p>{t.recurringOff}</p> : null}
    {trialUsed ? <StateNotice inline>{t.trialUsed}</StateNotice> : null}
    {preview ? <StateNotice inline>{t.preview}</StateNotice> : !account ? <p><Link href="/app">{t.signIn}</Link></p> : state.pending ? <StateNotice inline kind="loading">{t.loading}</StateNotice> : state.failed ? <StateNotice inline kind="error" action={{ label: t.retry, onClick: state.retry }}>{t.error}</StateNotice> : !available && !trialUsed ? <StateNotice inline>{t.unavailable}</StateNotice> : null}
    <LegalPaymentNotice notice={t.checkoutNotice}/>
    <p>{t.authority}</p>
    <Button disabled={!available || busy} onClick={() => void purchase()}>{busy ? t.pending : t.pay}</Button>
    {error ? <StateNotice inline kind="error">{error}</StateNotice> : null}
  </div>;
}
