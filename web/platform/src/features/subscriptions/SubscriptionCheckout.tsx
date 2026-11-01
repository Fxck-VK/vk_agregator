"use client";

import { Button } from "@/components/ui/Button/Button";
import { StateNotice } from "@/components/ui/AsyncState/AsyncState";
import { CreditAmount } from "@/components/ui/CreditAmount/CreditAmount";
import { LegalPaymentNotice } from "@/features/legal/LegalLinks";
import { PaymentStatus } from "@/features/payments/PaymentStatus";
import { formatPaymentPrice } from "@/features/payments/payments";
import { subscriptionPlanPreview, type SubscriptionPreviewId } from "@/features/pricing/subscription-preview";
import { useLocale } from "@/i18n/LocaleProvider";
import Link from "@/i18n/Link";
import { subscriptionCopy } from "./copy";
import { useSubscriptionCheckout } from "./use-subscription-checkout";
import styles from "./subscriptions.module.css";

export function SubscriptionCheckout({ planID, preview = false }: { planID: SubscriptionPreviewId; preview?: boolean }) {
  const locale = useLocale(), t = subscriptionCopy[locale];
  const plan = subscriptionPlanPreview(planID), next = subscriptionPlanPreview(plan.next_plan_code);
  const checkout = useSubscriptionCheckout(preview);
  const {account,state,consent,busy,error,paymentID,autoRenew,setConsent} = checkout;
  const trialUsed = plan.trial && state.data?.trial_used === true;
  const available = checkout.available(planID);
  if (paymentID) return <PaymentStatus paymentID={paymentID} onDone={checkout.resetPayment}/>;
  return <div className={styles.checkout} aria-label={t.offer}>
    <p className={styles.summary}><strong>{plan.name} · {formatPaymentPrice(checkout.displayAmount(planID), locale)} ₽ / {plan.period_days} {t.days}</strong><CreditAmount value={plan.stars}/></p>
    {plan.trial ? <p>{t.trial}</p> : null}
    <p>{t.next}: {formatPaymentPrice(next.amount, locale)} ₽ · {next.name} · {next.stars.toLocaleString(locale)} {t.stars} / {next.period_days} {t.days}, {t.after} {plan.period_days} {t.days}.</p>
    <p>{t.expiry}</p>
    <label className={styles.consent}><input type="checkbox" checked={consent} disabled={busy || !state.data?.recurring_available || preview} onChange={event => setConsent(event.target.checked)}/><span>{t.consent}</span></label>
    {!autoRenew ? <p>{t.renewOff}</p> : null}
    {state.data && !state.data.recurring_available ? <p>{t.recurringOff}</p> : null}
    {trialUsed ? <StateNotice inline>{t.trialUsed}</StateNotice> : null}
    {preview ? <StateNotice inline>{t.preview}</StateNotice> : !account ? <p><Link href="/app">{t.signIn}</Link></p> : state.pending ? <StateNotice inline kind="loading">{t.loading}</StateNotice> : state.failed ? <StateNotice inline kind="error" action={{ label: t.retry, onClick: state.retry }}>{t.error}</StateNotice> : !available && !trialUsed ? <StateNotice inline>{t.unavailable}</StateNotice> : null}
    <LegalPaymentNotice notice={t.checkoutNotice}/>
    <p>{t.authority}</p>
    <Button disabled={!available || busy} onClick={() => void checkout.purchase(planID)}>{busy ? t.pending : t.pay}</Button>
    {error ? <StateNotice inline kind="error">{error}</StateNotice> : null}
  </div>;
}
